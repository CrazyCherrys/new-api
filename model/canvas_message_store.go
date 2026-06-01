package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	canvasChatMessagesTable  = "canvas_chat_messages"
	canvasImageMessagesTable = "canvas_image_messages"
	canvasVideoMessagesTable = "canvas_video_messages"
)

type CanvasChatMessage struct {
	Id               int    `gorm:"primaryKey"`
	SessionId        int    `gorm:"index:idx_canvas_chat_messages_session_deleted_created,priority:1;not null"`
	UserId           int    `gorm:"index;not null"`
	Mode             string `gorm:"size:16;index;not null"`
	Role             string `gorm:"size:16;not null"`
	Prompt           string `gorm:"type:text"`
	ReasoningContent string `gorm:"type:text"`
	ClientRequestId  string `gorm:"size:64;default:''"`
	Status           string `gorm:"size:32;index;default:''"`
	TaskId           string `gorm:"size:64;index;default:''"`
	TaskType         string `gorm:"size:32;index;default:''"`
	Metadata         string `gorm:"type:text"`
	ErrorMessage     string `gorm:"type:text"`
	CreatedTime      int64  `gorm:"bigint;index:idx_canvas_chat_messages_session_deleted_created,priority:3"`
	UpdatedTime      int64  `gorm:"bigint"`
	DeletedTime      int64  `gorm:"bigint;index:idx_canvas_chat_messages_session_deleted_created,priority:2;default:0"`
}

func (CanvasChatMessage) TableName() string {
	return canvasChatMessagesTable
}

type CanvasImageMessage struct {
	Id               int    `gorm:"primaryKey"`
	SessionId        int    `gorm:"index:idx_canvas_image_messages_session_deleted_created,priority:1;not null"`
	UserId           int    `gorm:"index;not null"`
	Mode             string `gorm:"size:16;index;not null"`
	Role             string `gorm:"size:16;not null"`
	Prompt           string `gorm:"type:text"`
	ReasoningContent string `gorm:"type:text"`
	ClientRequestId  string `gorm:"size:64;default:''"`
	Status           string `gorm:"size:32;index;default:''"`
	TaskId           string `gorm:"size:64;index;default:''"`
	TaskType         string `gorm:"size:32;index;default:''"`
	Metadata         string `gorm:"type:text"`
	ErrorMessage     string `gorm:"type:text"`
	CreatedTime      int64  `gorm:"bigint;index:idx_canvas_image_messages_session_deleted_created,priority:3"`
	UpdatedTime      int64  `gorm:"bigint"`
	DeletedTime      int64  `gorm:"bigint;index:idx_canvas_image_messages_session_deleted_created,priority:2;default:0"`
}

func (CanvasImageMessage) TableName() string {
	return canvasImageMessagesTable
}

type CanvasVideoMessage struct {
	Id               int    `gorm:"primaryKey"`
	SessionId        int    `gorm:"index:idx_canvas_video_messages_session_deleted_created,priority:1;not null"`
	UserId           int    `gorm:"index;not null"`
	Mode             string `gorm:"size:16;index;not null"`
	Role             string `gorm:"size:16;not null"`
	Prompt           string `gorm:"type:text"`
	ReasoningContent string `gorm:"type:text"`
	ClientRequestId  string `gorm:"size:64;default:''"`
	Status           string `gorm:"size:32;index;default:''"`
	TaskId           string `gorm:"size:64;index;default:''"`
	TaskType         string `gorm:"size:32;index;default:''"`
	Metadata         string `gorm:"type:text"`
	ErrorMessage     string `gorm:"type:text"`
	CreatedTime      int64  `gorm:"bigint;index:idx_canvas_video_messages_session_deleted_created,priority:3"`
	UpdatedTime      int64  `gorm:"bigint"`
	DeletedTime      int64  `gorm:"bigint;index:idx_canvas_video_messages_session_deleted_created,priority:2;default:0"`
}

func (CanvasVideoMessage) TableName() string {
	return canvasVideoMessagesTable
}

type canvasMessageStore struct {
	db        *gorm.DB
	tableName string
	mode      string
}

func canvasMessageStoreForMode(mode string) (*canvasMessageStore, error) {
	mode = NormalizeCanvasMode(mode)
	if mode == "" {
		return nil, fmt.Errorf("invalid canvas mode")
	}
	if available, reason := CanvasAvailabilityStatus(); !available {
		if reason == "" {
			reason = "canvas is unavailable"
		}
		return nil, fmt.Errorf("%s", reason)
	}
	if !CanvasUsesDedicatedMessageDBs() {
		if DB == nil {
			return nil, fmt.Errorf("canvas main database is not initialized")
		}
		return &canvasMessageStore{
			db:        DB,
			tableName: "canvas_messages",
			mode:      mode,
		}, nil
	}

	switch mode {
	case CanvasModeChat:
		if CANVAS_CHAT_DB == nil {
			return nil, fmt.Errorf("canvas chat database is not initialized")
		}
		return &canvasMessageStore{db: CANVAS_CHAT_DB, tableName: canvasChatMessagesTable, mode: mode}, nil
	case CanvasModeImage:
		if CANVAS_IMAGE_DB == nil {
			return nil, fmt.Errorf("canvas image database is not initialized")
		}
		return &canvasMessageStore{db: CANVAS_IMAGE_DB, tableName: canvasImageMessagesTable, mode: mode}, nil
	case CanvasModeVideo:
		if CANVAS_VIDEO_DB == nil {
			return nil, fmt.Errorf("canvas video database is not initialized")
		}
		return &canvasMessageStore{db: CANVAS_VIDEO_DB, tableName: canvasVideoMessagesTable, mode: mode}, nil
	default:
		return nil, fmt.Errorf("invalid canvas mode")
	}
}

func (s *canvasMessageStore) query() *gorm.DB {
	return s.db.Table(s.tableName)
}

func (s *canvasMessageStore) prepareMessage(message *CanvasMessage) error {
	if message == nil {
		return nil
	}
	if strings.TrimSpace(message.Mode) == "" {
		message.Mode = s.mode
	}
	if NormalizeCanvasMode(message.Mode) != s.mode {
		return fmt.Errorf("canvas message mode mismatch")
	}
	now := common.GetTimestamp()
	if message.CreatedTime == 0 {
		message.CreatedTime = now
	}
	if message.UpdatedTime == 0 {
		message.UpdatedTime = now
	}
	return nil
}

func (s *canvasMessageStore) create(message *CanvasMessage) error {
	if err := s.prepareMessage(message); err != nil {
		return err
	}
	return s.query().Create(message).Error
}

func (s *canvasMessageStore) createBatch(messages []*CanvasMessage) error {
	filtered := make([]*CanvasMessage, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			continue
		}
		if err := s.prepareMessage(message); err != nil {
			return err
		}
		filtered = append(filtered, message)
	}
	if len(filtered) == 0 {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		return tx.Table(s.tableName).Create(filtered).Error
	})
}

func (s *canvasMessageStore) list(userId int, sessionId int) ([]*CanvasMessage, error) {
	var messages []*CanvasMessage
	err := s.query().
		Where("user_id = ? AND session_id = ? AND mode = ? AND deleted_time = 0", userId, sessionId, s.mode).
		Order("created_time ASC").
		Order("id ASC").
		Find(&messages).Error
	return messages, err
}

func (s *canvasMessageStore) listPage(userId int, sessionId int, limit int, beforeCreatedTime int64, beforeID int) ([]*CanvasMessage, bool, error) {
	var messages []*CanvasMessage
	query := s.query().
		Where("user_id = ? AND session_id = ? AND mode = ? AND deleted_time = 0", userId, sessionId, s.mode)
	if beforeID > 0 {
		query = query.Where("(created_time < ?) OR (created_time = ? AND id < ?)", beforeCreatedTime, beforeCreatedTime, beforeID)
	}
	query = query.Order("created_time DESC").Order("id DESC")
	if limit <= 0 {
		err := query.Find(&messages).Error
		return messages, false, err
	}
	if err := query.Limit(limit + 1).Find(&messages).Error; err != nil {
		return nil, false, err
	}
	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}
	return messages, hasMore, nil
}

func (s *canvasMessageStore) getBySessionAndID(userId int, sessionId int, id int) (*CanvasMessage, error) {
	var message CanvasMessage
	err := s.query().
		Where("id = ? AND session_id = ? AND user_id = ? AND mode = ? AND deleted_time = 0", id, sessionId, userId, s.mode).
		First(&message).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &message, nil
}

func (s *canvasMessageStore) count(userId int, sessionId int) (int64, error) {
	var count int64
	err := s.query().
		Where("user_id = ? AND session_id = ? AND mode = ? AND deleted_time = 0", userId, sessionId, s.mode).
		Count(&count).Error
	return count, err
}

func (s *canvasMessageStore) updateFields(userId int, id int, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	updates["updated_time"] = common.GetTimestamp()
	return s.query().
		Where("id = ? AND user_id = ? AND mode = ? AND deleted_time = 0", id, userId, s.mode).
		Updates(updates).Error
}

func (s *canvasMessageStore) updateFieldsByTask(userId int, taskId string, taskType string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	updates["updated_time"] = common.GetTimestamp()
	return s.query().
		Where("user_id = ? AND task_id = ? AND task_type = ? AND mode = ? AND deleted_time = 0", userId, taskId, taskType, s.mode).
		Updates(updates).Error
}

func (s *canvasMessageStore) softDeleteSession(userId int, sessionId int, deletedTime int64) error {
	return s.query().
		Where("user_id = ? AND session_id = ? AND mode = ? AND deleted_time = 0", userId, sessionId, s.mode).
		Updates(map[string]interface{}{
			"deleted_time": deletedTime,
			"updated_time": deletedTime,
		}).Error
}

func (s *canvasMessageStore) softDeleteIDs(userId int, ids []int, deletedTime int64) error {
	if len(ids) == 0 {
		return nil
	}
	return s.query().
		Where("user_id = ? AND id IN ? AND mode = ? AND deleted_time = 0", userId, ids, s.mode).
		Updates(map[string]interface{}{
			"deleted_time": deletedTime,
			"updated_time": deletedTime,
		}).Error
}

func (s *canvasMessageStore) listSuccessful(userId int, sessionId int, afterMessageId int, beforeMessageId int, limit int, descending bool) ([]*CanvasMessage, error) {
	var messages []*CanvasMessage
	query := s.query().
		Where("user_id = ? AND session_id = ? AND mode = ? AND status = ? AND deleted_time = 0",
			userId, sessionId, s.mode, CanvasMessageStatusSuccess)
	if afterMessageId > 0 {
		query = query.Where("id > ?", afterMessageId)
	}
	if beforeMessageId > 0 {
		query = query.Where("id < ?", beforeMessageId)
	}
	if descending {
		query = query.Order("created_time DESC").Order("id DESC")
	} else {
		query = query.Order("created_time ASC").Order("id ASC")
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&messages).Error; err != nil {
		return nil, err
	}
	if descending && len(messages) > 1 {
		for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
			messages[left], messages[right] = messages[right], messages[left]
		}
	}
	return messages, nil
}

func (s *canvasMessageStore) countSuccessfulAfter(userId int, sessionId int, afterMessageId int) (int64, error) {
	var count int64
	query := s.query().
		Where("user_id = ? AND session_id = ? AND mode = ? AND status = ? AND deleted_time = 0",
			userId, sessionId, s.mode, CanvasMessageStatusSuccess)
	if afterMessageId > 0 {
		query = query.Where("id > ?", afterMessageId)
	}
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (s *canvasMessageStore) latestSuccessful(userId int, sessionId int) (*CanvasMessage, error) {
	var message CanvasMessage
	err := s.query().
		Where("user_id = ? AND session_id = ? AND mode = ? AND status = ? AND deleted_time = 0",
			userId, sessionId, s.mode, CanvasMessageStatusSuccess).
		Order("created_time DESC").
		Order("id DESC").
		First(&message).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &message, nil
}

func CreateCanvasMessageForMode(mode string, message *CanvasMessage) error {
	store, err := canvasMessageStoreForMode(mode)
	if err != nil {
		return err
	}
	return store.create(message)
}

func CreateCanvasMessagesForMode(mode string, messages []*CanvasMessage) error {
	store, err := canvasMessageStoreForMode(mode)
	if err != nil {
		return err
	}
	return store.createBatch(messages)
}

func ListCanvasMessagesForMode(mode string, userId int, sessionId int) ([]*CanvasMessage, error) {
	store, err := canvasMessageStoreForMode(mode)
	if err != nil {
		return nil, err
	}
	return store.list(userId, sessionId)
}

func ListCanvasMessagesPageForMode(mode string, userId int, sessionId int, limit int, beforeCreatedTime int64, beforeID int) ([]*CanvasMessage, bool, error) {
	store, err := canvasMessageStoreForMode(mode)
	if err != nil {
		return nil, false, err
	}
	return store.listPage(userId, sessionId, limit, beforeCreatedTime, beforeID)
}

func GetCanvasMessageByModeSessionAndID(mode string, userId int, sessionId int, id int) (*CanvasMessage, error) {
	store, err := canvasMessageStoreForMode(mode)
	if err != nil {
		return nil, err
	}
	return store.getBySessionAndID(userId, sessionId, id)
}

func CountCanvasMessagesByMode(mode string, userId int, sessionId int) (int64, error) {
	store, err := canvasMessageStoreForMode(mode)
	if err != nil {
		return 0, err
	}
	return store.count(userId, sessionId)
}

func UpdateCanvasMessageFieldsByMode(mode string, userId int, id int, updates map[string]interface{}) error {
	store, err := canvasMessageStoreForMode(mode)
	if err != nil {
		return err
	}
	return store.updateFields(userId, id, updates)
}

func UpdateCanvasMessageFieldsByTask(mode string, userId int, taskId string, taskType string, updates map[string]interface{}) error {
	store, err := canvasMessageStoreForMode(mode)
	if err != nil {
		return err
	}
	return store.updateFieldsByTask(userId, taskId, taskType, updates)
}

func SoftDeleteCanvasMessagesByMode(mode string, userId int, sessionId int, deletedTime int64) error {
	store, err := canvasMessageStoreForMode(mode)
	if err != nil {
		return err
	}
	return store.softDeleteSession(userId, sessionId, deletedTime)
}

func SoftDeleteCanvasMessagesByIDs(mode string, userId int, ids []int, deletedTime int64) error {
	store, err := canvasMessageStoreForMode(mode)
	if err != nil {
		return err
	}
	return store.softDeleteIDs(userId, ids, deletedTime)
}

func ListSuccessfulCanvasMessagesBefore(mode string, userId int, sessionId int, beforeMessageId int) ([]*CanvasMessage, error) {
	store, err := canvasMessageStoreForMode(mode)
	if err != nil {
		return nil, err
	}
	return store.listSuccessful(userId, sessionId, 0, beforeMessageId, 0, false)
}

func ListRecentSuccessfulCanvasMessages(mode string, userId int, sessionId int, afterMessageId int, beforeMessageId int, limit int) ([]*CanvasMessage, error) {
	if limit <= 0 {
		return []*CanvasMessage{}, nil
	}
	store, err := canvasMessageStoreForMode(mode)
	if err != nil {
		return nil, err
	}
	return store.listSuccessful(userId, sessionId, afterMessageId, beforeMessageId, limit, true)
}

func CountSuccessfulCanvasMessagesAfter(mode string, userId int, sessionId int, afterMessageId int) (int64, error) {
	store, err := canvasMessageStoreForMode(mode)
	if err != nil {
		return 0, err
	}
	return store.countSuccessfulAfter(userId, sessionId, afterMessageId)
}

func ListSuccessfulCanvasMessagesAfter(mode string, userId int, sessionId int, afterMessageId int, limit int) ([]*CanvasMessage, error) {
	store, err := canvasMessageStoreForMode(mode)
	if err != nil {
		return nil, err
	}
	return store.listSuccessful(userId, sessionId, afterMessageId, 0, limit, false)
}

func GetLatestSuccessfulCanvasMessage(mode string, userId int, sessionId int) (*CanvasMessage, error) {
	store, err := canvasMessageStoreForMode(mode)
	if err != nil {
		return nil, err
	}
	return store.latestSuccessful(userId, sessionId)
}
