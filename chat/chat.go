package chat

import(
	"zincsearch/model"
	"zincsearch/lib"
    "github.com/redis/go-redis/v9"
	"time"
	"strconv"
	"zincsearch/db"
)

func GetDBKey(botid int64, chattype string)string{
	if chattype == "private"{
		return "chatlist_" + strconv.FormatInt(botid, 10)
	}else{
		return "grouplist_" + strconv.FormatInt(botid, 10)
	}
	return ""
}

func GetGroupThread(botid, userid int64)(model.GroupThreadInfo, error){
	key := "chat_groupthread_" + strconv.FormatInt(botid, 10) + "_" + strconv.FormatInt(userid, 10)
	var info model.GroupThreadInfo
	if err := db.GetStruct(key, &info); err != nil && err != redis.Nil{
		lib.XLogErr("GetStruct", key, err)
		return info, err
	}
	lib.XLogInfo("threadinfo", botid, userid)
	return info, nil
}

func SetGroupThread(botid, userid, groupid int64, threadid int)error{
	key := "chat_groupthread_" + strconv.FormatInt(botid, 10) + "_" + strconv.FormatInt(userid, 10)
	info := model.GroupThreadInfo{
		GroupID:groupid,
		ThreadID:threadid,
	}
	if err := db.SetStruct(key, info); err != nil{
		lib.XLogErr("SetStruct", key, err)
		return err
	}
	lib.XLogInfo("set group thread succ", botid, userid, groupid, threadid)
	return nil
}

func GetChatBotDetail(botid int64)(model.ChatBotDetail,error){
	key := "chat_chatbotdetail_" + strconv.FormatInt(botid, 10)
	var detail model.ChatBotDetail
	if err := db.GetStruct(key, &detail); err != nil && err != redis.Nil{
		lib.XLogErr("GetStruct", key, err)
		return detail, err
	}
	lib.XLogInfo("botdetail", botid, detail)
	return detail, nil
}

func SetChatBotDetail(botid int64, detail model.ChatBotDetail)error{
	key := "chat_chatbotdetail_" + strconv.FormatInt(botid, 10)
	if err := db.SetStruct(key, detail); err != nil{
		lib.XLogErr("SetStruct", key, err)
		return err
	}
	lib.XLogInfo("SetChatBotDetail succ", key, detail)
	return nil
}

func GetChatList(botid int64, chattype string)(model.ChatSessionList, error){
	var chat_list model.ChatSessionList
	key := GetDBKey(botid, chattype)
	err := db.GetStruct(key, &chat_list)
	if err != nil && err != redis.Nil{
		lib.XLogErr("GetStruct", key, err)
		return chat_list, err
	}
	return chat_list, nil
}

func SetChatList(botid int64, chattype string, chat_list model.ChatSessionList)error{
	key := GetDBKey(botid, chattype)
	return db.SetStruct(key, chat_list)
}

func AddChat(botid, chatid int64, chattype, status string)error{
	new_chat := model.ChatSession{
		Status: status,
		ChatID: chatid,
		Type: chattype,
	}
	chat_list, err := GetChatList(botid, chattype)
	if err != nil{
		lib.XLogErr("GetChatList", botid, err)
		return err
	}
	var new_chat_list model.ChatSessionList
	new_chat_list.ChatList = append(new_chat_list.ChatList, new_chat)
	for _, item := range chat_list.ChatList{
		if item.ChatID != chatid{
			new_chat_list.ChatList = append(new_chat_list.ChatList, item)
		}
	}
	if err := SetChatList(botid, chattype, new_chat_list); err != nil{
		lib.XLogErr("SetChatList", botid, chattype, err)
		return err
	}
	lib.XLogInfo("add chat succ, botid", botid, "chatid", chatid, "chattype", chattype, "status", status)
	return nil
}

func DelChat(botid, chatid int64, chattype string)error{
	chat_list, err := GetChatList(botid, chattype)
	if err != nil{
		lib.XLogErr("GetChatList", botid, err)
		return err
	}
	var new_chat_list model.ChatSessionList
	for _, item := range chat_list.ChatList{
		if item.ChatID != chatid{
			new_chat_list.ChatList = append(new_chat_list.ChatList, item)
		}
	}
	if err := SetChatList(botid, chattype, new_chat_list); err != nil{
		lib.XLogErr("SetChatList", botid, chatid, chattype, err)
		return err
	}
	lib.XLogInfo("delete chat succ, botid", botid, "chatid", chatid, "chattype", chattype)
	return nil
}

type ChatBot struct{
	Bot model.TBot
	OwnerID int64
	MyID int64
	GroupID int64
}

func NewChatBot(userid, botid int64, bot_token string)ChatBot{
	botapi := model.TBot{BotKey:"bot" + bot_token}
	botapi.ShutdownChannel = make(chan interface{})
	return ChatBot{Bot:botapi, OwnerID:userid, MyID:botid}
}

func (b *ChatBot) SendText(chatid int64, text string){
	config := model.SendMessageConfig{
		ChatID: chatid,
		Text: text,
	}
	b.Bot.Call(&config)
}

func (b *ChatBot) ForwardMessage(msg *model.Message){
	config := model.ForwardMessageConfig{
		ChatID: b.OwnerID,
		FromChatID: msg.Chat.ID,
		MessageID: msg.MessageID,
	}
	if err := b.Bot.Call(&config); err != nil{
		lib.XLogErr("forward msg", config, err)
		b.SendText(msg.Chat.ID, "消息发送失败,请稍后重试...")
	}
	if err := AddChat(b.MyID, msg.Chat.ID, "private", "member"); err != nil{
		lib.XLogErr("botid", b.MyID, "AddChat chatid", msg.Chat.ID)
	}
}

func (b *ChatBot) ForwardMessageToChat(msg *model.Message, chatid int64){
	config := model.SendMessageConfig{
		ChatID: chatid,
		Text: msg.Text,
		Entities: msg.Entities,
	}
	if err := b.Bot.Call(&config); err != nil{
		lib.XLogErr("botid", b.MyID, "forward msg to chat", config, err)
		b.SendText(msg.Chat.ID, "发送失败")
	}
}

func (b *ChatBot) Stop(){
	close(b.Bot.ShutdownChannel)
	lib.XLogInfo("botid", b.MyID, "close bot")
}

func (b *ChatBot) HandleChatStatus(cm *model.ChatMemberUpdated){
	if cm.NewChatMember.User.ID != b.MyID{
		return
	}
	chattype := cm.Chat.Type
	chatid := cm.Chat.ID
	new_status := cm.NewChatMember.Status
	if new_status == "member" || new_status == "administrator"{
		if cm.From.ID != b.OwnerID && cm.From.UserName != "GroupAnonymousBot"{
			lib.XLogErr("botid", b.MyID, "not oepr by owner, skip", cm.From.ID, cm.From.UserName, b.OwnerID)
			return
		}
		if err := AddChat(b.MyID, chatid, chattype, new_status); err != nil{
			lib.XLogErr("botid", b.MyID, "AddChat", chatid, chattype, new_status, err)
		}
	}else if chattype != "private" && (new_status == "left" || new_status == "kicked"){
		if err := DelChat(b.MyID, chatid, chattype); err != nil{
			lib.XLogErr("botid", b.MyID, "DelChat", chatid, chattype, err)
		}
	}
}

func (b *ChatBot) HandleMessage(msg *model.Message){
	mode := "private"
	bot_detail, err := GetChatBotDetail(b.MyID)
	if err == nil && len(bot_detail.Mode) > 0{
		mode = bot_detail.Mode
	}
	lib.XLogInfo("botid", b.MyID, "HandleMessage", mode)
	if mode != "private" && msg.Chat.Type != "private"{
		chat_list, err := GetChatList(b.MyID, mode)
		if err != nil {
			lib.XLogErr("botid", b.MyID, "GetChatList", err)
			return 
		}
		lib.XLogInfo("botid", b.MyID, "mychatlist", chat_list)
		hit := false
		for _, item := range chat_list.ChatList{
			if item.ChatID == msg.Chat.ID{
				hit = true
			}
		}
		if !hit{
			lib.XLogErr("botid", b.MyID, "not owner's group, skip", b.OwnerID, msg.Chat.ID, chat_list)
			return
		}
	}
	if bot_detail.GroupID != 0{
		b.GroupID = bot_detail.GroupID
	}
	// 回复消息
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.MessageID > 0 && msg.ReplyToMessage.ForwardFrom != nil && msg.ReplyToMessage.ForwardFrom.ID != 0{
		if msg.Chat.ID == b.OwnerID || mode != "private"{
			b.ForwardMessageToChat(msg, msg.ReplyToMessage.ForwardFrom.ID)
		}else{
			lib.XLogErr("botid", b.MyID, "skip invalid reply message", b.OwnerID, mode, msg.Chat.ID)
		}
	}else if msg.Chat.Type == "private"{
		if mode == "private"{
			lib.XLogInfo("botid", b.MyID, "forward to private")
			b.ForwardMessage(msg)
		}else{
			lib.XLogInfo("botid", b.MyID, "forward to group")
			b.ForwardMessageToGroup(msg)
		}
	}
}

func (b *ChatBot)ForwardMessageToGroup(msg *model.Message){
	if msg.From == nil{
		lib.XLogErr("botid", b.MyID, "missing from", msg.Chat.ID, msg.MessageID)
		return
	}
	from_userid := msg.From.ID
	thread_info, err := GetGroupThread(b.MyID, from_userid)
	if err != nil {
		lib.XLogErr("botid", b.MyID, "GetGroupThread", from_userid)
		return
	}
	config := model.ForwardMessageConfig{
		ChatID: b.GroupID,
		FromChatID: msg.Chat.ID,
		MessageID: msg.MessageID,
	}
	update := false
	lib.XLogInfo("botid", b.MyID, "gorupid", thread_info, b.GroupID)
	if thread_info.GroupID == b.GroupID{
		if thread_info.ThreadID > 0{
			config.MessageThreadId = thread_info.ThreadID
		}
	}else{
		thread_info.GroupID = b.GroupID
		topic_name := "会话" + strconv.FormatInt(time.Now().Unix(), 10) + "|" + msg.From.FirstName + msg.From.LastName + "|" + strconv.FormatInt(msg.From.ID, 10)
		create_topic := model.CreateForumTopicConfig{
			ChatID: b.GroupID,
			Name: topic_name,
		}
		if err := b.Bot.Call(&create_topic); err != nil{
			lib.XLogErr("botid", b.MyID, "create topic", create_topic, err)
			return
		}
		thread_info.ThreadID = create_topic.Response.MessageThreadId
		config.MessageThreadId = thread_info.ThreadID
		update = true
	}
	if err := b.Bot.Call(&config); err != nil{
		lib.XLogErr("botid", b.MyID, "forward message", config, err)
		return
	}
	lib.XLogInfo("botid", b.MyID, "send to group", config)
	if update {
		if err := SetGroupThread(b.MyID, from_userid, b.GroupID, thread_info.ThreadID); err != nil{
			lib.XLogErr("botid", b.MyID, "SetGroupThread", from_userid, b.GroupID, thread_info.ThreadID, err)
			return
		}
		lib.XLogInfo("botid", b.MyID, "new thread", from_userid, b.GroupID, thread_info.ThreadID)
	}
}

func (b *ChatBot)Run(){
	config := model.UpdateConfig{}
	config.Offset = 0
	config.Limit = 100
	config.Timeout = 10
	ch := b.Bot.GetUpdateChan(&config)
	for update := range ch {
		if update.MyChatMember != nil && update.MyChatMember.Date > 0{
			go b.HandleChatStatus(update.MyChatMember)
			continue
		}
		if update.Message != nil && update.Message.MessageID > 0{
			go b.HandleMessage(update.Message)
			continue
		}
	}
}
