package model

type CanvasAudioTask struct {
	ID              string `json:"id" gorm:"primaryKey"`
	UserID          string `json:"userId"`
	UserDisplayName string `json:"userDisplayName"`
	Source          string `json:"source"`
	SourceID        string `json:"sourceId"`
	NodeID          string `json:"nodeId"`
	Model           string `json:"model"`
	ChannelID       string `json:"channelId"`
	UserChannelID   string `json:"userChannelId"`
	ChannelName     string `json:"channelName"`
	Status          string `json:"status"`
	Progress        int    `json:"progress"`
	Prompt          string `json:"prompt" gorm:"type:text"`
	Endpoint        string `json:"endpoint"`
	ContentType     string `json:"contentType"`
	RequestBody     string `json:"requestBody" gorm:"type:text"`
	ResponseBody    string `json:"responseBody" gorm:"type:text"`
	Error           string `json:"error" gorm:"type:text"`
	ErrorDetail     string `json:"errorDetail" gorm:"type:text"`
	AudioURL        string `json:"audioUrl" gorm:"type:text"`
	StorageKey      string `json:"storageKey"`
	MimeType        string `json:"mimeType"`
	Bytes           int64  `json:"bytes"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
	StartedAt       string `json:"startedAt"`
	CompletedAt     string `json:"completedAt"`
}
