package main

import (
	"zincsearch/lib"
	"zincsearch/model"
	"unicode/utf16"
	"fmt"
    "github.com/redis/go-redis/v9"
	"strings"
	"encoding/base64"
	"os"
	"bufio"
	"io"
	"zincsearch/db"
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

func showJsReport(msg *model.Message){
	reply_msg := msg.ReplyToMessage
	feed := transferCaption(reply_msg.Caption)
	if len(feed.UserName) == 0{
		lib.XLogErr("empty username", reply_msg.Caption, feed)
		return
	}
	caption, emtities := generateCaptionAndEmtites(feed)
	caption += "\n"

	var index model.JsReportIndex
	err := db.GetStruct("jsreport_index_" + feed.UserName, &index)
	if err != nil {
		lib.XLogErr("GetStruct", feed.UserName)
		return
	}
	for i, v := range index.Keys{
		if i >= 10{
			break
		}
		var report model.JsReport
		err = db.GetStruct(v, &report)
		if err != nil{
			continue
		}
		title := report.Ly + "_" + report.Time + "_的验证报告"
		item := model.MessageEntity{
			Type: "text_link",
			URL: "https://t.me/" + report.GroupUserName + "/" + strconv.Itoa(report.MessageID),
			Offset: GetUTF16Len(caption),
			Length: GetUTF16Len(title),
		}
		emtities = append(emtities, item)
		caption += title + "\n"
	}

	if len(index.Keys) > 10{
		caption += "\n👇️更多报告请前往，校友点评频道查看👇️\nhttps://t.me/guangzhoureport\n"
	}

	config := model.SendMessageConfig{
		ChatID: msg.Chat.ID,
		Text: caption,
		Entities: emtities,
		ReplyParams: model.ReplyParameters{
			MessageID: msg.ReplyToMessage.MessageID,
			ChatID: msg.ReplyToMessage.Chat.ID,
		},
	}

	tb.Call(&config)
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

	for update := range ch {
		if update.Message != nil{
			if update.Message.Chat.Type == "private"{
				lib.XLogErr("private", update.Message.Chat.Type)
				continue
			}
			// 拉评论数据
			if update.Message.ReplyToMessage != nil && len(update.Message.ReplyToMessage.Caption) > 0{
				showJsReport(update.Message)
				continue
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

func getValue(line string)string{
	if strings.Contains(line, "："){
		return strings.TrimSpace(strings.Split(line, "：")[1])
	}else{
		return strings.TrimSpace(strings.Split(line, "】")[1])
	}
	return ""
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
		}else if strings.Contains(line, "【温馨提醒】") || strings.Contains(line, "点击查看老师资料") || strings.Contains(line, "联邦报告"){
			continue
		}else{
			report.FreeTalk += line + "\n"
		}
	}
	lib.XLogInfo(report)
	tmp := report.UserName + "_" + report.Ly + "_" + report.Time
    key := "jsreport_" + base64.StdEncoding.EncodeToString([]byte(tmp))
	err := db.SetStruct(key, report)
	if err != nil{
		lib.XLogErr("SetStruct", key, err)
		return err
	}
	var index model.JsReportIndex
	err = db.GetStruct("jsreport_index" + report.UserName, &index)
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

func sendText(chatid int64, text string){
	config := model.SendMessageConfig{}
	config.ChatID = chatid
	config.Text = text
	tb.Call(&config)
}
