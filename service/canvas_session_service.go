package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

type CanvasMessageWithTask struct {
	*model.CanvasMessage
	ImageTask *model.ImageGenerationTaskSummary `json:"image_task,omitempty"`
	VideoTask *dto.VideoGenerationTaskSummary   `json:"video_task,omitempty"`
}

type CreateCanvasSessionInput struct {
	Mode                   string
	Title                  string
	CurrentModel           string
	ChatTemperature        *float64
	ChatContextCount       *int
	SystemPrompt           *string
	SummaryEnabled         *bool
	SummaryTriggerMessages *int
	SummaryRecentMessages  *int
}

type UpdateCanvasSessionInput struct {
	Title                  *string
	Pinned                 *bool
	CurrentModel           *string
	ChatTemperature        *float64
	ChatContextCount       *int
	SystemPrompt           *string
	SummaryEnabled         *bool
	SummaryTriggerMessages *int
	SummaryRecentMessages  *int
	ClearContextMessageId  *int
	ClearContextToLatest   *bool
}

type CreateCanvasMessageInput struct {
	Prompt          string
	ModelId         string
	Group           string
	RequestEndpoint string
	Params          string
	Stream          *bool
	Temperature     *float64
	ContextCount    *int
	ClientRequestId string
}

var (
	createImageGenerationTaskForCanvas = CreateImageGenerationTask
	createVideoGenerationTaskForCanvas = CreateVideoGenerationTask
)

const (
	defaultCanvasSessionListLimit = 20
	maxCanvasSessionListLimit     = 100
)

type CanvasSessionListPage struct {
	Items   []*model.CanvasSession `json:"items"`
	HasMore bool                   `json:"has_more"`
}

func ListCanvasSessions(userId int, mode string, limit int, offset int) (*CanvasSessionListPage, error) {
	rawMode := strings.TrimSpace(mode)
	normalizedMode := model.NormalizeCanvasMode(rawMode)
	if rawMode != "" && normalizedMode == "" {
		return nil, fmt.Errorf("invalid canvas mode")
	}
	if limit <= 0 {
		limit = defaultCanvasSessionListLimit
	}
	if limit > maxCanvasSessionListLimit {
		limit = maxCanvasSessionListLimit
	}
	if offset < 0 {
		offset = 0
	}
	items, hasMore, err := model.ListCanvasSessions(userId, normalizedMode, limit, offset)
	if err != nil {
		return nil, err
	}
	return &CanvasSessionListPage{
		Items:   items,
		HasMore: hasMore,
	}, nil
}

func CreateCanvasSession(userId int, input CreateCanvasSessionInput) (*model.CanvasSession, error) {
	mode := model.NormalizeCanvasMode(input.Mode)
	if mode == "" {
		return nil, fmt.Errorf("invalid canvas mode")
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = defaultCanvasSessionTitle(mode)
	}
	session := &model.CanvasSession{
		UserId:           userId,
		Mode:             mode,
		Title:            truncateCanvasTitle(title),
		CurrentModel:     strings.TrimSpace(input.CurrentModel),
		TitleManuallySet: strings.TrimSpace(input.Title) != "",
	}
	chatTemperature := canvasChatDefaultTemperature
	chatContextCount := canvasChatDefaultContextCount
	systemPrompt := ""
	summaryEnabled := canvasChatSummaryEnabledDefault
	summaryTriggerMessages := canvasChatSummaryTriggerMessagesDefault
	summaryRecentMessages := canvasChatSummaryRecentMessagesDefault
	if mode == model.CanvasModeChat {
		chatTemperature = normalizeCanvasChatTemperatureValue(input.ChatTemperature, canvasChatDefaultTemperature)
		chatContextCount = normalizeCanvasChatContextCountValue(input.ChatContextCount, canvasChatDefaultContextCount)
		systemPrompt = normalizeCanvasChatSystemPromptValue(input.SystemPrompt)
		summaryEnabled = normalizeCanvasChatSummaryEnabledValue(input.SummaryEnabled, canvasChatSummaryEnabledDefault)
		summaryTriggerMessages = normalizeCanvasChatSummaryTriggerMessagesValue(input.SummaryTriggerMessages, canvasChatSummaryTriggerMessagesDefault)
		summaryRecentMessages = normalizeCanvasChatSummaryRecentMessagesValue(input.SummaryRecentMessages, canvasChatSummaryRecentMessagesDefault)
		session.ChatTemperature = chatTemperature
		session.ChatContextCount = chatContextCount
		session.SystemPrompt = systemPrompt
		session.SummaryEnabled = summaryEnabled
		session.SummaryTriggerMessages = summaryTriggerMessages
		session.SummaryRecentMessages = summaryRecentMessages
	}
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Select("*").Create(session).Error; err != nil {
			return err
		}
		if mode != model.CanvasModeChat {
			return nil
		}
		return tx.Exec(
			`UPDATE canvas_sessions
			 SET chat_temperature = ?, chat_context_count = ?, system_prompt = ?, summary_enabled = ?, summary_trigger_messages = ?, summary_recent_messages = ?
			 WHERE id = ? AND user_id = ? AND deleted_time = 0`,
			chatTemperature,
			chatContextCount,
			systemPrompt,
			canvasChatSummaryEnabledDBValue(summaryEnabled),
			summaryTriggerMessages,
			summaryRecentMessages,
			session.Id,
			userId,
		).Error
	})
	if err != nil {
		return nil, err
	}
	return model.GetCanvasSessionByID(userId, session.Id)
}

func UpdateCanvasSession(userId int, id int, input UpdateCanvasSessionInput) (*model.CanvasSession, error) {
	session, err := model.GetCanvasSessionByID(userId, id)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, fmt.Errorf("canvas session not found")
	}

	updates := map[string]interface{}{}
	if input.Title != nil {
		title := truncateCanvasTitle(strings.TrimSpace(*input.Title))
		if title == "" {
			return nil, fmt.Errorf("title is required")
		}
		updates["title"] = title
		updates["title_manually_set"] = true
	}
	if input.Pinned != nil {
		updates["pinned"] = *input.Pinned
	}
	if input.CurrentModel != nil {
		updates["current_model"] = strings.TrimSpace(*input.CurrentModel)
	}
	if input.ChatTemperature != nil {
		if session.Mode != model.CanvasModeChat {
			return nil, fmt.Errorf("chat configuration is only supported for chat sessions")
		}
		updates["chat_temperature"] = normalizeCanvasChatTemperatureValue(input.ChatTemperature, session.ChatTemperature)
	}
	if input.ChatContextCount != nil {
		if session.Mode != model.CanvasModeChat {
			return nil, fmt.Errorf("chat configuration is only supported for chat sessions")
		}
		updates["chat_context_count"] = normalizeCanvasChatContextCountValue(input.ChatContextCount, session.ChatContextCount)
	}
	if input.SystemPrompt != nil {
		if session.Mode != model.CanvasModeChat {
			return nil, fmt.Errorf("chat configuration is only supported for chat sessions")
		}
		updates["system_prompt"] = normalizeCanvasChatSystemPromptValue(input.SystemPrompt)
	}
	if input.SummaryEnabled != nil {
		if session.Mode != model.CanvasModeChat {
			return nil, fmt.Errorf("chat configuration is only supported for chat sessions")
		}
		updates["summary_enabled"] = normalizeCanvasChatSummaryEnabledValue(input.SummaryEnabled, session.SummaryEnabled)
	}
	if input.SummaryTriggerMessages != nil {
		if session.Mode != model.CanvasModeChat {
			return nil, fmt.Errorf("chat configuration is only supported for chat sessions")
		}
		updates["summary_trigger_messages"] = normalizeCanvasChatSummaryTriggerMessagesValue(input.SummaryTriggerMessages, session.SummaryTriggerMessages)
	}
	if input.SummaryRecentMessages != nil {
		if session.Mode != model.CanvasModeChat {
			return nil, fmt.Errorf("chat configuration is only supported for chat sessions")
		}
		updates["summary_recent_messages"] = normalizeCanvasChatSummaryRecentMessagesValue(input.SummaryRecentMessages, session.SummaryRecentMessages)
	}
	if input.ClearContextMessageId != nil && input.ClearContextToLatest != nil {
		return nil, fmt.Errorf("clear context input is ambiguous")
	}
	if input.ClearContextMessageId != nil {
		if session.Mode != model.CanvasModeChat {
			return nil, fmt.Errorf("clear context is only supported for chat sessions")
		}
		clearContextMessageID, err := validateCanvasSessionClearContextMessageID(userId, session.Id, *input.ClearContextMessageId)
		if err != nil {
			return nil, err
		}
		updates["clear_context_message_id"] = clearContextMessageID
		updates["last_summarized_message_id"] = getCanvasChatSummaryCursorForClearContext(session, clearContextMessageID)
	}
	if input.ClearContextToLatest != nil {
		if session.Mode != model.CanvasModeChat {
			return nil, fmt.Errorf("clear context is only supported for chat sessions")
		}
		clearContextMessageID := 0
		if *input.ClearContextToLatest {
			clearContextMessageID, err = getLatestCanvasChatClearContextMessageID(userId, session.Id)
			if err != nil {
				return nil, err
			}
		}
		updates["clear_context_message_id"] = clearContextMessageID
		updates["last_summarized_message_id"] = getCanvasChatSummaryCursorForClearContext(session, clearContextMessageID)
	}
	if err := model.UpdateCanvasSessionFields(userId, id, updates); err != nil {
		return nil, err
	}
	return model.GetCanvasSessionByID(userId, id)
}

func ListCanvasMessages(userId int, sessionId int) ([]*CanvasMessageWithTask, error) {
	session, err := model.GetCanvasSessionByID(userId, sessionId)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, fmt.Errorf("canvas session not found")
	}
	messages, err := model.ListCanvasMessages(userId, sessionId)
	if err != nil {
		return nil, err
	}
	return attachCanvasMessageTasks(userId, messages), nil
}

func CreateCanvasMessage(userId int, sessionId int, input CreateCanvasMessageInput) ([]*CanvasMessageWithTask, error) {
	return CreateCanvasMessageWithContext(context.Background(), userId, sessionId, input)
}

func CreateCanvasMessageWithContext(ctx context.Context, userId int, sessionId int, input CreateCanvasMessageInput) ([]*CanvasMessageWithTask, error) {
	session, err := model.GetCanvasSessionByID(userId, sessionId)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, fmt.Errorf("canvas session not found")
	}
	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	clientRequestId := strings.TrimSpace(input.ClientRequestId)

	var taskMessage *model.CanvasMessage
	var cleanupCreatedTask func()
	switch session.Mode {
	case model.CanvasModeImage:
		task, err := createImageGenerationTaskForCanvas(userId, input.ModelId, input.Group, prompt, input.RequestEndpoint, input.Params)
		if err != nil {
			return nil, err
		}
		taskMessage = &model.CanvasMessage{
			SessionId:       sessionId,
			UserId:          userId,
			Mode:            session.Mode,
			Role:            model.CanvasMessageRoleAssistant,
			Prompt:          prompt,
			ClientRequestId: clientRequestId,
			Status:          task.Status,
			TaskId:          strconv.Itoa(task.Id),
			TaskType:        model.CanvasTaskTypeImage,
		}
		cleanupCreatedTask = func() {
			if err := DeleteImageGenerationTask(task); err != nil {
				common.SysLog(fmt.Sprintf("Failed to cleanup image task %d after canvas message failure: %v", task.Id, err))
			}
		}
	case model.CanvasModeVideo:
		task, err := createVideoGenerationTaskForCanvas(userId, input.ModelId, prompt, input.RequestEndpoint, input.Params)
		if err != nil {
			return nil, err
		}
		taskMessage = &model.CanvasMessage{
			SessionId:       sessionId,
			UserId:          userId,
			Mode:            session.Mode,
			Role:            model.CanvasMessageRoleAssistant,
			Prompt:          prompt,
			ClientRequestId: clientRequestId,
			Status:          task.Status,
			TaskId:          strconv.FormatInt(task.ID, 10),
			TaskType:        model.CanvasTaskTypeVideo,
		}
		cleanupCreatedTask = func() {
			if err := deleteVideoGenerationTaskForCanvas(userId, task.ID); err != nil {
				common.SysLog(fmt.Sprintf("Failed to cleanup video task %d after canvas message failure: %v", task.ID, err))
			}
		}
	case model.CanvasModeChat:
		return createCanvasChatMessage(ctx, userId, sessionId, session, input)
	default:
		return nil, fmt.Errorf("invalid canvas mode")
	}

	createdMessages := []*model.CanvasMessage{}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		now := common.GetTimestamp()
		if !session.TitleManuallySet {
			var messageCount int64
			if err := tx.Model(&model.CanvasMessage{}).
				Where("user_id = ? AND session_id = ? AND deleted_time = 0", userId, session.Id).
				Count(&messageCount).Error; err != nil {
				return err
			}
			if messageCount == 0 {
				if err := tx.Model(&model.CanvasSession{}).
					Where("id = ? AND user_id = ? AND deleted_time = 0", session.Id, userId).
					Updates(map[string]interface{}{
						"title":        truncateCanvasTitle(prompt),
						"updated_time": now,
					}).Error; err != nil {
					return err
				}
			}
		}

		userMessage := &model.CanvasMessage{
			SessionId:       sessionId,
			UserId:          userId,
			Mode:            session.Mode,
			Role:            model.CanvasMessageRoleUser,
			Prompt:          prompt,
			ClientRequestId: clientRequestId,
			Status:          "success",
			CreatedTime:     now,
			UpdatedTime:     now,
		}
		if err := tx.Create(userMessage).Error; err != nil {
			return err
		}
		createdMessages = append(createdMessages, userMessage)

		taskMessage.CreatedTime = now
		taskMessage.UpdatedTime = now
		if err := tx.Create(taskMessage).Error; err != nil {
			return err
		}
		createdMessages = append(createdMessages, taskMessage)

		sessionUpdates := map[string]interface{}{
			"updated_time": now,
		}
		if strings.TrimSpace(input.ModelId) != "" {
			sessionUpdates["current_model"] = strings.TrimSpace(input.ModelId)
		}
		return tx.Model(&model.CanvasSession{}).
			Where("id = ? AND user_id = ? AND deleted_time = 0", sessionId, userId).
			Updates(sessionUpdates).Error
	})
	if err != nil {
		if cleanupCreatedTask != nil {
			cleanupCreatedTask()
		}
		return nil, err
	}
	return attachCanvasMessageTasks(userId, createdMessages), nil
}

func DeleteCanvasSession(userId int, sessionId int) error {
	session, err := model.GetCanvasSessionByID(userId, sessionId)
	if err != nil {
		return err
	}
	if session == nil {
		return fmt.Errorf("canvas session not found")
	}
	messages, err := model.ListCanvasMessages(userId, sessionId)
	if err != nil {
		return err
	}

	for _, message := range messages {
		if message != nil &&
			message.Mode == model.CanvasModeChat &&
			message.Role == model.CanvasMessageRoleAssistant &&
			message.Status == model.CanvasMessageStatusGenerating {
			return fmt.Errorf("running chat session cannot be deleted")
		}
		if message == nil || strings.TrimSpace(message.TaskId) == "" {
			continue
		}
		switch message.TaskType {
		case model.CanvasTaskTypeImage:
			taskID, parseErr := strconv.Atoi(message.TaskId)
			if parseErr != nil {
				return parseErr
			}
			task, err := model.GetImageTaskByID(taskID)
			if err != nil {
				return err
			}
			if task != nil && task.UserId == userId {
				if task.Status == model.ImageTaskStatusPending || task.Status == model.ImageTaskStatusGenerating {
					return fmt.Errorf("running task cannot be deleted")
				}
				if err := DeleteImageGenerationTask(task); err != nil {
					return err
				}
			}
		case model.CanvasTaskTypeVideo:
			taskID, parseErr := strconv.ParseInt(message.TaskId, 10, 64)
			if parseErr != nil {
				return parseErr
			}
			if err := deleteVideoGenerationTaskForCanvas(userId, taskID); err != nil {
				return err
			}
		}
	}

	deletedTime := common.GetTimestamp()
	if err := model.SoftDeleteCanvasMessages(userId, sessionId, deletedTime); err != nil {
		return err
	}
	if err := model.SoftDeleteCanvasSession(userId, sessionId, deletedTime); err != nil {
		return err
	}
	InvalidateImageGenerationLocalAssetAccessCache()
	return nil
}

func SyncCanvasMessageTaskStatus(userId int, taskType string, taskId string) error {
	taskId = strings.TrimSpace(taskId)
	if taskId == "" {
		return nil
	}
	updates := map[string]interface{}{}
	switch taskType {
	case model.CanvasTaskTypeImage:
		id, err := strconv.Atoi(taskId)
		if err != nil {
			return err
		}
		task, err := model.GetImageTaskByID(id)
		if err != nil {
			return err
		}
		if task == nil || task.UserId != userId {
			return nil
		}
		updates["status"] = task.Status
		updates["error_message"] = task.ErrorMessage
	case model.CanvasTaskTypeVideo:
		id, err := strconv.ParseInt(taskId, 10, 64)
		if err != nil {
			return err
		}
		task, err := model.GetUserVideoTaskByID(userId, id, nil)
		if err != nil {
			return err
		}
		updates["status"] = task.Status.ToVideoStatus()
		updates["error_message"] = task.FailReason
	default:
		return nil
	}
	return model.DB.Model(&model.CanvasMessage{}).
		Where("user_id = ? AND task_id = ? AND task_type = ? AND deleted_time = 0", userId, taskId, taskType).
		Updates(updates).Error
}

func attachCanvasMessageTasks(userId int, messages []*model.CanvasMessage) []*CanvasMessageWithTask {
	result := make([]*CanvasMessageWithTask, 0, len(messages))
	for _, message := range messages {
		item := &CanvasMessageWithTask{CanvasMessage: message}
		if message == nil || strings.TrimSpace(message.TaskId) == "" {
			result = append(result, item)
			continue
		}
		switch message.TaskType {
		case model.CanvasTaskTypeImage:
			taskID, err := strconv.Atoi(message.TaskId)
			if err == nil {
				if task, taskErr := model.GetImageTaskByID(taskID); taskErr == nil && task != nil && task.UserId == userId {
					item.ImageTask = model.BuildImageGenerationTaskSummary(task)
					item.Status = task.Status
					item.ErrorMessage = task.ErrorMessage
				}
			}
		case model.CanvasTaskTypeVideo:
			taskID, err := strconv.ParseInt(message.TaskId, 10, 64)
			if err == nil {
				if task, taskErr := model.GetUserVideoTaskByID(userId, taskID, nil); taskErr == nil && task != nil {
					item.VideoTask = buildVideoTaskSummary(task)
					item.Status = item.VideoTask.Status
					item.ErrorMessage = task.FailReason
				}
			}
		}
		result = append(result, item)
	}
	return result
}

func maybeAutoTitleCanvasSession(userId int, session *model.CanvasSession, prompt string) error {
	if session == nil || session.TitleManuallySet || strings.TrimSpace(prompt) == "" {
		return nil
	}
	messageCount, err := model.CountCanvasMessages(userId, session.Id)
	if err != nil {
		return err
	}
	if messageCount > 0 {
		return nil
	}
	title := truncateCanvasTitle(prompt)
	if title == "" {
		return nil
	}
	return model.UpdateCanvasSessionFields(userId, session.Id, map[string]interface{}{
		"title": title,
	})
}

func defaultCanvasSessionTitle(mode string) string {
	switch mode {
	case model.CanvasModeImage:
		return "新图片会话"
	case model.CanvasModeVideo:
		return "新视频会话"
	case model.CanvasModeChat:
		return "新对话"
	default:
		return "新会话"
	}
}

func truncateCanvasTitle(title string) string {
	title = strings.Join(strings.Fields(strings.TrimSpace(title)), " ")
	if len([]rune(title)) <= 80 {
		return title
	}
	runes := []rune(title)
	return strings.TrimSpace(string(runes[:80]))
}

func deleteVideoGenerationTaskForCanvas(userId int, id int64) error {
	task, err := model.GetUserVideoTaskByID(userId, id, nil)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	if task.Status == model.TaskStatusQueued || task.Status == model.TaskStatusInProgress || task.Status == model.TaskStatusSubmitted || task.Status == model.TaskStatusNotStart {
		return fmt.Errorf("running task cannot be deleted")
	}
	deleteVideoTaskStoredAssets(task)
	return model.DB.Delete(&model.Task{}, task.ID).Error
}

func validateCanvasSessionClearContextMessageID(userId int, sessionId int, messageID int) (int, error) {
	if messageID < 0 {
		return 0, fmt.Errorf("invalid clear context message id")
	}
	if messageID == 0 {
		return 0, nil
	}

	var message model.CanvasMessage
	err := model.DB.
		Where("id = ? AND session_id = ? AND user_id = ? AND mode = ? AND status = ? AND deleted_time = 0",
			messageID, sessionId, userId, model.CanvasModeChat, model.CanvasMessageStatusSuccess).
		First(&message).Error
	if err == gorm.ErrRecordNotFound {
		return 0, fmt.Errorf("clear context message not found")
	}
	if err != nil {
		return 0, err
	}
	return message.Id, nil
}

func getLatestCanvasChatClearContextMessageID(userId int, sessionId int) (int, error) {
	var message model.CanvasMessage
	err := model.DB.
		Where("session_id = ? AND user_id = ? AND mode = ? AND status = ? AND deleted_time = 0",
			sessionId, userId, model.CanvasModeChat, model.CanvasMessageStatusSuccess).
		Order("created_time DESC").
		Order("id DESC").
		First(&message).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return message.Id, nil
}

func normalizeCanvasChatTemperatureValue(input *float64, fallback float64) float64 {
	if input == nil {
		input = common.GetPointer(fallback)
	}
	value := *input
	if value < 0 {
		value = 0
	}
	if value > 2 {
		value = 2
	}
	return value
}

func normalizeCanvasChatContextCountValue(input *int, fallback int) int {
	if input == nil {
		input = common.GetPointer(fallback)
	}
	value := *input
	if value < 0 {
		return 0
	}
	if value > canvasChatMaxContextCount {
		return canvasChatMaxContextCount
	}
	return value
}

func normalizeCanvasChatSystemPromptValue(input *string) string {
	if input == nil {
		return ""
	}
	return strings.TrimSpace(*input)
}

func normalizeCanvasChatSummaryEnabledValue(input *bool, fallback bool) bool {
	if input == nil {
		return fallback
	}
	return *input
}

func normalizeCanvasChatSummaryTriggerMessagesValue(input *int, fallback int) int {
	if input == nil {
		input = common.GetPointer(fallback)
	}
	value := *input
	if value < 0 {
		return 0
	}
	if value > 200 {
		return 200
	}
	return value
}

func normalizeCanvasChatSummaryRecentMessagesValue(input *int, fallback int) int {
	if input == nil {
		input = common.GetPointer(fallback)
	}
	value := *input
	if value < 0 {
		return 0
	}
	if value > 200 {
		return 200
	}
	return value
}

func canvasChatSummaryEnabledDBValue(enabled bool) any {
	if common.UsingPostgreSQL {
		return enabled
	}
	if enabled {
		return 1
	}
	return 0
}
