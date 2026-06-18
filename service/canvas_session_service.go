package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"gorm.io/gorm"
)

type CanvasImageMessageTask struct {
	Id                int    `json:"id"`
	ModelId           string `json:"model_id"`
	SelectedGroup     string `json:"selected_group"`
	Prompt            string `json:"prompt"`
	Status            string `json:"status"`
	RequestEndpoint   string `json:"request_endpoint"`
	Params            string `json:"params"`
	ImageUrl          string `json:"image_url"`
	ThumbnailUrl      string `json:"thumbnail_url"`
	ResultAssetStatus string `json:"result_asset_status"`
	ImageMetadata     string `json:"image_metadata"`
	ErrorMessage      string `json:"error_message"`
	CreatedTime       int64  `json:"created_time"`
	StartedTime       int64  `json:"started_time"`
	CompletedTime     int64  `json:"completed_time"`
	RequestType       string `json:"request_type"`
	ReferenceCount    int    `json:"reference_count"`
	HasMask           bool   `json:"has_mask"`
}

type CanvasVideoMessageTask struct {
	dto.VideoGenerationTaskSummary
}

type CanvasMessageWithTask struct {
	*model.CanvasMessage
	ImageTask       *CanvasImageMessageTask `json:"image_task,omitempty"`
	VideoTask       *CanvasVideoMessageTask `json:"video_task,omitempty"`
	ReferenceImages []string                `json:"reference_images,omitempty"`
	ReferenceImage  string                  `json:"reference_image,omitempty"`
}

type CreateCanvasSessionInput struct {
	Mode                   string
	Title                  string
	CurrentModel           string
	ChatTemperature        *float64
	ChatContextCount       *int
	WebSearchEnabled       *bool
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
	WebSearchEnabled       *bool
	SystemPrompt           *string
	SummaryEnabled         *bool
	SummaryTriggerMessages *int
	SummaryRecentMessages  *int
	ClearContextMessageId  *int
	ClearContextToLatest   *bool
}

type CreateCanvasMessageInput struct {
	Prompt           string
	ModelId          string
	Group            string
	RequestEndpoint  string
	Params           string
	Attachments      []dto.CanvasChatAttachment
	WebSearchEnabled *bool
	Stream           *bool
	Temperature      *float64
	ContextCount     *int
	ClientRequestId  string
}

var (
	createImageGenerationTaskForCanvas = CreateImageGenerationTask
	createVideoGenerationTaskForCanvas = CreateCanvasVideoGenerationTask
)

const (
	defaultCanvasSessionListLimit = 20
	maxCanvasSessionListLimit     = 100
	defaultCanvasMessagePageLimit = 100
	maxCanvasMessagePageLimit     = 200
)

type CanvasSessionListPage struct {
	Items   []*model.CanvasSession `json:"items"`
	HasMore bool                   `json:"has_more"`
}

type CanvasMessageTimelinePage struct {
	Items      []*CanvasMessageWithTask `json:"items"`
	HasMore    bool                     `json:"has_more"`
	NextCursor string                   `json:"next_cursor,omitempty"`
}

type canvasMessageTimelineCursor struct {
	CreatedTime int64
	ID          int
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
	if err := model.EnsureCanvasModeAvailable(mode); err != nil {
		return nil, err
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
	webSearchEnabled := false
	if mode == model.CanvasModeChat {
		chatTemperature = normalizeCanvasChatTemperatureValue(input.ChatTemperature, canvasChatDefaultTemperature)
		chatContextCount = normalizeCanvasChatContextCountValue(input.ChatContextCount, canvasChatDefaultContextCount)
		webSearchEnabled = normalizeCanvasChatWebSearchEnabledValue(input.WebSearchEnabled, false)
		systemPrompt = normalizeCanvasChatSystemPromptValue(input.SystemPrompt)
		summaryEnabled = normalizeCanvasChatSummaryEnabledValue(input.SummaryEnabled, canvasChatSummaryEnabledDefault)
		summaryTriggerMessages = normalizeCanvasChatSummaryTriggerMessagesValue(input.SummaryTriggerMessages, canvasChatSummaryTriggerMessagesDefault)
		summaryRecentMessages = normalizeCanvasChatSummaryRecentMessagesValue(input.SummaryRecentMessages, canvasChatSummaryRecentMessagesDefault)
		session.ChatTemperature = chatTemperature
		session.ChatContextCount = chatContextCount
		session.WebSearchEnabled = webSearchEnabled
		session.SystemPrompt = systemPrompt
		session.SummaryEnabled = summaryEnabled
		session.SummaryTriggerMessages = summaryTriggerMessages
		session.SummaryRecentMessages = summaryRecentMessages
	}
	if err := model.CreateCanvasSession(session); err != nil {
		return nil, err
	}
	return model.GetCanvasSessionByPublicID(userId, session.PublicId)
}

func UpdateCanvasSession(userId int, id int, input UpdateCanvasSessionInput) (*model.CanvasSession, error) {
	session, err := model.GetCanvasSessionByID(userId, id)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, fmt.Errorf("canvas session not found")
	}
	return UpdateCanvasSessionWithResolvedSession(userId, session, input)
}

func UpdateCanvasSessionWithResolvedSession(userId int, session *model.CanvasSession, input UpdateCanvasSessionInput) (*model.CanvasSession, error) {
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
	if input.WebSearchEnabled != nil {
		if session.Mode != model.CanvasModeChat {
			return nil, fmt.Errorf("chat configuration is only supported for chat sessions")
		}
		updates["web_search_enabled"] = normalizeCanvasChatWebSearchEnabledValue(input.WebSearchEnabled, session.WebSearchEnabled)
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
			latestMessageID, latestErr := getLatestCanvasChatClearContextMessageID(userId, session.Id)
			if latestErr != nil {
				return nil, latestErr
			}
			clearContextMessageID = latestMessageID
		}
		updates["clear_context_message_id"] = clearContextMessageID
		updates["last_summarized_message_id"] = getCanvasChatSummaryCursorForClearContext(session, clearContextMessageID)
	}
	if err := model.UpdateCanvasSessionFieldsWithSession(userId, session, updates); err != nil {
		return nil, err
	}
	return model.GetCanvasSessionByPublicID(userId, session.PublicId)
}

func ListCanvasMessages(userId int, sessionId int) ([]*CanvasMessageWithTask, error) {
	session, err := model.GetCanvasSessionByID(userId, sessionId)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, fmt.Errorf("canvas session not found")
	}
	messages, err := model.ListCanvasMessagesForMode(session.Mode, userId, sessionId)
	if err != nil {
		return nil, err
	}
	return attachCanvasMessageTasks(userId, messages), nil
}

func ListCanvasMessageTimeline(userId int, sessionId int, limit int, cursor string) (*CanvasMessageTimelinePage, error) {
	session, err := model.GetCanvasSessionByID(userId, sessionId)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, fmt.Errorf("canvas session not found")
	}
	return ListCanvasMessageTimelineWithResolvedSession(userId, session, limit, cursor)
}

func ListCanvasMessageTimelineWithResolvedSession(userId int, session *model.CanvasSession, limit int, cursor string) (*CanvasMessageTimelinePage, error) {
	if session == nil {
		return nil, fmt.Errorf("canvas session not found")
	}
	limit = normalizeCanvasMessagePageLimit(limit)
	beforeCursor, err := parseCanvasMessageTimelineCursor(session.Mode, userId, session.Id, cursor)
	if err != nil {
		return nil, err
	}

	messages, hasMore, err := model.ListCanvasMessagesPageForMode(
		session.Mode,
		userId,
		session.Id,
		limit,
		beforeCursor.CreatedTime,
		beforeCursor.ID,
	)
	if err != nil {
		return nil, err
	}
	if len(messages) > 1 {
		for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
			messages[left], messages[right] = messages[right], messages[left]
		}
	}

	nextCursor := ""
	if hasMore && len(messages) > 0 {
		nextCursor = encodeCanvasMessageTimelineCursor(messages[0])
	}
	return &CanvasMessageTimelinePage{
		Items:      attachCanvasMessageTasks(userId, messages),
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
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
	return CreateCanvasMessageWithResolvedSession(ctx, userId, session, input)
}

func CreateCanvasMessageWithResolvedSession(ctx context.Context, userId int, session *model.CanvasSession, input CreateCanvasMessageInput) ([]*CanvasMessageWithTask, error) {
	if session == nil {
		return nil, fmt.Errorf("canvas session not found")
	}
	if err := model.EnsureCanvasModeAvailable(session.Mode); err != nil {
		return nil, err
	}
	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	clientRequestId := strings.TrimSpace(input.ClientRequestId)
	if replay, ok, replayErr := loadCanvasMessageReplayByClientRequestID(session.Mode, userId, session.Id, clientRequestId); replayErr != nil {
		return nil, replayErr
	} else if ok {
		return replay, nil
	}

	var taskMessage *model.CanvasMessage
	var cleanupCreatedTask func()
	switch session.Mode {
	case model.CanvasModeImage:
		task, err := createImageGenerationTaskForCanvas(userId, input.ModelId, input.Group, prompt, input.RequestEndpoint, input.Params)
		if err != nil {
			return nil, err
		}
		taskMessage = &model.CanvasMessage{
			SessionId:       session.Id,
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
			SessionId:       session.Id,
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
		return createCanvasChatMessage(ctx, userId, session.Id, session, input)
	default:
		return nil, fmt.Errorf("invalid canvas mode")
	}

	messageCount, err := model.CountCanvasMessagesByMode(session.Mode, userId, session.Id)
	if err != nil {
		if cleanupCreatedTask != nil {
			cleanupCreatedTask()
		}
		return nil, err
	}

	now := common.GetTimestamp()
	userMessage := &model.CanvasMessage{
		SessionId:       session.Id,
		UserId:          userId,
		Mode:            session.Mode,
		Role:            model.CanvasMessageRoleUser,
		Prompt:          prompt,
		ClientRequestId: clientRequestId,
		Status:          model.CanvasMessageStatusSuccess,
		CreatedTime:     now,
		UpdatedTime:     now,
	}
	taskMessage.CreatedTime = now
	taskMessage.UpdatedTime = now
	createdMessages := []*model.CanvasMessage{userMessage, taskMessage}
	if err := model.CreateCanvasMessagesForMode(session.Mode, createdMessages); err != nil {
		if cleanupCreatedTask != nil {
			cleanupCreatedTask()
		}
		return nil, err
	}

	sessionUpdates := map[string]interface{}{
		"updated_time": now,
	}
	if strings.TrimSpace(input.ModelId) != "" {
		sessionUpdates["current_model"] = strings.TrimSpace(input.ModelId)
	}
	if !session.TitleManuallySet && messageCount == 0 {
		sessionUpdates["title"] = truncateCanvasTitle(prompt)
	}
	if err := model.UpdateCanvasSessionFieldsWithSession(userId, session, sessionUpdates); err != nil {
		cleanupCreatedCanvasMessages(session.Mode, userId, createdMessages)
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
	return DeleteCanvasSessionWithResolvedSession(userId, session)
}

func DeleteCanvasSessionWithResolvedSession(userId int, session *model.CanvasSession) error {
	if session == nil {
		return fmt.Errorf("canvas session not found")
	}
	sessionId := session.Id
	messageRefs, err := model.ListCanvasSessionMessageTaskRefsForMode(session.Mode, userId, sessionId)
	if err != nil {
		return err
	}

	messageIDs := make([]int, 0, len(messageRefs))
	imageTaskIDs := make([]int, 0)
	videoTaskIDs := make([]int64, 0)
	seenImageTaskIDs := make(map[int]struct{})
	seenVideoTaskIDs := make(map[int64]struct{})

	for _, message := range messageRefs {
		if message == nil {
			continue
		}
		if message.Id > 0 {
			messageIDs = append(messageIDs, message.Id)
		}
		if session.Mode == model.CanvasModeChat &&
			message.Role == model.CanvasMessageRoleAssistant &&
			message.Status == model.CanvasMessageStatusGenerating {
			return fmt.Errorf("running chat session cannot be deleted")
		}
		if strings.TrimSpace(message.TaskId) == "" {
			continue
		}
		switch message.TaskType {
		case model.CanvasTaskTypeImage:
			taskID, parseErr := strconv.Atoi(message.TaskId)
			if parseErr != nil {
				return parseErr
			}
			if taskID <= 0 {
				continue
			}
			if _, ok := seenImageTaskIDs[taskID]; ok {
				continue
			}
			seenImageTaskIDs[taskID] = struct{}{}
			imageTaskIDs = append(imageTaskIDs, taskID)
		case model.CanvasTaskTypeVideo:
			taskID, parseErr := strconv.ParseInt(message.TaskId, 10, 64)
			if parseErr != nil {
				return parseErr
			}
			if taskID <= 0 {
				continue
			}
			if _, ok := seenVideoTaskIDs[taskID]; ok {
				continue
			}
			seenVideoTaskIDs[taskID] = struct{}{}
			videoTaskIDs = append(videoTaskIDs, taskID)
		}
	}

	imageTasks, err := model.GetImageTasksByUserAndIDs(userId, imageTaskIDs)
	if err != nil {
		return err
	}
	for _, task := range imageTasks {
		if task == nil {
			continue
		}
		if task.Status == model.ImageTaskStatusPending || task.Status == model.ImageTaskStatusGenerating {
			return fmt.Errorf("running task cannot be deleted")
		}
	}

	videoTasks, err := model.GetCanvasVideoTasksByIDs(userId, videoTaskIDs, nil)
	if err != nil {
		return err
	}
	for _, task := range videoTasks {
		if task == nil {
			continue
		}
		if task.Status == model.TaskStatusQueued || task.Status == model.TaskStatusInProgress || task.Status == model.TaskStatusSubmitted || task.Status == model.TaskStatusNotStart {
			return fmt.Errorf("running task cannot be deleted")
		}
	}

	cleanupJobs, err := buildCanvasAssetCleanupJobs(imageTasks, videoTasks)
	if err != nil {
		return err
	}

	deletedTime := common.GetTimestamp()
	if err := deleteCanvasSessionData(userId, session, messageIDs, imageTasks, videoTasks, cleanupJobs, deletedTime); err != nil {
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
	messageMode := ""
	switch taskType {
	case model.CanvasTaskTypeImage:
		messageMode = model.CanvasModeImage
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
		messageMode = model.CanvasModeVideo
		id, err := strconv.ParseInt(taskId, 10, 64)
		if err != nil {
			return err
		}
		task, err := model.GetCanvasVideoTaskByID(userId, id, nil)
		if err != nil {
			return err
		}
		updates["status"] = task.Status.ToVideoStatus()
		updates["error_message"] = task.FailReason
	default:
		return nil
	}
	return model.UpdateCanvasMessageFieldsByTask(messageMode, userId, taskId, taskType, updates)
}

func attachCanvasMessageTasks(userId int, messages []*model.CanvasMessage) []*CanvasMessageWithTask {
	imageTasksByID, imageRefsByID := loadCanvasImageTasksByID(userId, messages)
	videoTasksByID, videoRefsByID := loadCanvasVideoTasksByID(userId, messages)

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
				if task, ok := imageTasksByID[taskID]; ok {
					item.ImageTask = task
					item.Status = task.Status
					item.ErrorMessage = task.ErrorMessage
				}
				setCanvasMessageReferenceImages(item, imageRefsByID[taskID])
			}
		case model.CanvasTaskTypeVideo:
			taskID, err := strconv.ParseInt(message.TaskId, 10, 64)
			if err == nil {
				if task, ok := videoTasksByID[taskID]; ok {
					item.VideoTask = task
					item.Status = task.Status
					item.ErrorMessage = task.FailReason
				}
				setCanvasMessageReferenceImages(item, videoRefsByID[taskID])
			}
		}
		result = append(result, item)
	}
	return result
}

func normalizeCanvasMessagePageLimit(limit int) int {
	if limit <= 0 {
		return defaultCanvasMessagePageLimit
	}
	if limit > maxCanvasMessagePageLimit {
		return maxCanvasMessagePageLimit
	}
	return limit
}

func encodeCanvasMessageTimelineCursor(message *model.CanvasMessage) string {
	if message == nil || message.Id <= 0 || message.CreatedTime < 0 {
		return ""
	}
	return fmt.Sprintf("%d:%d", message.CreatedTime, message.Id)
}

func parseCanvasMessageTimelineCursor(mode string, userId int, sessionId int, cursor string) (*canvasMessageTimelineCursor, error) {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return &canvasMessageTimelineCursor{}, nil
	}

	parts := strings.Split(cursor, ":")
	if len(parts) == 2 {
		createdTime, createdErr := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
		id, idErr := strconv.Atoi(strings.TrimSpace(parts[1]))
		if createdErr == nil && idErr == nil && createdTime >= 0 && id > 0 {
			return &canvasMessageTimelineCursor{
				CreatedTime: createdTime,
				ID:          id,
			}, nil
		}
	}

	id, err := strconv.Atoi(cursor)
	if err != nil || id <= 0 {
		return nil, fmt.Errorf("invalid canvas message cursor")
	}
	message, err := model.GetCanvasMessageByModeSessionAndID(mode, userId, sessionId, id)
	if err != nil {
		return nil, err
	}
	if message == nil {
		return nil, fmt.Errorf("invalid canvas message cursor")
	}
	return &canvasMessageTimelineCursor{
		CreatedTime: message.CreatedTime,
		ID:          message.Id,
	}, nil
}

func loadCanvasImageTasksByID(userId int, messages []*model.CanvasMessage) (map[int]*CanvasImageMessageTask, map[int][]string) {
	taskIDs := make([]int, 0)
	seen := make(map[int]struct{})
	for _, message := range messages {
		if message == nil || message.TaskType != model.CanvasTaskTypeImage {
			continue
		}
		taskID, err := strconv.Atoi(strings.TrimSpace(message.TaskId))
		if err != nil || taskID <= 0 {
			continue
		}
		if _, ok := seen[taskID]; ok {
			continue
		}
		seen[taskID] = struct{}{}
		taskIDs = append(taskIDs, taskID)
	}

	tasksByID := make(map[int]*CanvasImageMessageTask, len(taskIDs))
	refsByID := make(map[int][]string, len(taskIDs))
	if len(taskIDs) == 0 {
		return tasksByID, refsByID
	}

	tasks, err := model.GetImageTasksByUserAndIDs(userId, taskIDs)
	if err != nil {
		common.SysLog(fmt.Sprintf("Failed to batch load canvas image tasks: %v", err))
		return tasksByID, refsByID
	}
	for _, task := range tasks {
		if task == nil {
			continue
		}
		refs, refsErr := collectStoredReferenceImages(task.Params)
		if refsErr != nil {
			common.SysLog(fmt.Sprintf("Failed to read canvas image task %d references: %v", task.Id, refsErr))
		}
		modelTask := *task
		FillImageGenerationTaskSummary(&modelTask)
		modelTask.Params = SanitizeImageGenerationParamsForResponse(modelTask.Params)
		tasksByID[task.Id] = buildCanvasImageMessageTask(&modelTask)
		if len(refs) > 0 {
			refsByID[task.Id] = refs
		}
	}
	return tasksByID, refsByID
}

func buildCanvasImageMessageTask(task *model.ImageGenerationTask) *CanvasImageMessageTask {
	if task == nil {
		return nil
	}
	return &CanvasImageMessageTask{
		Id:                task.Id,
		ModelId:           task.ModelId,
		SelectedGroup:     task.SelectedGroup,
		Prompt:            task.Prompt,
		Status:            task.Status,
		RequestEndpoint:   task.RequestEndpoint,
		Params:            task.Params,
		ImageUrl:          task.ImageUrl,
		ThumbnailUrl:      task.ThumbnailUrl,
		ResultAssetStatus: model.NormalizeImageTaskResultAssetStatus(task.ResultAssetStatus),
		ImageMetadata:     task.ImageMetadata,
		ErrorMessage:      task.ErrorMessage,
		CreatedTime:       task.CreatedTime,
		StartedTime:       task.EffectiveStartedTime(),
		CompletedTime:     task.CompletedTime,
		RequestType:       task.RequestType,
		ReferenceCount:    task.ReferenceCount,
		HasMask:           task.HasMask,
	}
}

func loadCanvasVideoTasksByID(userId int, messages []*model.CanvasMessage) (map[int64]*CanvasVideoMessageTask, map[int64][]string) {
	taskIDs := make([]int64, 0)
	seen := make(map[int64]struct{})
	for _, message := range messages {
		if message == nil || message.TaskType != model.CanvasTaskTypeVideo {
			continue
		}
		taskID, err := strconv.ParseInt(strings.TrimSpace(message.TaskId), 10, 64)
		if err != nil || taskID <= 0 {
			continue
		}
		if _, ok := seen[taskID]; ok {
			continue
		}
		seen[taskID] = struct{}{}
		taskIDs = append(taskIDs, taskID)
	}

	tasksByID := make(map[int64]*CanvasVideoMessageTask, len(taskIDs))
	refsByID := make(map[int64][]string, len(taskIDs))
	if len(taskIDs) == 0 {
		return tasksByID, refsByID
	}

	tasks, err := model.GetCanvasVideoTasksByIDs(userId, taskIDs, nil)
	if err != nil {
		common.SysLog(fmt.Sprintf("Failed to batch load canvas video tasks: %v", err))
		return tasksByID, refsByID
	}
	mappingsByModel, mappingsResolved := loadVideoTaskMappingsByModelID(tasks)
	for _, task := range tasks {
		if task == nil {
			continue
		}
		summary := buildCanvasVideoTaskSummaryWithResolvedMapping(
			task,
			mappingsByModel[extractVideoTaskModelID(task)],
			mappingsResolved,
		)
		if summary != nil {
			tasksByID[task.ID] = &CanvasVideoMessageTask{
				VideoGenerationTaskSummary: *summary,
			}
		}
		refsByID[task.ID] = collectCanvasVideoReferenceImages(task)
	}
	return tasksByID, refsByID
}

func collectCanvasVideoReferenceImages(task *model.Task) []string {
	if task == nil || strings.TrimSpace(task.Properties.RequestParams) == "" {
		return nil
	}
	var req relaycommon.TaskSubmitReq
	if err := common.UnmarshalJsonStr(task.Properties.RequestParams, &req); err != nil {
		return nil
	}
	seen := make(map[string]struct{}, 3)
	refs := make([]string, 0, 3)
	appendRef := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		refs = append(refs, value)
	}
	appendRef(req.Image)
	appendRef(req.InputReference)
	for _, image := range req.Images {
		appendRef(image)
	}
	return refs
}

func setCanvasMessageReferenceImages(item *CanvasMessageWithTask, refs []string) {
	if item == nil || len(refs) == 0 {
		return
	}
	item.ReferenceImages = append([]string(nil), refs...)
	item.ReferenceImage = refs[0]
}

func maybeAutoTitleCanvasSession(userId int, session *model.CanvasSession, prompt string) error {
	if session == nil || session.TitleManuallySet || strings.TrimSpace(prompt) == "" {
		return nil
	}
	messageCount, err := model.CountCanvasMessagesByMode(session.Mode, userId, session.Id)
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
	return model.UpdateCanvasSessionFieldsWithSession(userId, session, map[string]interface{}{
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
	task, err := model.GetCanvasVideoTaskByID(userId, id, nil)
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
	return model.DeleteCanvasVideoTasksByIDs(userId, []int64{task.ID}, nil)
}

func deleteCanvasSessionData(userId int, session *model.CanvasSession, messageIDs []int, imageTasks []*model.ImageGenerationTask, videoTasks []*model.Task, cleanupJobs []*model.CanvasAssetCleanupJob, deletedTime int64) error {
	if session == nil {
		return fmt.Errorf("canvas session not found")
	}
	imageTaskIDs := make([]int, 0, len(imageTasks))
	videoTaskIDs := make([]int64, 0, len(videoTasks))
	queueSlotsToRelease := 0
	for _, task := range imageTasks {
		if task == nil || task.UserId != userId {
			continue
		}
		imageTaskIDs = append(imageTaskIDs, task.Id)
		if task.Status == model.ImageTaskStatusPending || task.Status == model.ImageTaskStatusGenerating {
			queueSlotsToRelease++
		}
	}
	for _, task := range videoTasks {
		if task == nil || task.UserId != userId {
			continue
		}
		videoTaskIDs = append(videoTaskIDs, task.ID)
	}
	if err := model.DeleteCanvasSessionDataWithSession(userId, session, messageIDs, imageTaskIDs, videoTaskIDs, cleanupJobs, deletedTime); err != nil {
		return err
	}

	if queueSlotsToRelease > 0 {
		if err := model.ReleaseUserImageGenerationQueueSlots(userId, queueSlotsToRelease); err != nil {
			common.SysLog(fmt.Sprintf("Failed to release image generation queue slots for user %d after canvas delete: %v", userId, err))
		}
	}
	return nil
}

func loadCanvasMessageReplayByClientRequestID(mode string, userId int, sessionId int, clientRequestId string) ([]*CanvasMessageWithTask, bool, error) {
	clientRequestId = strings.TrimSpace(clientRequestId)
	if clientRequestId == "" {
		return nil, false, nil
	}
	messages, err := model.ListCanvasMessagesByClientRequestIDForMode(mode, userId, sessionId, clientRequestId)
	if err != nil {
		return nil, false, err
	}
	if len(messages) == 0 {
		return nil, false, nil
	}
	return attachCanvasMessageTasks(userId, messages), true, nil
}

func validateCanvasSessionClearContextMessageID(userId int, sessionId int, messageID int) (int, error) {
	if messageID < 0 {
		return 0, fmt.Errorf("invalid clear context message id")
	}
	if messageID == 0 {
		return 0, nil
	}

	message, err := model.GetCanvasMessageByModeSessionAndID(model.CanvasModeChat, userId, sessionId, messageID)
	if err != nil {
		return 0, err
	}
	if message == nil || message.Status != model.CanvasMessageStatusSuccess {
		return 0, fmt.Errorf("clear context message not found")
	}
	return message.Id, nil
}

func getLatestCanvasChatClearContextMessageID(userId int, sessionId int) (int, error) {
	message, err := model.GetLatestSuccessfulCanvasMessage(model.CanvasModeChat, userId, sessionId)
	if err != nil {
		return 0, err
	}
	if message == nil {
		return 0, nil
	}
	return message.Id, nil
}

func cleanupCreatedCanvasMessages(mode string, userId int, messages []*model.CanvasMessage) {
	ids := make([]int, 0, len(messages))
	for _, message := range messages {
		if message == nil || message.Id <= 0 {
			continue
		}
		ids = append(ids, message.Id)
	}
	if len(ids) == 0 {
		return
	}
	if err := model.SoftDeleteCanvasMessagesByIDs(mode, userId, ids, common.GetTimestamp()); err != nil {
		common.SysLog(fmt.Sprintf("Failed to cleanup canvas messages %v after session update failure: %v", ids, err))
	}
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

func normalizeCanvasChatWebSearchEnabledValue(input *bool, fallback bool) bool {
	if input == nil {
		return fallback
	}
	return *input
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
