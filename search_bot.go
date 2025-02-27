package main

import (
	"zincsearch/lib"
	"zincsearch/model"
	"zincsearch/db"
	"encoding/json"
	"unicode/utf16"
	"fmt"
	"log"
	"strings"
	"zincsearch/zincsearch"
	"os"
	"bufio"
	"strconv"
	"time"
	"io"
	"sync"
	"sort"
	"encoding/base64"
)

var g_sBotKey = ""
var g_iPageCount = int(10)
var g_bFreqCheck = false
var zincIndexName = ""
var zincSearchURL = ""
var zincSearchUser = ""
var zincSearchPasswd = ""
var tb model.TBot
var(
	g_chatmembercount_mutex sync.RWMutex
	g_chatinfo_mutex sync.RWMutex
	g_adfeedtitle_mutex sync.RWMutex
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
		}else if line[0 : idx] == "page_count"{
			if tmp, err := strconv.Atoi(line[idx + 1:]); err == nil{
				g_iPageCount = tmp
			}
		}else if line[0: idx] == "freq_check"{
			g_bFreqCheck = line[idx + 1:] == "1"
		}else if line[0: idx] == "index_name"{
			zincIndexName = line[idx + 1:]
		}else if line[0: idx] == "zincsearch_url_prefix"{
			zincSearchURL = line[idx + 1:]
		}else if line[0: idx] == "zincsearch_user"{
			zincSearchUser = line[idx + 1:]
		}else if line[0:idx] == "zincsearch_passwd"{
			zincSearchPasswd = line[idx + 1:]
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
		if update.CallbackQuery != nil{
			go handleCallback(update.UpdateID, update.CallbackQuery)
		}else if update.Message != nil {
			go handleMessage(update.UpdateID, update.Message)
		}
	}
}

func batchGetChatMemberCount(chatids []string)map[string]int{

	mapID2Count := make(map[string]int, len(chatids))
	var wg sync.WaitGroup
	for _, v := range chatids{
		wg.Add(1)
		go func(chatid string){
			defer wg.Done()
			config := model.GetChatMemberCountConfig{}
			config.ChatID = "@" + chatid
			param, err := json.Marshal(config)
			if err != nil {
				lib.XLogErr("marshal", config)
				return
			}
			rsp, err := tb.Request("getchatmembercount", string(param))
			if err != nil{
				lib.XLogErr("getchatmembercount", chatid)
			}else{
				var rsp_obj model.IntResult
				err = json.Unmarshal([]byte(rsp), &rsp_obj)
				if err != nil{
					lib.XLogErr("unmarshal", rsp)
					return
				}
				g_chatmembercount_mutex.Lock()
				mapID2Count[chatid] = rsp_obj.Result
				g_chatmembercount_mutex.Unlock()
			}
		}(v)
	}
	wg.Wait()
	return mapID2Count
}

func batchGetChatInfo(chatids []string)map[string]model.Chat{
	mapID2Chat := make(map[string]model.Chat)
	var wg sync.WaitGroup
	for _, v := range chatids{
		wg.Add(1)
		go func(chatid string){
			defer wg.Done()
			config := model.GetChatConfig{}
			config.ChatID = "@" + chatid
			err := tb.Call(&config)
			if err != nil{
				lib.XLogErr("GetChatInfo", chatid)
			}else{
				g_chatinfo_mutex.Lock()
				mapID2Chat[chatid] = config.Response
				g_chatinfo_mutex.Unlock()
			}
		}(v)
	}
	wg.Wait()
	return mapID2Chat
}

func getAdFeeds(key string)model.AdFeedList{
	var feeds model.AdFeedList
	if err := db.GetStruct(key, &feeds); err != nil{
		lib.XLogErr("empty adfeeds", key)
	}
	return feeds
}

func fillEmtities(updateid int, keyword string, from int)(string, []model.MessageEntity){
	var entities []model.MessageEntity
	var search_chatids []string
	var ad_chatids []string
	var top_chatids []string
	mapFeedTitle := make(map[string]string)
	mapSearchTitle := make(map[string]string)

	var wg sync.WaitGroup

	var total int

	msg_content := ""

	wg.Add(1)
	go func(){
		defer wg.Done()
		m, chatids, count, err := searchIndex(updateid, keyword, from, g_iPageCount)
		if err != nil {
			lib.XLogErr("searchindex", updateid, keyword, from, g_iPageCount)
			return
		}
		total = count
		search_chatids = chatids
		mapSearchTitle = m
	}()

	wg.Add(1)
	go func(){
		defer wg.Done()
		list := getAdFeeds("zincsearch_bot_adfeeds")
		feeds := list.Feeds
		sort.SliceStable(feeds, func(i, j int) bool {
			return feeds[j].Order < feeds[i].Order
		})
		now := time.Now().Unix()
		for _, v := range feeds{
			if v.TS <= now{
				lib.XLogErr("skip expire", v)
				continue
			}
			ad_chatids = append(ad_chatids, v.ChatID)
			g_adfeedtitle_mutex.Lock()
			mapFeedTitle[v.ChatID] = v.Title
			g_adfeedtitle_mutex.Unlock()
		}
	}()

	wg.Add(1)
	go func(){
		defer wg.Done()
		key := "zincsearch_bot_topfeeds_" + base64.StdEncoding.EncodeToString([]byte(keyword))
		list := getAdFeeds(key)
		feeds := list.Feeds
		sort.SliceStable(feeds, func(i, j int) bool {
			return feeds[j].Order < feeds[i].Order
		})
		now := time.Now().Unix()
		for _, v := range feeds{
			if v.TS <= now{
				lib.XLogErr("skip expire", v)
				continue
			}
			top_chatids = append(top_chatids, v.ChatID)
			g_adfeedtitle_mutex.Lock()
			mapFeedTitle[v.ChatID] = v.Title
			g_adfeedtitle_mutex.Unlock()
		}
	}()

	wg.Wait()

	results := ad_chatids
	for _, v := range top_chatids{
		results = append(results, v)
	}
	for _, v := range search_chatids{
		results = append(results, v)
	}

	if len(search_chatids) == 0{
		lib.XLogErr("empty results", updateid, keyword)
		if from == 0{
			msg_content = "暂无搜索结果"
		}else{
			msg_content = "暂无更多结果"
		}
		return msg_content, entities
	}

	var mapID2Chat map[string]model.Chat
	wg.Add(1)
	go func(){
		defer wg.Done()
		mapID2Chat = batchGetChatInfo(results)
	}()

	mapChat2Count := make(map[string]int)
	wg.Add(1)
	go func(){
		defer wg.Done()
		mapChat2Count = batchGetChatMemberCount(results)
	}()

	wg.Wait()

	top_des := "🪧  找老师搜索引擎说明\n"
	des := model.MessageEntity{
		Type:"text_link",
		URL: "https://t.me/c/2459149934/31",
		Offset: GetUTF16Len(msg_content),
		Length: GetUTF16Len(top_des),
	}
	entities = append(entities, des)
	msg_content += top_des

	// 广告
	for _, id := range ad_chatids{
		url := model.MessageEntity{}
		url.Type = "text_link"
		url.URL = fmt.Sprintf("https://t.me/%v", id)
		url.Offset = GetUTF16Len(msg_content)
		title := "🔥 " + mapFeedTitle[id]
		url.Length = GetUTF16Len(title)
		entities = append(entities, url)
		msg_content += title + "\n"
	}
	if len(ad_chatids) == 0{
		msg_content += "🔥 推广位招租中"
	}
	// 买了搜索关键词的
	for _, id := range top_chatids{
		url := model.MessageEntity{}
		url.Type = "text_link"
		url.URL = fmt.Sprintf("https://t.me/%v", id)
		url.Offset = GetUTF16Len(msg_content)
		title := "🔝 " + mapFeedTitle[id]
		url.Length = GetUTF16Len(title)
		entities = append(entities, url)
		msg_content += title + "\n"
	}
	if len(top_chatids) == 0{
		msg_content += "🔝  关键词广告位招租中 \n"
	}
	msg_content += "\n"
	// 命中关键词的
	count := from + 1
	for _, id := range search_chatids {
		url := model.MessageEntity{}
		url.Type = "text_link"
		url.URL = fmt.Sprintf("https://t.me/%v", id)
		url.Offset = GetUTF16Len(msg_content)
		chat := mapID2Chat[id]
		origin_title := chat.Title
		if len(origin_title) == 0{
			origin_title = mapSearchTitle[id]
		}
		logo := ""
		if chat.Type == "channel"{
			logo = "📢"
		}else if chat.Type == "group" || chat.Type == "supergroup"{
			logo = "👥"
		}
		user_count := mapChat2Count[id]
		str_user_count := strconv.Itoa(user_count)
		if user_count > 1000{
			str_user_count = strconv.Itoa(user_count / 1000) + "k"
		}
		title := strconv.Itoa(count) + ". " + logo + origin_title + " - " + str_user_count +"人"

		url.Length = GetUTF16Len(title)
		entities = append(entities, url)
		msg_content += title + "\n"
		count++
	}
	totalPages := (total + g_iPageCount - 1) / g_iPageCount
	if len(search_chatids) != 0{
		msg_content += fmt.Sprintf("\n🔍 搜索结果（第 %d/%d 页）\n", from / 10 + 1, totalPages)
	}
	return msg_content, entities
}

func handleCallback(updateid int, callback *model.CallbackQuery){
	values := strings.Split(callback.Data, "$$")
	if len(values) != 3{
		lib.XLogErr("invalid callback", *callback)
		return
	}
	keyword := values[0]
	from, err := strconv.Atoi(values[2])
	if err != nil{
		lib.XLogErr("invalid pagefrom", callback.Data)
		return
	}
	if from < 0 {
		from = 0
	}

	msg_config := model.EditMessageTextConfig{ChatID:callback.Message.Chat.ID, MessageID:callback.Message.MessageID}

	msg_content, entities := fillEmtities(updateid, keyword, from)

	msg_config.Entities = entities
	msg_config.Text = msg_content
	msg_config.LinkPreviewOption.IsDisable = true

	last_page := model.InlineKeyboardButton{Text:"上一页"}
	last_text := keyword + "$$" + strconv.Itoa(g_iPageCount) + "$$" + strconv.Itoa(from - g_iPageCount)
	last_page.CallbackData = &last_text
	next_page := model.InlineKeyboardButton{Text:"下一页"}
	next_text := keyword + "$$" + strconv.Itoa(g_iPageCount) + "$$" + strconv.Itoa(from + g_iPageCount)
	next_page.CallbackData = &next_text
	var buttons []model.InlineKeyboardButton
	buttons = append(buttons, last_page, next_page)

	var markup model.InlineKeyboardMarkup
	markup.InlineKeyboard = append(markup.InlineKeyboard, buttons)

	msg_config.ReplyMarkup = markup

	tb.Call(&msg_config)
}

func handleMessage(updateid int, msg *model.Message) {
	query := strings.TrimSpace(msg.Text)
	if query == "" {
		return
	}
	sendSearchResults(updateid, msg.Chat.ID, msg.MessageID, 0, query)
}

func handleCommand(msg *model.Message) {
	switch msg.Command() {
	case "start":
		sendText(msg.Chat.ID, "欢迎使用频道搜索引擎！\n私聊使用/add添加索引")
	case "help":
		sendText(msg.Chat.ID, "命令列表：\n/add 频道ID 关键词1 关键词2... - 添加索引\n/delete 频道ID - 删除索引")
	default:
		sendText(msg.Chat.ID, "未知命令")
	}
}

// 搜索ZincSearch
func searchIndex(updateid int, query string, page int, pageSize int) (map[string]string, []string, int, error) {

	searchReq := &zincsearch.SearchRequest{
		SearchType: "match",
		Query: map[string]interface{}{
			"term": query,
		},
		MaxResults: pageSize,
		From: page,
		SortFields: []string{"-user_count"},
	}

	//lib.XLogInfo(updateid, searchReq)
	client := zincsearch.NewClient("http://localhost:4080", zincSearchUser, zincSearchPasswd)
	result, err := client.Search(zincIndexName, searchReq)
	if err != nil {
		log.Fatal(err)
	}
	//lib.XLogInfo(searchReq)
	//lib.XLogInfo(result)

	mapChatID2Title := make(map[string]string, len(result.Hits.Hits))
	var ids []string
	for _, hit := range result.Hits.Hits {
		ids = append(ids, hit.ID)
		mapChatID2Title[hit.ID] = hit.Source["title"].(string)
	}
	return mapChatID2Title, ids, result.Hits.Total.Value, nil
}


func GetUTF16Len(content string)int{
	encodeContent := utf16.Encode([]rune(content))
	return len(encodeContent)
}

func sendText(chatid int64, text string){
	config := model.SendMessageConfig{}
	config.ChatID = chatid
	config.Text = text
	tb.Call(&config)
}

func replyText(chatid int64, msgid int, text string){
	config := model.SendMessageConfig{}
	config.ChatID = chatid
	config.Text = text
	config.ReplyParams.MessageID = msgid
	tb.Call(&config)
}

// 发送搜索结果（带分页）
func sendSearchResults(updateid int, chatID int64, messageID int, page int, query string) {
	msg_config := model.SendMessageConfig{}
	msg_config.ChatID = chatID
	msg_content, entities := fillEmtities(updateid, query, page)
	msg_config.Entities = entities
	msg_config.Text = msg_content
	msg_config.LinkPreviewOption.IsDisable = true

	last_page := model.InlineKeyboardButton{Text:"⬅️上一页"}
	last_text := query + "$$" + strconv.Itoa(g_iPageCount) + "$$" + strconv.Itoa(page - g_iPageCount)
	last_page.CallbackData = &last_text
	next_page := model.InlineKeyboardButton{Text:"下一页➡️"}
	next_text := query + "$$" + strconv.Itoa(g_iPageCount) + "$$" + strconv.Itoa(page + g_iPageCount)
	next_page.CallbackData = &next_text
	var buttons []model.InlineKeyboardButton
	buttons = append(buttons, last_page, next_page)

	land_url := "tg://resolve?domain=kkhelper_bot"
	feedback := model.InlineKeyboardButton{Text:"👣反馈搜索问题|添加频道|购买推广👣", URL: &land_url}

	var markup model.InlineKeyboardMarkup
	markup.InlineKeyboard = append(markup.InlineKeyboard, buttons, []model.InlineKeyboardButton{feedback})

	msg_config.ReplyMarkup = markup
	msg_config.ReplyParams.MessageID = messageID

	tb.Call(&msg_config)
}
