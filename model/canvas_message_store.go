package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"gorm.io/gorm"
)

const (
	canvasChatMessagesTable  = "canvas_chat_messages"
	canvasImageMessagesTable = "canvas_image_messages"
	canvasVideoMessagesTable = "canvas_video_messages"
)

type CanvasChatMessage struct {
	Id               int    `gorm:"primaryKey;index:idx_canvas_chat_messages_session_deleted_created,priority:4;index:idx_canvas_chat_messages_session_status_deleted_created,priority:5"`
	SessionId        int    `gorm:"index:idx_canvas_chat_messages_session_deleted_created,priority:1;index:idx_canvas_chat_messages_session_status_deleted_created,priority:1;not null"`
	UserId           int    `gorm:"index;index:idx_canvas_chat_messages_task_lookup,priority:1;not null"`
	Mode             string `gorm:"size:16;index;index:idx_canvas_chat_messages_task_lookup,priority:2;not null"`
	Role             string `gorm:"size:16;not null"`
	Prompt           string `gorm:"type:text"`
	ReasoningContent string `gorm:"type:text"`
	ClientRequestId  string `gorm:"size:64;default:''"`
	Status           string `gorm:"size:32;index;index:idx_canvas_chat_messages_session_status_deleted_created,priority:2;default:''"`
	TaskId           string `gorm:"size:64;index;index:idx_canvas_chat_messages_task_lookup,priority:4;default:''"`
	TaskType         string `gorm:"size:32;index;index:idx_canvas_chat_messages_task_lookup,priority:3;default:''"`
	Metadata         string `gorm:"type:text"`
	ErrorMessage     string `gorm:"type:text"`
	CreatedTime      int64  `gorm:"bigint;index:idx_canvas_chat_messages_session_deleted_created,priority:3;index:idx_canvas_chat_messages_session_status_deleted_created,priority:4"`
	UpdatedTime      int64  `gorm:"bigint"`
	DeletedTime      int64  `gorm:"bigint;index:idx_canvas_chat_messages_session_deleted_created,priority:2;index:idx_canvas_chat_messages_session_status_deleted_created,priority:3;index:idx_canvas_chat_messages_task_lookup,priority:5;default:0"`
}

func (CanvasChatMessage) TableName() string {
	return canvasChatMessagesTable
}

type CanvasImageMessage struct {
	Id               int    `gorm:"primaryKey;index:idx_canvas_image_messages_session_deleted_created,priority:4;index:idx_canvas_image_messages_session_status_deleted_created,priority:5"`
	SessionId        int    `gorm:"index:idx_canvas_image_messages_session_deleted_created,priority:1;index:idx_canvas_image_messages_session_status_deleted_created,priority:1;not null"`
	UserId           int    `gorm:"index;index:idx_canvas_image_messages_task_lookup,priority:1;not null"`
	Mode             string `gorm:"size:16;index;index:idx_canvas_image_messages_task_lookup,priority:2;not null"`
	Role             string `gorm:"size:16;not null"`
	Prompt           string `gorm:"type:text"`
	ReasoningContent string `gorm:"type:text"`
	ClientRequestId  string `gorm:"size:64;default:''"`
	Status           string `gorm:"size:32;index;index:idx_canvas_image_messages_session_status_deleted_created,priority:2;default:''"`
	TaskId           string `gorm:"size:64;index;index:idx_canvas_image_messages_task_lookup,priority:4;default:''"`
	TaskType         string `gorm:"size:32;index;index:idx_canvas_image_messages_task_lookup,priority:3;default:''"`
	Metadata         string `gorm:"type:text"`
	ErrorMessage     string `gorm:"type:text"`
	CreatedTime      int64  `gorm:"bigint;index:idx_canvas_image_messages_session_deleted_created,priority:3;index:idx_canvas_image_messages_session_status_deleted_created,priority:4"`
	UpdatedTime      int64  `gorm:"bigint"`
	DeletedTime      int64  `gorm:"bigint;index:idx_canvas_image_messages_session_deleted_created,priority:2;index:idx_canvas_image_messages_session_status_deleted_created,priority:3;index:idx_canvas_image_messages_task_lookup,priority:5;default:0"`
}

func (CanvasImageMessage) TableName() string {
	return canvasImageMessagesTable
}

type CanvasVideoMessage struct {
	Id               int    `gorm:"primaryKey;index:idx_canvas_video_messages_session_deleted_created,priority:4;index:idx_canvas_video_messages_session_status_deleted_created,priority:5"`
	SessionId        int    `gorm:"index:idx_canvas_video_messages_session_deleted_created,priority:1;index:idx_canvas_video_messages_session_status_deleted_created,priority:1;not null"`
	UserId           int    `gorm:"index;index:idx_canvas_video_messages_task_lookup,priority:1;not null"`
	Mode             string `gorm:"size:16;index;index:idx_canvas_video_messages_task_lookup,priority:2;not null"`
	Role             string `gorm:"size:16;not null"`
	Prompt           string `gorm:"type:text"`
	ReasoningContent string `gorm:"type:text"`
	ClientRequestId  string `gorm:"size:64;default:''"`
	Status           string `gorm:"size:32;index;index:idx_canvas_video_messages_session_status_deleted_created,priority:2;default:''"`
	TaskId           string `gorm:"size:64;index;index:idx_canvas_video_messages_task_lookup,priority:4;default:''"`
	TaskType         string `gorm:"size:32;index;index:idx_canvas_video_messages_task_lookup,priority:3;default:''"`
	Metadata         string `gorm:"type:text"`
	ErrorMessage     string `gorm:"type:text"`
	CreatedTime      int64  `gorm:"bigint;index:idx_canvas_video_messages_session_deleted_created,priority:3;index:idx_canvas_video_messages_session_status_deleted_created,priority:4"`
	UpdatedTime      int64  `gorm:"bigint"`
	DeletedTime      int64  `gorm:"bigint;index:idx_canvas_video_messages_session_deleted_created,priority:2;index:idx_canvas_video_messages_session_status_deleted_created,priority:3;index:idx_canvas_video_messages_task_lookup,priority:5;default:0"`
}

func (CanvasVideoMessage) TableName() string {
	return canvasVideoMessagesTable
}

type canvasMessageStore struct {
	db        *gorm.DB
	tableName string
	mode      string
}

type CanvasMessageTaskRef struct {
	Id       int
	Mode     string
	Role     string
	Status   string
	TaskId   string
	TaskType string
}

func canvasMessageStoreForMode(mode string) (*canvasMessageStore, error) {
	mode = NormalizeCanvasMode(mode)
	if mode == "" {
		return nil, fmt.Errorf("invalid canvas mode")
	}
	if available, reason := CanvasModeAvailabilityStatus(mode); !available {
		return nil, newCanvasModeUnavailableError(mode, reason)
	}
	if !CanvasModeUsesDedicatedMessageDB(mode) {
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

func canvasMessageStoreForModeWithDB(mode string, db *gorm.DB) (*canvasMessageStore, error) {
	store, err := canvasMessageStoreForMode(mode)
	if err != nil {
		return nil, err
	}
	if db == nil {
		return store, nil
	}
	return &canvasMessageStore{
		db:        db,
		tableName: store.tableName,
		mode:      store.mode,
	}, nil
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
	return s.createBatch([]*CanvasMessage{message})
}

func (s *canvasMessageStore) hydrateMessages(userId int, sessionId int, messages []*CanvasMessage) error {
	if s.mode != CanvasModeChat {
		return nil
	}
	return hydrateCanvasChatMessageMetadata(s.db, userId, sessionId, messages)
}

func (s *canvasMessageStore) createBatch(messages []*CanvasMessage) error {
	type persistedMessage struct {
		original    *CanvasMessage
		persisted   *CanvasMessage
		attachments []dto.CanvasChatAttachment
	}
	filtered := make([]persistedMessage, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			continue
		}
		if err := s.prepareMessage(message); err != nil {
			return err
		}
		persisted := *message
		attachments := []dto.CanvasChatAttachment(nil)
		if s.mode == CanvasModeChat && message.Role == CanvasMessageRoleUser {
			sanitizedMetadata, extractedAttachments, err := sanitizeCanvasChatMetadataForStorage(message.Metadata)
			if err != nil {
				return err
			}
			persisted.Metadata = sanitizedMetadata
			attachments = extractedAttachments
		}
		filtered = append(filtered, persistedMessage{
			original:    message,
			persisted:   &persisted,
			attachments: attachments,
		})
	}
	if len(filtered) == 0 {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		rows := make([]*CanvasMessage, 0, len(filtered))
		for _, item := range filtered {
			rows = append(rows, item.persisted)
		}
		if err := tx.Table(s.tableName).Create(rows).Error; err != nil {
			return err
		}
		for _, item := range filtered {
			item.original.Id = item.persisted.Id
			item.original.CreatedTime = item.persisted.CreatedTime
			item.original.UpdatedTime = item.persisted.UpdatedTime
			if len(item.attachments) == 0 {
				continue
			}
			if err := defaultCanvasChatAttachmentBodyStore.PersistMessageAttachments(tx, item.persisted, item.attachments); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *canvasMessageStore) list(userId int, sessionId int) ([]*CanvasMessage, error) {
	var messages []*CanvasMessage
	err := s.query().
		Where("user_id = ? AND session_id = ? AND mode = ? AND deleted_time = 0", userId, sessionId, s.mode).
		Order("created_time ASC").
		Order("id ASC").
		Find(&messages).Error
	if err != nil {
		return nil, err
	}
	if err := s.hydrateMessages(userId, sessionId, messages); err != nil {
		return nil, err
	}
	return messages, nil
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
		if err != nil {
			return nil, false, err
		}
		if err := s.hydrateMessages(userId, sessionId, messages); err != nil {
			return nil, false, err
		}
		return messages, false, nil
	}
	if err := query.Limit(limit + 1).Find(&messages).Error; err != nil {
		return nil, false, err
	}
	if err := s.hydrateMessages(userId, sessionId, messages); err != nil {
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
	if err := s.hydrateMessages(userId, sessionId, []*CanvasMessage{&message}); err != nil {
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
		Where("user_id = ? AND mode = ? AND task_type = ? AND task_id = ? AND deleted_time = 0", userId, s.mode, taskType, taskId).
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
	return forEachChunk(ids, func(chunk []int) error {
		return s.query().
			Where("user_id = ? AND id IN ? AND mode = ? AND deleted_time = 0", userId, chunk, s.mode).
			Updates(map[string]interface{}{
				"deleted_time": deletedTime,
				"updated_time": deletedTime,
			}).Error
	})
}

func (s *canvasMessageStore) restoreIDs(userId int, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	restoredTime := common.GetTimestamp()
	return forEachChunk(ids, func(chunk []int) error {
		return s.query().
			Where("user_id = ? AND id IN ? AND mode = ? AND deleted_time > 0", userId, chunk, s.mode).
			Updates(map[string]interface{}{
				"deleted_time": 0,
				"updated_time": restoredTime,
			}).Error
	})
}

func listCanvasMessageTaskRefsForMode(db *gorm.DB, tableName string, mode string, userId int, sessionId int) ([]*CanvasMessageTaskRef, error) {
	var refs []*CanvasMessageTaskRef
	err := db.Table(tableName).
		Select("id, mode, role, status, task_id, task_type").
		Where("user_id = ? AND session_id = ? AND mode = ? AND deleted_time = 0", userId, sessionId, mode).
		Order("id ASC").
		Find(&refs).Error
	return refs, err
}

func ListCanvasSessionMessageTaskRefsForMode(mode string, userId int, sessionId int) ([]*CanvasMessageTaskRef, error) {
	mode = NormalizeCanvasMode(mode)
	if mode == "" {
		return nil, fmt.Errorf("invalid canvas mode")
	}
	if available, reason := CanvasModeAvailabilityStatus(mode); !available {
		return nil, newCanvasModeUnavailableError(mode, reason)
	}
	if !CanvasModeUsesDedicatedMessageDB(mode) {
		if DB == nil {
			return nil, fmt.Errorf("canvas main database is not initialized")
		}
		return listCanvasMessageTaskRefsForMode(DB, "canvas_messages", mode, userId, sessionId)
	}

	store, err := canvasMessageStoreForMode(mode)
	if err != nil {
		return nil, err
	}
	return listCanvasMessageTaskRefsForMode(store.db, store.tableName, mode, userId, sessionId)
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
	if err := s.hydrateMessages(userId, sessionId, messages); err != nil {
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
	if err := s.hydrateMessages(userId, sessionId, []*CanvasMessage{&message}); err != nil {
		return nil, err
	}
	return &message, nil
}

func (s *canvasMessageStore) listByClientRequestID(userId int, sessionId int, clientRequestID string) ([]*CanvasMessage, error) {
	clientRequestID = strings.TrimSpace(clientRequestID)
	if clientRequestID == "" {
		return []*CanvasMessage{}, nil
	}
	var messages []*CanvasMessage
	if err := s.query().
		Where("user_id = ? AND session_id = ? AND mode = ? AND client_request_id = ? AND deleted_time = 0", userId, sessionId, s.mode, clientRequestID).
		Order("created_time ASC").
		Order("id ASC").
		Find(&messages).Error; err != nil {
		return nil, err
	}
	if err := s.hydrateMessages(userId, sessionId, messages); err != nil {
		return nil, err
	}
	return messages, nil
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

func SoftDeleteCanvasMessagesByIDsWithDB(db *gorm.DB, mode string, userId int, ids []int, deletedTime int64) error {
	store, err := canvasMessageStoreForModeWithDB(mode, db)
	if err != nil {
		return err
	}
	return store.softDeleteIDs(userId, ids, deletedTime)
}

func RestoreCanvasMessagesByIDs(mode string, userId int, ids []int) error {
	store, err := canvasMessageStoreForMode(mode)
	if err != nil {
		return err
	}
	return store.restoreIDs(userId, ids)
}

func ListCanvasMessagesByClientRequestIDForMode(mode string, userId int, sessionId int, clientRequestID string) ([]*CanvasMessage, error) {
	store, err := canvasMessageStoreForMode(mode)
	if err != nil {
		return nil, err
	}
	return store.listByClientRequestID(userId, sessionId, clientRequestID)
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
