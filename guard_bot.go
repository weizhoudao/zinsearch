package main

import (
	"regexp"
	"zincsearch/lib"
	"strings"
	"zincsearch/model"
	"os"
	"unicode/utf16"
	"bufio"
	"io"
)

type DetectionResult struct {
	IsSpam     bool
	Reasons    []string
	MatchedParts []string
}

var (
	g_promotion_path = ""
	g_drainage_path = ""
	g_profanity_path = ""
	g_str_adminuser = ""
	g_str_botkey = ""

	tb model.TBot

	urlRegex = regexp.MustCompile(`(?i)(http|https|ftp)://[^\s/$.?#].[^\s]*|([a-z0-9-]+\.)+[a-z]{2,}/?`)
	shortUrlRegex = regexp.MustCompile(`(?i)(bit\.ly|t\.co|goo\.gl|tinyurl\.com|j.mp|ow.ly|is.gd|buff.ly|adf\.ly)/\S+`)
	telegramUserRegex = regexp.MustCompile(`(?i)(@|＠)[a-z0-9_]{5,32}\b`)
	// 推广关键词
	promotionKeywords = []string{
		"优惠", "折扣", "免费领取", "限时抢购", "大促", "特价", 
		"v信", "威信", "微芯", "加微信", "客服电话", "立即购买",
		"低价", "省钱", "抢购", "促销", "清仓",
	}
	// 引流关键词
	drainageKeywords = []string{
		"加群", "QQ群", "入群", "关注公众号", "扫码加入",
		"点击咨询", "联系客服", "私信", "添加好友", "领红包",
		"福利群", "抖音关注", "快手关注", "关注获取","日结",
	}
	// 粗口词库（示例部分词汇，实际需要更完整的词库）
	profanityWords = []string{
		"傻逼", "混蛋", "他妈的", "fuck", "shit", "bitch", 
		"妈的", "去死", "屌丝", "操你", "日你", "王八蛋",
	}
)

func GetUTF16Len(content string)int{
	encodeContent := utf16.Encode([]rune(content))
	return len(encodeContent)
}

func DetectSpamMessage(message string) DetectionResult {
	result := DetectionResult{}
	// 检测网址
	if urls := detectUrls(message); len(urls) > 0 {
		result.Reasons = append(result.Reasons, "发送链接")
		result.MatchedParts = append(result.MatchedParts, urls...)
	}
	// 检测推广信息
	if matches := detectKeywords(message, promotionKeywords); len(matches) > 0 {
		result.Reasons = append(result.Reasons, "发送推广信息")
		result.MatchedParts = append(result.MatchedParts, matches...)
	}
	// 检测粗口
	if matches := detectProfanity(message); len(matches) > 0 {
		result.Reasons = append(result.Reasons, "发送粗口")
		result.MatchedParts = append(result.MatchedParts, matches...)
	}
	// 检测引流信息
	if matches := detectKeywords(message, drainageKeywords); len(matches) > 0 {
		result.Reasons = append(result.Reasons, "发送引流信息")
		result.MatchedParts = append(result.MatchedParts, matches...)
	}
	// Telegram用户检测
	if matches := detectTelegramUsers(message); len(matches) > 0 {
		result.Reasons = append(result.Reasons, "提及用户")
		result.MatchedParts = append(result.MatchedParts, matches...)
	}
	result.IsSpam = len(result.Reasons) > 0
	return result
}

// 检测Telegram用户函数
func detectTelegramUsers(message string) []string {
	// 提取所有匹配项并去重
	matches := telegramUserRegex.FindAllString(message, -1)
	cleanMatches := make([]string, 0)

	for _, m := range matches {
		// 统一转换为小写并去除特殊@符号
		normalized := strings.ToLower(strings.TrimPrefix(m, "＠"))
		cleanMatches = append(cleanMatches, "@"+normalized[1:])
	}
	return unique(cleanMatches)
}

func detectUrls(message string) []string {
	var urls []string
	// 检测普通网址
	urls = append(urls, urlRegex.FindAllString(message, -1)...)
	// 检测短网址
	urls = append(urls, shortUrlRegex.FindAllString(message, -1)...)
	return unique(urls)
}

func detectKeywords(message string, keywords []string) []string {
	var matches []string
	lowerMsg := strings.ToLower(message)
	for _, kw := range keywords {
		if strings.Contains(lowerMsg, strings.ToLower(kw)) {
			matches = append(matches, kw)
		}
	}
	return matches
}

func detectProfanity(message string) []string {
	var matches []string
	lowerMsg := strings.ToLower(message)
	for _, word := range profanityWords {
		if strings.Contains(lowerMsg, strings.ToLower(word)) {
			matches = append(matches, word)
		}
	}
	return matches
}

func unique(input []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range input {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func File2Dice(path string)[]string{
	var dice []string
	file, err := os.Open(path)
	if err != nil{
		lib.XLogErr("Open", path, err)
		return dice
	}
	defer file.Close()

	br := bufio.NewReader(file)
	for{
		a, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}
		line := strings.TrimSpace(string(a))
		dice = append(dice, line)
	}
	return dice
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
		key := line[0 : idx]
		value := line[idx + 1 :]
		if key == "key" {
			g_str_botkey = value
		}else if key == "admin"{
			g_str_adminuser = value 
		}else if key == "promotion_path"{
			g_promotion_path = value
		}else if key == "drainage_path"{
			g_drainage_path = value
		}else if key == "profanity_path"{
			g_profanity_path = value
		}
	}
}

func LoadDirtyWord(){
	if len(g_promotion_path) > 0{
		promotionKeywords = File2Dice(g_promotion_path)
	}
	if len(g_drainage_path) > 0{
		drainageKeywords = File2Dice(g_drainage_path)
	}
	if len(g_profanity_path) > 0{
		profanityWords = File2Dice(g_profanity_path)
	}
}

func CheckMessage(msg *model.Message){
	text := msg.Text
	if len(text) == 0{
		text = msg.Caption
	}
	result := DetectSpamMessage(text)
	if result.IsSpam{
		lib.XLogErr("spam", "reason", result.Reasons, "matchword", result.MatchedParts)
		delmsg := model.DeleteMessageConfig{
			ChatID:msg.Chat.ID,
			MessageID:msg.MessageID,
		}
		if err := tb.Call(&delmsg); err != nil{
			lib.XLogErr("DeleteMessage", delmsg, err)
			// send admin
		}else{
			lib.XLogInfo("delete message, content", text, "from", msg.From.UserName, "userid", msg.From.ID)
		}
		config := model.RestrictChatMemberConfig{
			ChatID:msg.Chat.ID,
			UserID:msg.From.ID,
			Permissions:model.ChatPermissions{
				CanSendMessages:false,
				CanSendMediaMessages:false,
				CanSendPolls:false,
				CanSendOtherMessages:false,
				CanAddWebPagePreviews:false,
				CanChangeInfo:false,
				CanInviteUsers:false,
				CanPinMessages:false,
			},
		}
		if err := tb.Call(&config); err != nil{
			lib.XLogErr("RestrictChatMember", config, err)
			return
		}else{
			lib.XLogInfo("RestrictChatMember", "user", msg.From.UserName, "userid", msg.From.ID)
		}

		at := model.MessageEntity{
			Type:"mention",
			Offset: 0,
			Length: GetUTF16Len("@" + msg.From.UserName),
		}
		emtities := []model.MessageEntity{at}
		msgcontent := "@" + msg.From.UserName + " "
		msgcontent += "本群不允许发送推广信息、粗口，如果误封请联系管理员解封"

		sendmsg := model.SendMessageConfig{
			ChatID:msg.Chat.ID,
			Text:msgcontent,
			Entities:emtities,
		}
		if err := tb.Call(&sendmsg); err != nil{
			lib.XLogErr("sendmsg", sendmsg, err)
		}
	}
}

func main() {
	InitConfig()
	LoadDirtyWord()

	tb.BotKey = g_str_botkey

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
			if update.Message != nil{
				if update.Message.IsCommand(){
					continue
				}
				if update.Message.From.UserName == "GroupAnonymousBot" || update.Message.From.FirstName == "Telegram"{
					continue
				}
				if len(update.Message.Text) > 0 || len(update.Message.Caption) > 0{
					go CheckMessage(update.Message)
				}
			}
		}
	}
}
