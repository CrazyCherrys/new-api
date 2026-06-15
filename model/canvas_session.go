package model

import (
	"fmt"
	"sort"
	"strconv"
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
	PublicId                string  `json:"public_id" gorm:"size:64;index"`
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

type canvasSessionStore struct {
	db   *gorm.DB
	mode string
}

func canvasSessionStoreForMode(mode string) (*canvasSessionStore, error) {
	mode = NormalizeCanvasMode(mode)
	if mode == "" {
		return nil, fmt.Errorf("invalid canvas mode")
	}
	db, err := canvasModeDataDB(mode)
	if err != nil {
		return nil, err
	}
	return &canvasSessionStore{
		db:   db,
		mode: mode,
	}, nil
}

func generateCanvasSessionPublicID(mode string) string {
	mode = NormalizeCanvasMode(mode)
	if mode == "" {
		return "cs_" + common.GetUUID()
	}
	return "cs_" + mode + "_" + common.GetUUID()
}

func canvasModeFromSessionPublicID(publicID string) string {
	publicID = strings.TrimSpace(publicID)
	for _, mode := range []string{CanvasModeChat, CanvasModeImage, CanvasModeVideo} {
		if strings.HasPrefix(strings.ToLower(publicID), "cs_"+mode+"_") {
			return mode
		}
	}
	return ""
}

func canvasSessionAvailableStores() ([]*canvasSessionStore, error) {
	stores := make([]*canvasSessionStore, 0, len([]string{CanvasModeChat, CanvasModeImage, CanvasModeVideo}))
	for _, mode := range []string{CanvasModeChat, CanvasModeImage, CanvasModeVideo} {
		store, err := canvasSessionStoreForMode(mode)
		if err != nil {
			if IsCanvasModeUnavailableError(err) {
				continue
			}
			return nil, err
		}
		stores = append(stores, store)
	}
	return stores, nil
}

func (s *canvasSessionStore) baseQuery() *gorm.DB {
	query := s.db.Model(&CanvasSession{})
	if normalizedMode := NormalizeCanvasMode(s.mode); normalizedMode != "" {
		query = query.Where("mode = ?", normalizedMode)
	}
	return query
}

func (s *canvasSessionStore) ensureSessionPublicID(session *CanvasSession) error {
	if session == nil {
		return nil
	}
	if strings.TrimSpace(session.PublicId) != "" {
		return nil
	}
	if s.db == nil {
		return fmt.Errorf("canvas session database is not initialized")
	}
	if session.Id <= 0 {
		return fmt.Errorf("canvas session public_id cannot be persisted before session is created")
	}
	publicID := generateCanvasSessionPublicID(session.Mode)
	result := s.baseQuery().
		Where("id = ? AND user_id = ? AND (public_id = '' OR public_id IS NULL)", session.Id, session.UserId).
		Update("public_id", publicID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		session.PublicId = publicID
		return nil
	}
	var row struct {
		PublicId string `gorm:"column:public_id"`
	}
	if err := s.baseQuery().
		Select("public_id").
		Where("id = ? AND user_id = ?", session.Id, session.UserId).
		Take(&row).Error; err != nil {
		return err
	}
	persistedPublicID := strings.TrimSpace(row.PublicId)
	if persistedPublicID == "" {
		return fmt.Errorf("canvas session public_id was not persisted")
	}
	session.PublicId = persistedPublicID
	return nil
}

func (s *canvasSessionStore) ensureSessionPublicIDs(sessions []*CanvasSession) error {
	for _, session := range sessions {
		if err := s.ensureSessionPublicID(session); err != nil {
			return err
		}
	}
	return nil
}

func (s *canvasSessionStore) create(session *CanvasSession) error {
	if session == nil {
		return nil
	}
	if s.db == nil {
		return fmt.Errorf("canvas session database is not initialized")
	}
	if NormalizeCanvasMode(session.Mode) != s.mode {
		return fmt.Errorf("canvas session mode mismatch")
	}
	now := common.GetTimestamp()
	if session.CreatedTime == 0 {
		session.CreatedTime = now
	}
	if session.UpdatedTime == 0 {
		session.UpdatedTime = now
	}
	if strings.TrimSpace(session.PublicId) == "" {
		session.PublicId = generateCanvasSessionPublicID(session.Mode)
	}
	chatTemperature := session.ChatTemperature
	chatContextCount := session.ChatContextCount
	webSearchEnabled := session.WebSearchEnabled
	systemPrompt := session.SystemPrompt
	summaryEnabled := session.SummaryEnabled
	summaryTriggerMessages := session.SummaryTriggerMessages
	summaryRecentMessages := session.SummaryRecentMessages
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Select("*").Create(session).Error; err != nil {
			return err
		}
		if session.Mode != CanvasModeChat {
			return nil
		}
		return tx.Model(&CanvasSession{}).
			Where("id = ? AND user_id = ? AND mode = ?", session.Id, session.UserId, CanvasModeChat).
			Select(
				"chat_temperature",
				"chat_context_count",
				"web_search_enabled",
				"system_prompt",
				"summary_enabled",
				"summary_trigger_messages",
				"summary_recent_messages",
			).
			Updates(map[string]interface{}{
				"chat_temperature":         chatTemperature,
				"chat_context_count":       chatContextCount,
				"web_search_enabled":       webSearchEnabled,
				"system_prompt":            systemPrompt,
				"summary_enabled":          summaryEnabled,
				"summary_trigger_messages": summaryTriggerMessages,
				"summary_recent_messages":  summaryRecentMessages,
			}).Error
	})
}

func (s *canvasSessionStore) getByID(userId int, id int) (*CanvasSession, error) {
	if s.db == nil {
		return nil, fmt.Errorf("canvas session database is not initialized")
	}
	var session CanvasSession
	err := s.baseQuery().Where("id = ? AND user_id = ? AND deleted_time = 0", id, userId).First(&session).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := s.ensureSessionPublicID(&session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (s *canvasSessionStore) getByPublicID(userId int, publicID string) (*CanvasSession, error) {
	if s.db == nil {
		return nil, fmt.Errorf("canvas session database is not initialized")
	}
	publicID = strings.TrimSpace(publicID)
	if publicID == "" {
		return nil, nil
	}
	var session CanvasSession
	err := s.baseQuery().Where("public_id = ? AND user_id = ? AND deleted_time = 0", publicID, userId).First(&session).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := s.ensureSessionPublicID(&session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (s *canvasSessionStore) list(userId int, mode string, limit int, offset int) ([]*CanvasSession, bool, error) {
	if s.db == nil {
		return nil, false, fmt.Errorf("canvas session database is not initialized")
	}
	var sessions []*CanvasSession
	query := s.baseQuery().Where("user_id = ? AND deleted_time = 0", userId)
	query = query.Order("pinned DESC").Order("updated_time DESC").Order("id DESC").Order("mode ASC").Order("public_id ASC")
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		err := query.Find(&sessions).Error
		if err != nil {
			return nil, false, err
		}
		if err := s.ensureSessionPublicIDs(sessions); err != nil {
			return nil, false, err
		}
		return sessions, false, nil
	}
	if err := query.Offset(offset).Limit(limit + 1).Find(&sessions).Error; err != nil {
		return nil, false, err
	}
	hasMore := len(sessions) > limit
	if hasMore {
		sessions = sessions[:limit]
	}
	if err := s.ensureSessionPublicIDs(sessions); err != nil {
		return nil, false, err
	}
	return sessions, hasMore, nil
}

func (s *canvasSessionStore) updateFields(userId int, id int, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	if s.db == nil {
		return fmt.Errorf("canvas session database is not initialized")
	}
	updates["updated_time"] = common.GetTimestamp()
	return s.baseQuery().
		Where("id = ? AND user_id = ? AND deleted_time = 0", id, userId).
		Updates(updates).Error
}

func (s *canvasSessionStore) touch(userId int, id int) error {
	if s.db == nil {
		return fmt.Errorf("canvas session database is not initialized")
	}
	now := common.GetTimestamp()
	return s.baseQuery().
		Where("id = ? AND user_id = ? AND deleted_time = 0", id, userId).
		Update("updated_time", now).Error
}

func (s *canvasSessionStore) softDeleteWithDB(db *gorm.DB, userId int, id int, deletedTime int64) error {
	if db == nil {
		db = s.db
	}
	if db == nil {
		return fmt.Errorf("canvas session database is not initialized")
	}
	return db.Model(&CanvasSession{}).
		Where("id = ? AND user_id = ? AND mode = ? AND deleted_time = 0", id, userId, s.mode).
		Updates(map[string]interface{}{
			"deleted_time": deletedTime,
			"updated_time": deletedTime,
		}).Error
}

func (s *canvasSessionStore) restore(userId int, id int) error {
	if s.db == nil {
		return fmt.Errorf("canvas session database is not initialized")
	}
	restoredTime := common.GetTimestamp()
	return s.baseQuery().
		Where("id = ? AND user_id = ? AND deleted_time > 0", id, userId).
		Updates(map[string]interface{}{
			"deleted_time": 0,
			"updated_time": restoredTime,
		}).Error
}

func (s *canvasSessionStore) deleteWithDB(db *gorm.DB, userId int, id int) error {
	if db == nil {
		db = s.db
	}
	if db == nil {
		return fmt.Errorf("canvas session database is not initialized")
	}
	return db.Where("id = ? AND user_id = ? AND mode = ? AND deleted_time = 0", id, userId, s.mode).
		Delete(&CanvasSession{}).Error
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
	store, err := canvasSessionStoreForMode(session.Mode)
	if err != nil {
		return err
	}
	return store.create(session)
}

func GetCanvasSessionByID(userId int, id int) (*CanvasSession, error) {
	if id <= 0 {
		return nil, nil
	}
	stores, err := canvasSessionAvailableStores()
	if err != nil {
		return nil, err
	}
	matches := make([]*CanvasSession, 0, len(stores))
	for _, store := range stores {
		session, getErr := store.getByID(userId, id)
		if getErr != nil {
			return nil, getErr
		}
		if session != nil {
			matches = append(matches, session)
		}
	}
	if len(matches) == 0 {
		return nil, nil
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].UpdatedTime == matches[j].UpdatedTime {
			return matches[i].Mode < matches[j].Mode
		}
		return matches[i].UpdatedTime > matches[j].UpdatedTime
	})
	return matches[0], nil
}

func GetCanvasSessionByPublicID(userId int, publicID string) (*CanvasSession, error) {
	publicID = strings.TrimSpace(publicID)
	if publicID == "" {
		return nil, nil
	}
	if mode := canvasModeFromSessionPublicID(publicID); mode != "" {
		store, err := canvasSessionStoreForMode(mode)
		if err != nil {
			return nil, err
		}
		return store.getByPublicID(userId, publicID)
	}
	stores, err := canvasSessionAvailableStores()
	if err != nil {
		return nil, err
	}
	for _, store := range stores {
		session, getErr := store.getByPublicID(userId, publicID)
		if getErr != nil {
			return nil, getErr
		}
		if session != nil {
			return session, nil
		}
	}
	return nil, nil
}

func GetCanvasSessionByIdentifier(userId int, identifier string) (*CanvasSession, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, nil
	}
	if strings.HasPrefix(strings.ToLower(identifier), "cs_") {
		return GetCanvasSessionByPublicID(userId, identifier)
	}
	if id, err := strconv.Atoi(identifier); err == nil && id > 0 {
		return GetCanvasSessionByID(userId, id)
	}
	return GetCanvasSessionByPublicID(userId, identifier)
}

func ListCanvasSessions(userId int, mode string, limit int, offset int) ([]*CanvasSession, bool, error) {
	if normalizedMode := NormalizeCanvasMode(mode); normalizedMode != "" {
		store, err := canvasSessionStoreForMode(normalizedMode)
		if err != nil {
			return nil, false, err
		}
		return store.list(userId, normalizedMode, limit, offset)
	}
	stores, err := canvasSessionAvailableStores()
	if err != nil {
		return nil, false, err
	}
	if offset < 0 {
		offset = 0
	}
	sessions := make([]*CanvasSession, 0)
	perStoreLimit := 0
	if limit > 0 {
		perStoreLimit = offset + limit
	}
	anyStoreHasMore := false
	for _, store := range stores {
		items, storeHasMore, listErr := store.list(userId, store.mode, perStoreLimit, 0)
		if listErr != nil {
			return nil, false, listErr
		}
		anyStoreHasMore = anyStoreHasMore || storeHasMore
		sessions = append(sessions, items...)
	}
	sort.SliceStable(sessions, func(i, j int) bool {
		return canvasSessionLess(sessions[i], sessions[j])
	})
	if limit <= 0 {
		return sessions, false, nil
	}
	if offset >= len(sessions) {
		return []*CanvasSession{}, false, nil
	}
	end := offset + limit
	if end > len(sessions) {
		end = len(sessions)
	}
	return sessions[offset:end], end < len(sessions) || anyStoreHasMore, nil
}

func canvasSessionLess(left *CanvasSession, right *CanvasSession) bool {
	if left == nil {
		return right != nil
	}
	if right == nil {
		return false
	}
	if left.Pinned != right.Pinned {
		return left.Pinned
	}
	if left.UpdatedTime != right.UpdatedTime {
		return left.UpdatedTime > right.UpdatedTime
	}
	if left.Id != right.Id {
		return left.Id > right.Id
	}
	if left.Mode != right.Mode {
		return left.Mode < right.Mode
	}
	return left.PublicId < right.PublicId
}

func UpdateCanvasSessionFields(userId int, id int, updates map[string]interface{}) error {
	session, err := GetCanvasSessionByID(userId, id)
	if err != nil {
		return err
	}
	if session == nil {
		return nil
	}
	return UpdateCanvasSessionFieldsWithSession(userId, session, updates)
}

func UpdateCanvasSessionFieldsWithSession(userId int, session *CanvasSession, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	if session == nil {
		return nil
	}
	if session.UserId != userId || session.Id <= 0 {
		return fmt.Errorf("canvas session not found")
	}
	store, err := canvasSessionStoreForMode(session.Mode)
	if err != nil {
		return err
	}
	return store.updateFields(userId, session.Id, updates)
}

func TouchCanvasSession(userId int, id int) error {
	session, err := GetCanvasSessionByID(userId, id)
	if err != nil {
		return err
	}
	if session == nil {
		return nil
	}
	return TouchCanvasSessionWithSession(userId, session)
}

func TouchCanvasSessionWithSession(userId int, session *CanvasSession) error {
	if session == nil {
		return nil
	}
	if session.UserId != userId || session.Id <= 0 {
		return fmt.Errorf("canvas session not found")
	}
	store, err := canvasSessionStoreForMode(session.Mode)
	if err != nil {
		return err
	}
	return store.touch(userId, session.Id)
}

func SoftDeleteCanvasSession(userId int, id int, deletedTime int64) error {
	return SoftDeleteCanvasSessionWithDB(DB, userId, id, deletedTime)
}

func SoftDeleteCanvasSessionWithDB(db *gorm.DB, userId int, id int, deletedTime int64) error {
	session, err := GetCanvasSessionByID(userId, id)
	if err != nil {
		return err
	}
	if session == nil {
		return nil
	}
	return SoftDeleteCanvasSessionWithSessionAndDB(db, userId, session, deletedTime)
}

func SoftDeleteCanvasSessionWithSession(userId int, session *CanvasSession, deletedTime int64) error {
	return SoftDeleteCanvasSessionWithSessionAndDB(nil, userId, session, deletedTime)
}

func SoftDeleteCanvasSessionWithSessionAndDB(db *gorm.DB, userId int, session *CanvasSession, deletedTime int64) error {
	if session == nil {
		return nil
	}
	if session.UserId != userId || session.Id <= 0 {
		return fmt.Errorf("canvas session not found")
	}
	store, err := canvasSessionStoreForMode(session.Mode)
	if err != nil {
		return err
	}
	return store.softDeleteWithDB(db, userId, session.Id, deletedTime)
}

func RestoreCanvasSessionByMode(mode string, userId int, id int) error {
	store, err := canvasSessionStoreForMode(mode)
	if err != nil {
		return err
	}
	return store.restore(userId, id)
}

func DeleteCanvasSessionWithSession(userId int, session *CanvasSession) error {
	return DeleteCanvasSessionWithSessionAndDB(nil, userId, session)
}

func DeleteCanvasSessionWithSessionAndDB(db *gorm.DB, userId int, session *CanvasSession) error {
	if session == nil {
		return nil
	}
	if session.UserId != userId || session.Id <= 0 {
		return fmt.Errorf("canvas session not found")
	}
	store, err := canvasSessionStoreForMode(session.Mode)
	if err != nil {
		return err
	}
	return store.deleteWithDB(db, userId, session.Id)
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

func DeleteCanvasSessionData(userId int, sessionId int, messageMode string, messageIDs []int, imageTaskIDs []int, videoTaskIDs []int64, cleanupJobs []*CanvasAssetCleanupJob, deletedTime int64) error {
	session, err := GetCanvasSessionByID(userId, sessionId)
	if err != nil {
		return err
	}
	if session == nil {
		return nil
	}
	return DeleteCanvasSessionDataWithSession(userId, session, messageIDs, imageTaskIDs, videoTaskIDs, cleanupJobs, deletedTime)
}

func DeleteCanvasSessionDataWithSession(userId int, session *CanvasSession, messageIDs []int, imageTaskIDs []int, videoTaskIDs []int64, cleanupJobs []*CanvasAssetCleanupJob, deletedTime int64) error {
	if session == nil {
		return nil
	}
	if session.UserId != userId || session.Id <= 0 {
		return fmt.Errorf("canvas session not found")
	}
	messageMode := NormalizeCanvasMode(session.Mode)
	if messageMode == "" {
		return fmt.Errorf("invalid canvas mode")
	}
	db, err := canvasModeDataDB(messageMode)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := CreateCanvasAssetCleanupJobsWithDB(tx, cleanupJobs); err != nil {
			return err
		}
		if err := DeleteImageTasksByUserAndIDsWithDB(tx, userId, imageTaskIDs); err != nil {
			return err
		}
		if err := DeleteCanvasVideoTasksByIDsWithDB(tx, userId, videoTaskIDs, nil); err != nil {
			return err
		}
		if messageMode == CanvasModeChat {
			if err := DeleteCanvasChatMessageAttachmentsBySessionWithDB(tx, userId, session.Id); err != nil {
				return err
			}
		}
		if err := DeleteCanvasMessagesByModeWithDB(tx, messageMode, userId, session.Id); err != nil {
			return err
		}
		return DeleteCanvasSessionWithSessionAndDB(tx, userId, session)
	})
}
