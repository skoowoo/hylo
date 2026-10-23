// Package wechat implements the WeChat iLink Bot HTTP protocol.
// Adapted from https://github.com/formulahendry/wechat-acp (TypeScript).
package wechat

const ChannelVersion = "1.0.2"

const (
	MessageTypeNone = 0
	MessageTypeUser = 1
	MessageTypeBot  = 2
)

const (
	MessageItemTypeNone  = 0
	MessageItemTypeText  = 1
	MessageItemTypeImage = 2
	MessageItemTypeVoice = 3
	MessageItemTypeFile  = 4
	MessageItemTypeVideo = 5
)

const (
	MessageStateNew        = 0
	MessageStateGenerating = 1
	MessageStateFinish     = 2
)

const (
	TypingStatusTyping = 1
	TypingStatusCancel = 2
)

// BaseInfo is sent with every POST body.
type BaseInfo struct {
	ChannelVersion string `json:"channel_version,omitempty"`
}

// TextItem holds plain text content.
type TextItem struct {
	Text string `json:"text,omitempty"`
}

// RefMessage is a quoted reply reference.
type RefMessage struct {
	MessageItem *MessageItem `json:"message_item,omitempty"`
	Title       string       `json:"title,omitempty"`
}

// CDNMedia is the encrypted blob behind a WeChat image.
type CDNMedia struct {
	EncryptQueryParam string `json:"encrypt_query_param,omitempty"`
	AESKey            string `json:"aes_key,omitempty"`
}

// ImageItem is a type-2 image. AESKey, when set, is 32 hex characters.
type ImageItem struct {
	Media  *CDNMedia `json:"media,omitempty"`
	AESKey string    `json:"aeskey,omitempty"`
}

// MessageItem is one content block inside a WeChat message.
type MessageItem struct {
	Type      int         `json:"type,omitempty"`
	TextItem  *TextItem   `json:"text_item,omitempty"`
	ImageItem *ImageItem  `json:"image_item,omitempty"`
	VoiceItem *VoiceItem  `json:"voice_item,omitempty"`
	RefMsg    *RefMessage `json:"ref_msg,omitempty"`
}

// VoiceItem may carry a server-side transcription in Text.
type VoiceItem struct {
	Text string `json:"text,omitempty"`
}

// Message is one WeChat iLink message envelope.
type Message struct {
	FromUserID   string        `json:"from_user_id,omitempty"`
	ToUserID     string        `json:"to_user_id,omitempty"`
	ClientID     string        `json:"client_id,omitempty"`
	GroupID      string        `json:"group_id,omitempty"`
	MessageType  int           `json:"message_type,omitempty"`
	MessageState int           `json:"message_state,omitempty"`
	ItemList     []MessageItem `json:"item_list,omitempty"`
	ContextToken string        `json:"context_token,omitempty"`
}

// GetUpdatesResp is the long-poll response.
type GetUpdatesResp struct {
	Ret                  int       `json:"ret,omitempty"`
	ErrCode              int       `json:"errcode,omitempty"`
	ErrMsg               string    `json:"errmsg,omitempty"`
	Msgs                 []Message `json:"msgs,omitempty"`
	GetUpdatesBuf        string    `json:"get_updates_buf,omitempty"`
	LongPollingTimeoutMs int       `json:"longpolling_timeout_ms,omitempty"`
}

// SendMessageReq wraps an outbound message.
type SendMessageReq struct {
	Msg *Message `json:"msg,omitempty"`
}

// SendTypingReq controls the typing indicator.
type SendTypingReq struct {
	ILinkUserID  string `json:"ilink_user_id"`
	TypingTicket string `json:"typing_ticket"`
	Status       int    `json:"status"`
}

// GetConfigResp returns session config such as typing ticket.
type GetConfigResp struct {
	TypingTicket string `json:"typing_ticket,omitempty"`
}

// QrcodeResp is returned by get_bot_qrcode.
type QrcodeResp struct {
	Qrcode           string `json:"qrcode"`
	QrcodeImgContent string `json:"qrcode_img_content"`
}

// QrcodeStatusResp is returned by get_qrcode_status.
type QrcodeStatusResp struct {
	Status      string `json:"status"`
	BotToken    string `json:"bot_token,omitempty"`
	BaseURL     string `json:"baseurl,omitempty"`
	ILinkBotID  string `json:"ilink_bot_id,omitempty"`
	ILinkUserID string `json:"ilink_user_id,omitempty"`
}

// TokenData is persisted after QR login.
type TokenData struct {
	Token     string `json:"token"`
	BaseURL   string `json:"baseUrl"`
	AccountID string `json:"accountId"`
	UserID    string `json:"userId"`
	SavedAt   string `json:"savedAt"`
}
