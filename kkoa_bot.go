package main

import (
	"zincsearch/lib"
	"zincsearch/model"
	"unicode/utf16"
	"fmt"
	"time"
    "github.com/redis/go-redis/v9"
	"encoding/json"
	"strings"
	"encoding/base64"
	"zincsearch/zincsearch"
	"os"
	"bufio"
	"io"
	"zincsearch/db"
	"sync"
	"strconv"
)

var g_sBotKey = ""
var g_sBakKey = ""
var tb model.TBot
var adminuser = ""
var adminchatid = int64(0)
var zincsearch_url = ""
var zincsearch_user = ""
var zincsearch_passwd = ""

const (
	mediaGroupWaitTime = 1000 * time.Millisecond // 媒体组等待时间
	targetChatID      = -1002295970194              // 名师汇 目标会话ID
	reportChatID = -1002302072618 // 报告群
	reportGroupUserName = "guangzhoureport"
)

type MediaGroupCache struct {
	sync.Mutex
	groups map[string][]model.InputMedia
	timers map[string]*time.Timer
}

// 处理媒体组消息
func (c *MediaGroupCache) handleMediaGroup(msg *model.Message) {
	c.Lock()
	defer c.Unlock()

	mgID := msg.MediaGroupID
	media := createInputMedia(msg)

	// 创建或更新定时器
	if timer, exists := c.timers[mgID]; exists {
		timer.Reset(mediaGroupWaitTime)
	} else {
		c.timers[mgID] = time.AfterFunc(mediaGroupWaitTime, func() {
			c.sendMediaGroup(mgID)
		})
	}

	// 添加媒体到组
	c.groups[mgID] = append(c.groups[mgID], media)
}

// 发送缓存的媒体组
func (c *MediaGroupCache) sendMediaGroup(mgID string) {
	c.Lock()
	defer c.Unlock()

	medias, exists := c.groups[mgID]
	if !exists || len(medias) == 0 {
		return
	}

	// 创建请求
	config := model.SendMediaGroupConfig{ChatID:targetChatID}
	config.Media = medias

	if err := tb.Call(&config); err != nil{
		lib.XLogErr("Call", config, err)
	}else{
		for _, v := range config.Response{
			if len(v.Caption) == 0{
				continue
			}
			feed := transferCaption(v.Caption)
			feed.MessageID = v.MessageID
			feed.ChatID = targetChatID

			username := strings.TrimSpace(feed.UserName)
			key := "jsfeed_" + username
			if err := db.SetStruct(key, feed); err != nil {
				lib.XLogErr("SetStruct", err, key, feed)
			}
			index_key := "jsfeed_index"
			var index_list model.JsIndex
			if err := db.GetStruct(index_key, &index_list); err != nil && err != redis.Nil{
				lib.XLogErr("GetStruct", err, index_key)
			}else{
				index_list.List = append(index_list.List, username)
				err = db.SetStruct(index_key, index_list)
				if err != nil {
					lib.XLogErr("SetStruct", err, index_key, index_list)
				}
			}
		}
	}

	// 清理缓存
	delete(c.groups, mgID)
	delete(c.timers, mgID)
}

// 创建InputMedia对象
func createInputMedia(msg *model.Message) model.InputMedia {
	defer func() {
		if err := recover(); err != nil {
			lib.XLogErr("excption", err)
		}
	}()
	var media model.InputMedia
	switch {
	case msg.Photo != nil:
		fileID := msg.Photo[len(msg.Photo)-1].FileID
		media.Type = "photo"
		media.Media = fileID
	case msg.Video != nil:
		fileID := msg.Video.FileID
		media.Type = "video"
		media.Media = fileID
	}
	if len(msg.Caption) > 0{
		feed := transferCaption(msg.Caption)
		lib.XLogInfo(feed)
		new_caption, captionEntities := generateCaptionAndEmtites(feed)
		media.Caption = new_caption + "评论区输入\"" + "我爱" + feed.Name + "\"查看校友点评\n"
		media.CaptionEmtities = captionEntities
	}
	return media
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
		}else if line[0:idx] == "search_user"{
			zincsearch_user = line[idx + 1:]
		}else if line[0:idx] == "search_passwd"{
			zincsearch_passwd = line[idx + 1:]
		}else if line[0:idx] == "search_url"{
			zincsearch_url = line[idx + 1:]
		}else if line[0:idx] == "bak_key"{
			g_sBakKey = line[idx + 1:]
		}
	}
}

func extractMedia(msg *model.Message, photos, videos []string)([]string, []string){
	if len(msg.Photo) > 0{
		photos = append(photos, msg.Photo[len(msg.Photo) - 1].FileID)
	}
	if msg.Video != nil {
		videos = append(videos, msg.Video.FileID)
	}
	//lib.XLogInfo("extract", photos, videos)
	return photos, videos
}

func main() {
	InitConfig()
	tb.BotKey = g_sBotKey
	if len(g_sBakKey) > 0{
		tb.BakKey = g_sBakKey
		tb.UseBakKey = true
	}

	config := model.UpdateConfig{}
	config.Offset = 0
	config.Limit = 100
	config.Timeout = 10
	ch := tb.GetUpdateChan(&config)

	cache := &MediaGroupCache{
		groups: make(map[string][]model.InputMedia),
		timers: make(map[string]*time.Timer),
	}

	cur_cmd := ""
	for update := range ch {
		if update.Message != nil{
			if update.Message.Chat.Type != "private"{
				lib.XLogErr("not private", update.Message.Chat.Type)
				continue
			}
			// 非管理员发的反馈消息，如果是command直接执行，否则转发
			if update.Message.From.UserName != adminuser{
				lib.XLogErr("not admin", update.Message.From.UserName)
				forwardMessage(update.Message)
				continue
			}
			// 管理员回复的消息，转发给原始发消息的用户
			if update.Message.ReplyToMessage != nil && update.Message.ReplyToMessage.ForwardFrom != nil{
				forwardMessageToChat(update.Message, update.Message.ReplyToMessage.ForwardFrom.ID)
				continue
			}
			if isCommand(update.Message.Text){
				if update.Message.Text == "clear"{
					cur_cmd = ""
					lib.XLogInfo("clear command")
				}else{
					cur_cmd = update.Message.Text
					lib.XLogInfo("change command", cur_cmd)
				}
				continue
			}
			if cur_cmd != ""{
				// 处理媒体组消息
				if mgID := update.Message.MediaGroupID; mgID != "" {
					cache.handleMediaGroup(update.Message)
				}else{
					handleCommand(cur_cmd, update.Message)
				}
			}
		}
	}
}

func forwardMessageToChat(msg *model.Message, chatid int64){
	config := model.SendMessageConfig{
		ChatID: chatid,
		Text: msg.Text,
		Entities: msg.Entities,
	}
	tb.Call(&config)
}

func forwardMessage(msg *model.Message){
	if adminchatid == 0{
		sendText(msg.Chat.ID, "消息发送失败，请稍后重试...")
	}
	config := model.ForwardMessageConfig{
		ChatID: adminchatid,
		FromChatID: msg.Chat.ID,
		MessageID: msg.MessageID,
	}
	tb.Call(&config)
}

func isCommand(text string)bool{
	cmds := []string{"report_index", "report_detail", "import_report", "clear_jsindex", "show_jsdetail", "list_jsindex", "import_js", "create_index", "list_index", "delete_index", "insert_document", "clear", "delete_document", "add_adfeed", "list_adfeed", "delete_adfeed", "add_topfeed", "list_topfeed", "delete_topfeed", "get_chatid"}
	for _, v := range cmds{
		if text == v{
			return true
		}
	}
	return false
}

func createIndex(index_name string)error{
	client := zincsearch.NewClient(zincsearch_url, zincsearch_user, zincsearch_passwd)
	settings := &zincsearch.IndexSettings{
		Name: index_name,
		NumberOfShards:   3,
		Mappings: map[string]interface{}{
			"properties": map[string]interface{}{
				"title": zincsearch.FieldSetting{Type:"text", Index:true},
				"description":   zincsearch.FieldSetting{Type: "text", Index:true},
				"chat_id":  zincsearch.FieldSetting{Type: "keyword"},
				"user_count": zincsearch.FieldSetting{Type: "integer", Index:true},
				"js_name": zincsearch.FieldSetting{Type:"text", Index:true},
				"js_type": zincsearch.FieldSetting{Type: "keyword"},
				"tags": zincsearch.FieldSetting{Type: "text", Index:true},
				"location": zincsearch.FieldSetting{Type: "text", Index:true},
			},
		},
	}
	return client.CreateIndex(settings)
}

func listIndex(chatid int64)error{
	client := zincsearch.NewClient(zincsearch_url, zincsearch_user, zincsearch_passwd)
	indexs, err := client.ListIndexes()
	if err != nil{
		sendText(chatid, "操作失败")
		return err
	}
	lib.XLogInfo(indexs)
	text := ""
	for _, v := range indexs{
		text += v + "\n"
	}
	sendText(chatid, text)
	return nil
}

func deleteIndex(index_name string)error{
	client := zincsearch.NewClient(zincsearch_url, zincsearch_user, zincsearch_passwd)
	return client.DeleteIndex(index_name)
}

func batchInsertDocument(chatid int64, text string)error{
	data := strings.TrimSpace(text)
	lines := strings.Split(data, "\n")
	for _, v := range lines{
		if err := insertDocument(chatid, v); err != nil{
			return err
		}
	}
	return nil
}

func insertDocument(chatid int64, text string)error{
	data := strings.TrimSpace(text)
	values := strings.Split(data, " ")
	if len(values) < 5{
		sendText(chatid, "输入错误，请按以下格式: index_name chatid jsname jstype location tags1 tags2 ...")
		return nil
	}
	// index_name chatid jsname jstype tags1 tags2 ....
	user_count_config := model.GetChatMemberCountConfig{ChatID: "@"+values[1]}
	param, err := json.Marshal(user_count_config)
	if err != nil{
		lib.XLogErr("marlshal", err, user_count_config)
		sendText(chatid, "操作失败")
		return err
	}
	rsp, err := tb.Request("getchatmembercount", string(param))
	if err != nil{
		lib.XLogErr("request", err, param)
		sendText(chatid, "操作失败")
		return err
	}
	var user_count_obj model.IntResult
	err = json.Unmarshal([]byte(rsp), &user_count_obj)
	if err != nil {
		lib.XLogErr("unmarshal", err)
		sendText(chatid, "操作失败")
		return err
	}

	chatinfo_config := model.GetChatConfig{ChatID: "@" + values[1]}
	err = tb.Call(&chatinfo_config)
	if err != nil{
		lib.XLogErr("getchatinfo", err)
		sendText(chatid, "操作失败")
		return err
	}
	chat := chatinfo_config.Response

	str_tags := ""
	for i, v := range values{
		if i < 5{
			continue
		}
		str_tags += "#" + v
	}

	client := zincsearch.NewClient(zincsearch_url, zincsearch_user, zincsearch_passwd)
	doc := zincsearch.Document{
		Title: chat.Title,
		Description: chat.Description,
		ChatID: values[1],
		UserCount: user_count_obj.Result,
		JsName: values[2],
		JsType: values[3],
		Location: values[4],
		Tags: str_tags,
	}
	return client.UpdateDocument(values[0], values[1], doc)
}

func deleteDocument(chatid int64, text string)error{
	data := strings.TrimSpace(text)
	values := strings.Split(data, " ")
	if len(values) != 2{
		sendText(chatid, "操作失败，请按照以下格式输入：index_name chatid")
		return nil
	}
	client := zincsearch.NewClient(zincsearch_url, zincsearch_user, zincsearch_passwd)
	return client.DeleteDocument(values[0], values[1])
}

func getAdFeeds(key string)model.AdFeedList{
	var feeds model.AdFeedList
	if err := db.GetStruct(key, &feeds); err != nil{
		lib.XLogErr("empty adfeeds", key)
	}
	return feeds
}

func addAdfeed(chatid int64, text string)error{
	data := strings.TrimSpace(text)
	values := strings.Split(data, " ")
	if len(values) != 2{
		sendText(chatid, "操作失败，请按照以下格式输入：chatid order")
		return nil
	}

	list := getAdFeeds("zincsearch_bot_adfeeds")
	chatinfo_config := model.GetChatConfig{ChatID: "@" + values[0]}
	err := tb.Call(&chatinfo_config)
	if err != nil{
		lib.XLogErr("getchatinfo", err)
		sendText(chatid, "操作失败")
		return err
	}
	order, err := strconv.Atoi(values[1])
	if err != nil{
		sendText(chatid, "操作失败，order转换失败")
		return err
	}
	chat := chatinfo_config.Response
	feed := model.AdFeed{
		Title: chat.Title,
		ChatID: values[0],
		Order: order,
		TS: time.Now().Unix() + 3600 * 24 * 30,
	}
	list.Feeds = append(list.Feeds, feed)

	err = db.SetStruct("zincsearch_bot_adfeeds", list)
	if err != nil {
		sendText(chatid, "操作失败")
		return err
	}
	return nil
}

func listAdfeed(chatid int64)error{
	list := getAdFeeds("zincsearch_bot_adfeeds")
	text := ""
	for _, v := range list.Feeds{
		text += v.Title + " " + v.ChatID + " " + strconv.Itoa(v.Order) + "\n"
	}
	sendText(chatid, text)
	return nil
}

func deleteAdfeed(chatid int64, text string)error{
	channelid := strings.TrimSpace(text)
	list := getAdFeeds("zincsearch_bot_adfeeds")
	var newlist  model.AdFeedList
	for _, v := range list.Feeds{
		if v.ChatID != channelid{
			newlist.Feeds = append(newlist.Feeds, v)
		}
	}
	return db.SetStruct("zincsearch_bot_adfeeds", newlist)
}

func addTopfeed(chatid int64, text string)error{
	data := strings.TrimSpace(text)
	values := strings.Split(data, " ")
	if len(values) != 3{
		sendText(chatid, "操作失败，请按照以下格式输入：关键词 chatid")
		return nil
	}
	key := "zincsearch_bot_topfeeds_" + base64.StdEncoding.EncodeToString([]byte(values[0]))
	list := getAdFeeds(key)

	chatinfo_config := model.GetChatConfig{ChatID: "@" + values[1]}
	err := tb.Call(&chatinfo_config)
	if err != nil{
		lib.XLogErr("getchatinfo", err)
		sendText(chatid, "操作失败")
		return err
	}
	order, err := strconv.Atoi(values[2])
	if err != nil{
		sendText(chatid, "操作失败，order转换失败")
		return err
	}
	chat := chatinfo_config.Response
	feed := model.AdFeed{
		Title: chat.Title,
		ChatID: values[1],
		Order: order,
		TS: time.Now().Unix() + 3600 * 24 * 30,
	}
	list.Feeds = append(list.Feeds, feed)

	err = db.SetStruct(key, list)
	if err != nil {
		sendText(chatid, "操作失败")
		return err
	}
	return nil
}

func listTopfeed(chatid int64, text string)error{
	data := strings.TrimSpace(text)
	key := "zincsearch_bot_topfeeds_" + base64.StdEncoding.EncodeToString([]byte(data))
	list := getAdFeeds(key)
	text = ""
	for _, v := range list.Feeds{
		text += v.Title + " " + v.ChatID + " " + strconv.Itoa(v.Order) + "\n"
	}
	sendText(chatid, text)
	return nil
}

func deleteTopfeed(chatid int64, text string)error{
	data := strings.TrimSpace(text)
	values := strings.Split(data, " ")
	if len(values) != 2{
		sendText(chatid, "操作失败，请按照以下格式输入：关键词 chatid")
		return nil
	}
	key := "zincsearch_bot_topfeeds_" + base64.StdEncoding.EncodeToString([]byte(values[0]))
	list := getAdFeeds(key)
	var newlist model.AdFeedList
	for _, v := range list.Feeds{
		if v.ChatID != values[1]{
			newlist.Feeds = append(newlist.Feeds, v)
		}
	}
	return db.SetStruct(key, newlist)
}

func insertForwardMessagev4(chatid int64, text string){
	raw := strings.TrimSpace(text)
	lines := strings.Split(raw, "\n")
	username := ""
	js_name := ""
	location := ""
	str_tag := ""
	xiegang := "/"
	for _, v := range lines{
		tmp := strings.TrimSpace(v)
		if strings.HasPrefix(tmp, "地址"){
			if len(strings.Split(tmp, ":")) > 1{
				location = strings.Split(tmp, ":")[1]
			}
			if len(strings.Split(tmp, "：")) > 1{
				location = strings.Split(tmp, "：")[1]
			}
		}
		if strings.HasPrefix(tmp, "坐标"){
			if len(strings.Split(tmp, ":")) > 1{
				location = strings.Split(tmp, ":")[1]
			}
			if len(strings.Split(tmp, "：")) > 1{
				location = strings.Split(tmp, "：")[1]
			}
		}
		if strings.HasPrefix(tmp, "艺名"){
			if len(strings.Split(tmp, ":")) > 1{
				js_name = strings.Split(tmp, ":")[1]
			}
			if len(strings.Split(tmp, "：")) > 1{
				js_name = strings.Split(tmp, "：")[1]
			}
		}
		if strings.HasPrefix(tmp, "频道"){
			var values []string
			if len(strings.Split(tmp, ":")) > 1{
				values = strings.Split(tmp, ":")
			}else{
				values = strings.Split(tmp, "：")
			}
			url := ""
			for i, v := range values{
				if i > 0{
					url += v
				}
			}
			url = strings.TrimSpace(url)
			if strings.HasPrefix(url, "@"){
				username = url[1:]
			}else{
				idx := strings.LastIndex(url, xiegang)
				username = url[idx + 1:]
			}
		}
		str_tag += tmp + " "
	}
	str_tag += " 广州"
	data := "search_qm " + username + " " + js_name + " " + location + " " + str_tag
	lib.XLogInfo(data)
	if err := insertDocument(chatid, data); err != nil{
		time.Sleep(1)
		insertDocument(chatid, data)
	}
}

func insertForwardMessagev3(chatid int64, text string){
	raw := strings.TrimSpace(text)
	lines := strings.Split(raw, "\n")
	username := ""
	js_name := ""
	location := ""
	str_tag := ""
	xiegang := "/"
	for _, v := range lines{
		tmp := strings.TrimSpace(v)
		if strings.HasPrefix(tmp, "📍位置"){
			if len(strings.Split(tmp, ":")) > 1{
				location = strings.Split(tmp, ":")[1]
			}
			if len(strings.Split(tmp, "：")) > 1{
				location = strings.Split(tmp, "：")[1]
			}
		}
		if strings.HasPrefix(tmp, "坐标"){
			if len(strings.Split(tmp, ":")) > 1{
				location = strings.Split(tmp, ":")[1]
			}
			if len(strings.Split(tmp, "：")) > 1{
				location = strings.Split(tmp, "：")[1]
			}
		}
		if strings.HasPrefix(tmp, "🌸艺名"){
			if len(strings.Split(tmp, ":")) > 1{
				js_name = strings.Split(tmp, ":")[1]
			}
			if len(strings.Split(tmp, "：")) > 1{
				js_name = strings.Split(tmp, "：")[1]
			}
		}
		if strings.HasPrefix(tmp, "💧频道"){
			var values []string
			if len(strings.Split(tmp, ":")) > 1{
				values = strings.Split(tmp, ":")
			}else{
				values = strings.Split(tmp, "：")
			}
			url := ""
			for i, v := range values{
				if i > 0{
					url += v
				}
			}
			url = strings.TrimSpace(url)
			if strings.HasPrefix(url, "@"){
				username = url[1:]
			}else{
				idx := strings.LastIndex(url, xiegang)
				username = url[idx + 1:]
			}
		}
		str_tag += tmp + " "
	}
	str_tag += " 广州"
	data := "search_qm " + username + " " + js_name + " " + location + " " + str_tag
	lib.XLogInfo(data)
	if err := insertDocument(chatid, data); err != nil{
		time.Sleep(1)
		insertDocument(chatid, data)
	}
}

// 广州修车公开榜
func insertForwardMessage(chatid int64, text string){
	raw := strings.TrimSpace(text)
	lines := strings.Split(raw, "\n")
	username := ""
	js_name := ""
	location := ""
	str_tag := ""
	xiegang := "/"
	for _, v := range lines{
		tmp := strings.TrimSpace(v)
		if strings.HasPrefix(tmp, "地址: #"){
			location = strings.Split(v, "#")[1]
		}
		if strings.HasPrefix(tmp, "艺名: #"){
			js_name = strings.Split(v, "#")[1]
		}
		if strings.HasPrefix(tmp, "频道:"){
			values := strings.Split(tmp, ":")
			url := ""
			for i, v := range values{
				if i > 0{
					url += v
				}
			}
			url = strings.TrimSpace(url)
			if strings.HasPrefix(url, "@"){
				username = url[1:]
			}else{
				idx := strings.LastIndex(url, xiegang)
				username = url[idx + 1:]
			}
		}
		str_tag += tmp
	}
	data := "search_qm " + username + " " + js_name + " " + location + " " + str_tag
	lib.XLogInfo(data)
	insertDocument(chatid, data)
}

// 广州修车公开资源榜
func insertForwardMessagev2(chatid int64, text string){
	raw := strings.TrimSpace(text)
	lines := strings.Split(raw, "\n")
	username := ""
	js_name := ""
	location := ""
	str_tag := ""
	xiegang := "/"
	for _, v := range lines{
		tmp := strings.TrimSpace(v)
		if strings.HasPrefix(tmp, "位置:"){
			location = strings.Split(v, "位置：#")[1]
		}
		if strings.HasPrefix(tmp, "花名：#"){
			js_name = strings.Split(v, "花名：#")[1]
		}
		if strings.HasPrefix(tmp, "频道："){
			values := strings.Split(tmp, "：")
			url := ""
			for i, v := range values{
				if i > 0{
					url += v
				}
			}
			url = strings.TrimSpace(url)
			if strings.HasPrefix(url, "@"){
				username = url[1:]
			}else{
				idx := strings.LastIndex(url, xiegang)
				username = url[idx + 1:]
			}
		}
		str_tag += tmp + " "
	}
	str_tag += " 广州"
	data := "search_qm " + username + " " + js_name + " " + location + " " + str_tag
	lib.XLogInfo(data)
	insertDocument(chatid, data)
}

func transferCaption(input string)(model.JsFeed){
	var feed model.JsFeed
	if len(input) == 0{
		return feed
	}
	text := strings.TrimSpace(input)
	lines := strings.Split(text, "\n")
	tags := ""
	for _, line := range lines{
		if strings.Contains(line, "地址"){
			tmp := strings.TrimSpace(line)
			values := strings.Split(tmp, ":")
			if len(values) <= 1{
				values = strings.Split(tmp, "：")
			}
			location := strings.TrimSpace(values[1])
			if strings.HasPrefix(location, "#"){
				feed.Location = location[1:]
			}else{
				feed.Location = location
			}
		}
		if strings.Contains(line, "艺名"){
			tmp := strings.TrimSpace(line)
			values := strings.Split(tmp, ":")
			if len(values) <= 1{
				values = strings.Split(tmp, "：")
			}
			name := strings.TrimSpace(values[1])
			names := strings.Split(name, " ")
			for i, v := range names{
				if i == 0{
					if strings.HasPrefix(v, "#"){
						feed.Name = v[1:]
					}else{
						feed.Name = v
					}
				}
				tags += " " + v
			}
		}
		if strings.Contains(line, "价格"){
			tmp := strings.TrimSpace(line)
			values := strings.Split(tmp, ":")
			if len(values) <= 1{
				values = strings.Split(tmp, "：")
			}
			prices := strings.Split(values[1], " ")
			for _, v := range prices {
				if strings.HasPrefix(v, "#"){
					feed.Price = append(feed.Price, strings.TrimSpace(v[1:]))
				}else{
					feed.Price = append(feed.Price, strings.TrimSpace(v))
				}
			}
		}
		if strings.Contains(line, "频道"){
			tmp := strings.TrimSpace(line)
			var values []string
			if len(strings.Split(tmp, ":")) > 1{
				values = strings.Split(tmp, ":")
			}else{
				values = strings.Split(tmp, "：")
			}
			url := ""
			for i, v := range values{
				if i > 0{
					url += v
				}
			}
			url = strings.TrimSpace(url)
			if strings.HasPrefix(url, "@"){
				feed.ChannelUserName = url[1:]
			}else{
				xiegang := "/"
				idx := strings.LastIndex(url, xiegang)
				feed.ChannelUserName = "@" + url[idx + 1:]
			}
		}
		if strings.Contains(line, "私聊"){
			tmp := strings.TrimSpace(line)
			values := strings.Split(tmp, ":")
			if len(values) <= 1{
				values = strings.Split(tmp, "：")
			}
			feed.UserName = strings.TrimSpace(values[1])
		}
		if strings.Contains(line, "状态") || strings.Contains(line, "标签"){
			tmp := strings.TrimSpace(line)
			values := strings.Split(tmp, ":")
			if len(values) <= 1{
				values = strings.Split(tmp, "：")
			}
			tags += " " + strings.TrimSpace(values[1])
		}
	}
	tag_item := strings.Split(strings.TrimSpace(tags), " ")
	for _, v := range tag_item{
		feed.Tags = append(feed.Tags, strings.TrimSpace(v))
	}
	return feed
}

func generateCaptionAndEmtites(feed model.JsFeed)(string, []model.MessageEntity){
	text := "艺名: " + feed.Name + "\n" + "地址: " + feed.Location + "\n"
	text += "价格: "
	for _, v := range feed.Price{
		text += v + " "
	}
	text += "\n"
	text += "私聊: "
	siliao := model.MessageEntity{
		Type:"mention",
		Offset: GetUTF16Len(text),
		Length: GetUTF16Len(feed.UserName),
	}
	var emtities []model.MessageEntity
	emtities = append(emtities, siliao)
	text += feed.UserName + "\n"
	text += "频道: "
	pindao := model.MessageEntity{
		Type:"mention",
		Offset: GetUTF16Len(text),
		Length: GetUTF16Len(feed.ChannelUserName),
	}
	emtities = append(emtities, pindao)
	text += feed.ChannelUserName + "\n"
	text += "标签: "
	for _, v := range feed.Tags{
		tag := model.MessageEntity{
			Type: "hashtag",
			Offset: GetUTF16Len(text),
			Length: GetUTF16Len(v),
		}
		text += v + " "
		emtities = append(emtities, tag)
	}
	text += "\n"
	return text, emtities
}

func GetUTF16Len(content string)int{
	encodeContent := utf16.Encode([]rune(content))
	return len(encodeContent)
}

func listJsIndex(chatid int64)error{
	index_key := "jsfeed_index"
	var index_list model.JsIndex
	err := db.GetStruct(index_key, &index_list)
	if err != nil && err != redis.Nil{
		sendText(chatid, "操作失败")
		return err
	}
	if len(index_list.List) == 0{
		sendText(chatid, "列表为空")
	}else{
		text := ""
		for _, v := range index_list.List{
			text += v + "\n"
		}
		sendText(chatid, text)
	}
	return nil
}

func showJsDetail(chatid int64, text string)error{
	data := strings.TrimSpace(text)
	key := "jsfeed_" + data
	var feed model.JsFeed
	if err := db.GetStruct(key, &feed); err != nil {
		sendText(chatid, "操作失败")
		return err
	}
	str := fmt.Sprintf("%v\n", feed)
	sendText(chatid, str)
	return nil
}

func clearJsIndex(chatid int64)error{
	index_key := "jsfeed_index"
	return db.Del(index_key)
}

func getValue(line string)string{
	value := ""
	if strings.Contains(line, "："){
		value = strings.TrimSpace(strings.Split(line, "：")[1])
	}else{
		value = strings.TrimSpace(strings.Split(line, "】")[1])
	}
	value = strings.ReplaceAll(value, ":", "")
	value = strings.ReplaceAll(value, "：", "")
	return strings.TrimSpace(value)
}

func importReport(chatid int64, text string)error{
	var report model.JsReport
	data := strings.TrimSpace(text)
	lines := strings.Split(data, "\n")
	for _, line := range lines{
		line = strings.TrimSpace(line)
		if strings.Contains(line, "留名】"){
			report.Ly = getValue(line)
		}else if strings.Contains(line, "【个人评价】"){
			if strings.Contains(line, "好评"){
				report.Mark = "9"
			}else if strings.Contains(line, "中评"){
				report.Mark = "6"
			}else if strings.Contains(line, "差评"){
				report.Mark = "2"
			}
		}else if strings.Contains(line, "【人照几成】"){
			report.Yanzhi = getValue(line)
		}else if strings.Contains(line, "【验证时间】"){
			report.Time = getValue(line)
		}else if strings.Contains(line, "艺名】") || strings.Contains(line, "花名】"){
			report.Js = getValue(line)
		}else if strings.Contains(line, "【所在位置】"){
			report.Location = getValue(line)
		}else if strings.Contains(line, "【联系方式】"){
			report.UserName = getValue(line)
		}else if strings.Contains(line, "【修车水费】"){
			report.Price = getValue(line)
		}else if strings.Contains(line, "【服务态度】"){
			report.Taidu = getValue(line)
		}else if strings.Contains(line, "点击查看老师资料") || strings.Contains(line, "【温馨提醒】") || strings.Contains(line, "点击查看老师资料") || strings.Contains(line, "联邦报告"){
			continue
		}else if strings.Contains(line, "】"){
			line = strings.ReplaceAll(line, ":", "")
			line = strings.ReplaceAll(line, "：", "")
			report.ExtraItem += line + "\n"
		}else{
			report.FreeTalk += line + "\n"
		}
	}
	report.GroupUserName = reportGroupUserName
	content := "【校友汇】https://t.me/gzxiaoyou\n"
	content += "【点评校友】" + report.Ly + "\n"
	content += "【老师艺名】" + report.Js + "\n"
	content += "【联系方式】" + report.UserName + "\n"
	content += "【课室位置】" + report.Location + "\n"
	content += "【上课时间】" + report.Time + "\n"
	content += "【推荐程度】" + report.Mark + "\n"
	content += "【上课费用】" + report.Price + "\n"
	content += "【老师颜值】" + report.Yanzhi + "\n"
	content += "【上课态度】" + report.Taidu + "\n"
	content += report.ExtraItem
	content += "【自由点评】\n" + report.FreeTalk + "\n"

	config := model.SendMessageConfig{}
	config.ChatID = reportChatID
	config.Text = content
	if err := tb.Call(&config); err != nil{
		sendText(chatid, "导入失败")
		return err
	}
	new_msg := config.Response
	report.MessageID = new_msg.MessageID
	lib.XLogInfo(report)

	tmp := report.UserName + "_" + report.Ly + "_" + report.Time
    key := "jsreport_" + base64.StdEncoding.EncodeToString([]byte(tmp))
	err := db.SetStruct(key, report)
	if err != nil{
		lib.XLogErr("SetStruct", key, err)
		return err
	}
	var index model.JsReportIndex
	err = db.GetStruct("jsreport_index_" + report.UserName, &index)
	if err != nil && err != redis.Nil{
		lib.XLogErr("GetStruct", err)
		return err
	}
	index.Keys = append(index.Keys, key)
	return db.SetStruct("jsreport_index_" + report.UserName, index)
}

func reportIndex(chatid int64, text string){
	data := strings.TrimSpace(text)
	var index model.JsReportIndex
	err := db.GetStruct("jsreport_index_" + data, &index)
	if err != nil && err != redis.Nil{
		lib.XLogErr("GetStruct", err)
		return
	}
	str := ""
	for _, v:= range index.Keys{
		str += v + "\n"
	}
	sendText(chatid, str)
}

func reportDetail(chatid int64, text string){
	var report model.JsReport
	err := db.GetStruct(strings.TrimSpace(text), &report)
	if err != nil {
		lib.XLogErr("GetStruct", text)
		return
	}
	sendText(chatid, fmt.Sprintf("%v\n", report))
}

func handleCommand(cmd string, msg *model.Message){
	if cmd == "report_index"{
		reportIndex(msg.Chat.ID, msg.Text)
	}else if cmd == "report_detail"{
		reportDetail(msg.Chat.ID, msg.Text)
	}else if cmd == "import_report"{
		if err := importReport(msg.Chat.ID, msg.Text); err != nil{
			lib.XLogErr("importReport", err)
		}
	}else if cmd == "clear_jsindex"{
		if err := clearJsIndex(msg.Chat.ID); err != nil{
			lib.XLogErr("clearJsIndex", err)
		}
	}else if cmd == "list_jsindex"{
		if err := listJsIndex(msg.Chat.ID); err != nil{
			lib.XLogErr("listJsIndex", err)
		}
	}else if cmd == "show_jsdetail"{
		if err := showJsDetail(msg.Chat.ID, msg.Text); err != nil{
			lib.XLogErr("showJsDetail", msg.Text, err)
		}
	}else if cmd == "create_index"{
		if err := createIndex(msg.Text); err != nil{
			lib.XLogErr("createIndex", err, msg.Text)
		}
	}else if cmd == "list_index"{
		if err := listIndex(msg.Chat.ID); err != nil{
			lib.XLogErr("listIndex", err)
		}
	}else if cmd == "delete_index"{
		if err := deleteIndex(msg.Text); err != nil{
			lib.XLogErr("deleteIndex", err, msg.Text)
		}
	}else if cmd == "insert_document"{
		if len(msg.Caption) > 0{
			insertForwardMessagev4(msg.Chat.ID, msg.Caption)
		}
		return
		if err := batchInsertDocument(msg.Chat.ID, msg.Text); err != nil{
			lib.XLogErr("insertDocument", err, msg.Text)
		}
	}else if cmd == "delete_document"{
		if err := deleteDocument(msg.Chat.ID, msg.Text); err != nil{
			lib.XLogErr("deleteDocument", err, msg.Text)
		}
	}else if cmd == "add_adfeed"{
		if err := addAdfeed(msg.Chat.ID, msg.Text); err != nil {
			lib.XLogErr("addAdfeed", err, msg.Text)
		}
	}else if cmd == "list_adfeed"{
		if err := listAdfeed(msg.Chat.ID); err != nil{
			lib.XLogErr("listAdfeed", err)
		}
	}else if cmd == "delete_adfeed"{
		if err := deleteAdfeed(msg.Chat.ID, msg.Text); err != nil{
			lib.XLogErr("deleteAdfeed", err, msg.Text)
		}
	}else if cmd == "add_topfeed"{
		if err := addTopfeed(msg.Chat.ID, msg.Text); err != nil{
			lib.XLogErr("addTopfeed", err, msg.Text)
		}
	}else if cmd == "list_topfeed"{
		if err := listTopfeed(msg.Chat.ID, msg.Text); err != nil{
			lib.XLogErr("listTopfeed", err, msg.Text)
		}
	}else if cmd == "delete_topfeed"{
		if err := deleteTopfeed(msg.Chat.ID, msg.Text); err != nil{
			lib.XLogErr("deleteTopfeed", err, msg.Text)
		}
	}else if cmd == "get_chatid"{
		adminchatid = msg.Chat.ID
		lib.XLogInfo("admin chatid", msg.Chat.ID)
		sendText(msg.Chat.ID, "获取chatid成功")
	}
}

func sendText(chatid int64, text string){
	config := model.SendMessageConfig{}
	config.ChatID = chatid
	config.Text = text
	tb.Call(&config)
}
