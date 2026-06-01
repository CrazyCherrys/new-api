package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	CanvasModeImage = "image"
	CanvasModeVideo = "video"
	CanvasModeChat  = "chat"

	CanvasMessageRoleUser      = "user"
	CanvasMessageRoleAssistant = "assistant"

	CanvasMessageStatusPlaceholder = "placeholder"
	CanvasMessageStatusGenerating  = "generating"
	CanvasMessageStatusSuccess     = "success"
	CanvasMessageStatusFailed      = "failed"
	CanvasMessageStatusStopped     = "stopped"

	CanvasTaskTypeImage = "image_generation"
	CanvasTaskTypeVideo = "video_generation"
)

type CanvasSession struct {
	Id                      int     `json:"id" gorm:"primaryKey"`
	UserId                  int     `json:"user_id" gorm:"index:idx_canvas_sessions_user_mode_deleted,priority:1;index:idx_canvas_sessions_user_updated,priority:1;not null"`
	Mode                    string  `json:"mode" gorm:"size:16;index:idx_canvas_sessions_user_mode_deleted,priority:2;not null"`
	Title                   string  `json:"title" gorm:"size:255;not null;default:''"`
	CurrentModel            string  `json:"current_model" gorm:"size:255;not null;default:''"`
	CurrentGroup            string  `json:"current_group" gorm:"size:64;not null;default:''"`
	ChatTemperature         float64 `json:"chat_temperature" gorm:"default:0.7"`
	ChatContextCount        int     `json:"chat_context_count" gorm:"default:8"`
	SystemPrompt            string  `json:"system_prompt" gorm:"type:text"`
	SummaryEnabled          bool    `json:"summary_enabled" gorm:"default:true"`
	SummaryTriggerMessages  int     `json:"summary_trigger_messages" gorm:"default:8"`
	SummaryRecentMessages   int     `json:"summary_recent_messages" gorm:"default:8"`
	SummaryPrompt           string  `json:"summary_prompt" gorm:"type:text"`
	LastSummarizedMessageId int     `json:"last_summarized_message_id" gorm:"default:0"`
	ClearContextMessageId   int     `json:"clear_context_message_id" gorm:"default:0"`
	Pinned                  bool    `json:"pinned" gorm:"index;default:false"`
	TitleManuallySet        bool    `json:"title_manually_set" gorm:"default:false"`
	CreatedTime             int64   `json:"created_time" gorm:"bigint;index"`
	UpdatedTime             int64   `json:"updated_time" gorm:"bigint;index:idx_canvas_sessions_user_updated,priority:2"`
	DeletedTime             int64   `json:"deleted_time" gorm:"bigint;index:idx_canvas_sessions_user_mode_deleted,priority:3;default:0"`
}

type CanvasMessage struct {
	Id               int    `json:"id" gorm:"primaryKey"`
	SessionId        int    `json:"session_id" gorm:"index:idx_canvas_messages_session_deleted_created,priority:1;not null"`
	UserId           int    `json:"user_id" gorm:"index;not null"`
	Mode             string `json:"mode" gorm:"size:16;index;not null"`
	Role             string `json:"role" gorm:"size:16;not null"`
	Prompt           string `json:"prompt" gorm:"type:text"`
	ReasoningContent string `json:"reasoning_content" gorm:"type:text"`
	ClientRequestId  string `json:"client_request_id" gorm:"size:64;default:''"`
	Status           string `json:"status" gorm:"size:32;index;default:''"`
	TaskId           string `json:"task_id" gorm:"size:64;index;default:''"`
	TaskType         string `json:"task_type" gorm:"size:32;index;default:''"`
	Metadata         string `json:"metadata" gorm:"type:text"`
	ErrorMessage     string `json:"error_message" gorm:"type:text"`
	CreatedTime      int64  `json:"created_time" gorm:"bigint;index:idx_canvas_messages_session_deleted_created,priority:3"`
	UpdatedTime      int64  `json:"updated_time" gorm:"bigint"`
	DeletedTime      int64  `json:"deleted_time" gorm:"bigint;index:idx_canvas_messages_session_deleted_created,priority:2;default:0"`
}

func NormalizeCanvasMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case CanvasModeImage:
		return CanvasModeImage
	case CanvasModeVideo:
		return CanvasModeVideo
	case CanvasModeChat:
		return CanvasModeChat
	default:
		return ""
	}
}

func CreateCanvasSession(session *CanvasSession) error {
	if session == nil {
		return nil
	}
	now := common.GetTimestamp()
	if session.CreatedTime == 0 {
		session.CreatedTime = now
	}
	if session.UpdatedTime == 0 {
		session.UpdatedTime = now
	}
	return DB.Select("*").Create(session).Error
}

func GetCanvasSessionByID(userId int, id int) (*CanvasSession, error) {
	var session CanvasSession
	err := DB.Where("id = ? AND user_id = ? AND deleted_time = 0", id, userId).First(&session).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func ListCanvasSessions(userId int, mode string, limit int, offset int) ([]*CanvasSession, bool, error) {
	var sessions []*CanvasSession
	query := DB.Where("user_id = ? AND deleted_time = 0", userId)
	if normalizedMode := NormalizeCanvasMode(mode); normalizedMode != "" {
		query = query.Where("mode = ?", normalizedMode)
	}
	query = query.Order("pinned DESC").Order("updated_time DESC").Order("id DESC")
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		err := query.Find(&sessions).Error
		return sessions, false, err
	}
	err := query.Offset(offset).Limit(limit + 1).Find(&sessions).Error
	if err != nil {
		return nil, false, err
	}
	hasMore := len(sessions) > limit
	if hasMore {
		sessions = sessions[:limit]
	}
	return sessions, hasMore, nil
}

func UpdateCanvasSessionFields(userId int, id int, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	updates["updated_time"] = common.GetTimestamp()
	return DB.Model(&CanvasSession{}).
		Where("id = ? AND user_id = ? AND deleted_time = 0", id, userId).
		Updates(updates).Error
}

func TouchCanvasSession(userId int, id int) error {
	now := common.GetTimestamp()
	return DB.Model(&CanvasSession{}).
		Where("id = ? AND user_id = ? AND deleted_time = 0", id, userId).
		Update("updated_time", now).Error
}

func SoftDeleteCanvasSession(userId int, id int, deletedTime int64) error {
	return DB.Model(&CanvasSession{}).
		Where("id = ? AND user_id = ? AND deleted_time = 0", id, userId).
		Updates(map[string]interface{}{
			"deleted_time": deletedTime,
			"updated_time": deletedTime,
		}).Error
}

func CreateCanvasMessage(message *CanvasMessage) error {
	if message == nil {
		return nil
	}
	now := common.GetTimestamp()
	if message.CreatedTime == 0 {
		message.CreatedTime = now
	}
	if message.UpdatedTime == 0 {
		message.UpdatedTime = now
	}
	return DB.Create(message).Error
}

func ListCanvasMessages(userId int, sessionId int) ([]*CanvasMessage, error) {
	var messages []*CanvasMessage
	err := DB.Where("user_id = ? AND session_id = ? AND deleted_time = 0", userId, sessionId).
		Order("created_time ASC").
		Order("id ASC").
		Find(&messages).Error
	return messages, err
}

func ListCanvasMessagesByModePage(userId int, sessionId int, mode string, limit int, beforeCreatedTime int64, beforeID int) ([]*CanvasMessage, bool, error) {
	var messages []*CanvasMessage
	query := DB.Where("user_id = ? AND session_id = ? AND deleted_time = 0", userId, sessionId)
	if normalizedMode := NormalizeCanvasMode(mode); normalizedMode != "" {
		query = query.Where("mode = ?", normalizedMode)
	}
	if beforeID > 0 {
		query = query.Where("(created_time < ?) OR (created_time = ? AND id < ?)", beforeCreatedTime, beforeCreatedTime, beforeID)
	}
	query = query.Order("created_time DESC").Order("id DESC")
	if limit <= 0 {
		err := query.Find(&messages).Error
		return messages, false, err
	}
	err := query.Limit(limit + 1).Find(&messages).Error
	if err != nil {
		return nil, false, err
	}
	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}
	return messages, hasMore, nil
}

func GetCanvasMessageBySessionAndID(userId int, sessionId int, id int) (*CanvasMessage, error) {
	var message CanvasMessage
	err := DB.Where("id = ? AND session_id = ? AND user_id = ? AND deleted_time = 0", id, sessionId, userId).
		First(&message).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &message, nil
}

func CountCanvasMessages(userId int, sessionId int) (int64, error) {
	var count int64
	err := DB.Model(&CanvasMessage{}).
		Where("user_id = ? AND session_id = ? AND deleted_time = 0", userId, sessionId).
		Count(&count).Error
	return count, err
}

func UpdateCanvasMessageFields(userId int, id int, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	updates["updated_time"] = common.GetTimestamp()
	return DB.Model(&CanvasMessage{}).
		Where("id = ? AND user_id = ? AND deleted_time = 0", id, userId).
		Updates(updates).Error
}

func SoftDeleteCanvasMessages(userId int, sessionId int, deletedTime int64) error {
	return DB.Model(&CanvasMessage{}).
		Where("user_id = ? AND session_id = ? AND deleted_time = 0", userId, sessionId).
		Updates(map[string]interface{}{
			"deleted_time": deletedTime,
			"updated_time": deletedTime,
		}).Error
}
