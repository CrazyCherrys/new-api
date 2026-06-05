package model

import (
	"fmt"
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
	Id                      int     `json:"id" gorm:"primaryKey;index:idx_canvas_sessions_user_deleted_pinned_updated,priority:5;index:idx_canvas_sessions_user_mode_deleted_pinned_updated,priority:6"`
	UserId                  int     `json:"user_id" gorm:"index:idx_canvas_sessions_user_deleted_pinned_updated,priority:1;index:idx_canvas_sessions_user_mode_deleted_pinned_updated,priority:1;not null"`
	Mode                    string  `json:"mode" gorm:"size:16;index:idx_canvas_sessions_user_mode_deleted_pinned_updated,priority:2;not null"`
	Title                   string  `json:"title" gorm:"size:255;not null;default:''"`
	CurrentModel            string  `json:"current_model" gorm:"size:255;not null;default:''"`
	CurrentGroup            string  `json:"current_group" gorm:"size:64;not null;default:''"`
	ChatTemperature         float64 `json:"chat_temperature" gorm:"default:0.7"`
	ChatContextCount        int     `json:"chat_context_count" gorm:"default:8"`
	WebSearchEnabled        bool    `json:"web_search_enabled" gorm:"default:false"`
	SystemPrompt            string  `json:"system_prompt" gorm:"type:text"`
	SummaryEnabled          bool    `json:"summary_enabled" gorm:"default:true"`
	SummaryTriggerMessages  int     `json:"summary_trigger_messages" gorm:"default:8"`
	SummaryRecentMessages   int     `json:"summary_recent_messages" gorm:"default:8"`
	SummaryPrompt           string  `json:"summary_prompt" gorm:"type:text"`
	LastSummarizedMessageId int     `json:"last_summarized_message_id" gorm:"default:0"`
	ClearContextMessageId   int     `json:"clear_context_message_id" gorm:"default:0"`
	Pinned                  bool    `json:"pinned" gorm:"index:idx_canvas_sessions_user_deleted_pinned_updated,priority:3;index:idx_canvas_sessions_user_mode_deleted_pinned_updated,priority:4;default:false"`
	TitleManuallySet        bool    `json:"title_manually_set" gorm:"default:false"`
	CreatedTime             int64   `json:"created_time" gorm:"bigint;index"`
	UpdatedTime             int64   `json:"updated_time" gorm:"bigint;index:idx_canvas_sessions_user_deleted_pinned_updated,priority:4;index:idx_canvas_sessions_user_mode_deleted_pinned_updated,priority:5"`
	DeletedTime             int64   `json:"deleted_time" gorm:"bigint;index:idx_canvas_sessions_user_deleted_pinned_updated,priority:2;index:idx_canvas_sessions_user_mode_deleted_pinned_updated,priority:3;default:0"`
}

type CanvasMessage struct {
	Id               int    `json:"id" gorm:"primaryKey;index:idx_canvas_messages_session_deleted_created,priority:4;index:idx_canvas_messages_session_status_deleted_created,priority:5"`
	SessionId        int    `json:"session_id" gorm:"index:idx_canvas_messages_session_deleted_created,priority:1;index:idx_canvas_messages_session_status_deleted_created,priority:1;not null"`
	UserId           int    `json:"user_id" gorm:"index;index:idx_canvas_messages_task_lookup,priority:1;not null"`
	Mode             string `json:"mode" gorm:"size:16;index;index:idx_canvas_messages_task_lookup,priority:2;not null"`
	Role             string `json:"role" gorm:"size:16;not null"`
	Prompt           string `json:"prompt" gorm:"type:text"`
	ReasoningContent string `json:"reasoning_content" gorm:"type:text"`
	ClientRequestId  string `json:"client_request_id" gorm:"size:64;default:''"`
	Status           string `json:"status" gorm:"size:32;index;index:idx_canvas_messages_session_status_deleted_created,priority:2;default:''"`
	TaskId           string `json:"task_id" gorm:"size:64;index;index:idx_canvas_messages_task_lookup,priority:4;default:''"`
	TaskType         string `json:"task_type" gorm:"size:32;index;index:idx_canvas_messages_task_lookup,priority:3;default:''"`
	Metadata         string `json:"metadata" gorm:"type:text"`
	ErrorMessage     string `json:"error_message" gorm:"type:text"`
	CreatedTime      int64  `json:"created_time" gorm:"bigint;index:idx_canvas_messages_session_deleted_created,priority:3;index:idx_canvas_messages_session_status_deleted_created,priority:4"`
	UpdatedTime      int64  `json:"updated_time" gorm:"bigint"`
	DeletedTime      int64  `json:"deleted_time" gorm:"bigint;index:idx_canvas_messages_session_deleted_created,priority:2;index:idx_canvas_messages_session_status_deleted_created,priority:3;index:idx_canvas_messages_task_lookup,priority:5;default:0"`
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
	return SoftDeleteCanvasSessionWithDB(DB, userId, id, deletedTime)
}

func SoftDeleteCanvasSessionWithDB(db *gorm.DB, userId int, id int, deletedTime int64) error {
	if db == nil {
		db = DB
	}
	return db.Model(&CanvasSession{}).
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
	return CreateCanvasMessageForMode(message.Mode, message)
}

func ListCanvasMessages(userId int, sessionId int) ([]*CanvasMessage, error) {
	mode, err := getCanvasMessageModeBySession(userId, sessionId)
	if err != nil {
		return nil, err
	}
	return ListCanvasMessagesForMode(mode, userId, sessionId)
}

func ListCanvasMessagesByModePage(userId int, sessionId int, mode string, limit int, beforeCreatedTime int64, beforeID int) ([]*CanvasMessage, bool, error) {
	return ListCanvasMessagesPageForMode(mode, userId, sessionId, limit, beforeCreatedTime, beforeID)
}

func GetCanvasMessageBySessionAndID(userId int, sessionId int, id int) (*CanvasMessage, error) {
	mode, err := getCanvasMessageModeBySession(userId, sessionId)
	if err != nil {
		return nil, err
	}
	return GetCanvasMessageByModeSessionAndID(mode, userId, sessionId, id)
}

func CountCanvasMessages(userId int, sessionId int) (int64, error) {
	mode, err := getCanvasMessageModeBySession(userId, sessionId)
	if err != nil {
		return 0, err
	}
	return CountCanvasMessagesByMode(mode, userId, sessionId)
}

func UpdateCanvasMessageFields(userId int, id int, updates map[string]interface{}) error {
	if !CanvasUsesDedicatedMessageDBs() {
		if len(updates) == 0 {
			return nil
		}
		updates["updated_time"] = common.GetTimestamp()
		return DB.Model(&CanvasMessage{}).
			Where("id = ? AND user_id = ? AND deleted_time = 0", id, userId).
			Updates(updates).Error
	}
	return fmt.Errorf("canvas message mode is required when dedicated canvas databases are enabled")
}

func SoftDeleteCanvasMessages(userId int, sessionId int, deletedTime int64) error {
	mode, err := getCanvasMessageModeBySession(userId, sessionId)
	if err != nil {
		return err
	}
	return SoftDeleteCanvasMessagesByMode(mode, userId, sessionId, deletedTime)
}

func getCanvasMessageModeBySession(userId int, sessionId int) (string, error) {
	session, err := GetCanvasSessionByID(userId, sessionId)
	if err != nil {
		return "", err
	}
	if session == nil {
		return "", fmt.Errorf("canvas session not found")
	}
	return session.Mode, nil
}
