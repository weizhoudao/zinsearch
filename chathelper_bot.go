package main

import (
	"context"
	"zincsearch/lib"
	"fmt"
	"zincsearch/db"
	"zincsearch/chat"
	"zincsearch/model"
    "github.com/redis/go-redis/v9"
	"log"
	"sync"
	"unicode/utf16"
	"time"
	"errors"
	"os"
	"bufio"
	"strings"
	"strconv"
	"io"
)

var g_sBotKey = ""
var g_follow_groups = []string{}
var adminuser = ""

func GetUTF16Len(content string)int{
	encodeContent := utf16.Encode([]rune(content))
	return len(encodeContent)
}

func GetVipInfo(userid int64)(model.VipInfo, error){
	key := "chat_vipinfo_" + strconv.FormatInt(userid, 10)
	var info model.VipInfo
	err := db.GetStruct(key, &info)
	if err != nil && err != redis.Nil{
		lib.XLogErr("GetStruct", key, err)
		return info, err
	}
	if err == redis.Nil{
		lib.XLogInfo("user not vip", userid)
	}
	return info, nil
}

func SetVipInfo(userid, expire int64)error{
	info := model.VipInfo{
		ID:userid,
		Expire:expire,
	}
	key := "chat_vipinfo_" + strconv.FormatInt(userid, 10)
	if err := db.SetStruct(key, info); err != nil{
		lib.XLogErr("SetStruct", key, err)
		return err
	}
	lib.XLogInfo("add vip succ", userid, expire)
	return nil
}

func DelVipInfo(userid int64)error{
	key := "chat_vipinfo_" + strconv.FormatInt(userid, 10)
	return db.Del(key)
}

func GetUserList()(model.UserList, error){
	var user_list model.UserList
	err := db.GetStruct("chat_userlist", &user_list)
	if err != nil && err != redis.Nil{
		lib.XLogErr("GetStruct", err)
		return user_list, err
	}
	return user_list, nil
}

func SetUserList(user_list model.UserList)error{
	return db.SetStruct("chat_userlist", user_list)
}

func AddUser(userid int64)error{
	user_list, err := GetUserList()
	if err != nil{
		lib.XLogErr("GetUserList", err)
		return err
	}
	for _, id := range user_list.IDList{
		if id == userid{
			lib.XLogInfo("user already exist", userid)
			return nil
		}
	}
	user_list.IDList = append(user_list.IDList, userid)
	if err := SetUserList(user_list); err != nil{
		lib.XLogErr("SetUserList", userid, err)
		return err;
	}
	lib.XLogInfo("add new user", userid)
	return nil
}

func GetChatBotList(userid int64)(model.ChatBotList, error){
	str_userid := strconv.FormatInt(userid, 10)
	key := "chat_chatbotlist_" + str_userid
	var bot_list model.ChatBotList
	err := db.GetStruct(key, &bot_list)
	if err != nil && err != redis.Nil{
		lib.XLogErr("GetStruct", key, err)
		return bot_list, err
	}
	return bot_list, nil
}

func AddChatBot(userid, botid int64, token string)error{
	bot_list, err := GetChatBotList(userid)
	if err != nil{
		lib.XLogErr("GetChatBotList", userid, err)
		return err
	}
	var new_bot_list model.ChatBotList
	for _, item := range bot_list.BotList{
		if item.ID != botid{
			new_bot_list.BotList = append(new_bot_list.BotList, item)
		}
	}
	bot := model.ChatBot{Token:token, ID:botid}
	new_bot_list.BotList = append(new_bot_list.BotList, bot)
	key := "chat_chatbotlist_" + strconv.FormatInt(userid, 10)
	err = db.SetStruct(key, new_bot_list)
	if err != nil{
		lib.XLogErr("SetStruct", key, err)
	}
	lib.XLogInfo("add chat bot succ", key, userid, botid, token)
	return nil
}

func SetChatBotList(userid int64, bot_list model.ChatBotList)error{
	str_userid := strconv.FormatInt(userid, 10)
	key := "chat_chatbotlist_" + str_userid
	return db.SetStruct(key, bot_list)
}

func GetOperStatus(userid int64)(string, error){
	status := "none"
	key := "chat_operstatus_" + strconv.FormatInt(userid, 10)
	var oper_status model.OperStatus
	err := db.GetStruct(key, &oper_status)
	if err != nil && err != redis.Nil{
		lib.XLogErr("GetStruct", err, key)
		return status, err
	}
	if err != redis.Nil{
		status = oper_status.Status
	}
	return status, nil
}

func SetOperStatus(userid int64, status string)error{
	oper_status := model.OperStatus{Status:status}
	key := "chat_operstatus_" + strconv.FormatInt(userid, 10)
	return db.SetStruct(key, oper_status)
}

type Task struct {
	ID      string
	Cancel  context.CancelFunc
	Running bool
}

type Bot struct {
	BotAPI   *model.TBot
	Tasks    map[string]*Task
	TasksMux sync.Mutex
}

func NewBot(token string) (*Bot, error) {
	botAPI := model.TBot{BotKey: token}
	return &Bot{
		BotAPI: &botAPI,
		Tasks:  make(map[string]*Task),
	}, nil
}

func (b *Bot) SendText(chatid int64, text string)error{
	config := model.SendMessageConfig{ChatID:chatid, Text:text}
	return b.BotAPI.Call(&config)
}

func (b *Bot) GetTaskID(userid, botid int64)string{
	taskID := "taskid_" + strconv.FormatInt(userid, 10) + "_" + strconv.FormatInt(botid, 10)
	return taskID
}

func (b *Bot) StartTask(userid, botid int64, bot_token string) {
	ctx, cancel := context.WithCancel(context.Background())

	taskID := b.GetTaskID(userid, botid)
	lib.XLogInfo("StartTask", userid, botid, taskID)

	b.TasksMux.Lock()
	b.Tasks[taskID] = &Task{
		ID:      taskID,
		Cancel:  cancel,
		Running: true,
	}
	b.TasksMux.Unlock()

	// 启动任务协程
	go func() {
		defer cancel()
		bot := chat.NewChatBot(userid, botid, bot_token)
		lib.XLogInfo("Task running", taskID, time.Now())
		go bot.Run()
		for {
			select {
			case <-ctx.Done():
				bot.Stop()
				lib.XLogInfo("Task stopped", taskID)
				return
			default:
				continue
			}
		}
	}()
}

func (b *Bot) StopTask(userid, botid int64) {
	b.TasksMux.Lock()
	defer b.TasksMux.Unlock()

	taskID := b.GetTaskID(userid, botid)
	lib.XLogInfo("StopTask", userid, botid, taskID)

	if task, exists := b.Tasks[taskID]; exists && task.Running {
		task.Cancel()
		task.Running = false
		delete(b.Tasks, taskID)
	}
}

func (b *Bot) HandleStart(logid string, chatid int64)error{
	msg := b.NewStartMessage(chatid)
	config := model.SendMessageConfig{
		ChatID: chatid,
		Text: msg.Text,
		ReplyMarkup: *(msg.ReplyMarkup),
	}
	if err := b.BotAPI.Call(&config); err != nil{
		lib.XLogErr(logid, "SendMessage, config", config, "err", err)
		return err
	}
	return nil
}

func (b *Bot) HandleNewBot(logid string, chatid int64, msgid int, callbackdata string)error{
	bot_list, err := GetChatBotList(chatid)
	if err != nil{
		lib.XLogErr(logid, "GetChatBotList, userid", chatid)
		b.SendText(chatid, "系统繁忙,请稍候再试")
		return err
	}
	// 检查用户是否关注了群列表
	for _, groupid := range g_follow_groups{
		config := model.GetChatMemberConfig{
			ChatID:"@" + groupid,
			UserID:chatid,
		}
		if err := b.BotAPI.Call(&config); err != nil{
			lib.XLogErr(logid, "GetChatMember", groupid, chatid, err)
			b.SendText(chatid, "系统繁忙,请稍候再试")
			return err
		}
		if config.Response.Status != "member" && config.Response.Status != "administrator" && config.Response.Status != "creator"{
			lib.XLogErr("not group member", groupid, chatid)
			b.SendText(chatid, "使用双向机器人需要先关注/加入我们的群/频道: @" + groupid)
			return nil
		}
	}
	vipinfo, err := GetVipInfo(chatid)
	if err != nil{
		lib.XLogErr(logid, "GetVipInfo, userid", chatid, "err", err)
		b.SendText(chatid, "系统繁忙,请稍候再试")
		return err
	}
	isVip := vipinfo.Expire > time.Now().Unix()
	if !isVip && len(bot_list.BotList) > 0{
		b.SendText(chatid, "非会员仅能添加一个机器人, 购买会员请联系管理员")
		return nil
	}
	
	is_callback := len(callbackdata) > 0
	text := "只需两步即可创建一个双向机器人:\n"
	text += "1. 打开 "
	botfather := model.MessageEntity{
		Type:"mention",
		Offset: GetUTF16Len(text),
		Length: GetUTF16Len("@BotFather"),
	}
	text += "@BotFather"
	text += ", 然后"
	newbot := model.MessageEntity{
		Type:"text_link",
		URL: "http://telegra.ph/Create-Bot-Livegram-FAQ-03-29",
		Offset: GetUTF16Len(text),
		Length: GetUTF16Len("新建一个机器人"),
	}
	text += "新建一个机器人\n"
	text += "2. 接着你会得到一个api token(类似于123456:GJIELGMG的字符串), 然后将这个token发送给我即可\n"
	text += "**__注意!__**: 如果这个token已经在其他的平台中使用了,则双向机器人不会创建成功"
	entities := []model.MessageEntity{botfather, newbot}
	if is_callback{
		config := model.EditMessageTextConfig{
			ChatID: chatid,
			MessageID: msgid,
			Text: text,
			Entities: entities,
		}
		return_text := strconv.FormatInt(chatid, 10) + "_return_start_" + strconv.Itoa(msgid)
		return_btn := model.InlineKeyboardButton{Text:"返回上一步"}
		return_btn.CallbackData = &return_text
		var markup model.InlineKeyboardMarkup
		markup.InlineKeyboard = append(markup.InlineKeyboard, []model.InlineKeyboardButton{return_btn})
		config.ReplyMarkup = markup
		if err := b.BotAPI.Call(&config); err != nil{
			lib.XLogErr(logid, "SendMessage, config", config, "err", err);
		}
	}else{
		config := model.SendMessageConfig{
			ChatID: chatid,
			Text: text,
			Entities: entities,
		}
		if err := b.BotAPI.Call(&config); err != nil{
			lib.XLogErr(logid, "SendMessage, config", config, "err", err);
		}
	}
	if err := SetOperStatus(chatid, "wait"); err != nil{
		lib.XLogErr(logid, "SetOperStatus, chatid", chatid, "err", err)
		return err
	}
	return nil
}

func (b *Bot) HandleMyBot(logid string, chatid int64, msgid int, callback_data string)error{
	is_callback := len(callback_data) > 0
	bot_list, err := GetChatBotList(chatid)
	if err != nil {
		lib.XLogErr(logid, "GetChatBotList, chatid", chatid, "err", err)
		return err
	}
	if len(bot_list.BotList) == 0{
		config := model.SendMessageConfig{ChatID:chatid, Text:"你还没有创建双向机器人"}
		if err := b.BotAPI.Call(&config); err != nil{
			lib.XLogErr(logid, "SendMessage, err", err)
			b.SendText(chatid, "系统异常,请稍后重试")
			return err
		}
		return nil
	}
	msg, err := b.NewMyBotMessage(chatid, msgid, bot_list)
	if err != nil{
		lib.XLogErr(logid, "NewMyBotMessage, chatid", chatid, "msgid", msgid, "err", err)
		return err
	}
	if is_callback{
		config := model.EditMessageTextConfig{ChatID:chatid, MessageID:msgid, Text:msg.Text}
		config.ReplyMarkup = *(msg.ReplyMarkup)
		if err := b.BotAPI.Call(&config); err != nil{
			lib.XLogErr(logid, "EditMessage, err", err, "config", config)
			b.SendText(chatid, "系统异常,请稍候重试")
			return err
		}
	}else{
		config := model.SendMessageConfig{ChatID:chatid, Text:msg.Text}
		config.ReplyMarkup = *(msg.ReplyMarkup)
		if err := b.BotAPI.Call(&config); err != nil{
			lib.XLogErr(logid, "SendMessage, err", err, "config", config)
			b.SendText(chatid, "系统异常,请稍候重试")
			return err
		}
	}
	return nil
}

func (b *Bot) NewMyBotMessage(chatid int64, msgid int, bot_list model.ChatBotList)(model.Message, error){
	msg := model.Message{
		MessageID: msgid,
	}
	text := "以下是你创建的双向机器人列表:\n"
	user_chatid := strconv.FormatInt(chatid, 10)
	var markup model.InlineKeyboardMarkup
	for _, item := range bot_list.BotList{
		bot := model.TBot{BotKey:"bot" + item.Token}
		config := model.GetMeConfig{}
		if err := bot.Call(&config); err != nil{
			lib.XLogErr("GetMe", err)
			b.SendText(chatid, "系统异常,请稍后重试")
			return msg, err
		}
		btn := model.InlineKeyboardButton{Text: "@" + config.Response.UserName}
		callback_data := user_chatid + "_viewbot_" + strconv.FormatInt(item.ID, 10)
		btn.CallbackData = &callback_data
		markup.InlineKeyboard = append(markup.InlineKeyboard, []model.InlineKeyboardButton{btn})
	}
	return_text := strconv.FormatInt(chatid, 10) + "_return_start_" + strconv.Itoa(msgid)
	return_btn := model.InlineKeyboardButton{Text:"返回上一步"}
	return_btn.CallbackData = &return_text
	markup.InlineKeyboard = append(markup.InlineKeyboard, []model.InlineKeyboardButton{return_btn})
	msg.Text = text
	msg.ReplyMarkup = &markup
	return msg, nil
}

func (b *Bot) HandleReturnMyBot(logid string, chatid int64, msgid int)error{
	bot_list, err := GetChatBotList(chatid)
	if err != nil {
		lib.XLogErr(logid, "GetChatBotList, chatid", chatid, "err", err)
		return err
	}
	msg, err := b.NewMyBotMessage(chatid, msgid, bot_list)
	if err != nil{
		lib.XLogErr(logid, "NewMyBotMessage, chatid", chatid, "msgid", msgid, "err", err)
		return err
	}
	config := model.EditMessageTextConfig{
		ChatID: chatid,
		MessageID: msgid,
		Text: msg.Text,
		Entities: msg.Entities,
		ReplyMarkup: *(msg.ReplyMarkup),
	}
	if err := b.BotAPI.Call(&config); err != nil{
		lib.XLogErr(logid, "EditedMessage, config", config, "err", err)
		return err
	}
	return nil
}

func (b *Bot) HandleViewBot(logid string, chatid int64, msgid int, callback_data string)error{
	values := strings.Split(callback_data, "_")
	if len(values) != 3{
		lib.XLogErr(logid, "invalid callbackdata, chatid", chatid, "callbackdata", callback_data)
		return errors.New("invalid callbackdata")
	}
	botid, err := strconv.ParseInt(values[2], 10, 64)
	if err != nil{
		lib.XLogErr(logid, "invalid botid, err", err, "callbackdata", callback_data)
		return err
	}
	msg, err := b.NewViewBotMessage(chatid, botid, msgid)
	if err != nil{
		lib.XLogErr(logid, "NewViewBotMessage, chatid", chatid, "botid", botid, "err", err)
		return err
	}

	msg_config := model.EditMessageTextConfig{
		ChatID: chatid,
		MessageID: msgid,
		Text: msg.Text,
		Entities: msg.Entities,
		ReplyMarkup: *(msg.ReplyMarkup),
	}
	return b.BotAPI.Call(&msg_config)
}

func (b *Bot)HandleConfirmDelete(logid string, chatid int64, msgid int, callback_data string)error{
	values := strings.Split(callback_data, "_")
	if len(values) != 3{
		lib.XLogErr(logid, "invalid callbackdata, chatid", chatid, "callbackdata", callback_data)
		return errors.New("invalid callbackdata")
	}
	botid, err := strconv.ParseInt(values[2], 10, 64)
	if err != nil{
		lib.XLogErr(logid, "invalid botid, err", err, "callbackdata", callback_data)
		return err
	}
	bot_list, err := GetChatBotList(chatid)
	if err != nil{
		lib.XLogErr(logid, "GetChatBotList, chatid", chatid, "err", err)
		return err
	}
	var new_bot_list model.ChatBotList
	for _, item := range bot_list.BotList{
		if item.ID != botid{
			new_bot_list.BotList = append(new_bot_list.BotList, item)
		}
	}
	b.StopTask(chatid, botid)
	SetChatBotList(chatid, new_bot_list)
	del_config := model.DeleteMessageConfig{
		ChatID:chatid,
		MessageID:msgid,
	}
	b.BotAPI.Call(&del_config)
	return b.SendText(chatid, "成功销毁双向机器人")
}
	

func (b *Bot)HandleDeleteBot(logid string, chatid int64, msgid int, callback_data string)error{
	values := strings.Split(callback_data, "_")
	if len(values) != 3{
		lib.XLogErr(logid, "invalid callbackdata, chatid", chatid, "callbackdata", callback_data)
		return errors.New("invalid callbackdata")
	}
	botid, err := strconv.ParseInt(values[2], 10, 64)
	if err != nil{
		lib.XLogErr(logid, "invalid botid, err", err, "callbackdata", callback_data)
		return err
	}
	bot_list, err := GetChatBotList(chatid)
	if err != nil{
		lib.XLogErr(logid, "GetChatBotList, chatid", chatid, "err", err)
		return err
	}
	found := false
	username := ""
	var new_bot_list model.ChatBotList
	for _, item := range bot_list.BotList{
		if item.ID == botid{
			found = true
			bot := model.TBot{BotKey:"bot" + item.Token}
			config := model.GetMeConfig{}
			if err := bot.Call(&config); err != nil{
				lib.XLogErr(logid, "GetMe", err)
				b.SendText(chatid, "系统异常,请稍后重试")
				return err
			}
			username = config.Response.UserName
			break
		}else{
			new_bot_list.BotList = append(new_bot_list.BotList, item)
		}
	}
	if !found{
		b.StopTask(chatid, botid)
		SetChatBotList(chatid, new_bot_list)
		return b.SendText(chatid, "成功销毁双向机器人")
	}
	text := "你确定要销毁双向机器人 "
	bot_at := model.MessageEntity{
		Type:"mention",
		Offset: GetUTF16Len(text),
		Length: GetUTF16Len("@" + username),
	}
	text += "@" + username + "?"

	str_msgid := strconv.Itoa(msgid)

	str_chatid := strconv.FormatInt(chatid, 10)
	str_botid := strconv.FormatInt(botid, 10)
	confirm := model.InlineKeyboardButton{Text:"销毁双向机器人"}
	confirm_callback := str_chatid + "_confirmdelete_" + str_botid
	confirm.CallbackData = &confirm_callback

	return_text := str_chatid + "_return_viewbot_" + str_msgid + "_" + str_botid
	return_btn := model.InlineKeyboardButton{Text:"返回上一步"}
	return_btn.CallbackData = &return_text

	var markup model.InlineKeyboardMarkup
	markup.InlineKeyboard = append(markup.InlineKeyboard, []model.InlineKeyboardButton{confirm}, []model.InlineKeyboardButton{return_btn})
	entities := []model.MessageEntity{bot_at}

	config := model.EditMessageTextConfig{
		ChatID:chatid,
		MessageID:msgid,
		Entities:entities,
		Text:text,
		ReplyMarkup:markup,
	}
	return b.BotAPI.Call(&config)
}

func (b *Bot) HandleStatBot(logid string, chatid int64, callback_data string)error{
	values := strings.Split(callback_data, "_")
	if len(values) != 3{
		lib.XLogErr(logid, "invalid callbackdata, chatid", chatid, "callbackdata", callback_data)
		return errors.New("invalid callbackdata")
	}
	botid, err := strconv.ParseInt(values[2], 10, 64)
	if err != nil{
		lib.XLogErr(logid, "invalid botid, err", err, "callbackdata", callback_data)
		return err
	}
	chat_list, err := chat.GetChatList(botid, "private")
	if err != nil{
		lib.XLogErr(logid, "GetChatList, botid", botid, "err", err)
		return err
	}
	text := fmt.Sprintf("双向机器人一共处理了来自%d个用户的消息", len(chat_list.ChatList))
	return b.SendText(chatid, text)
}

func (b *Bot) HandleSwitch(logid string, chatid int64, msgid int, callback_data string)error{
	vipinfo, err := GetVipInfo(chatid)
	if err != nil{
		lib.XLogErr(logid, "GetVipInfo, chatid", chatid, "err", err)
		return err
	}
	isVip := vipinfo.Expire > time.Now().Unix()

	values := strings.Split(callback_data, "_")
	if len(values) < 4{
		lib.XLogErr(logid, "invalid callbackdata, chatid", chatid, "callbackdata", callback_data)
		return errors.New("invalid callbackdata")
	}
	if !isVip && values[3] == "group"{
		b.SendText(chatid, "仅会员才可以切换到群聊模式")
		return nil
	}
	botid, err := strconv.ParseInt(values[2], 10, 64)
	if err != nil{
		lib.XLogErr(logid, "ParseInt, callbackdata", callback_data)
		return err
	}
	userid := chatid
	bot_list, err := GetChatBotList(userid)
	if err != nil{
		lib.XLogErr(logid, "GetChatBotList, err", err)
		return err
	}
	bot_token := ""
	for _, item := range bot_list.BotList{
		if item.ID == botid{
			bot_token = item.Token
		}
	}
	tmp_bot := model.TBot{BotKey:"bot" + bot_token}
	if values[3] == "private" || (values[3] == "group" && len(values) == 5){
		bot_detail := model.ChatBotDetail{
			Token: bot_token,
			ID: botid,
			Mode: values[3],
		}
		if len(values) == 5{
			groupid, err := strconv.ParseInt(values[4], 10, 64)
			if err != nil{
				lib.XLogErr(logid, "ParseInt, callbackdata", callback_data)
				return err
			}
			bot_detail.GroupID = groupid
		}
		if err := chat.SetChatBotDetail(botid, bot_detail); err != nil{
			lib.XLogErr(logid, "SetChatBotDetail, botid", botid, "detail", bot_detail, "err", err)
			return err
		}
		lib.XLogInfo(logid, "set bot detail succ, botid", botid, "detail", bot_detail)
		b.SendText(chatid, "切换成功")
		return nil
	}
	chat_list, err := chat.GetChatList(botid, "group")
	if err != nil {
		lib.XLogErr(logid, "GetChatList, botid", botid, "err", err)
		return err
	}
	found := false
	groups := []int64{}
	for _, item := range chat_list.ChatList{
		if item.Status == "left" || item.Status == "kicked"{
			continue
		}
		groups = append(groups, item.ChatID)
		found = true
	}
	if !found{
		b.SendText(chatid, "机器人暂未加入任一群聊，无法操作")
		return nil
	}
	text := "请选择消息要转发到哪个群聊中:\n"
	var markup model.InlineKeyboardMarkup
	for _, groupid := range groups{
		config := model.GetChatConfig{ChatID:groupid}
		err := tmp_bot.Call(&config)
		if err != nil{
			lib.XLogErr(logid, "GetChat, config", config, "err", err, tmp_bot.BotKey)
			return err
		}
		btn := model.InlineKeyboardButton{Text:config.Response.Title}
		callbackdata := callback_data + "_" + strconv.FormatInt(groupid, 10)
		btn.CallbackData = &callbackdata
		markup.InlineKeyboard = append(markup.InlineKeyboard, []model.InlineKeyboardButton{btn})
	}

	return_text := strconv.FormatInt(chatid, 10) + "_return_viewbot_" + strconv.Itoa(msgid) + "_" + values[2]
	return_btn := model.InlineKeyboardButton{Text:"返回上一步"}
	return_btn.CallbackData = &return_text
	markup.InlineKeyboard = append(markup.InlineKeyboard, []model.InlineKeyboardButton{return_btn})

	config := model.EditMessageTextConfig{
		ChatID:chatid,
		Text:text,
		MessageID:msgid,
		ReplyMarkup:markup,
	}
	if err := b.BotAPI.Call(&config); err != nil{
		lib.XLogErr(logid, "EditMessageText, config", config, "err", err)
		return err
	}
	return nil
}

func (b *Bot)NewStartMessage(chatid int64)(model.Message){
	msg := model.Message{
		Text: "欢迎使用双向助手, 小助手可以帮你快速搭建一个双向聊天机器人\n",
	}
	user_chatid := strconv.FormatInt(chatid, 10)
	newbot := model.InlineKeyboardButton{Text:"创建双向机器人"}
	newbot_callback := user_chatid + "_" + "newbot"
	newbot.CallbackData = &newbot_callback
	mybot := model.InlineKeyboardButton{Text:"查看我的双向机器人"}
	mybot_callback := user_chatid + "_" + "mybot"
	mybot.CallbackData = &mybot_callback
	buyvip := model.InlineKeyboardButton{Text:"购买会员"}
	buyvip_url := "https://t.me/chathelperkfbot"
	buyvip.URL = &buyvip_url
	var markup model.InlineKeyboardMarkup
	markup.InlineKeyboard = append(markup.InlineKeyboard, []model.InlineKeyboardButton{newbot}, []model.InlineKeyboardButton{mybot}, []model.InlineKeyboardButton{buyvip})
	msg.ReplyMarkup = &markup
	return msg
}

func (b *Bot)HandleReturnStart(logid string, chatid int64, msgid int)error{
	msg := b.NewStartMessage(chatid)
	config := model.EditMessageTextConfig{
		ChatID: chatid,
		MessageID: msgid,
		Text: msg.Text,
		Entities: msg.Entities,
		ReplyMarkup: *(msg.ReplyMarkup),
	}
	if err := b.BotAPI.Call(&config); err != nil{
		lib.XLogErr(logid, "EditedMessage, config", config, "error", err)
		return err
	}
	if err := SetOperStatus(chatid, "init"); err != nil{
		lib.XLogErr(logid, "SetOperStatus, chatid", chatid, "err", err)
		return err
	}
	return nil
}

func (b *Bot)NewViewBotMessage(chatid, botid int64, msgid int)(model.Message, error){
	msg := model.Message{}
	bot_list, err := GetChatBotList(chatid)
	if err != nil{
		lib.XLogErr("GetChatBotList", chatid, err)
		return msg, err
	}
	bot_token := ""
	for _, item := range bot_list.BotList{
		if item.ID == botid{
			bot_token = item.Token
			break;
		}
	}
	if len(bot_token) == 0{
		lib.XLogErr("botid not found", chatid, botid)
		return msg, errors.New("invalid botid")
	}
	bot := model.TBot{BotKey:"bot" + bot_token}
	config := model.GetMeConfig{}
	if err := bot.Call(&config); err != nil{
		lib.XLogErr("GetMe", err)
		return msg, err
	}
	text := "你可以对 "
	bot_at := model.MessageEntity{
		Type:"mention",
		Offset: GetUTF16Len(text),
		Length: GetUTF16Len("@" + config.Response.UserName),
	}
	str_chatid := strconv.FormatInt(chatid, 10)
	str_botid := strconv.FormatInt(botid, 10)
	text += "@" + config.Response.UserName + " 进行以下操作:\n"
	delbot := model.InlineKeyboardButton{Text:"销毁双向机器人"}
	delbot_callback := str_chatid + "_delete_" + str_botid
	delbot.CallbackData = &delbot_callback

	statbot := model.InlineKeyboardButton{Text:"查看统计数据"}
	statbot_callback := str_chatid + "_stat_" + str_botid 
	statbot.CallbackData = &statbot_callback

	switchmode := model.InlineKeyboardButton{}
	detail, err := chat.GetChatBotDetail(botid)
	if err != nil{
		lib.XLogErr("GetChatBotDetail", botid, err)
		return msg, err
	}
	switchcallbackdata := str_chatid + "_switch_" + str_botid + "_"
	if len(detail.Mode) == 0 || detail.Mode == "private"{
		switchmode.Text = "切换到群聊模式(会员专享)"
		switchcallbackdata += "group"
	}else{
		switchmode.Text = "切换到单聊模式"
		switchcallbackdata += "private"
	}
	switchmode.CallbackData = &switchcallbackdata

	var markup model.InlineKeyboardMarkup
	markup.InlineKeyboard = append(markup.InlineKeyboard, []model.InlineKeyboardButton{delbot}, []model.InlineKeyboardButton{statbot}, []model.InlineKeyboardButton{switchmode})

	return_text := str_chatid + "_return_mybot_" + strconv.Itoa(msgid)
	return_btn := model.InlineKeyboardButton{Text:"返回上一步"}
	return_btn.CallbackData = &return_text
	markup.InlineKeyboard = append(markup.InlineKeyboard, []model.InlineKeyboardButton{return_btn})

	entities := []model.MessageEntity{bot_at}

	msg.Text = text
	msg.MessageID = msgid
	msg.Entities = entities
	msg.ReplyMarkup = &markup

	return msg, nil
}

func (b *Bot)HandleReturnViewBot(logid string, chatid, botid int64, msgid int)error{
	msg, err := b.NewViewBotMessage(chatid, botid, msgid)
	if err != nil{
		lib.XLogErr(logid, "NewViewBotMessage, chatid", chatid, "botid", botid, "err", err)
		return err
	}
	config := model.EditMessageTextConfig{
		ChatID: chatid,
		MessageID: msgid,
		Text: msg.Text,
		Entities: msg.Entities,
		ReplyMarkup: *(msg.ReplyMarkup),
	}
	if err := b.BotAPI.Call(&config); err != nil{
		lib.XLogErr(logid, "EditedMessage, config", config, "err", err)
		return err
	}
	return nil
}

func (b *Bot)HandleReturn(logid string, chatid int64, callback_data string)error{
	values := strings.Split(callback_data, "_")
	if len(values) < 4{
		return errors.New("invalid callbackdata")
	}
	msgid, err := strconv.Atoi(values[3])
	if err != nil{
		lib.XLogErr(logid, "Itoa, callbackdata", callback_data)
		return err
	}
	if values[2] == "start"{
		if err := b.HandleReturnStart(logid, chatid ,msgid); err != nil{
			lib.XLogErr(logid, "HandleReturnStart, chatid", chatid, "msgid", msgid, "err", err)
		}
	}else if values[2] == "mybot"{
		if err := b.HandleReturnMyBot(logid, chatid, msgid); err != nil{
			lib.XLogErr(logid, "HandleReturnMyBot, chatid", chatid, "msgid", msgid, "err", err)
		}
	}else if values[2] == "viewbot"{
		botid, err := strconv.ParseInt(values[4], 10, 64)
		if err != nil{
			lib.XLogErr(logid, "ParseInt, err", err, "callbackdata", callback_data)
			return err
		}
		if err := b.HandleReturnViewBot(logid, chatid, botid, msgid); err != nil{
			lib.XLogErr(logid, "HandleReturnViewBot, chatid", chatid, "msgid", msgid, "err", err)
		}
	}

	return nil
}

func (b *Bot) HandleCallback(callback *model.CallbackQuery){
	callback_data := callback.Data
	values := strings.Split(callback_data, "_")
	if len(values) < 2{
		lib.XLogErr(callback.ID, "invalid callbackdata", callback_data)
		return
	}
	cmd := values[1]
	msg := callback.Message
	logid := callback.ID
	if cmd == "return"{
		if err := b.HandleReturn(logid, msg.Chat.ID, callback_data); err != nil{
			lib.XLogErr(logid, "HandleReturn, err", err, "callbackdata", callback_data)
		}
	}else if cmd == "newbot"{
		if err := b.HandleNewBot(logid, msg.Chat.ID, msg.MessageID, callback_data); err != nil{
			lib.XLogErr(logid, "HandleNewBot, err", err, "callbackdata", callback_data)
		}
	}else if cmd == "mybot"{
		if err := b.HandleMyBot(logid, msg.Chat.ID, msg.MessageID, callback_data); err != nil{
			lib.XLogErr(logid, "HandleMyBot, err", err, "callbackdata", callback_data)
		}
	}else if cmd == "viewbot"{
		if err := b.HandleViewBot(logid, msg.Chat.ID, msg.MessageID, callback_data); err != nil{
			lib.XLogErr(logid, "HandleViewBot, err", err, "callbackdata", callback_data)
		}
	}else if cmd == "stat"{
		if err := b.HandleStatBot(logid, msg.Chat.ID, callback_data); err != nil{
			lib.XLogErr(logid, "HandleStatBot, err", err, "callbackdata", callback_data)
		}
		return
	}else if cmd == "delete"{
		if err := b.HandleDeleteBot(logid, msg.Chat.ID, msg.MessageID, callback_data); err != nil{
			lib.XLogErr(logid, "HandleDeleteBot, err", err, "callbackdata", callback_data)
		}
	}else if cmd == "switch"{
		if err := b.HandleSwitch(logid, msg.Chat.ID, msg.MessageID, callback_data); err != nil{
			lib.XLogErr(logid, "HandleSwitch, err", err, "callbackdata", callback_data)
		}
	}else if cmd == "confirmdelete"{
		if err := b.HandleConfirmDelete(logid, msg.Chat.ID, msg.MessageID, callback_data); err != nil{
			lib.XLogErr(logid, "HandleConfirmDelete, err", err, "callbackdata", callback_data)
		}
	}
	answer := model.AnswerCallbackQueryConfig{
		CallbackID:callback.ID,
	}
	b.BotAPI.Call(&answer)
}

func (b *Bot) HandleNonCommand(chatid int64, text string){
	if len(text) == 0{
		return
	}
	status, err := GetOperStatus(chatid)
	if err != nil {
		lib.XLogErr("GetOperStatus", err, chatid)
		b.SendText(chatid, "系统异常,请稍候重试")
		return
	}
	token := strings.TrimSpace(text)
	if status == "wait"{
		bot := model.TBot{BotKey:"bot" + token}
		config := model.GetMeConfig{}
		if err := bot.Call(&config); err != nil{
			lib.XLogErr("GetMe", err, chatid)
			b.SendText(chatid, "请提供合法的api token")
			return
		}
		if err := AddChatBot(chatid, config.Response.ID, token); err != nil{
			lib.XLogErr("AddChatBot", chatid, err)
			b.SendText(chatid, "创建失败,请稍后重试")
			return
		}
		b.StartTask(chatid, config.Response.ID, token)
		b.SendText(chatid, "创建成功")
		SetOperStatus(chatid, "init")
		AddUser(chatid)
	}
}

func (b *Bot) LoadTask()error{
	user_list, err := GetUserList()
	if err != nil{
		lib.XLogErr("GetUserList", err)
		return err
	}
	lib.XLogInfo("total user count", len(user_list.IDList))
	for _, userid := range user_list.IDList{
		bot_list, err := GetChatBotList(userid)
		if err != nil{
			lib.XLogErr("LoadTask error, GetChatBotList", userid, err)
			continue
		}
		for _, bot := range bot_list.BotList{
			b.StartTask(userid, bot.ID, bot.Token)
			lib.XLogInfo("start task", userid, bot.ID)
		}
	}
	return nil
}

func (b *Bot)SetVip(chatid int64, args string){
	data := strings.TrimSpace(args)
	values := strings.Split(data, " ")
	if len(values) != 2{
		lib.XLogErr("invalid args", args)
		return
	}
	userid, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil{
		lib.XLogErr("ParseInt", args, err)
		return
	}
	expire, err := strconv.ParseInt(values[1], 10, 64)
	if err != nil{
		lib.XLogErr("ParseInt", args, err)
		return
	}
	if err := SetVipInfo(userid, expire); err != nil{
		lib.XLogErr("SetVipInfo", userid, expire, err)
	}
	b.SendText(chatid, "设置成功")
}

func (b *Bot)GetVip(chatid int64, args string){
	data := strings.TrimSpace(args)
	userid, err := strconv.ParseInt(data, 10, 64)
	if err != nil{
		lib.XLogErr("ParseInt", data, err)
		return
	}
	info, err := GetVipInfo(userid)
	if err != nil{
		lib.XLogErr("GetVipInfo", userid, err)
		return
	}
	if info.ID != userid{
		b.SendText(chatid, "暂无用户的会员信息")
		return
	}
	text := args + " 的会员有效期至:" + time.Unix(info.Expire, 0).Format("2006-01-02 15:04:05")
	b.SendText(chatid, text)
}

func (b *Bot)DelVip(chatid int64, args string){
	data := strings.TrimSpace(args)
	userid, err := strconv.ParseInt(data, 10, 64)
	if err != nil{
		lib.XLogErr("ParseInt", data, err)
		return
	}
	if err := DelVipInfo(userid); err != nil{
		lib.XLogErr("DelVipInfo", userid, err)
	}
	b.SendText(chatid, "删除成功")
}

func (b *Bot) HandleUpdates() {
	
	config := model.UpdateConfig{}
	config.Offset = 0
	config.Limit = 100
	config.Timeout = 10
	ch := b.BotAPI.GetUpdateChan(&config)

	for update := range ch {
		if update.EditedMessage != nil{
			continue
		}
		if update.CallbackQuery != nil{
			go b.HandleCallback(update.CallbackQuery)
			continue
		}
		if update.Message == nil {
			continue
		}
		if !update.Message.IsCommand(){
			go b.HandleNonCommand(update.Message.Chat.ID, update.Message.Text)
			continue
		}
		command := update.Message.Command()
		args := update.Message.CommandArguments()

		logid := strconv.Itoa(update.UpdateID)
		switch command {
		case "start":
			go b.HandleStart(logid, update.Message.Chat.ID)
			continue
		case "newbot":
			go b.HandleNewBot(logid, update.Message.Chat.ID, update.Message.MessageID, "")
			continue
		case "mybot":
			go b.HandleMyBot(logid, update.Message.Chat.ID, update.Message.MessageID, "")
			continue
		case "setvip":
			go b.SetVip(update.Message.Chat.ID, args)
			continue
		case "delvip":
			go b.DelVip(update.Message.Chat.ID, args)
			continue
		case "getvip":
			go b.GetVip(update.Message.Chat.ID, args)
			continue
		}
	}
}

func InitConfig(){
	if len(os.Args) != 2 {
		lib.XLogErr("invalid usage! example: ./program config_file")
		panic("invalid usage")
	}
	config_file := os.Args[1]
	config, err := os.Open(config_file)
	if err != nil {
		lib.XLogErr("open config fail", config_file)
		panic("load config error")
	}
	defer config.Close()

	br := bufio.NewReader(config)
	for {
		a, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}
		line := string(a)
		idx := strings.Index(line, "=")
		if idx == -1 {
			lib.XLogErr("invalid config", line)
			break
		}
		lib.XLogInfo("config line", line)
		if line[0 : idx] == "key" {
			g_sBotKey = line[idx + 1:]
		}else if line[0: idx] == "admin"{
			adminuser = line[idx + 1:]
		}else if line[0: idx] == "follow_groups"{
			groups := strings.Split(line[idx + 1:], ",")
			g_follow_groups = groups
		}
	}
}

func main() {
	InitConfig()
	bot, err := NewBot(g_sBotKey)
	if err != nil {
		log.Panic(err)
	}
	bot.LoadTask()
	bot.HandleUpdates()
}
