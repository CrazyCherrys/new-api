package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"gorm.io/gorm"
)

const canvasChatMessageAttachmentsTable = "canvas_chat_message_attachments"

type CanvasChatMessageAttachment struct {
	Id          int    `gorm:"primaryKey"`
	MessageId   int    `gorm:"index:idx_canvas_chat_attachments_message_sort,priority:1;index:idx_canvas_chat_attachments_user_session_message_sort,priority:3;not null"`
	UserId      int    `gorm:"index:idx_canvas_chat_attachments_user_session_message_sort,priority:1;not null"`
	SessionId   int    `gorm:"index:idx_canvas_chat_attachments_user_session_message_sort,priority:2;not null"`
	SortOrder   int    `gorm:"index:idx_canvas_chat_attachments_message_sort,priority:2;index:idx_canvas_chat_attachments_user_session_message_sort,priority:4"`
	Kind        string `gorm:"size:16;not null"`
	Name        string `gorm:"size:255;not null"`
	MimeType    string `gorm:"size:128;not null"`
	Data        string `gorm:"type:text"`
	CreatedTime int64  `gorm:"bigint"`
	UpdatedTime int64  `gorm:"bigint"`
}

func (CanvasChatMessageAttachment) TableName() string {
	return canvasChatMessageAttachmentsTable
}

type CanvasChatAttachmentBodyStore interface {
	PersistMessageAttachments(db *gorm.DB, message *CanvasMessage, attachments []dto.CanvasChatAttachment) error
	LoadMessageAttachments(db *gorm.DB, userId int, sessionId int, messageIDs []int) (map[int][]dto.CanvasChatAttachment, error)
	DeleteSessionAttachments(db *gorm.DB, userId int, sessionId int) error
}

type gormCanvasChatAttachmentBodyStore struct{}

var defaultCanvasChatAttachmentBodyStore CanvasChatAttachmentBodyStore = &gormCanvasChatAttachmentBodyStore{}

func canvasMetadataString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func (s *gormCanvasChatAttachmentBodyStore) PersistMessageAttachments(db *gorm.DB, message *CanvasMessage, attachments []dto.CanvasChatAttachment) error {
	if db == nil || message == nil || len(attachments) == 0 {
		return nil
	}
	now := common.GetTimestamp()
	rows := make([]*CanvasChatMessageAttachment, 0, len(attachments))
	for index, attachment := range attachments {
		rows = append(rows, &CanvasChatMessageAttachment{
			MessageId:   message.Id,
			UserId:      message.UserId,
			SessionId:   message.SessionId,
			SortOrder:   index,
			Kind:        strings.TrimSpace(attachment.Kind),
			Name:        strings.TrimSpace(attachment.Name),
			MimeType:    strings.TrimSpace(attachment.MimeType),
			Data:        strings.TrimSpace(attachment.Data),
			CreatedTime: now,
			UpdatedTime: now,
		})
	}
	return db.Table(canvasChatMessageAttachmentsTable).Create(rows).Error
}

func (s *gormCanvasChatAttachmentBodyStore) LoadMessageAttachments(db *gorm.DB, userId int, sessionId int, messageIDs []int) (map[int][]dto.CanvasChatAttachment, error) {
	result := make(map[int][]dto.CanvasChatAttachment)
	if db == nil || userId <= 0 || sessionId <= 0 || len(messageIDs) == 0 {
		return result, nil
	}
	var rows []*CanvasChatMessageAttachment
	err := forEachChunk(messageIDs, func(chunk []int) error {
		var partial []*CanvasChatMessageAttachment
		if err := db.Table(canvasChatMessageAttachmentsTable).
			Where("user_id = ? AND session_id = ? AND message_id IN ?", userId, sessionId, chunk).
			Order("message_id ASC").
			Order("sort_order ASC").
			Order("id ASC").
			Find(&partial).Error; err != nil {
			return err
		}
		rows = append(rows, partial...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row == nil {
			continue
		}
		result[row.MessageId] = append(result[row.MessageId], dto.CanvasChatAttachment{
			Kind:     strings.TrimSpace(row.Kind),
			Name:     strings.TrimSpace(row.Name),
			MimeType: strings.TrimSpace(row.MimeType),
			Data:     strings.TrimSpace(row.Data),
		})
	}
	return result, nil
}

func (s *gormCanvasChatAttachmentBodyStore) DeleteSessionAttachments(db *gorm.DB, userId int, sessionId int) error {
	if db == nil || userId <= 0 || sessionId <= 0 {
		return nil
	}
	return db.Table(canvasChatMessageAttachmentsTable).
		Where("user_id = ? AND session_id = ?", userId, sessionId).
		Delete(&CanvasChatMessageAttachment{}).Error
}

func DeleteCanvasChatMessageAttachmentsBySessionWithDB(db *gorm.DB, userId int, sessionId int) error {
	if db == nil {
		resolvedDB, err := canvasModeDataDB(CanvasModeChat)
		if err != nil {
			return err
		}
		db = resolvedDB
	}
	return defaultCanvasChatAttachmentBodyStore.DeleteSessionAttachments(db, userId, sessionId)
}

func DeleteCanvasChatMessageAttachmentsBySession(userId int, sessionId int) error {
	return DeleteCanvasChatMessageAttachmentsBySessionWithDB(nil, userId, sessionId)
}

func canvasChatAttachmentMetadataEntries(attachments []dto.CanvasChatAttachment, includeData bool) []map[string]any {
	if len(attachments) == 0 {
		return nil
	}
	entries := make([]map[string]any, 0, len(attachments))
	for _, attachment := range attachments {
		entry := map[string]any{
			"kind":      strings.TrimSpace(attachment.Kind),
			"name":      strings.TrimSpace(attachment.Name),
			"mime_type": strings.TrimSpace(attachment.MimeType),
		}
		if includeData {
			entry["data"] = strings.TrimSpace(attachment.Data)
		}
		entries = append(entries, entry)
	}
	return entries
}

func parseCanvasChatAttachmentsFromMetadata(raw string) ([]dto.CanvasChatAttachment, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var metadata map[string]any
	if err := common.UnmarshalJsonStr(raw, &metadata); err != nil {
		return nil, err
	}
	attachmentsRaw, ok := metadata["attachments"].([]any)
	if !ok {
		return nil, nil
	}
	attachments := make([]dto.CanvasChatAttachment, 0, len(attachmentsRaw))
	for _, item := range attachmentsRaw {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		kind := canvasMetadataString(entry["kind"])
		name := canvasMetadataString(entry["name"])
		mimeType := canvasMetadataString(entry["mime_type"])
		data := canvasMetadataString(entry["data"])
		if kind == "" || name == "" || mimeType == "" || data == "" {
			continue
		}
		attachments = append(attachments, dto.CanvasChatAttachment{
			Kind:     kind,
			Name:     name,
			MimeType: mimeType,
			Data:     data,
		})
	}
	return attachments, nil
}

func sanitizeCanvasChatMetadataForStorage(raw string) (string, []dto.CanvasChatAttachment, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil, nil
	}
	var metadata map[string]any
	if err := common.UnmarshalJsonStr(raw, &metadata); err != nil {
		return "", nil, err
	}
	attachments, err := parseCanvasChatAttachmentsFromMetadata(raw)
	if err != nil {
		return "", nil, err
	}
	if len(attachments) == 0 {
		return raw, nil, nil
	}
	metadata["attachments"] = canvasChatAttachmentMetadataEntries(attachments, false)
	sanitized, err := common.Marshal(metadata)
	if err != nil {
		return "", nil, err
	}
	return string(sanitized), attachments, nil
}

func hydrateCanvasChatMetadataWithAttachments(raw string, attachments []dto.CanvasChatAttachment) (string, error) {
	raw = strings.TrimSpace(raw)
	if len(attachments) == 0 {
		return raw, nil
	}
	metadata := make(map[string]any)
	if raw != "" {
		if err := common.UnmarshalJsonStr(raw, &metadata); err != nil {
			return "", err
		}
	}
	metadata["attachments"] = canvasChatAttachmentMetadataEntries(attachments, true)
	hydrated, err := common.Marshal(metadata)
	if err != nil {
		return "", err
	}
	return string(hydrated), nil
}

func hydrateCanvasChatMessageMetadata(db *gorm.DB, userId int, sessionId int, messages []*CanvasMessage) error {
	if db == nil || userId <= 0 || sessionId <= 0 || len(messages) == 0 {
		return nil
	}
	messageIDs := make([]int, 0, len(messages))
	for _, message := range messages {
		if message == nil || message.Role != CanvasMessageRoleUser || message.Id <= 0 {
			continue
		}
		messageIDs = append(messageIDs, message.Id)
	}
	if len(messageIDs) == 0 {
		return nil
	}
	attachmentsByMessageID, err := defaultCanvasChatAttachmentBodyStore.LoadMessageAttachments(db, userId, sessionId, messageIDs)
	if err != nil {
		return err
	}
	for _, message := range messages {
		if message == nil || message.Role != CanvasMessageRoleUser {
			continue
		}
		attachments := attachmentsByMessageID[message.Id]
		if len(attachments) == 0 {
			continue
		}
		hydrated, err := hydrateCanvasChatMetadataWithAttachments(message.Metadata, attachments)
		if err != nil {
			return err
		}
		message.Metadata = hydrated
	}
	return nil
}
