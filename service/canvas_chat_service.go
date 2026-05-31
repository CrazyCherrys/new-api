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
	"gorm.io/gorm"
)

const (
	canvasChatDefaultTemperature            = 0.7
	canvasChatDefaultContextCount           = 8
	canvasChatSummaryEnabledDefault         = true
	canvasChatSummaryTriggerMessagesDefault = 8
	canvasChatSummaryRecentMessagesDefault  = 8
	canvasChatMaxContextCount               = 64
	canvasChatRelayScannerInitialBufferSize = 64 << 10
	canvasChatRelayScannerMaxBufferSize     = 64 << 20
	canvasChatDeltaFlushMinChars            = 48
	canvasChatDeltaFlushInterval            = 700 * time.Millisecond
	canvasChatSummaryRequestTimeout         = 2 * time.Minute
	canvasChatSummaryMaxLengthRunes         = 6000
)

type canvasChatMessageMetadata struct {
	ChatModel              string   `json:"chat_model,omitempty"`
	ChatGroup              string   `json:"chat_group,omitempty"`
	Temperature            *float64 `json:"temperature,omitempty"`
	ContextCount           *int     `json:"context_count,omitempty"`
	SystemPrompt           string   `json:"system_prompt,omitempty"`
	SummaryEnabled         *bool    `json:"summary_enabled,omitempty"`
	SummaryTriggerMessages *int     `json:"summary_trigger_messages,omitempty"`
	SummaryRecentMessages  *int     `json:"summary_recent_messages,omitempty"`
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
	FinalGroup       string
	Temperature      *float64
	ContextCount     int
	Metadata         string
	ClientRequestId  string
	UserMessage      *model.CanvasMessage
	AssistantMessage *model.CanvasMessage
}

type canvasChatRelayRequest struct {
	UserId      int
	UserGroup   string
	ModelId     string
	Group       string
	Temperature *float64
	Messages    []dto.Message
}

type canvasChatRelayResult struct {
	Text string
}

type canvasChatStreamCallbacks struct {
	OnDelta     func(prepared *canvasChatPreparedRequest, delta string)
	OnCompleted func(prepared *canvasChatPreparedRequest)
	OnError     func(prepared *canvasChatPreparedRequest)
}

type canvasChatSSEEvent struct {
	Session  *model.CanvasSession     `json:"session,omitempty"`
	Message  *CanvasMessageWithTask   `json:"message,omitempty"`
	Messages []*CanvasMessageWithTask `json:"messages,omitempty"`
	Delta    string                   `json:"delta,omitempty"`
	Error    string                   `json:"error,omitempty"`
}

var (
	callCanvasChatRelay           = defaultCallCanvasChatRelay
	queueCanvasChatBackgroundTask = defaultQueueCanvasChatBackgroundTask
	canvasChatSummaryLocks        sync.Map
)

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
		OnDelta: func(prepared *canvasChatPreparedRequest, delta string) {
			_ = emitCanvasChatEvent(c, "canvas.message.delta", canvasChatSSEEvent{
				Message: &CanvasMessageWithTask{CanvasMessage: prepared.AssistantMessage},
				Delta:   delta,
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

	finalModel, err := resolveCanvasChatModel(userId, session, input.ModelId)
	if err != nil {
		return nil, err
	}
	finalGroup, err := resolveCanvasChatGroup(userId, user.Group, session, input.Group, finalModel)
	if err != nil {
		return nil, err
	}

	temperatureValue := resolveCanvasChatTemperature(session, input.Temperature)
	contextCount := resolveCanvasChatContextCount(session, input.ContextCount)
	metadata, err := buildCanvasChatMessageMetadata(session, finalModel, finalGroup, temperatureValue, contextCount)
	if err != nil {
		return nil, err
	}

	now := common.GetTimestamp()
	clientRequestId := strings.TrimSpace(input.ClientRequestId)
	prepared := &canvasChatPreparedRequest{
		UserId:          userId,
		UserGroup:       user.Group,
		Session:         session,
		Prompt:          prompt,
		FinalModel:      finalModel,
		FinalGroup:      finalGroup,
		Temperature:     temperatureValue,
		ContextCount:    contextCount,
		Metadata:        metadata,
		ClientRequestId: clientRequestId,
		UserMessage: &model.CanvasMessage{
			SessionId:       sessionId,
			UserId:          userId,
			Mode:            model.CanvasModeChat,
			Role:            model.CanvasMessageRoleUser,
			Prompt:          prompt,
			ClientRequestId: clientRequestId,
			Status:          model.CanvasMessageStatusSuccess,
			Metadata:        metadata,
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
			Metadata:        metadata,
			CreatedTime:     now,
			UpdatedTime:     now,
		},
	}

	err = model.DB.Transaction(func(tx *gorm.DB) error {
		sessionUpdates := map[string]interface{}{
			"current_model":      finalModel,
			"current_group":      finalGroup,
			"chat_temperature":   normalizeCanvasChatTemperatureValue(temperatureValue, session.ChatTemperature),
			"chat_context_count": normalizeCanvasChatContextCountValue(common.GetPointer(contextCount), session.ChatContextCount),
			"updated_time":       now,
		}
		if !session.TitleManuallySet {
			var messageCount int64
			if err := tx.Model(&model.CanvasMessage{}).
				Where("user_id = ? AND session_id = ? AND deleted_time = 0", userId, session.Id).
				Count(&messageCount).Error; err != nil {
				return err
			}
			if messageCount == 0 {
				sessionUpdates["title"] = truncateCanvasTitle(prompt)
			}
		}
		if err := tx.Model(&model.CanvasSession{}).
			Where("id = ? AND user_id = ? AND deleted_time = 0", session.Id, userId).
			Updates(sessionUpdates).Error; err != nil {
			return err
		}
		if err := tx.Create(prepared.UserMessage).Error; err != nil {
			return err
		}
		if err := tx.Create(prepared.AssistantMessage).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	prepared.Session.CurrentModel = finalModel
	prepared.Session.CurrentGroup = finalGroup
	if !prepared.Session.TitleManuallySet {
		messageCount, countErr := model.CountCanvasMessages(userId, session.Id)
		if countErr == nil && messageCount == 2 {
			prepared.Session.Title = truncateCanvasTitle(prompt)
		}
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
	var pendingDelta strings.Builder
	lastPersistedPrompt := ""
	lastFlushTime := time.Now()

	flushAssistantDelta := func(force bool) error {
		currentPrompt := fullTextBuilder.String()
		shouldPersist := currentPrompt != lastPersistedPrompt
		if !force && pendingDelta.Len() == 0 && !shouldPersist {
			return nil
		}
		if shouldPersist {
			if err := model.UpdateCanvasMessageFields(prepared.UserId, prepared.AssistantMessage.Id, map[string]interface{}{
				"prompt":        currentPrompt,
				"status":        model.CanvasMessageStatusGenerating,
				"error_message": "",
			}); err != nil {
				return err
			}
			prepared.AssistantMessage.Prompt = currentPrompt
			prepared.AssistantMessage.Status = model.CanvasMessageStatusGenerating
			prepared.AssistantMessage.ErrorMessage = ""
			lastPersistedPrompt = currentPrompt
		}
		if callbacks != nil && pendingDelta.Len() > 0 && callbacks.OnDelta != nil {
			callbacks.OnDelta(prepared, pendingDelta.String())
		}
		pendingDelta.Reset()
		lastFlushTime = time.Now()
		return nil
	}

	_, err = callCanvasChatRelay(ctx, canvasChatRelayRequest{
		UserId:      prepared.UserId,
		UserGroup:   prepared.UserGroup,
		ModelId:     prepared.FinalModel,
		Group:       prepared.FinalGroup,
		Temperature: prepared.Temperature,
		Messages:    relayMessages,
	}, func(delta string) error {
		if delta == "" {
			return nil
		}
		fullTextBuilder.WriteString(delta)
		pendingDelta.WriteString(delta)
		if pendingDelta.Len() >= canvasChatDeltaFlushMinChars || time.Since(lastFlushTime) >= canvasChatDeltaFlushInterval {
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
	if err := model.UpdateCanvasMessageFields(prepared.UserId, prepared.AssistantMessage.Id, map[string]interface{}{
		"prompt":        fullTextBuilder.String(),
		"status":        model.CanvasMessageStatusSuccess,
		"error_message": "",
	}); err != nil {
		return nil, err
	}

	prepared.AssistantMessage.Prompt = fullTextBuilder.String()
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
	if err := model.UpdateCanvasMessageFields(prepared.UserId, prepared.AssistantMessage.Id, map[string]interface{}{
		"prompt":        prepared.AssistantMessage.Prompt,
		"status":        status,
		"error_message": errorMessage,
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
	candidates := []string{
		strings.TrimSpace(inputModel),
		strings.TrimSpace(session.CurrentModel),
	}
	for _, candidate := range candidates {
		if candidate != "" {
			return candidate, nil
		}
	}
	models, err := listUserCanvasChatModels(userId)
	if err != nil {
		return "", err
	}
	if len(models) == 0 {
		return "", fmt.Errorf("no available chat model")
	}
	return models[0], nil
}

func resolveCanvasChatGroup(userId int, userGroup string, session *model.CanvasSession, inputGroup string, modelId string) (string, error) {
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

func listUserCanvasChatModels(userId int) ([]string, error) {
	user, err := model.GetUserCache(userId)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	modelSet := make(map[string]struct{})
	for group := range GetUserUsableGroups(user.Group) {
		for _, modelName := range model.GetGroupEnabledModels(group) {
			trimmed := strings.TrimSpace(modelName)
			if trimmed == "" {
				continue
			}
			modelSet[trimmed] = struct{}{}
		}
	}
	result := make([]string, 0, len(modelSet))
	for modelName := range modelSet {
		result = append(result, modelName)
	}
	sort.Strings(result)
	return result, nil
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
	if group == "auto" {
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
	channel, err := model.GetRandomSatisfiedChannel(group, modelId, 0)
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

func buildCanvasChatMessageMetadata(session *model.CanvasSession, modelId string, group string, temperature *float64, contextCount int) (string, error) {
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
		SystemPrompt:           systemPrompt,
		SummaryEnabled:         common.GetPointer(summaryEnabled),
		SummaryTriggerMessages: common.GetPointer(summaryTriggerMessages),
		SummaryRecentMessages:  common.GetPointer(summaryRecentMessages),
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
	history, err := listCanvasSuccessfulChatMessages(prepared.UserId, prepared.Session.Id, prepared.UserMessage.Id)
	if err != nil {
		return nil, err
	}

	effectiveSummary, summaryCursor, err := getCanvasChatActiveSummary(prepared.Session)
	if err != nil {
		return nil, err
	}
	cutoffID := prepared.Session.ClearContextMessageId
	if prepared.Session.SummaryEnabled && summaryCursor > cutoffID {
		cutoffID = summaryCursor
	}

	filteredHistory := make([]*model.CanvasMessage, 0, len(history))
	for _, message := range history {
		if message == nil || message.Id <= cutoffID {
			continue
		}
		filteredHistory = append(filteredHistory, message)
	}

	if prepared.ContextCount > 0 {
		maxMessages := prepared.ContextCount * 2
		if len(filteredHistory) > maxMessages {
			filteredHistory = filteredHistory[len(filteredHistory)-maxMessages:]
		}
	} else {
		filteredHistory = filteredHistory[:0]
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
		relayMessages = append(relayMessages, dto.Message{
			Role:    message.Role,
			Content: message.Prompt,
		})
	}
	relayMessages = append(relayMessages, dto.Message{
		Role:    model.CanvasMessageRoleUser,
		Content: prepared.Prompt,
	})
	return relayMessages, nil
}

func listCanvasSuccessfulChatMessages(userId int, sessionId int, beforeMessageId int) ([]*model.CanvasMessage, error) {
	var messages []*model.CanvasMessage
	query := model.DB.Where("user_id = ? AND session_id = ? AND mode = ? AND status = ? AND deleted_time = 0",
		userId, sessionId, model.CanvasModeChat, model.CanvasMessageStatusSuccess)
	if beforeMessageId > 0 {
		query = query.Where("id < ?", beforeMessageId)
	}
	err := query.Order("created_time ASC").Order("id ASC").Find(&messages).Error
	return messages, err
}

func defaultCallCanvasChatRelay(ctx context.Context, request canvasChatRelayRequest, onDelta func(delta string) error) (*canvasChatRelayResult, error) {
	relayRequest := dto.GeneralOpenAIRequest{
		Model:       request.ModelId,
		Messages:    request.Messages,
		Stream:      common.GetPointer(true),
		Temperature: request.Temperature,
		StreamOptions: &dto.StreamOptions{
			IncludeUsage: true,
		},
	}
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
		text, parseErr := parseCanvasChatResponseBody(body)
		if parseErr != nil {
			return nil, parseErr
		}
		if onDelta != nil && text != "" {
			if err := onDelta(text); err != nil {
				return nil, err
			}
		}
		return &canvasChatRelayResult{Text: text}, nil
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, canvasChatRelayScannerInitialBufferSize), canvasChatRelayScannerMaxBufferSize)

	var fullText strings.Builder
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
		deltaText := extractCanvasChatDeltaText(&chunk)
		if deltaText == "" {
			continue
		}
		fullText.WriteString(deltaText)
		if onDelta != nil {
			if err := onDelta(deltaText); err != nil {
				return nil, err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return &canvasChatRelayResult{Text: fullText.String()}, err
	}

	return &canvasChatRelayResult{Text: fullText.String()}, nil
}

func extractCanvasChatDeltaText(chunk *dto.ChatCompletionsStreamResponse) string {
	if chunk == nil {
		return ""
	}
	var builder strings.Builder
	for _, choice := range chunk.Choices {
		builder.WriteString(choice.Delta.GetContentString())
	}
	return builder.String()
}

func parseCanvasChatResponseBody(body []byte) (string, error) {
	var response dto.OpenAITextResponse
	if err := common.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to decode chat relay response: %w", err)
	}
	if len(response.Choices) == 0 {
		return "", nil
	}
	return response.Choices[0].Message.StringContent(), nil
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

	modelId, err := resolveCanvasChatModel(userId, session, fallbackModel)
	if err != nil {
		return err
	}
	user, err := model.GetUserCache(userId)
	if err != nil {
		return err
	}
	if user == nil {
		return fmt.Errorf("user not found")
	}
	group, err := resolveCanvasChatGroup(userId, user.Group, session, session.CurrentGroup, modelId)
	if err != nil {
		return err
	}

	history, err := listCanvasSuccessfulChatMessages(userId, sessionId, 0)
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
	unsummarized := make([]*model.CanvasMessage, 0, len(history))
	for _, message := range history {
		if message == nil || message.Id <= startID {
			continue
		}
		unsummarized = append(unsummarized, message)
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
	if len(unsummarized) <= recentWindowMessages+triggerMessages {
		return nil
	}
	summaryCount := len(unsummarized) - recentWindowMessages
	if summaryCount <= 0 {
		return nil
	}
	toSummarize := unsummarized[:summaryCount]
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
