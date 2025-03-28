package model

type ChatSession struct{
	Status string `json:"status"`
	ChatID int64 `json:"chat_id"`
	Type string `json:"type"`
}

type ChatSessionList struct{
	ChatList []ChatSession `json:"chat_list"`
}

type UserList struct {
	IDList []int64 `json:"id_list"`
}

type ChatBot struct{
	Token string `json:"token"`
	ID int64 `json:"id"`
}

type ChatBotDetail struct{
	Token string `json:"token"`
	ID int64 `json:"id"`
	Mode string `json:"mode"` // private group supergroup
	GroupID int64 `json:"groupid"`
}

type ChatBotList struct{
	BotList []ChatBot `json:"bot_list"`
}

type OperStatus struct {
	Status string `json:"status"`
}

type VipInfo struct{
	ID int64 `json:"id"`
	Expire int64 `json:"expire"`
}

type GroupThreadInfo struct{
	GroupID int64 `json:"groupid"`
	ThreadID int `json:"thread_id"`
}
