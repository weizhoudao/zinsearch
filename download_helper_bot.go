package main

import (
	"zincsearch/lib"
	"zincsearch/model"
	"strings"
	"os"
	"bufio"
	"io"
)

var g_sBotKey = ""
var tb model.TBot
var adminuser = ""
var adminchatid = int64(0)
var zincsearch_url = ""
var zincsearch_user = ""
var zincsearch_passwd = ""


const (
	targetChatID      = -1002295970194              // 名师汇 目标会话ID
	targetUserName = "guangzhouqm"
	shouluChatID = -1002486530943   // 收录榜
	shouluUserName = "guangzhoujs"
	reportChatID = -1002302072618 // 报告群
	reportGroupUserName = "guangzhoureport"
)

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
		}
	}
}

func main() {
	InitConfig()
	tb.BotKey = g_sBotKey

	config := model.UpdateConfig{}
	config.Offset = 0
	config.Limit = 100
	config.Timeout = 10
	ch := tb.GetUpdateChan(&config)

	for update := range ch {
		if update.Message != nil{
			if update.Message.Chat.Type != "private"{
				lib.XLogErr("not private", update.Message.Chat.Type)
				continue
			}
			// 非管理员发的反馈消息
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
		}
	}
}

func forwardMessageToChat(msg *model.Message, chatid int64){
	config := model.SendMessageConfig{
		ChatID: chatid,
		Text: msg.Text,
		Entities: msg.Entities,
	}
	if err := tb.Call(&config); err != nil{
		lib.XLogErr("call error", config, err)
		sendText(msg.Chat.ID, "发送失败")
	}
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
	if err := tb.Call(&config); err != nil{
		lib.XLogErr("call error", config, err)
		sendText(msg.Chat.ID, "消息发送失败,请稍后重试...")
	}
}

func sendText(chatid int64, text string){
	config := model.SendMessageConfig{}
	config.ChatID = chatid
	config.Text = text
	tb.Call(&config)
}
