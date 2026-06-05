package service

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relayhelper "github.com/QuantumNous/new-api/relay/helper"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

const (
	canvasChatDefaultTemperature            = 0.7
	canvasChatDefaultContextCount           = 8
	canvasChatSummaryEnabledDefault         = true
	canvasChatSummaryTriggerMessagesDefault = 8
	canvasChatSummaryRecentMessagesDefault  = 8
	canvasChatMaxContextCount               = 64
	canvasChatMaxImageAttachments           = 1
	canvasChatMaxFileAttachments            = 1
	canvasChatRelayScannerInitialBufferSize = 64 << 10
	canvasChatRelayScannerMaxBufferSize     = 64 << 20
	canvasChatDeltaFlushMinChars            = 48
	canvasChatDeltaFlushInterval            = 700 * time.Millisecond
	canvasChatSummaryRequestTimeout         = 2 * time.Minute
	canvasChatSummaryMaxLengthRunes         = 6000
)

type canvasChatMessageMetadata struct {
	ChatModel              string                     `json:"chat_model,omitempty"`
	ChatGroup              string                     `json:"chat_group,omitempty"`
	Temperature            *float64                   `json:"temperature,omitempty"`
	ContextCount           *int                       `json:"context_count,omitempty"`
	WebSearchEnabled       *bool                      `json:"web_search_enabled,omitempty"`
	SystemPrompt           string                     `json:"system_prompt,omitempty"`
	SummaryEnabled         *bool                      `json:"summary_enabled,omitempty"`
	SummaryTriggerMessages *int                       `json:"summary_trigger_messages,omitempty"`
	SummaryRecentMessages  *int                       `json:"summary_recent_messages,omitempty"`
	Attachments            []dto.CanvasChatAttachment `json:"attachments,omitempty"`
}

type canvasChatSummaryBranch struct {
	ClearContextMessageId   int    `json:"clear_context_message_id"`
	LastSummarizedMessageId int    `json:"last_summarized_message_id"`
	Prompt                  string `json:"prompt,omitempty"`
}

type canvasChatSummaryState struct {
	Default  *canvasChatSummaryBranch  `json:"default,omitempty"`
	Branches []canvasChatSummaryBranch `json:"branches,omitempty"`
}

type canvasChatPreparedRequest struct {
	UserId           int
	UserGroup        string
	Session          *model.CanvasSession
	Prompt           string
	FinalModel       string
	RequestEndpoint  string
	FinalGroup       string
	Temperature      *float64
	ContextCount     int
	WebSearchEnabled bool
	ClientRequestId  string
	UserMessage      *model.CanvasMessage
	AssistantMessage *model.CanvasMessage
}

type canvasChatRelayRequest struct {
	UserId           int
	UserGroup        string
	ModelId          string
	RequestEndpoint  string
	Group            string
	Temperature      *float64
	WebSearchEnabled bool
	Messages         []dto.Message
}

type canvasChatRelayDelta struct {
	Content          string
	ReasoningContent string
}

type canvasChatRelayResult struct {
	Text             string
	ReasoningContent string
}

type canvasChatStreamCallbacks struct {
	OnDelta     func(prepared *canvasChatPreparedRequest, delta canvasChatRelayDelta)
	OnCompleted func(prepared *canvasChatPreparedRequest)
	OnError     func(prepared *canvasChatPreparedRequest)
}

type canvasChatSSEEvent struct {
	Session        *model.CanvasSession     `json:"session,omitempty"`
	Message        *CanvasMessageWithTask   `json:"message,omitempty"`
	Messages       []*CanvasMessageWithTask `json:"messages,omitempty"`
	Delta          string                   `json:"delta,omitempty"`
	ReasoningDelta string                   `json:"reasoning_delta,omitempty"`
	Error          string                   `json:"error,omitempty"`
}

type CanvasChatModelOption struct {
	RequestModel      string   `json:"request_model"`
	DisplayName       string   `json:"display_name"`
	ModelSeries       string   `json:"model_series"`
	RequestEndpoint   string   `json:"request_endpoint"`
	Description       string   `json:"description,omitempty"`
	VendorIcon        string   `json:"vendor_icon,omitempty"`
	ChatCapabilities  []string `json:"chat_capabilities"`
	Usable            bool     `json:"usable"`
	UnavailableReason string   `json:"unavailable_reason,omitempty"`
	AvailableGroups   []string `json:"available_groups"`
}

type CanvasChatModelMappingDiagnostic struct {
	RequestModel         string   `json:"request_model"`
	DisplayName          string   `json:"display_name"`
	RequestEndpoint      string   `json:"request_endpoint"`
	HasEnabledAbility    bool     `json:"has_enabled_ability"`
	HasCompatibleChannel bool     `json:"has_compatible_channel"`
	VisibleInCanvas      bool     `json:"visible_in_canvas"`
	AbilityGroups        []string `json:"ability_groups"`
	CompatibleGroups     []string `json:"compatible_groups"`
	AvailableGroups      []string `json:"available_groups"`
	WarningMessages      []string `json:"warning_messages,omitempty"`
}

type canvasChatAbilityChannel struct {
	model.Ability
	ChannelType   int `json:"channel_type"`
	ChannelStatus int `json:"channel_status"`
}

type canvasChatUserModelContext struct {
	User            *model.UserBase
	UsableGroups    map[string]string
	CandidateGroups []string
}

type canvasChatTokenRoute struct {
	UserGroup     string
	SelectedGroup string
}

var (
	callCanvasChatRelay           = defaultCallCanvasChatRelay
	queueCanvasChatBackgroundTask = defaultQueueCanvasChatBackgroundTask
	canvasChatSummaryLocks        sync.Map
)

func normalizeCanvasChatAttachmentKind(kind string) string {
	return strings.ToLower(strings.TrimSpace(kind))
}

func normalizeCanvasChatAttachmentMimeType(mimeType string) string {
	normalized := strings.ToLower(strings.TrimSpace(mimeType))
	if idx := strings.Index(normalized, ";"); idx >= 0 {
		normalized = strings.TrimSpace(normalized[:idx])
	}
	return normalized
}

func normalizeCanvasChatAttachments(raw []dto.CanvasChatAttachment) ([]dto.CanvasChatAttachment, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	normalized := make([]dto.CanvasChatAttachment, 0, len(raw))
	imageCount := 0
	fileCount := 0
	for _, attachment := range raw {
		kind := normalizeCanvasChatAttachmentKind(attachment.Kind)
		name := strings.TrimSpace(attachment.Name)
		mimeType := normalizeCanvasChatAttachmentMimeType(attachment.MimeType)
		data := strings.TrimSpace(attachment.Data)
		if kind == "" || name == "" || mimeType == "" || data == "" {
			return nil, fmt.Errorf("chat attachment kind, name, mime_type and data are required")
		}

		detectedMimeType, _, err := DecodeBase64FileData(data)
		if err != nil {
			return nil, fmt.Errorf("invalid chat attachment data: %w", err)
		}
		detectedMimeType = normalizeCanvasChatAttachmentMimeType(detectedMimeType)
		if detectedMimeType == "" {
			return nil, fmt.Errorf("invalid chat attachment mime type")
		}
		if mimeType != detectedMimeType {
			return nil, fmt.Errorf("chat attachment mime type mismatch: %s", name)
		}

		switch kind {
		case "image":
			if !strings.HasPrefix(detectedMimeType, "image/") {
				return nil, fmt.Errorf("chat image attachment must use image/* mime type")
			}
			imageCount++
			if imageCount > canvasChatMaxImageAttachments {
				return nil, fmt.Errorf("each chat message supports at most %d image attachment", canvasChatMaxImageAttachments)
			}
		case "file":
			switch detectedMimeType {
			case "application/pdf", "text/plain":
			default:
				return nil, fmt.Errorf("chat file attachment mime type %s is not supported", detectedMimeType)
			}
			fileCount++
			if fileCount > canvasChatMaxFileAttachments {
				return nil, fmt.Errorf("each chat message supports at most %d file attachment", canvasChatMaxFileAttachments)
			}
		default:
			return nil, fmt.Errorf("unsupported chat attachment kind: %s", attachment.Kind)
		}

		normalized = append(normalized, dto.CanvasChatAttachment{
			Kind:     kind,
			Name:     name,
			MimeType: detectedMimeType,
			Data:     data,
		})
	}
	return normalized, nil
}

func enforceCanvasChatAttachmentCapabilities(chatMapping *model.ModelMapping, attachments []dto.CanvasChatAttachment) error {
	if len(attachments) == 0 {
		return nil
	}
	if chatMapping == nil {
		return fmt.Errorf("chat model is required")
	}

	hasImageUpload, err := model.HasChatCapability(chatMapping.ChatCapabilities, model.ChatCapabilityImageUpload)
	if err != nil {
		return err
	}
	hasFileUpload, err := model.HasChatCapability(chatMapping.ChatCapabilities, model.ChatCapabilityFileUpload)
	if err != nil {
		return err
	}

	for _, attachment := range attachments {
		switch attachment.Kind {
		case "image":
			if !hasImageUpload {
				return fmt.Errorf("chat model %s does not support image upload", strings.TrimSpace(chatMapping.RequestModel))
			}
		case "file":
			if !hasFileUpload {
				return fmt.Errorf("chat model %s does not support file upload", strings.TrimSpace(chatMapping.RequestModel))
			}
		}
	}
	return nil
}

func parseCanvasChatMessageMetadata(raw string) (*canvasChatMessageMetadata, error) {
	if strings.TrimSpace(raw) == "" {
		return &canvasChatMessageMetadata{}, nil
	}
	var metadata canvasChatMessageMetadata
	if err := common.UnmarshalJsonStr(raw, &metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}

func extractCanvasChatAttachmentsFromMetadata(raw string) []dto.CanvasChatAttachment {
	metadata, err := parseCanvasChatMessageMetadata(raw)
	if err != nil || metadata == nil || len(metadata.Attachments) == 0 {
		return nil
	}
	attachments, err := normalizeCanvasChatAttachments(metadata.Attachments)
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to parse canvas chat attachments from metadata: %v", err))
		return nil
	}
	return attachments
}

func cloneCanvasChatCapabilities(capabilities []string) []string {
	if len(capabilities) == 0 {
		return []string{}
	}
	return append([]string(nil), capabilities...)
}

func modelSupportsCanvasChatWebSearch(chatMapping *model.ModelMapping) (bool, error) {
	if chatMapping == nil {
		return false, nil
	}
	return model.HasChatCapability(chatMapping.ChatCapabilities, model.ChatCapabilityWebSearch)
}

func buildCanvasChatRelayMessage(role string, prompt string, attachments []dto.CanvasChatAttachment) (dto.Message, error) {
	message := dto.Message{
		Role: role,
	}
	if len(attachments) == 0 {
		message.Content = prompt
		return message, nil
	}

	content := make([]any, 0, len(attachments)+1)
	content = append(content, map[string]any{
		"type": dto.ContentTypeText,
		"text": prompt,
	})
	for _, attachment := range attachments {
		switch attachment.Kind {
		case "image":
			content = append(content, map[string]any{
				"type": dto.ContentTypeImageURL,
				"image_url": map[string]any{
					"url":    attachment.Data,
					"detail": "high",
				},
			})
		case "file":
			content = append(content, map[string]any{
				"type": dto.ContentTypeFile,
				"file": map[string]any{
					"filename":  attachment.Name,
					"file_data": attachment.Data,
				},
			})
		default:
			return dto.Message{}, fmt.Errorf("unsupported chat attachment kind: %s", attachment.Kind)
		}
	}
	message.Content = content
	return message, nil
}

func createCanvasChatMessage(ctx context.Context, userId int, sessionId int, session *model.CanvasSession, input CreateCanvasMessageInput) ([]*CanvasMessageWithTask, error) {
	prepared, err := prepareCanvasChatMessage(userId, sessionId, session, input)
	if err != nil {
		return nil, err
	}
	return executeCanvasChatRun(ctx, prepared, nil)
}

func StreamCanvasChatMessage(c *gin.Context, userId int, sessionId int, input CreateCanvasMessageInput) error {
	if c == nil {
		return fmt.Errorf("stream context is required")
	}
	session, err := model.GetCanvasSessionByID(userId, sessionId)
	if err != nil {
		return err
	}
	if session == nil {
		return fmt.Errorf("canvas session not found")
	}
	prepared, err := prepareCanvasChatMessage(userId, sessionId, session, input)
	if err != nil {
		return err
	}

	relayhelper.SetEventStreamHeaders(c)
	initialMessages := attachCanvasMessageTasks(userId, []*model.CanvasMessage{
		prepared.UserMessage,
		prepared.AssistantMessage,
	})
	if err := emitCanvasChatEvent(c, "canvas.message.created", canvasChatSSEEvent{
		Session:  prepared.Session,
		Messages: initialMessages,
	}); err != nil {
		_, _ = finalizeCanvasChatRunError(prepared, nil, err)
		return nil
	}

	callbacks := &canvasChatStreamCallbacks{
		OnDelta: func(prepared *canvasChatPreparedRequest, delta canvasChatRelayDelta) {
			_ = emitCanvasChatEvent(c, "canvas.message.delta", canvasChatSSEEvent{
				Message:        &CanvasMessageWithTask{CanvasMessage: prepared.AssistantMessage},
				Delta:          delta.Content,
				ReasoningDelta: delta.ReasoningContent,
			})
		},
		OnCompleted: func(prepared *canvasChatPreparedRequest) {
			_ = emitCanvasChatEvent(c, "canvas.message.completed", canvasChatSSEEvent{
				Session: prepared.Session,
				Message: &CanvasMessageWithTask{CanvasMessage: prepared.AssistantMessage},
			})
		},
		OnError: func(prepared *canvasChatPreparedRequest) {
			if c.Request != nil && c.Request.Context().Err() != nil {
				return
			}
			_ = emitCanvasChatEvent(c, "canvas.message.error", canvasChatSSEEvent{
				Session: prepared.Session,
				Message: &CanvasMessageWithTask{CanvasMessage: prepared.AssistantMessage},
				Error:   prepared.AssistantMessage.ErrorMessage,
			})
		},
	}
	_, _ = executeCanvasChatRun(c.Request.Context(), prepared, callbacks)
	return nil
}

func prepareCanvasChatMessage(userId int, sessionId int, session *model.CanvasSession, input CreateCanvasMessageInput) (*canvasChatPreparedRequest, error) {
	if session == nil {
		return nil, fmt.Errorf("canvas session not found")
	}
	if session.Mode != model.CanvasModeChat {
		return nil, fmt.Errorf("canvas session is not in chat mode")
	}

	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	user, err := model.GetUserCache(userId)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	chatMapping, err := resolveCanvasChatModelMapping(userId, session, input.ModelId)
	if err != nil {
		return nil, err
	}
	finalModel := strings.TrimSpace(chatMapping.RequestModel)
	attachments, err := normalizeCanvasChatAttachments(input.Attachments)
	if err != nil {
		return nil, err
	}
	if err := enforceCanvasChatAttachmentCapabilities(chatMapping, attachments); err != nil {
		return nil, err
	}
	finalGroup, err := resolveCanvasChatGroup(userId, user.Group, session, input.Group, chatMapping)
	if err != nil {
		return nil, err
	}

	temperatureValue := resolveCanvasChatTemperature(session, input.Temperature)
	contextCount := resolveCanvasChatContextCount(session, input.ContextCount)
	webSearchEnabled, err := resolveCanvasChatWebSearchEnabled(chatMapping, session, input.WebSearchEnabled)
	if err != nil {
		return nil, err
	}
	assistantMetadata, err := buildCanvasChatMessageMetadata(session, finalModel, finalGroup, temperatureValue, contextCount, webSearchEnabled, nil)
	if err != nil {
		return nil, err
	}
	userMetadata := assistantMetadata
	if len(attachments) > 0 {
		userMetadata, err = buildCanvasChatMessageMetadata(session, finalModel, finalGroup, temperatureValue, contextCount, webSearchEnabled, attachments)
		if err != nil {
			return nil, err
		}
	}

	now := common.GetTimestamp()
	clientRequestId := strings.TrimSpace(input.ClientRequestId)
	prepared := &canvasChatPreparedRequest{
		UserId:           userId,
		UserGroup:        user.Group,
		Session:          session,
		Prompt:           prompt,
		FinalModel:       finalModel,
		RequestEndpoint:  strings.TrimSpace(chatMapping.RequestEndpoint),
		FinalGroup:       finalGroup,
		Temperature:      temperatureValue,
		ContextCount:     contextCount,
		WebSearchEnabled: webSearchEnabled,
		ClientRequestId:  clientRequestId,
		UserMessage: &model.CanvasMessage{
			SessionId:       sessionId,
			UserId:          userId,
			Mode:            model.CanvasModeChat,
			Role:            model.CanvasMessageRoleUser,
			Prompt:          prompt,
			ClientRequestId: clientRequestId,
			Status:          model.CanvasMessageStatusSuccess,
			Metadata:        userMetadata,
			CreatedTime:     now,
			UpdatedTime:     now,
		},
		AssistantMessage: &model.CanvasMessage{
			SessionId:       sessionId,
			UserId:          userId,
			Mode:            model.CanvasModeChat,
			Role:            model.CanvasMessageRoleAssistant,
			Prompt:          "",
			ClientRequestId: clientRequestId,
			Status:          model.CanvasMessageStatusGenerating,
			Metadata:        assistantMetadata,
			CreatedTime:     now,
			UpdatedTime:     now,
		},
	}

	messageCount, err := model.CountCanvasMessagesByMode(model.CanvasModeChat, userId, session.Id)
	if err != nil {
		return nil, err
	}
	if err := model.CreateCanvasMessagesForMode(model.CanvasModeChat, []*model.CanvasMessage{
		prepared.UserMessage,
		prepared.AssistantMessage,
	}); err != nil {
		return nil, err
	}

	sessionUpdates := map[string]interface{}{
		"current_model":      finalModel,
		"current_group":      finalGroup,
		"chat_temperature":   normalizeCanvasChatTemperatureValue(temperatureValue, session.ChatTemperature),
		"chat_context_count": normalizeCanvasChatContextCountValue(common.GetPointer(contextCount), session.ChatContextCount),
		"web_search_enabled": webSearchEnabled,
		"updated_time":       now,
	}
	if !session.TitleManuallySet && messageCount == 0 {
		sessionUpdates["title"] = truncateCanvasTitle(prompt)
	}
	if err := model.UpdateCanvasSessionFields(userId, session.Id, sessionUpdates); err != nil {
		cleanupCreatedCanvasMessages(model.CanvasModeChat, userId, []*model.CanvasMessage{
			prepared.UserMessage,
			prepared.AssistantMessage,
		})
		return nil, err
	}

	prepared.Session.CurrentModel = finalModel
	prepared.Session.CurrentGroup = finalGroup
	prepared.Session.WebSearchEnabled = webSearchEnabled
	if !prepared.Session.TitleManuallySet && messageCount == 0 {
		prepared.Session.Title = truncateCanvasTitle(prompt)
	}
	prepared.Session.UpdatedTime = now
	return prepared, nil
}

func executeCanvasChatRun(ctx context.Context, prepared *canvasChatPreparedRequest, callbacks *canvasChatStreamCallbacks) ([]*CanvasMessageWithTask, error) {
	if prepared == nil {
		return nil, fmt.Errorf("canvas chat request is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	relayMessages, err := buildCanvasChatRelayMessages(prepared)
	if err != nil {
		return finalizeCanvasChatRunError(prepared, callbacks, err)
	}

	var fullTextBuilder strings.Builder
	var fullReasoningBuilder strings.Builder
	var pendingTextDelta strings.Builder
	var pendingReasoningDelta strings.Builder
	lastPersistedPrompt := ""
	lastPersistedReasoningContent := ""
	lastFlushTime := time.Now()

	flushAssistantDelta := func(force bool) error {
		currentPrompt := fullTextBuilder.String()
		currentReasoningContent := fullReasoningBuilder.String()
		shouldPersist := currentPrompt != lastPersistedPrompt || currentReasoningContent != lastPersistedReasoningContent
		if !force && pendingTextDelta.Len() == 0 && pendingReasoningDelta.Len() == 0 && !shouldPersist {
			return nil
		}
		if shouldPersist {
			if err := model.UpdateCanvasMessageFieldsByMode(model.CanvasModeChat, prepared.UserId, prepared.AssistantMessage.Id, map[string]interface{}{
				"prompt":            currentPrompt,
				"reasoning_content": currentReasoningContent,
				"status":            model.CanvasMessageStatusGenerating,
				"error_message":     "",
			}); err != nil {
				return err
			}
			prepared.AssistantMessage.Prompt = currentPrompt
			prepared.AssistantMessage.ReasoningContent = currentReasoningContent
			prepared.AssistantMessage.Status = model.CanvasMessageStatusGenerating
			prepared.AssistantMessage.ErrorMessage = ""
			lastPersistedPrompt = currentPrompt
			lastPersistedReasoningContent = currentReasoningContent
		}
		if callbacks != nil && callbacks.OnDelta != nil &&
			(pendingTextDelta.Len() > 0 || pendingReasoningDelta.Len() > 0) {
			callbacks.OnDelta(prepared, canvasChatRelayDelta{
				Content:          pendingTextDelta.String(),
				ReasoningContent: pendingReasoningDelta.String(),
			})
		}
		pendingTextDelta.Reset()
		pendingReasoningDelta.Reset()
		lastFlushTime = time.Now()
		return nil
	}

	_, err = callCanvasChatRelay(ctx, canvasChatRelayRequest{
		UserId:           prepared.UserId,
		UserGroup:        prepared.UserGroup,
		ModelId:          prepared.FinalModel,
		RequestEndpoint:  prepared.RequestEndpoint,
		Group:            prepared.FinalGroup,
		Temperature:      prepared.Temperature,
		WebSearchEnabled: prepared.WebSearchEnabled,
		Messages:         relayMessages,
	}, func(delta canvasChatRelayDelta) error {
		if delta.Content == "" && delta.ReasoningContent == "" {
			return nil
		}
		fullTextBuilder.WriteString(delta.Content)
		fullReasoningBuilder.WriteString(delta.ReasoningContent)
		pendingTextDelta.WriteString(delta.Content)
		pendingReasoningDelta.WriteString(delta.ReasoningContent)
		if pendingTextDelta.Len()+pendingReasoningDelta.Len() >= canvasChatDeltaFlushMinChars ||
			time.Since(lastFlushTime) >= canvasChatDeltaFlushInterval {
			return flushAssistantDelta(false)
		}
		return nil
	})
	if err != nil {
		_ = flushAssistantDelta(true)
		return finalizeCanvasChatRunError(prepared, callbacks, err)
	}

	if err := flushAssistantDelta(true); err != nil {
		return finalizeCanvasChatRunError(prepared, callbacks, err)
	}
	if err := model.UpdateCanvasMessageFieldsByMode(model.CanvasModeChat, prepared.UserId, prepared.AssistantMessage.Id, map[string]interface{}{
		"prompt":            fullTextBuilder.String(),
		"reasoning_content": fullReasoningBuilder.String(),
		"status":            model.CanvasMessageStatusSuccess,
		"error_message":     "",
	}); err != nil {
		return nil, err
	}

	prepared.AssistantMessage.Prompt = fullTextBuilder.String()
	prepared.AssistantMessage.ReasoningContent = fullReasoningBuilder.String()
	prepared.AssistantMessage.Status = model.CanvasMessageStatusSuccess
	prepared.AssistantMessage.ErrorMessage = ""

	if callbacks != nil && callbacks.OnCompleted != nil {
		callbacks.OnCompleted(prepared)
	}
	queueCanvasChatSummary(prepared)
	return attachCanvasMessageTasks(prepared.UserId, []*model.CanvasMessage{
		prepared.UserMessage,
		prepared.AssistantMessage,
	}), nil
}

func finalizeCanvasChatRunError(prepared *canvasChatPreparedRequest, callbacks *canvasChatStreamCallbacks, runErr error) ([]*CanvasMessageWithTask, error) {
	if prepared == nil {
		return nil, runErr
	}
	status := model.CanvasMessageStatusFailed
	errorMessage := normalizeCanvasChatRunError(runErr)
	if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) {
		status = model.CanvasMessageStatusStopped
		errorMessage = "聊天已停止"
	}
	if err := model.UpdateCanvasMessageFieldsByMode(model.CanvasModeChat, prepared.UserId, prepared.AssistantMessage.Id, map[string]interface{}{
		"prompt":            prepared.AssistantMessage.Prompt,
		"reasoning_content": prepared.AssistantMessage.ReasoningContent,
		"status":            status,
		"error_message":     errorMessage,
	}); err != nil {
		return nil, err
	}
	prepared.AssistantMessage.Status = status
	prepared.AssistantMessage.ErrorMessage = errorMessage
	if callbacks != nil && callbacks.OnError != nil {
		callbacks.OnError(prepared)
	}
	return attachCanvasMessageTasks(prepared.UserId, []*model.CanvasMessage{
		prepared.UserMessage,
		prepared.AssistantMessage,
	}), runErr
}

func resolveCanvasChatModel(userId int, session *model.CanvasSession, inputModel string) (string, error) {
	mapping, err := resolveCanvasChatModelMapping(userId, session, inputModel)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(mapping.RequestModel), nil
}

func resolveCanvasChatModelMapping(userId int, session *model.CanvasSession, inputModel string) (*model.ModelMapping, error) {
	candidates := []string{
		strings.TrimSpace(inputModel),
		strings.TrimSpace(session.CurrentModel),
	}
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		mapping, _, err := getUsableCanvasChatModelMappingForUser(userId, candidate)
		return mapping, err
	}
	options, err := ListUserCanvasChatModelOptions(userId)
	if err != nil {
		return nil, err
	}
	for _, option := range options {
		if !option.Usable {
			continue
		}
		mapping, err := model.GetActiveModelMappingByRequestModel(option.RequestModel)
		if err != nil {
			return nil, err
		}
		if mapping != nil {
			return mapping, nil
		}
	}
	return nil, fmt.Errorf("no available chat model")
}

func resolveCanvasChatGroup(userId int, userGroup string, session *model.CanvasSession, inputGroup string, mapping *model.ModelMapping) (string, error) {
	if mapping == nil {
		return "", fmt.Errorf("chat model mapping is required")
	}
	modelId := strings.TrimSpace(mapping.RequestModel)
	tryGroups := make([]string, 0, 2)
	appendCandidate := func(group string) {
		group = strings.TrimSpace(group)
		if group == "" {
			return
		}
		for _, existing := range tryGroups {
			if existing == group {
				return
			}
		}
		tryGroups = append(tryGroups, group)
	}
	appendCandidate(session.CurrentGroup)
	appendCandidate(inputGroup)

	for _, group := range tryGroups {
		ok, err := canvasChatGroupSupportsModel(userId, userGroup, group, modelId)
		if err != nil {
			return "", err
		}
		if ok {
			return group, nil
		}
	}

	candidateGroups, err := listCanvasChatCandidateGroups(userId, userGroup)
	if err != nil {
		return "", err
	}
	for _, group := range candidateGroups {
		ok, groupErr := canvasChatGroupSupportsModel(userId, userGroup, group, modelId)
		if groupErr != nil {
			return "", groupErr
		}
		if ok {
			return group, nil
		}
	}
	return "", fmt.Errorf("no available group for chat model %s", modelId)
}

func ListUserCanvasChatModels(userId int) ([]string, error) {
	return listUserCanvasChatModels(userId)
}

func ListUserCanvasChatModelCatalog(userId int) ([]*dto.CanvasChatModelCatalogItem, error) {
	return listUserCanvasChatModelCatalog(userId)
}

func ListUserCanvasChatModelOptions(userId int) ([]CanvasChatModelOption, error) {
	return listUserCanvasChatModelOptions(userId)
}

func listUserCanvasChatModels(userId int) ([]string, error) {
	options, err := listUserCanvasChatModelOptions(userId)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(options))
	for _, option := range options {
		if !option.Usable {
			continue
		}
		result = append(result, option.RequestModel)
	}
	return result, nil
}

func listUserCanvasChatModelCatalog(userId int) ([]*dto.CanvasChatModelCatalogItem, error) {
	options, err := listUserCanvasChatModelOptions(userId)
	if err != nil {
		return nil, err
	}
	catalog := make([]*dto.CanvasChatModelCatalogItem, 0, len(options))
	for _, option := range options {
		if !option.Usable {
			continue
		}
		displayName := strings.TrimSpace(option.DisplayName)
		if displayName == "" {
			displayName = strings.TrimSpace(option.RequestModel)
		}
		catalog = append(catalog, &dto.CanvasChatModelCatalogItem{
			RequestModel:     strings.TrimSpace(option.RequestModel),
			DisplayName:      displayName,
			ModelSeries:      strings.TrimSpace(option.ModelSeries),
			RequestEndpoint:  strings.TrimSpace(option.RequestEndpoint),
			Description:      strings.TrimSpace(option.Description),
			VendorIcon:       strings.TrimSpace(option.VendorIcon),
			ChatCapabilities: cloneCanvasChatCapabilities(option.ChatCapabilities),
		})
	}
	sort.Slice(catalog, func(i, j int) bool {
		return catalog[i].RequestModel < catalog[j].RequestModel
	})
	return catalog, nil
}

func listUserCanvasChatModelOptions(userId int) ([]CanvasChatModelOption, error) {
	ctx, err := buildCanvasChatUserModelContext(userId)
	if err != nil {
		return nil, err
	}

	activeMappings, _, err := model.GetActiveChatModelMappings(0, 1000)
	if err != nil {
		return nil, err
	}
	requestModels := make([]string, 0, len(activeMappings))
	for _, mapping := range activeMappings {
		if mapping == nil {
			continue
		}
		requestModel := strings.TrimSpace(mapping.RequestModel)
		if requestModel == "" {
			continue
		}
		requestModels = append(requestModels, requestModel)
	}
	recordsByModel, err := listCanvasChatAbilityChannelsByModels(requestModels)
	if err != nil {
		return nil, err
	}
	metadataByModel, err := model.GetCatalogDisplayMetadataByModelNames(requestModels)
	if err != nil {
		return nil, err
	}

	options := make([]CanvasChatModelOption, 0, len(activeMappings))
	for _, mapping := range activeMappings {
		if mapping == nil {
			continue
		}
		option, visibleInCanvas, err := buildCanvasChatModelOptionForUserContext(
			ctx,
			mapping,
			recordsByModel[strings.TrimSpace(mapping.RequestModel)],
		)
		if err != nil {
			return nil, err
		}
		if !visibleInCanvas {
			continue
		}
		if metadata, ok := metadataByModel[strings.TrimSpace(mapping.RequestModel)]; ok {
			option.Description = strings.TrimSpace(metadata.Description)
			option.VendorIcon = strings.TrimSpace(metadata.VendorIcon)
		}
		options = append(options, *option)
	}
	return options, nil
}

func buildCanvasChatUserModelContext(userId int) (*canvasChatUserModelContext, error) {
	user, err := model.GetUserCache(userId)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	candidateGroups, err := listCanvasChatCandidateGroups(userId, user.Group)
	if err != nil {
		return nil, err
	}
	return &canvasChatUserModelContext{
		User:            user,
		UsableGroups:    GetUserUsableGroups(user.Group),
		CandidateGroups: candidateGroups,
	}, nil
}

func buildCanvasChatModelOptionForUserContext(ctx *canvasChatUserModelContext, mapping *model.ModelMapping, records []canvasChatAbilityChannel) (*CanvasChatModelOption, bool, error) {
	if ctx == nil || ctx.User == nil {
		return nil, false, fmt.Errorf("canvas chat user context is required")
	}
	if mapping == nil {
		return nil, false, fmt.Errorf("canvas chat model mapping is required")
	}

	userHasEnabledAbility := false
	userHasEnabledChannel := false
	for _, record := range records {
		groupName := strings.TrimSpace(record.Group)
		if groupName == "" {
			continue
		}
		if _, ok := ctx.UsableGroups[groupName]; !ok {
			continue
		}
		userHasEnabledAbility = true
		if record.ChannelStatus != common.ChannelStatusEnabled {
			continue
		}
		userHasEnabledChannel = true
	}

	availableGroups := make([]string, 0, len(ctx.CandidateGroups))
	for _, groupName := range ctx.CandidateGroups {
		ok, err := canvasChatGroupSupportsModel(ctx.User.Id, ctx.User.Group, groupName, mapping.RequestModel)
		if err != nil {
			return nil, false, err
		}
		if ok {
			availableGroups = append(availableGroups, groupName)
		}
	}

	chatCapabilities, err := model.EffectiveChatCapabilities(mapping.ChatCapabilities)
	if err != nil {
		return nil, false, err
	}
	option := &CanvasChatModelOption{
		RequestModel:     strings.TrimSpace(mapping.RequestModel),
		DisplayName:      strings.TrimSpace(mapping.DisplayName),
		ModelSeries:      strings.TrimSpace(mapping.ModelSeries),
		RequestEndpoint:  strings.TrimSpace(mapping.RequestEndpoint),
		ChatCapabilities: cloneCanvasChatCapabilities(chatCapabilities),
		Usable:           len(availableGroups) > 0,
		AvailableGroups:  availableGroups,
	}
	if !option.Usable {
		option.UnavailableReason = buildCanvasChatModelUnavailableReason(userHasEnabledAbility, userHasEnabledChannel)
	}
	return option, userHasEnabledChannel, nil
}

func getUsableCanvasChatModelMappingForUser(userId int, requestModel string) (*model.ModelMapping, *CanvasChatModelOption, error) {
	requestModel = strings.TrimSpace(requestModel)
	if requestModel == "" {
		return nil, nil, fmt.Errorf("chat model is required")
	}

	mapping, err := model.GetActiveModelMappingByRequestModel(requestModel)
	if err != nil {
		return nil, nil, err
	}
	if mapping == nil || mapping.ModelType != 1 || strings.TrimSpace(mapping.RequestEndpoint) == "" {
		return nil, nil, fmt.Errorf("聊天模型 %s 不存在或未启用", requestModel)
	}

	ctx, err := buildCanvasChatUserModelContext(userId)
	if err != nil {
		return nil, nil, err
	}
	recordsByModel, err := listCanvasChatAbilityChannelsByModels([]string{requestModel})
	if err != nil {
		return nil, nil, err
	}
	option, _, err := buildCanvasChatModelOptionForUserContext(ctx, mapping, recordsByModel[requestModel])
	if err != nil {
		return nil, nil, err
	}
	if option.Usable {
		return mapping, option, nil
	}
	reason := strings.TrimSpace(option.UnavailableReason)
	if reason == "" {
		reason = "当前不可用"
	}
	return nil, option, fmt.Errorf("聊天模型 %s 当前不可用：%s", requestModel, reason)
}

func listCanvasChatAbilityChannelsByModels(requestModels []string) (map[string][]canvasChatAbilityChannel, error) {
	modelSet := make(map[string]struct{}, len(requestModels))
	filteredModels := make([]string, 0, len(requestModels))
	for _, requestModel := range requestModels {
		requestModel = strings.TrimSpace(requestModel)
		if requestModel == "" {
			continue
		}
		if _, ok := modelSet[requestModel]; ok {
			continue
		}
		modelSet[requestModel] = struct{}{}
		filteredModels = append(filteredModels, requestModel)
	}
	result := make(map[string][]canvasChatAbilityChannel, len(filteredModels))
	if len(filteredModels) == 0 {
		return result, nil
	}

	var records []canvasChatAbilityChannel
	err := model.DB.Table("abilities").
		Select("abilities.*, channels.type as channel_type, channels.status as channel_status").
		Joins("left join channels on abilities.channel_id = channels.id").
		Where("abilities.enabled = ? AND abilities.model IN ?", true, filteredModels).
		Scan(&records).Error
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		requestModel := strings.TrimSpace(record.Model)
		if requestModel == "" {
			continue
		}
		result[requestModel] = append(result[requestModel], record)
	}
	return result, nil
}

func listValidCanvasChatTokenRoutes() ([]canvasChatTokenRoute, error) {
	var tokens []*model.Token
	now := common.GetTimestamp()
	if err := model.DB.
		Where("status = ?", common.TokenStatusEnabled).
		Where("(expired_time = -1 OR expired_time > ?)", now).
		Where("(unlimited_quota = ? OR remain_quota > 0)", true).
		Find(&tokens).Error; err != nil {
		return nil, err
	}

	userGroups := make(map[int]string, len(tokens))
	routes := make([]canvasChatTokenRoute, 0, len(tokens))
	for _, token := range tokens {
		if token == nil {
			continue
		}
		userGroup, ok := userGroups[token.UserId]
		if !ok {
			user, err := model.GetUserById(token.UserId, false)
			if err != nil {
				return nil, err
			}
			if user == nil || user.Status != common.UserStatusEnabled {
				userGroups[token.UserId] = ""
				continue
			}
			userGroup = strings.TrimSpace(user.Group)
			userGroups[token.UserId] = userGroup
		}
		if userGroup == "" {
			continue
		}

		selectedGroup := strings.TrimSpace(token.Group)
		if selectedGroup == "" {
			selectedGroup = userGroup
		}
		if selectedGroup == "" {
			continue
		}
		routes = append(routes, canvasChatTokenRoute{
			UserGroup:     userGroup,
			SelectedGroup: selectedGroup,
		})
	}
	return routes, nil
}

func buildCanvasChatModelUnavailableReason(userHasEnabledAbility bool, userHasEnabledChannel bool) string {
	if !userHasEnabledAbility {
		return "当前用户可用分组里没有任何启用能力声明该模型"
	}
	if !userHasEnabledChannel {
		return "当前用户可用分组里没有任何可达渠道支持该模型"
	}
	return "当前没有可达分组/令牌支持该模型"
}

func DiagnoseCanvasChatModelMapping(requestModel string) (*CanvasChatModelMappingDiagnostic, error) {
	requestModel = strings.TrimSpace(requestModel)
	if requestModel == "" {
		return nil, nil
	}

	mapping, err := model.GetModelMappingByRequestModel(requestModel)
	if err != nil {
		return nil, err
	}
	if mapping == nil || mapping.ModelType != 1 || mapping.Status != 1 || strings.TrimSpace(mapping.RequestEndpoint) == "" {
		return nil, nil
	}

	recordsByModel, err := listCanvasChatAbilityChannelsByModels([]string{requestModel})
	if err != nil {
		return nil, err
	}
	abilityGroups := make(map[string]struct{})
	compatibleGroups := make(map[string]struct{})
	for _, record := range recordsByModel[requestModel] {
		groupName := strings.TrimSpace(record.Group)
		if groupName == "" {
			continue
		}
		abilityGroups[groupName] = struct{}{}
		if record.ChannelStatus != common.ChannelStatusEnabled {
			continue
		}
		compatibleGroups[groupName] = struct{}{}
	}

	tokenRoutes, err := listValidCanvasChatTokenRoutes()
	if err != nil {
		return nil, err
	}
	availableGroups := make(map[string]struct{})
	for _, route := range tokenRoutes {
		ok, groupErr := canvasChatSelectedGroupHasChannel(route.UserGroup, route.SelectedGroup, requestModel)
		if groupErr != nil {
			return nil, groupErr
		}
		if ok {
			availableGroups[route.SelectedGroup] = struct{}{}
		}
	}

	warnings := make([]string, 0, 1)
	switch {
	case len(abilityGroups) == 0:
		warnings = append(warnings, "映射已保存，但没有任何启用能力声明该模型，Canvas 不会显示")
	case len(compatibleGroups) == 0:
		warnings = append(warnings, "映射已保存，但没有任何启用渠道声明该模型，Canvas 不会显示")
	case len(availableGroups) == 0:
		warnings = append(warnings, "映射已保存，但当前没有可达分组/令牌支持该模型")
	}

	return &CanvasChatModelMappingDiagnostic{
		RequestModel:         requestModel,
		DisplayName:          strings.TrimSpace(mapping.DisplayName),
		RequestEndpoint:      strings.TrimSpace(mapping.RequestEndpoint),
		HasEnabledAbility:    len(abilityGroups) > 0,
		HasCompatibleChannel: len(compatibleGroups) > 0,
		VisibleInCanvas:      len(availableGroups) > 0,
		AbilityGroups:        sortedCanvasChatGroupNames(abilityGroups),
		CompatibleGroups:     sortedCanvasChatGroupNames(compatibleGroups),
		AvailableGroups:      sortedCanvasChatGroupNames(availableGroups),
		WarningMessages:      warnings,
	}, nil
}

func sortedCanvasChatGroupNames(groupSet map[string]struct{}) []string {
	if len(groupSet) == 0 {
		return []string{}
	}
	groups := make([]string, 0, len(groupSet))
	for groupName := range groupSet {
		groupName = strings.TrimSpace(groupName)
		if groupName == "" {
			continue
		}
		groups = append(groups, groupName)
	}
	sort.Strings(groups)
	return groups
}

func listCanvasChatCandidateGroups(userId int, userGroup string) ([]string, error) {
	tokens, err := model.GetUserAvailableTokens(userId)
	if err != nil {
		return nil, fmt.Errorf("failed to get user tokens: %w", err)
	}
	usableGroups := GetUserUsableGroups(userGroup)
	groupSet := make(map[string]struct{})
	for _, token := range tokens {
		if token == nil {
			continue
		}
		groupName := strings.TrimSpace(token.Group)
		if groupName == "" {
			groupName = strings.TrimSpace(userGroup)
		}
		if groupName == "" {
			continue
		}
		if groupName != "auto" {
			if _, ok := usableGroups[groupName]; !ok {
				continue
			}
		}
		groupSet[groupName] = struct{}{}
	}
	groups := make([]string, 0, len(groupSet))
	for groupName := range groupSet {
		groups = append(groups, groupName)
	}
	sort.Strings(groups)
	return groups, nil
}

func canvasChatGroupSupportsModel(userId int, userGroup string, group string, modelId string) (bool, error) {
	group = strings.TrimSpace(group)
	modelId = strings.TrimSpace(modelId)
	if group == "" || modelId == "" {
		return false, nil
	}
	tokens, err := getUserAvailableTokensByGroup(userId, userGroup, group)
	if err != nil {
		return false, err
	}
	if len(tokens) == 0 {
		return false, nil
	}
	return canvasChatSelectedGroupHasChannel(userGroup, group, modelId)
}

func canvasChatSelectedGroupHasChannel(userGroup string, selectedGroup string, modelId string) (bool, error) {
	selectedGroup = strings.TrimSpace(selectedGroup)
	modelId = strings.TrimSpace(modelId)
	if selectedGroup == "" || modelId == "" {
		return false, nil
	}
	if selectedGroup == "auto" {
		for _, autoGroup := range GetUserAutoGroup(userGroup) {
			channel, channelErr := model.GetRandomSatisfiedChannel(autoGroup, modelId, 0)
			if channelErr != nil {
				return false, channelErr
			}
			if channel != nil {
				return true, nil
			}
		}
		return false, nil
	}
	channel, err := model.GetRandomSatisfiedChannel(selectedGroup, modelId, 0)
	if err != nil {
		return false, err
	}
	return channel != nil, nil
}

func resolveCanvasChatTemperature(session *model.CanvasSession, input *float64) *float64 {
	fallback := canvasChatDefaultTemperature
	if session != nil {
		fallback = session.ChatTemperature
	}
	return common.GetPointer(normalizeCanvasChatTemperatureValue(input, fallback))
}

func resolveCanvasChatContextCount(session *model.CanvasSession, input *int) int {
	fallback := canvasChatDefaultContextCount
	if session != nil {
		fallback = session.ChatContextCount
	}
	return normalizeCanvasChatContextCountValue(input, fallback)
}

func resolveCanvasChatWebSearchEnabled(chatMapping *model.ModelMapping, session *model.CanvasSession, input *bool) (bool, error) {
	supported, err := modelSupportsCanvasChatWebSearch(chatMapping)
	if err != nil {
		return false, err
	}
	if !supported {
		return false, nil
	}
	fallback := false
	if session != nil {
		fallback = session.WebSearchEnabled
	}
	return normalizeCanvasChatWebSearchEnabledValue(input, fallback), nil
}

func buildCanvasChatMessageMetadata(session *model.CanvasSession, modelId string, group string, temperature *float64, contextCount int, webSearchEnabled bool, attachments []dto.CanvasChatAttachment) (string, error) {
	systemPrompt := ""
	summaryEnabled := canvasChatSummaryEnabledDefault
	summaryTriggerMessages := canvasChatSummaryTriggerMessagesDefault
	summaryRecentMessages := canvasChatSummaryRecentMessagesDefault
	if session != nil {
		systemPrompt = strings.TrimSpace(session.SystemPrompt)
		summaryEnabled = session.SummaryEnabled
		summaryTriggerMessages = session.SummaryTriggerMessages
		summaryRecentMessages = session.SummaryRecentMessages
	}
	metadata, err := common.Marshal(canvasChatMessageMetadata{
		ChatModel:              strings.TrimSpace(modelId),
		ChatGroup:              strings.TrimSpace(group),
		Temperature:            temperature,
		ContextCount:           common.GetPointer(contextCount),
		WebSearchEnabled:       common.GetPointer(webSearchEnabled),
		SystemPrompt:           systemPrompt,
		SummaryEnabled:         common.GetPointer(summaryEnabled),
		SummaryTriggerMessages: common.GetPointer(summaryTriggerMessages),
		SummaryRecentMessages:  common.GetPointer(summaryRecentMessages),
		Attachments:            attachments,
	})
	if err != nil {
		return "", err
	}
	return string(metadata), nil
}

func buildCanvasChatRelayMessages(prepared *canvasChatPreparedRequest) ([]dto.Message, error) {
	if prepared == nil {
		return nil, fmt.Errorf("canvas chat request is nil")
	}

	effectiveSummary, summaryCursor, err := getCanvasChatActiveSummary(prepared.Session)
	if err != nil {
		return nil, err
	}
	cutoffID := prepared.Session.ClearContextMessageId
	if prepared.Session.SummaryEnabled && summaryCursor > cutoffID {
		cutoffID = summaryCursor
	}

	filteredHistory := []*model.CanvasMessage{}
	if prepared.ContextCount > 0 {
		filteredHistory, err = listRecentCanvasSuccessfulChatMessages(
			prepared.UserId,
			prepared.Session.Id,
			cutoffID,
			prepared.UserMessage.Id,
			prepared.ContextCount*2,
		)
		if err != nil {
			return nil, err
		}
	}

	systemRole := canvasChatSystemRole(prepared.FinalModel)
	relayMessages := make([]dto.Message, 0, len(filteredHistory)+3)
	if systemPrompt := strings.TrimSpace(prepared.Session.SystemPrompt); systemPrompt != "" {
		relayMessages = append(relayMessages, dto.Message{
			Role:    systemRole,
			Content: systemPrompt,
		})
	}
	if prepared.Session.SummaryEnabled && effectiveSummary != "" {
		relayMessages = append(relayMessages, dto.Message{
			Role:    systemRole,
			Content: "以下是当前对话的摘要记忆，请仅将其作为历史上下文使用：\n" + effectiveSummary,
		})
	}
	for _, message := range filteredHistory {
		historyAttachments := []dto.CanvasChatAttachment(nil)
		if message.Role == model.CanvasMessageRoleUser {
			historyAttachments = extractCanvasChatAttachmentsFromMetadata(message.Metadata)
		}
		relayMessage, relayErr := buildCanvasChatRelayMessage(message.Role, message.Prompt, historyAttachments)
		if relayErr != nil {
			return nil, relayErr
		}
		relayMessages = append(relayMessages, relayMessage)
	}
	currentAttachments := []dto.CanvasChatAttachment(nil)
	if prepared.UserMessage != nil {
		currentAttachments = extractCanvasChatAttachmentsFromMetadata(prepared.UserMessage.Metadata)
	}
	currentMessage, err := buildCanvasChatRelayMessage(model.CanvasMessageRoleUser, prepared.Prompt, currentAttachments)
	if err != nil {
		return nil, err
	}
	relayMessages = append(relayMessages, currentMessage)
	return relayMessages, nil
}

func listCanvasSuccessfulChatMessages(userId int, sessionId int, beforeMessageId int) ([]*model.CanvasMessage, error) {
	return model.ListSuccessfulCanvasMessagesBefore(model.CanvasModeChat, userId, sessionId, beforeMessageId)
}

func listRecentCanvasSuccessfulChatMessages(userId int, sessionId int, afterMessageId int, beforeMessageId int, limit int) ([]*model.CanvasMessage, error) {
	return model.ListRecentSuccessfulCanvasMessages(model.CanvasModeChat, userId, sessionId, afterMessageId, beforeMessageId, limit)
}

func countCanvasSuccessfulChatMessagesAfter(userId int, sessionId int, afterMessageId int) (int64, error) {
	return model.CountSuccessfulCanvasMessagesAfter(model.CanvasModeChat, userId, sessionId, afterMessageId)
}

func listCanvasSuccessfulChatMessagesAfter(userId int, sessionId int, afterMessageId int, limit int) ([]*model.CanvasMessage, error) {
	return model.ListSuccessfulCanvasMessagesAfter(model.CanvasModeChat, userId, sessionId, afterMessageId, limit)
}

func defaultCallCanvasChatRelay(ctx context.Context, request canvasChatRelayRequest, onDelta func(delta canvasChatRelayDelta) error) (*canvasChatRelayResult, error) {
	relayRequest := dto.GeneralOpenAIRequest{
		Model:       request.ModelId,
		Messages:    request.Messages,
		Stream:      common.GetPointer(true),
		Temperature: request.Temperature,
		StreamOptions: &dto.StreamOptions{
			IncludeUsage: true,
		},
	}
	applyCanvasChatRelayWebSearch(&relayRequest, request.RequestEndpoint, request.WebSearchEnabled)
	jsonData, err := common.Marshal(relayRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chat relay request: %w", err)
	}

	tokenKey, err := getUserValidTokenByGroup(request.UserId, request.UserGroup, request.Group)
	if err != nil {
		return nil, fmt.Errorf("failed to get user token: %w", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	requestURL := fmt.Sprintf("http://127.0.0.1:%s/v1/chat/completions", port)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create chat relay request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+tokenKey)
	if strings.EqualFold(strings.TrimSpace(request.RequestEndpoint), "openai-response") {
		SetCanvasChatResponsesCompatHeader(httpReq.Header)
	}

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send chat relay request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chat relay error: %s", parseCanvasChatRelayError(body, resp.StatusCode))
	}

	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read chat relay response: %w", err)
		}
		result, parseErr := parseCanvasChatResponseBody(body)
		if parseErr != nil {
			return nil, parseErr
		}
		if onDelta != nil && (result.Text != "" || result.ReasoningContent != "") {
			if err := onDelta(canvasChatRelayDelta{
				Content:          result.Text,
				ReasoningContent: result.ReasoningContent,
			}); err != nil {
				return nil, err
			}
		}
		return result, nil
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, canvasChatRelayScannerInitialBufferSize), canvasChatRelayScannerMaxBufferSize)

	var fullText strings.Builder
	var fullReasoning strings.Builder
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") || strings.HasPrefix(line, "event:") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			break
		}

		var chunk dto.ChatCompletionsStreamResponse
		if err := common.Unmarshal(common.StringToByteSlice(payload), &chunk); err != nil {
			return nil, fmt.Errorf("failed to decode chat relay chunk: %w", err)
		}
		delta := extractCanvasChatDelta(&chunk)
		if delta.Content == "" && delta.ReasoningContent == "" {
			continue
		}
		fullText.WriteString(delta.Content)
		fullReasoning.WriteString(delta.ReasoningContent)
		if onDelta != nil {
			if err := onDelta(delta); err != nil {
				return nil, err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return &canvasChatRelayResult{
			Text:             fullText.String(),
			ReasoningContent: fullReasoning.String(),
		}, err
	}

	return &canvasChatRelayResult{
		Text:             fullText.String(),
		ReasoningContent: fullReasoning.String(),
	}, nil
}

func applyCanvasChatRelayWebSearch(relayRequest *dto.GeneralOpenAIRequest, requestEndpoint string, enabled bool) {
	if relayRequest == nil || !enabled {
		return
	}
	switch strings.ToLower(strings.TrimSpace(requestEndpoint)) {
	case "openai", "openai-response", "anthropic":
		relayRequest.WebSearchOptions = &dto.WebSearchOptions{
			SearchContextSize: "medium",
		}
	}
}

func extractCanvasChatDelta(chunk *dto.ChatCompletionsStreamResponse) canvasChatRelayDelta {
	delta := canvasChatRelayDelta{}
	if chunk == nil {
		return delta
	}
	for _, choice := range chunk.Choices {
		delta.Content += choice.Delta.GetContentString()
		delta.ReasoningContent += choice.Delta.GetReasoningContent()
	}
	return delta
}

func parseCanvasChatResponseBody(body []byte) (*canvasChatRelayResult, error) {
	var response dto.OpenAITextResponse
	if err := common.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to decode chat relay response: %w", err)
	}
	if len(response.Choices) == 0 {
		return &canvasChatRelayResult{}, nil
	}
	message := response.Choices[0].Message
	reasoningContent := strings.TrimSpace(message.ReasoningContent)
	if reasoningContent == "" {
		reasoningContent = message.Reasoning
	}
	return &canvasChatRelayResult{
		Text:             message.StringContent(),
		ReasoningContent: reasoningContent,
	}, nil
}

func parseCanvasChatRelayError(body []byte, statusCode int) string {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return fmt.Sprintf("status=%d", statusCode)
	}
	var parsed struct {
		Message string `json:"message"`
		Error   any    `json:"error"`
	}
	if err := common.Unmarshal(body, &parsed); err == nil {
		if strings.TrimSpace(parsed.Message) != "" {
			return parsed.Message
		}
		if parsed.Error != nil {
			return strings.TrimSpace(fmt.Sprintf("%v", parsed.Error))
		}
	}
	return fmt.Sprintf("status=%d, body=%s", statusCode, string(body))
}

func normalizeCanvasChatRunError(err error) string {
	if err == nil {
		return "聊天失败"
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return "聊天失败"
	}
	return message
}

func emitCanvasChatEvent(c *gin.Context, eventType string, payload canvasChatSSEEvent) error {
	if c == nil {
		return fmt.Errorf("stream context is nil")
	}
	jsonData, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	c.Render(-1, common.CustomEvent{Data: fmt.Sprintf("event: %s\n", eventType)})
	c.Render(-1, common.CustomEvent{Data: "data: " + string(jsonData)})
	return relayhelper.FlushWriter(c)
}

func queueCanvasChatSummary(prepared *canvasChatPreparedRequest) {
	if prepared == nil {
		return
	}
	sessionID := prepared.Session.Id
	lockValue, _ := canvasChatSummaryLocks.LoadOrStore(sessionID, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	queueCanvasChatBackgroundTask(func() {
		lock.Lock()
		defer lock.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), canvasChatSummaryRequestTimeout)
		defer cancel()
		if err := summarizeCanvasChatSession(ctx, prepared.UserId, sessionID, prepared.FinalModel, prepared.ContextCount); err != nil {
			common.SysLog(fmt.Sprintf("Failed to summarize canvas chat session %d: %v", sessionID, err))
		}
	})
}

func summarizeCanvasChatSession(ctx context.Context, userId int, sessionId int, fallbackModel string, contextCount int) error {
	session, err := model.GetCanvasSessionByID(userId, sessionId)
	if err != nil {
		return err
	}
	if session == nil || session.Mode != model.CanvasModeChat {
		return nil
	}

	chatMapping, err := resolveCanvasChatModelMapping(userId, session, fallbackModel)
	if err != nil {
		return err
	}
	modelId := strings.TrimSpace(chatMapping.RequestModel)
	user, err := model.GetUserCache(userId)
	if err != nil {
		return err
	}
	if user == nil {
		return fmt.Errorf("user not found")
	}
	group, err := resolveCanvasChatGroup(userId, user.Group, session, session.CurrentGroup, chatMapping)
	if err != nil {
		return err
	}

	existingSummary, lastSummarizedMessageID, err := getCanvasChatActiveSummary(session)
	if err != nil {
		return err
	}
	startID := session.ClearContextMessageId
	if lastSummarizedMessageID > startID {
		startID = lastSummarizedMessageID
	}

	contextCount = resolveCanvasChatContextCount(session, common.GetPointer(contextCount))
	if !session.SummaryEnabled {
		return nil
	}
	recentWindowMessages := session.SummaryRecentMessages
	contextWindowMessages := contextCount * 2
	if contextWindowMessages > recentWindowMessages {
		recentWindowMessages = contextWindowMessages
	}
	triggerMessages := session.SummaryTriggerMessages
	totalUnsummarized, err := countCanvasSuccessfulChatMessagesAfter(userId, sessionId, startID)
	if err != nil {
		return err
	}
	if totalUnsummarized <= int64(recentWindowMessages+triggerMessages) {
		return nil
	}
	summaryCount := int(totalUnsummarized) - recentWindowMessages
	if summaryCount <= 0 {
		return nil
	}
	toSummarize, err := listCanvasSuccessfulChatMessagesAfter(userId, sessionId, startID, summaryCount)
	if err != nil {
		return err
	}
	if len(toSummarize) == 0 {
		return nil
	}
	cutoffMessageID := toSummarize[len(toSummarize)-1].Id

	summaryMessages := buildCanvasChatSummaryRelayMessages(modelId, existingSummary, toSummarize)
	result, err := callCanvasChatRelay(ctx, canvasChatRelayRequest{
		UserId:      userId,
		UserGroup:   user.Group,
		ModelId:     modelId,
		Group:       group,
		Temperature: common.GetPointer(canvasChatDefaultTemperature),
		Messages:    summaryMessages,
	}, nil)
	if err != nil {
		return err
	}
	summaryText := sanitizeCanvasChatSummary(result.Text)
	if summaryText == "" {
		return nil
	}

	reloadedSession, err := model.GetCanvasSessionByID(userId, sessionId)
	if err != nil {
		return err
	}
	if reloadedSession == nil || reloadedSession.Mode != model.CanvasModeChat {
		return nil
	}
	if !reloadedSession.SummaryEnabled {
		return nil
	}
	if reloadedSession.ClearContextMessageId != session.ClearContextMessageId {
		return nil
	}
	if reloadedSession.LastSummarizedMessageId >= cutoffMessageID {
		return nil
	}
	if getCanvasChatSummaryCursorForClearContext(reloadedSession, reloadedSession.ClearContextMessageId) >= cutoffMessageID {
		return nil
	}
	storedSummaryPrompt, storedLastSummarizedMessageID, err := setCanvasChatSummary(
		reloadedSession,
		reloadedSession.ClearContextMessageId,
		summaryText,
		cutoffMessageID,
	)
	if err != nil {
		return err
	}
	return model.UpdateCanvasSessionFields(userId, sessionId, map[string]interface{}{
		"summary_prompt":             storedSummaryPrompt,
		"last_summarized_message_id": storedLastSummarizedMessageID,
	})
}

func parseCanvasChatSummaryState(session *model.CanvasSession) (*canvasChatSummaryState, error) {
	state := &canvasChatSummaryState{}
	if session == nil {
		return state, nil
	}
	raw := strings.TrimSpace(session.SummaryPrompt)
	if raw != "" && strings.HasPrefix(raw, "{") {
		parsed := &canvasChatSummaryState{}
		if err := common.UnmarshalJsonStr(raw, parsed); err == nil {
			normalizeCanvasChatSummaryState(parsed)
			if parsed.Default != nil || len(parsed.Branches) > 0 {
				return parsed, nil
			}
		}
	}
	if raw != "" || session.LastSummarizedMessageId > 0 {
		state.Default = &canvasChatSummaryBranch{
			ClearContextMessageId:   0,
			LastSummarizedMessageId: session.LastSummarizedMessageId,
			Prompt:                  raw,
		}
	}
	normalizeCanvasChatSummaryState(state)
	return state, nil
}

func normalizeCanvasChatSummaryState(state *canvasChatSummaryState) {
	if state == nil {
		return
	}
	normalizeBranch := func(branch *canvasChatSummaryBranch) *canvasChatSummaryBranch {
		if branch == nil {
			return nil
		}
		branch.Prompt = strings.TrimSpace(branch.Prompt)
		if branch.Prompt == "" || branch.LastSummarizedMessageId <= 0 {
			return nil
		}
		return branch
	}
	state.Default = normalizeBranch(state.Default)
	normalizedBranches := make([]canvasChatSummaryBranch, 0, len(state.Branches))
	for _, branch := range state.Branches {
		copied := branch
		if normalized := normalizeBranch(&copied); normalized != nil {
			normalizedBranches = append(normalizedBranches, *normalized)
		}
	}
	sort.Slice(normalizedBranches, func(i, j int) bool {
		if normalizedBranches[i].ClearContextMessageId != normalizedBranches[j].ClearContextMessageId {
			return normalizedBranches[i].ClearContextMessageId < normalizedBranches[j].ClearContextMessageId
		}
		return normalizedBranches[i].LastSummarizedMessageId < normalizedBranches[j].LastSummarizedMessageId
	})
	state.Branches = normalizedBranches
}

func getCanvasChatSummaryBranch(state *canvasChatSummaryState, clearContextMessageID int) *canvasChatSummaryBranch {
	if state == nil {
		return nil
	}
	if clearContextMessageID == 0 {
		return state.Default
	}
	for i := range state.Branches {
		if state.Branches[i].ClearContextMessageId == clearContextMessageID {
			return &state.Branches[i]
		}
	}
	return nil
}

func getCanvasChatActiveSummary(session *model.CanvasSession) (string, int, error) {
	state, err := parseCanvasChatSummaryState(session)
	if err != nil {
		return "", 0, err
	}
	branch := getCanvasChatSummaryBranch(state, session.ClearContextMessageId)
	if branch == nil {
		return "", 0, nil
	}
	return strings.TrimSpace(branch.Prompt), branch.LastSummarizedMessageId, nil
}

func setCanvasChatSummary(session *model.CanvasSession, clearContextMessageID int, prompt string, lastSummarizedMessageID int) (string, int, error) {
	state, err := parseCanvasChatSummaryState(session)
	if err != nil {
		return "", 0, err
	}
	prompt = sanitizeCanvasChatSummary(prompt)
	if clearContextMessageID == 0 {
		if prompt == "" || lastSummarizedMessageID <= 0 {
			state.Default = nil
		} else {
			state.Default = &canvasChatSummaryBranch{
				ClearContextMessageId:   0,
				LastSummarizedMessageId: lastSummarizedMessageID,
				Prompt:                  prompt,
			}
		}
	} else {
		found := false
		for i := range state.Branches {
			if state.Branches[i].ClearContextMessageId != clearContextMessageID {
				continue
			}
			found = true
			if prompt == "" || lastSummarizedMessageID <= 0 {
				state.Branches = append(state.Branches[:i], state.Branches[i+1:]...)
			} else {
				state.Branches[i].Prompt = prompt
				state.Branches[i].LastSummarizedMessageId = lastSummarizedMessageID
			}
			break
		}
		if !found && prompt != "" && lastSummarizedMessageID > 0 {
			state.Branches = append(state.Branches, canvasChatSummaryBranch{
				ClearContextMessageId:   clearContextMessageID,
				LastSummarizedMessageId: lastSummarizedMessageID,
				Prompt:                  prompt,
			})
		}
	}
	normalizeCanvasChatSummaryState(state)
	if state.Default == nil && len(state.Branches) == 0 {
		return "", 0, nil
	}
	if len(state.Branches) == 0 && state.Default != nil {
		return state.Default.Prompt, state.Default.LastSummarizedMessageId, nil
	}
	payload, err := common.Marshal(state)
	if err != nil {
		return "", 0, err
	}
	return string(payload), lastSummarizedMessageID, nil
}

func getCanvasChatSummaryCursorForClearContext(session *model.CanvasSession, clearContextMessageID int) int {
	state, err := parseCanvasChatSummaryState(session)
	if err != nil {
		return session.LastSummarizedMessageId
	}
	branch := getCanvasChatSummaryBranch(state, clearContextMessageID)
	if branch == nil {
		return 0
	}
	return branch.LastSummarizedMessageId
}

func buildCanvasChatSummaryRelayMessages(modelId string, existingSummary string, messages []*model.CanvasMessage) []dto.Message {
	systemRole := canvasChatSystemRole(modelId)
	parts := []string{
		"你负责维护长对话摘要记忆。",
		"请保留用户偏好、事实、约束、上下文约定、仍未解决的问题。",
		"忽略寒暄和重复内容，输出简洁中文要点。",
	}
	relayMessages := []dto.Message{
		{
			Role:    systemRole,
			Content: strings.Join(parts, "\n"),
		},
	}
	if strings.TrimSpace(existingSummary) != "" {
		relayMessages = append(relayMessages, dto.Message{
			Role:    systemRole,
			Content: "已有摘要：\n" + strings.TrimSpace(existingSummary),
		})
	}
	relayMessages = append(relayMessages, dto.Message{
		Role:    model.CanvasMessageRoleUser,
		Content: buildCanvasChatSummaryTranscript(messages),
	})
	return relayMessages
}

func buildCanvasChatSummaryTranscript(messages []*model.CanvasMessage) string {
	var builder strings.Builder
	builder.WriteString("请基于以下对话生成新的完整摘要：\n")
	for _, message := range messages {
		if message == nil {
			continue
		}
		builder.WriteString(message.Role)
		builder.WriteString(": ")
		builder.WriteString(strings.TrimSpace(message.Prompt))
		builder.WriteString("\n")
	}
	return builder.String()
}

func sanitizeCanvasChatSummary(summary string) string {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return ""
	}
	runes := []rune(summary)
	if len(runes) <= canvasChatSummaryMaxLengthRunes {
		return summary
	}
	return strings.TrimSpace(string(runes[:canvasChatSummaryMaxLengthRunes]))
}

func canvasChatSystemRole(modelId string) string {
	request := dto.GeneralOpenAIRequest{Model: modelId}
	return request.GetSystemRoleName()
}

func defaultQueueCanvasChatBackgroundTask(fn func()) {
	if fn == nil {
		return
	}
	gopool.Go(fn)
}
