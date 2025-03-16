package model

type UpdateConfig struct {
	Offset int `json:"offset"`
	Limit int64 `json:"limit"`
	Timeout int64 `json:"timeout"`
	AllowUpdates []string `json:"allowed_updates"`

	Response []Update `json:"result,omitempty"`
}

type GetMeConfig struct {
	Response User `json:"result,omitempty"`
}

type GetChatAdministratorsConfig struct {
	ChatID int64 `json:"chat_id"`
	Response []ChatMember `json:"result,omitempty`
}

type InputMedia struct {
	Type string `json:"type"`
	Media string `json:"media"`
	Caption string `json:"caption"`
	ParseMode string `json:"parse_mode"`
	HasSpoiler bool `json:"has_spoiler"`
	CaptionEmtities []MessageEntity `json:"caption_entities,omitempty"`
}

type BoolConfig struct {
}

type BoolResult struct {
	OK bool `json:"ok"`
	Result bool `json:"result"`
}

type IntResult struct{
	OK bool `json:"ok"`
	Result int `json:"result"`
}

type BanChatMemberConfig struct {
	BoolConfig
	ChatID int64 `json:"chat_id"`
	UserID int64 `json:"user_id"`
	RevokeMessages bool `json:"revoke_messages"`
}

type GetChatMemberCountConfig struct{
	ChatID any `json:"chat_id"`
}

type UnbanChatMemberConfig struct {
	BoolConfig
	ChatID int64 `json:"chat_id"`
	UserID int64 `json:"user_id"`
	OnlyIfBan bool `json:"only_if_banned"`
}

type SendMediaGroupConfig struct {
	ChatID int64 `json:"chat_id"`
	Media []InputMedia `json:"media"`

	Response []Message `json:"result,omitempty"`
}

type SendPhotoConfig struct {
	ChatID int64 `json:"chat_id"`
	Photo string `json:"photo"`
	Caption string `json:"caption"`
	Video string `json:"video"`
}

type SendDocumentConfig struct {
	ChatID int64 `json:"chat_id"`
	Document string `json:"document"`
	Caption string `json:"caption"`
}

type SendAnimationConfig struct {
	ChatID int64 `json:"chat_id"`
	Animation string `json:"animation"`
	Caption string `json:"caption"`
}

type SendContactConfig struct {
	ChatID int64 `json:"chat_id"`
	PhoneNumber string `json:"phone_number"`
	FirstName string `json:"first_name"`
}

type SendPollConfig struct {
	ChatID int64 `json:"chat_id"`
	Question string `json:"question"`
	Options []string `json:"options"`
}

type SendChatActionConfig struct {
	ChatID int64 `json:"chat_id"`
	Action string `json:"action"`
}

type SetChatPermissionsConfig struct {
	ChatID int64 `json:"chat_id"`
	Permissions ChatPermissions `json:"permissions"`
}

type RestrictChatMemberConfig struct {
	BoolConfig
	ChatID any `json:"chat_id"`
	UserID int64 `json:"user_id"`
	Permissions ChatPermissions `json:"permissions"`

	Response bool `json:"result,omitempty"`
}

type PromoteChatMemberConfig struct {
	BoolConfig
	ChatID int64 `json:"chat_id"`
	UserID int64 `json:"user_id"`
	CanPinMessages bool `json:"can_pin_messages"`
	CanDeleteMessages bool `json:"can_delete_messages"`
	CanEditMessages bool `json:"can_edit_messages"`
}

type ApproveChatJoinRequestConfig struct {
	BoolConfig
	ChatID int64 `json:"chat_id"`
	UserID int64 `json:"user_id"`
}

type DeclineChatJoinRequestConfig struct {
	BoolConfig
	ChatID int64 `json:"chat_id"`
	UserID int64 `json:"user_id"`
}

type ExportInviteLinkConfig struct {
	ChatID int64 `json:"chat_id"`
}

type ExportInviteLinkRsp struct {
	OK bool `json:"ok"`
	Result string `json:"result"`
}

type CreateChatInviteLinkConfig struct {
	ChatID int64 `json:"chat_id"`
	Name string `json:"name"`
}

type CreateChatInviteLinkRsp struct {
	OK bool `json:"ok"`
	Result ChatInviteLink `json:"result"`
}

type SetChatTitleConfig struct {
	ChatID int64 `json:"chat_id"`
	Title string `json:"title"`
}

type SetChatTitleRsp struct {
	OK bool `json:"ok"`
	Result bool `json:"Result"`
}

type SetChatDescriptionConfig struct {
	ChatID int64 `json:"chat_id"`
	Desc string `json:"description"`
}

type PinChatMessageConfig struct {
	ChatID int64 `json:"chat_id"`
	MessageID int `json:"message_id"`
}

type CreateForumTopicConfig struct {
	ChatID int64 `json:"chat_id"`
	Name string `json:"name"`
}

type CreateForumTopicResult struct {
	MessageThreadId int `json:"message_thread_id"`
	Name string `json:"name"`
}

type CreateForumTopicRsp struct {
	OK bool `json:"ok"`
	Result CreateForumTopicResult `json:"result"`
}

type CloseForumTopicConfig struct {
	ChatID int64 `json:"chat_id"`
	MessageThreadId int `json:"message_thread_id"`
}

type EditGeneralForumTopicConfig struct {
	ChatID int64 `json:"chat_id"`
	Name string `json:"name"`
}

type AnswerCallbackQueryConfig struct {
	CallbackID string `json:"callback_query_id"`
	Text string `json:"text"`
	ShowAlert bool `json:"show_alert"`
	Url string `json:"url"`
	CacheTime int64 `json:"cache_time"`

	Response bool `json:"result,omitempty"`
}

type DeleteMessageConfig struct {
	ChatID int64 `json:"chat_id"`
	MessageID int `json:"message_id"`

	Response bool `json:"result"`
}

type GetChatConfig struct {
	ChatID any `json:"chat_id,omitempty"`

	Response Chat `json:"result,omitempty"`
}

type SendMessageConfig struct {
	ChatID any `json:"chat_id,omitempty"`
	ThreadID int64 `json:"message_thread_id,omitempty"`
	Text string `json:"text,omitempty"`
	ParseMode string `json:"parse_mode,omitempty"`
	Entities []MessageEntity `json:"entities,omitempty"`
	LinkPreviewOption LinkPreviewOptions `json:"link_preview_options"`
	DisableNotify bool `json:"disable_notification,omitempty"`
	ProtectContent bool `json:"protect_content,omitempty"`
	ReplyParams ReplyParameters `json:"reply_parameters,omitempty"`
	ReplyMarkup any `json:"reply_markup,omitempty"`

	Response Message `json:"result,omitempty"`
}

type EditMessageTextConfig struct {
	ChatID any `json:"chat_id,omitempty"`
	MessageID int `json:"message_id,omitempty"`
	InlineMessageID string `json:"inline_message_id,omitempty"`
	Text string `json:"text,omitempty"`
	ParseMode string `json:"parse_mode,omitempty"`
	Entities []MessageEntity `json:"entities,omitempty"`
	LinkPreviewOption LinkPreviewOptions `json:"link_preview_options"`
	ReplyMarkup InlineKeyboardMarkup `json:"reply_markup,omitempty"`

	Response Message `json:"result,omitempty"`
}

type ForwardMessageConfig struct{
	ChatID any `json:"chat_id,omitempty"`
	FromChatID any `json:"from_chat_id,omitempty"`
	MessageID int `json:"message_id,omitempty"`

	Response Message `json:"result,omitempty"`
}

type AdFeed struct {
	Title string `json:"title,omitempty"`
	ChatID string `json:"chat_id,omitempty"`
	Order int `json:"order,omitempty"`
	TS int64 `json:"ts,omitempty"`
}

type AdFeedList struct{
	Feeds []AdFeed `json:"feeds,omitempty"`
}

type BotKey struct{
	Key string `json:"key"`
	RetryTime int64 `json:"retry_time"`
}

type BotKeyList struct{
	List BotKey `json:"list"`
}

type JsFeed struct{
	Name string `json:"name"`
	Location string `json:"location"`
	Price []string `json:"price"`
	Tags []string `json:"tags"`
	UserName string `json:"username"`
	ChannelUserName string `json:"channel_username"`
	ChatID int64 `json:"chatid"`
	MessageID int `json:"messageid"`
	YuniID string `json:"yuniid"`
	WechatID string `json:"wechatid"`
	QQ string `json:"qq"`
}

type JsFeedList struct{
	List []JsFeed `json:"list"`
}

type JsIndex struct {
	List []string `json:"list"`
}

type JsReport struct{
	Ly string `json:"ly"`
	Js string `json:"js"`
	Time string `json:"time"`
	UserName string `json:"username"`
	Location string `json:"location"`
	Price string `json:"price"`
	Mark string `json:"mark"`
	Yanzhi string `json:"yanzhi"`
	Taidu string `json:"taidu"`
	FreeTalk string `json:"freetalk"`
	ExtraItem string `json:"extra_item"`
	GroupUserName string `json:"group_username"`
	MessageID int `json:"message_id"`
}

type JsReportIndex struct{
	Keys []string `json:"keys"`
}
