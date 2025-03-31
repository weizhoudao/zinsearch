package main

import(
	"zincsearch/chat"
	"zincsearch/model"
	"zincsearch/lib"
	"strings"
	"strconv"
	"os"
	"bufio"
	"io"
)

var(
	g_str_botkey = ""
	g_target_userid = int64(0)
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
		key := line[0 : idx]
		value := line[idx + 1 :]
		if key == "key" {
			g_str_botkey = value
		}else if line[0: idx] == "admin"{
			tmp, err := strconv.ParseInt(value, 10, 64)
			if err != nil{
				panic(err)
			}
			g_target_userid = tmp
		}
	}
}

func main(){
	InitConfig()

	botapi := model.TBot{BotKey: "bot" + g_str_botkey}
	getme_config := model.GetMeConfig{}
	if err := botapi.Call(&getme_config); err != nil{
		panic(err)
	}

	bot := chat.NewChatBot(g_target_userid, getme_config.Response.ID, g_str_botkey)
	bot.Run()
}
