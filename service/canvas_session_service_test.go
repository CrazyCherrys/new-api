package service

import (
	"context"
	"fmt"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupCanvasSessionServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousUsingSQLite := common.UsingSQLite
	previousUsingMySQL := common.UsingMySQL
	previousUsingPostgreSQL := common.UsingPostgreSQL
	previousRedisEnabled := common.RedisEnabled
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousCreateImage := createImageGenerationTaskForCanvas
	previousCreateVideo := createVideoGenerationTaskForCanvas
	previousCallCanvasChatRelay := callCanvasChatRelay
	previousQueueCanvasChatTask := queueCanvasChatBackgroundTask

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.MemoryCacheEnabled = false
	model.InitCommonColumnNames()
	queueCanvasChatBackgroundTask = func(fn func()) {}
	t.Setenv("CANVAS_CHAT_SQL_DSN", "")
	t.Setenv("CANVAS_IMAGE_SQL_DSN", "")
	t.Setenv("CANVAS_VIDEO_SQL_DSN", "")

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db
	model.InitCanvasDBs()

	if err := db.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.Channel{},
		&model.Ability{},
		&model.ModelMapping{},
		&model.CanvasSession{},
		&model.CanvasMessage{},
		&model.CanvasAssetCleanupJob{},
		&model.ImageGenerationTask{},
		&model.ImageGenerationReferenceAsset{},
		&model.ImageGenerationTaskReferenceAsset{},
		&model.Task{},
	); err != nil {
		t.Fatalf("failed to migrate canvas session tables: %v", err)
	}

	t.Cleanup(func() {
		createImageGenerationTaskForCanvas = previousCreateImage
		createVideoGenerationTaskForCanvas = previousCreateVideo
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.UsingSQLite = previousUsingSQLite
		common.UsingMySQL = previousUsingMySQL
		common.UsingPostgreSQL = previousUsingPostgreSQL
		common.RedisEnabled = previousRedisEnabled
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		callCanvasChatRelay = previousCallCanvasChatRelay
		queueCanvasChatBackgroundTask = previousQueueCanvasChatTask

		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedCanvasChatCapability(t *testing.T, db *gorm.DB, userId int, userGroup string, tokenGroup string, abilityGroup string, modelId string) {
	t.Helper()

	user := &model.User{
		Id:       userId,
		Username: fmt.Sprintf("canvas-user-%d", userId),
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    userGroup,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	token := &model.Token{
		UserId:         userId,
		Key:            fmt.Sprintf("token-key-%d", userId),
		Status:         common.TokenStatusEnabled,
		Name:           fmt.Sprintf("token-%d", userId),
		ExpiredTime:    -1,
		UnlimitedQuota: true,
		Group:          tokenGroup,
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	channel := &model.Channel{
		Id:     userId + 1000,
		Type:   constant.ChannelTypeOpenAI,
		Key:    fmt.Sprintf("channel-key-%d", userId),
		Status: common.ChannelStatusEnabled,
		Name:   fmt.Sprintf("channel-%d", userId),
		Group:  abilityGroup,
		Models: modelId,
	}
	if err := db.Create(channel).Error; err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	ability := &model.Ability{
		Group:     abilityGroup,
		Model:     modelId,
		ChannelId: channel.Id,
		Enabled:   true,
		Weight:    0,
	}
	if err := db.Create(ability).Error; err != nil {
		t.Fatalf("failed to create ability: %v", err)
	}

	if err := db.Create(&model.ModelMapping{
		RequestModel:    modelId,
		ActualModel:     modelId,
		DisplayName:     modelId,
		ModelSeries:     "openai",
		ModelType:       1,
		Status:          1,
		RequestEndpoint: "openai",
	}).Error; err != nil {
		t.Fatalf("failed to create model mapping: %v", err)
	}
}

type canvasChatStreamTestEvent struct {
	Event   string
	RawData string
	Payload canvasChatSSEEvent
}

func parseCanvasChatStreamTestEvents(t *testing.T, raw string) []canvasChatStreamTestEvent {
	t.Helper()

	blocks := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n\n")
	events := make([]canvasChatStreamTestEvent, 0, len(blocks))
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		lines := strings.Split(block, "\n")
		item := canvasChatStreamTestEvent{}
		for _, line := range lines {
			if strings.HasPrefix(line, "event:") {
				item.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
				continue
			}
			if strings.HasPrefix(line, "data:") {
				item.RawData = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			}
		}
		if item.RawData == "" {
			continue
		}
		if err := common.Unmarshal(common.StringToByteSlice(item.RawData), &item.Payload); err != nil {
			t.Fatalf("failed to decode stream payload %q: %v", item.RawData, err)
		}
		events = append(events, item)
	}
	return events
}

func findCanvasChatStreamTestEvent(events []canvasChatStreamTestEvent, eventType string) *canvasChatStreamTestEvent {
	for i := range events {
		if events[i].Event == eventType {
			return &events[i]
		}
	}
	return nil
}

func TestCanvasSessionCreateRenamePinSortAndModeFilter(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)

	imageOld, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeImage, Title: "old image"})
	if err != nil {
		t.Fatalf("failed to create old image session: %v", err)
	}
	imageNew, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeImage, Title: "new image"})
	if err != nil {
		t.Fatalf("failed to create new image session: %v", err)
	}
	videoSession, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeVideo, Title: "video"})
	if err != nil {
		t.Fatalf("failed to create video session: %v", err)
	}
	if _, err := CreateCanvasSession(2, CreateCanvasSessionInput{Mode: model.CanvasModeImage, Title: "other user"}); err != nil {
		t.Fatalf("failed to create other user session: %v", err)
	}

	if err := db.Model(&model.CanvasSession{}).Where("id = ?", imageOld.Id).Updates(map[string]interface{}{"updated_time": int64(100)}).Error; err != nil {
		t.Fatalf("failed to adjust old image time: %v", err)
	}
	if err := db.Model(&model.CanvasSession{}).Where("id = ?", imageNew.Id).Updates(map[string]interface{}{"updated_time": int64(200)}).Error; err != nil {
		t.Fatalf("failed to adjust new image time: %v", err)
	}
	pinned := true
	renamed := "renamed old image"
	currentModel := "gpt-image-session"
	updated, err := UpdateCanvasSession(1, imageOld.Id, UpdateCanvasSessionInput{Title: &renamed, Pinned: &pinned, CurrentModel: &currentModel})
	if err != nil {
		t.Fatalf("failed to update image session: %v", err)
	}
	if updated.Title != renamed || !updated.Pinned || !updated.TitleManuallySet || updated.CurrentModel != currentModel {
		t.Fatalf("unexpected updated session: %#v", updated)
	}

	imagePage, err := ListCanvasSessions(1, model.CanvasModeImage, 20, 0)
	if err != nil {
		t.Fatalf("failed to list image sessions: %v", err)
	}
	sessions := imagePage.Items
	if len(sessions) != 2 {
		t.Fatalf("expected 2 image sessions for user, got %d", len(sessions))
	}
	if sessions[0].Id != imageOld.Id || sessions[1].Id != imageNew.Id {
		t.Fatalf("expected pinned old session first then newest, got ids %d, %d", sessions[0].Id, sessions[1].Id)
	}
	for _, session := range sessions {
		if session.Mode != model.CanvasModeImage || session.UserId != 1 {
			t.Fatalf("list returned wrong mode/user session: %#v", session)
		}
	}

	videoPage, err := ListCanvasSessions(1, model.CanvasModeVideo, 20, 0)
	if err != nil {
		t.Fatalf("failed to list video sessions: %v", err)
	}
	videoSessions := videoPage.Items
	if len(videoSessions) != 1 || videoSessions[0].Id != videoSession.Id {
		t.Fatalf("expected only the video session, got %#v", videoSessions)
	}
}

func TestCanvasSessionListPaginationAndMixedMode(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)

	created := make([]*model.CanvasSession, 0, 25)
	modes := []string{
		model.CanvasModeChat,
		model.CanvasModeImage,
		model.CanvasModeVideo,
	}
	for i := 0; i < 25; i++ {
		session, err := CreateCanvasSession(1, CreateCanvasSessionInput{
			Mode:  modes[i%len(modes)],
			Title: fmt.Sprintf("session-%02d", i),
		})
		if err != nil {
			t.Fatalf("failed to create session %d: %v", i, err)
		}
		created = append(created, session)
		if err := db.Model(&model.CanvasSession{}).
			Where("id = ?", session.Id).
			Updates(map[string]interface{}{
				"updated_time": int64(1000 + i),
				"pinned":       i == 3 || i == 17,
			}).Error; err != nil {
			t.Fatalf("failed to adjust session ordering data: %v", err)
		}
		session.UpdatedTime = int64(1000 + i)
		session.Pinned = i == 3 || i == 17
	}
	if _, err := CreateCanvasSession(2, CreateCanvasSessionInput{
		Mode:  model.CanvasModeChat,
		Title: "other-user",
	}); err != nil {
		t.Fatalf("failed to create other user session: %v", err)
	}

	expected := append([]*model.CanvasSession(nil), created...)
	sort.Slice(expected, func(i, j int) bool {
		if expected[i].Pinned != expected[j].Pinned {
			return expected[i].Pinned
		}
		if expected[i].UpdatedTime != expected[j].UpdatedTime {
			return expected[i].UpdatedTime > expected[j].UpdatedTime
		}
		return expected[i].Id > expected[j].Id
	})

	firstPage, err := ListCanvasSessions(1, "", 20, 0)
	if err != nil {
		t.Fatalf("failed to list mixed sessions: %v", err)
	}
	if len(firstPage.Items) != 20 {
		t.Fatalf("expected 20 mixed sessions on first page, got %d", len(firstPage.Items))
	}
	if !firstPage.HasMore {
		t.Fatal("expected first mixed page to report has_more=true")
	}
	for i, session := range firstPage.Items {
		if session.Id != expected[i].Id {
			t.Fatalf("unexpected first page order at %d: got %d want %d", i, session.Id, expected[i].Id)
		}
	}

	secondPage, err := ListCanvasSessions(1, "", 20, 20)
	if err != nil {
		t.Fatalf("failed to list second mixed page: %v", err)
	}
	if len(secondPage.Items) != 5 {
		t.Fatalf("expected 5 mixed sessions on second page, got %d", len(secondPage.Items))
	}
	if secondPage.HasMore {
		t.Fatal("expected second mixed page to report has_more=false")
	}
	seen := make(map[int]struct{}, len(firstPage.Items))
	for _, session := range firstPage.Items {
		seen[session.Id] = struct{}{}
	}
	for i, session := range secondPage.Items {
		if session.Id != expected[20+i].Id {
			t.Fatalf("unexpected second page order at %d: got %d want %d", i, session.Id, expected[20+i].Id)
		}
		if _, ok := seen[session.Id]; ok {
			t.Fatalf("session %d appeared in both mixed pages", session.Id)
		}
	}

	for _, mode := range modes {
		page, err := ListCanvasSessions(1, mode, 20, 0)
		if err != nil {
			t.Fatalf("failed to list mode %s: %v", mode, err)
		}
		expectedMode := make([]*model.CanvasSession, 0)
		for _, session := range expected {
			if session.Mode == mode {
				expectedMode = append(expectedMode, session)
			}
		}
		if len(page.Items) != len(expectedMode) {
			t.Fatalf("expected %d %s sessions, got %d", len(expectedMode), mode, len(page.Items))
		}
		for i, session := range page.Items {
			if session.Mode != mode {
				t.Fatalf("mode filter %s returned session with mode %s", mode, session.Mode)
			}
			if session.Id != expectedMode[i].Id {
				t.Fatalf("unexpected %s order at %d: got %d want %d", mode, i, session.Id, expectedMode[i].Id)
			}
		}
	}
}

func TestCreateCanvasImageMessagesPersistsTaskMessagesAndAutoTitle(t *testing.T) {
	setupCanvasSessionServiceTestDB(t)

	createImageGenerationTaskForCanvas = func(userId int, modelId string, selectedGroup string, prompt string, requestEndpoint string, params string) (*model.ImageGenerationTask, error) {
		task := &model.ImageGenerationTask{
			UserId:          userId,
			ModelId:         modelId,
			SelectedGroup:   selectedGroup,
			Prompt:          prompt,
			RequestEndpoint: requestEndpoint,
			Status:          model.ImageTaskStatusPending,
			Params:          params,
			CreatedTime:     common.GetTimestamp(),
		}
		if err := task.Insert(); err != nil {
			return nil, err
		}
		return task, nil
	}

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeImage})
	if err != nil {
		t.Fatalf("failed to create image session: %v", err)
	}
	created, err := CreateCanvasMessage(1, session.Id, CreateCanvasMessageInput{
		Prompt:          "first image prompt",
		ModelId:         "gpt-image-test",
		Group:           "default",
		RequestEndpoint: "openai",
		Params:          `{"size":"1024x1024"}`,
	})
	if err != nil {
		t.Fatalf("failed to create first image message: %v", err)
	}
	if len(created) != 2 || created[0].Role != model.CanvasMessageRoleUser || created[1].TaskType != model.CanvasTaskTypeImage {
		t.Fatalf("expected user+image task messages, got %#v", created)
	}
	if created[1].ImageTask == nil || created[1].ImageTask.Prompt != "first image prompt" {
		t.Fatalf("expected attached image task summary, got %#v", created[1])
	}

	reloadedSession, err := model.GetCanvasSessionByID(1, session.Id)
	if err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}
	if reloadedSession.Title != "first image prompt" || reloadedSession.TitleManuallySet || reloadedSession.CurrentModel != "gpt-image-test" {
		t.Fatalf("expected auto title from first prompt without manual flag, got %#v", reloadedSession)
	}

	manualTitle := "manual image title"
	if _, err := UpdateCanvasSession(1, session.Id, UpdateCanvasSessionInput{Title: &manualTitle}); err != nil {
		t.Fatalf("failed to rename session: %v", err)
	}
	if _, err := CreateCanvasMessage(1, session.Id, CreateCanvasMessageInput{
		Prompt:          "second image prompt",
		ModelId:         "gpt-image-test",
		Group:           "default",
		RequestEndpoint: "openai",
		Params:          `{}`,
	}); err != nil {
		t.Fatalf("failed to create second image message: %v", err)
	}

	messages, err := ListCanvasMessages(1, session.Id)
	if err != nil {
		t.Fatalf("failed to list image messages: %v", err)
	}
	if len(messages) != 4 {
		t.Fatalf("expected 4 persisted messages after two prompts, got %d", len(messages))
	}
	reloadedSession, _ = model.GetCanvasSessionByID(1, session.Id)
	if reloadedSession.Title != manualTitle {
		t.Fatalf("manual title should not be overwritten, got %q", reloadedSession.Title)
	}
	if reloadedSession.CurrentModel != "gpt-image-test" {
		t.Fatalf("expected current model to remain latest image model, got %q", reloadedSession.CurrentModel)
	}
}

func TestCreateCanvasVideoMessagePersistsTaskMessage(t *testing.T) {
	setupCanvasSessionServiceTestDB(t)

	createVideoGenerationTaskForCanvas = func(userId int, modelId string, prompt string, requestEndpoint string, rawParams string) (*dto.VideoGenerationTaskSummary, error) {
		task := &model.Task{
			UserId:     userId,
			TaskID:     "task_video_stub",
			Action:     constant.TaskActionTextGenerate,
			Status:     model.TaskStatusQueued,
			Progress:   "20%",
			SubmitTime: common.GetTimestamp(),
			Properties: model.Properties{
				Input:             prompt,
				OriginModelName:   modelId,
				UpstreamModelName: modelId,
				RequestParams:     rawParams,
			},
		}
		if err := model.DB.Create(task).Error; err != nil {
			return nil, err
		}
		return &dto.VideoGenerationTaskSummary{
			ID:              task.ID,
			TaskID:          task.TaskID,
			Status:          task.Status.ToVideoStatus(),
			Prompt:          prompt,
			ModelID:         modelId,
			RequestEndpoint: requestEndpoint,
			CreatedTime:     task.SubmitTime,
		}, nil
	}

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeVideo})
	if err != nil {
		t.Fatalf("failed to create video session: %v", err)
	}
	created, err := CreateCanvasMessage(1, session.Id, CreateCanvasMessageInput{
		Prompt:          "video prompt",
		ModelId:         "sora-test",
		RequestEndpoint: "openai-video",
		Params:          `{"duration":5}`,
	})
	if err != nil {
		t.Fatalf("failed to create video message: %v", err)
	}
	if len(created) != 2 || created[1].TaskType != model.CanvasTaskTypeVideo || created[1].VideoTask == nil {
		t.Fatalf("expected attached video task message, got %#v", created)
	}
	if created[1].Status != dto.VideoStatusQueued {
		t.Fatalf("expected queued video status, got %q", created[1].Status)
	}
	reloadedSession, err := model.GetCanvasSessionByID(1, session.Id)
	if err != nil {
		t.Fatalf("failed to reload video session: %v", err)
	}
	if reloadedSession.CurrentModel != "sora-test" {
		t.Fatalf("expected current video model to be persisted, got %q", reloadedSession.CurrentModel)
	}
}

func TestCreateCanvasImageMessagesPersistClientRequestID(t *testing.T) {
	setupCanvasSessionServiceTestDB(t)

	createImageGenerationTaskForCanvas = func(userId int, modelId string, selectedGroup string, prompt string, requestEndpoint string, params string) (*model.ImageGenerationTask, error) {
		task := &model.ImageGenerationTask{
			UserId:          userId,
			ModelId:         modelId,
			SelectedGroup:   selectedGroup,
			Prompt:          prompt,
			RequestEndpoint: requestEndpoint,
			Status:          model.ImageTaskStatusPending,
			Params:          params,
			CreatedTime:     common.GetTimestamp(),
		}
		if err := task.Insert(); err != nil {
			return nil, err
		}
		return task, nil
	}

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeImage})
	if err != nil {
		t.Fatalf("failed to create image session: %v", err)
	}
	created, err := CreateCanvasMessage(1, session.Id, CreateCanvasMessageInput{
		Prompt:          "client request image",
		ModelId:         "gpt-image-test",
		Group:           "default",
		RequestEndpoint: "openai",
		Params:          `{"size":"1024x1024"}`,
		ClientRequestId: "image-request-1",
	})
	if err != nil {
		t.Fatalf("failed to create image message: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("expected 2 created messages, got %d", len(created))
	}
	for _, message := range created {
		if message.ClientRequestId != "image-request-1" {
			t.Fatalf("expected created message client_request_id to persist, got %#v", message)
		}
	}

	reloaded, err := ListCanvasMessages(1, session.Id)
	if err != nil {
		t.Fatalf("failed to reload image messages: %v", err)
	}
	if len(reloaded) != 2 {
		t.Fatalf("expected 2 reloaded messages, got %d", len(reloaded))
	}
	for _, message := range reloaded {
		if message.ClientRequestId != "image-request-1" {
			t.Fatalf("expected reloaded client_request_id, got %#v", message)
		}
	}
}

func TestCreateCanvasVideoMessagesPersistClientRequestID(t *testing.T) {
	setupCanvasSessionServiceTestDB(t)

	createVideoGenerationTaskForCanvas = func(userId int, modelId string, prompt string, requestEndpoint string, rawParams string) (*dto.VideoGenerationTaskSummary, error) {
		task := &model.Task{
			UserId:     userId,
			TaskID:     "task_video_reqid",
			Action:     constant.TaskActionTextGenerate,
			Status:     model.TaskStatusQueued,
			Progress:   "10%",
			SubmitTime: common.GetTimestamp(),
			Properties: model.Properties{
				Input:             prompt,
				OriginModelName:   modelId,
				UpstreamModelName: modelId,
				RequestParams:     rawParams,
			},
		}
		if err := model.DB.Create(task).Error; err != nil {
			return nil, err
		}
		return &dto.VideoGenerationTaskSummary{
			ID:              task.ID,
			TaskID:          task.TaskID,
			Status:          task.Status.ToVideoStatus(),
			Prompt:          prompt,
			ModelID:         modelId,
			RequestEndpoint: requestEndpoint,
			CreatedTime:     task.SubmitTime,
		}, nil
	}

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeVideo})
	if err != nil {
		t.Fatalf("failed to create video session: %v", err)
	}
	created, err := CreateCanvasMessage(1, session.Id, CreateCanvasMessageInput{
		Prompt:          "video with request id",
		ModelId:         "sora-test",
		RequestEndpoint: "openai-video",
		Params:          `{"duration":5}`,
		ClientRequestId: "video-request-1",
	})
	if err != nil {
		t.Fatalf("failed to create video message: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("expected 2 created video messages, got %d", len(created))
	}
	for _, message := range created {
		if message.ClientRequestId != "video-request-1" {
			t.Fatalf("expected created video message client_request_id, got %#v", message)
		}
	}

	reloaded, err := ListCanvasMessages(1, session.Id)
	if err != nil {
		t.Fatalf("failed to reload video messages: %v", err)
	}
	if len(reloaded) != 2 {
		t.Fatalf("expected 2 reloaded video messages, got %d", len(reloaded))
	}
	for _, message := range reloaded {
		if message.ClientRequestId != "video-request-1" {
			t.Fatalf("expected reloaded video client_request_id, got %#v", message)
		}
	}
}

func TestCreateCanvasChatMessageAutoTitlesAndPersistsModelGroupAndMetadata(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)
	seedCanvasChatCapability(t, db, 1, "default", "default", "default", "gpt-chat-test")

	callCanvasChatRelay = func(ctx context.Context, request canvasChatRelayRequest, onDelta func(delta canvasChatRelayDelta) error) (*canvasChatRelayResult, error) {
		if request.ModelId != "gpt-chat-test" {
			t.Fatalf("expected model gpt-chat-test, got %q", request.ModelId)
		}
		if request.Group != "default" {
			t.Fatalf("expected group default, got %q", request.Group)
		}
		if len(request.Messages) != 1 || request.Messages[0].Role != model.CanvasMessageRoleUser || request.Messages[0].StringContent() != "first chat prompt" {
			t.Fatalf("unexpected relay messages: %#v", request.Messages)
		}
		if onDelta != nil {
			if err := onDelta(canvasChatRelayDelta{Content: "assistant reply"}); err != nil {
				return nil, err
			}
		}
		return &canvasChatRelayResult{Text: "assistant reply"}, nil
	}

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeChat})
	if err != nil {
		t.Fatalf("failed to create chat session: %v", err)
	}

	temperature := 0.2
	contextCount := 4
	created, err := CreateCanvasMessage(1, session.Id, CreateCanvasMessageInput{
		Prompt:       "first chat prompt",
		ModelId:      "gpt-chat-test",
		Temperature:  &temperature,
		ContextCount: &contextCount,
	})
	if err != nil {
		t.Fatalf("failed to create chat message: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("expected 2 chat messages, got %d", len(created))
	}
	if created[0].Role != model.CanvasMessageRoleUser || created[1].Role != model.CanvasMessageRoleAssistant {
		t.Fatalf("unexpected chat roles: %#v", created)
	}
	if created[1].Prompt != "assistant reply" || created[1].Status != model.CanvasMessageStatusSuccess {
		t.Fatalf("unexpected assistant message: %#v", created[1])
	}

	reloadedSession, err := model.GetCanvasSessionByID(1, session.Id)
	if err != nil {
		t.Fatalf("failed to reload chat session: %v", err)
	}
	if reloadedSession.Title != "first chat prompt" {
		t.Fatalf("expected auto title to use first prompt, got %q", reloadedSession.Title)
	}
	if reloadedSession.CurrentModel != "gpt-chat-test" {
		t.Fatalf("expected current_model to persist, got %q", reloadedSession.CurrentModel)
	}
	if reloadedSession.CurrentGroup != "default" {
		t.Fatalf("expected current_group to persist, got %q", reloadedSession.CurrentGroup)
	}

	var metadata canvasChatMessageMetadata
	if err := common.UnmarshalJsonStr(created[0].Metadata, &metadata); err != nil {
		t.Fatalf("failed to decode message metadata: %v", err)
	}
	if metadata.ChatModel != "gpt-chat-test" || metadata.ChatGroup != "default" {
		t.Fatalf("unexpected metadata snapshot: %#v", metadata)
	}
	if metadata.Temperature == nil || *metadata.Temperature != temperature {
		t.Fatalf("expected temperature %.1f, got %#v", temperature, metadata.Temperature)
	}
	if metadata.ContextCount == nil || *metadata.ContextCount != contextCount {
		t.Fatalf("expected context_count %d, got %#v", contextCount, metadata.ContextCount)
	}
}

func TestCreateCanvasChatSessionPersistsConfigAndAllowsZeroValueUpdates(t *testing.T) {
	setupCanvasSessionServiceTestDB(t)

	temperature := 1.5
	contextCount := 16
	systemPrompt := "You are a deliberate assistant."
	summaryEnabled := false
	summaryTriggerMessages := 12
	summaryRecentMessages := 6
	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{
		Mode:                   model.CanvasModeChat,
		CurrentModel:           "gpt-chat-test",
		ChatTemperature:        &temperature,
		ChatContextCount:       &contextCount,
		SystemPrompt:           &systemPrompt,
		SummaryEnabled:         &summaryEnabled,
		SummaryTriggerMessages: &summaryTriggerMessages,
		SummaryRecentMessages:  &summaryRecentMessages,
	})
	if err != nil {
		t.Fatalf("failed to create configured chat session: %v", err)
	}
	if session.ChatTemperature != temperature {
		t.Fatalf("expected chat temperature %.1f, got %.1f", temperature, session.ChatTemperature)
	}
	if session.ChatContextCount != contextCount {
		t.Fatalf("expected chat context count %d, got %d", contextCount, session.ChatContextCount)
	}
	if session.SystemPrompt != systemPrompt {
		t.Fatalf("expected system prompt %q, got %q", systemPrompt, session.SystemPrompt)
	}
	if session.SummaryEnabled != summaryEnabled {
		t.Fatalf("expected summary enabled %v, got %v", summaryEnabled, session.SummaryEnabled)
	}
	if session.SummaryTriggerMessages != summaryTriggerMessages {
		t.Fatalf("expected summary trigger messages %d, got %d", summaryTriggerMessages, session.SummaryTriggerMessages)
	}
	if session.SummaryRecentMessages != summaryRecentMessages {
		t.Fatalf("expected summary recent messages %d, got %d", summaryRecentMessages, session.SummaryRecentMessages)
	}

	zeroTemperature := 0.0
	zeroContextCount := 0
	emptySystemPrompt := ""
	summaryEnabled = true
	zeroSummaryTrigger := 0
	zeroSummaryRecent := 0
	updated, err := UpdateCanvasSession(1, session.Id, UpdateCanvasSessionInput{
		ChatTemperature:        &zeroTemperature,
		ChatContextCount:       &zeroContextCount,
		SystemPrompt:           &emptySystemPrompt,
		SummaryEnabled:         &summaryEnabled,
		SummaryTriggerMessages: &zeroSummaryTrigger,
		SummaryRecentMessages:  &zeroSummaryRecent,
	})
	if err != nil {
		t.Fatalf("failed to update chat config to zero values: %v", err)
	}
	if updated.ChatTemperature != zeroTemperature {
		t.Fatalf("expected zero chat temperature to persist, got %.1f", updated.ChatTemperature)
	}
	if updated.ChatContextCount != zeroContextCount {
		t.Fatalf("expected zero chat context count to persist, got %d", updated.ChatContextCount)
	}
	if updated.SystemPrompt != "" {
		t.Fatalf("expected empty system prompt to persist, got %q", updated.SystemPrompt)
	}
	if updated.SummaryEnabled != summaryEnabled {
		t.Fatalf("expected summary enabled toggle to persist, got %v", updated.SummaryEnabled)
	}
	if updated.SummaryTriggerMessages != zeroSummaryTrigger {
		t.Fatalf("expected zero summary trigger to persist, got %d", updated.SummaryTriggerMessages)
	}
	if updated.SummaryRecentMessages != zeroSummaryRecent {
		t.Fatalf("expected zero summary recent to persist, got %d", updated.SummaryRecentMessages)
	}
}

func TestCreateCanvasChatMessageUsesSessionConfigFallbacks(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)
	seedCanvasChatCapability(t, db, 1, "default", "default", "default", "gpt-chat-test")
	systemPrompt := "Focus on concise answers."

	callCanvasChatRelay = func(ctx context.Context, request canvasChatRelayRequest, onDelta func(delta canvasChatRelayDelta) error) (*canvasChatRelayResult, error) {
		if request.ModelId != "gpt-chat-test" {
			t.Fatalf("expected session current model fallback, got %q", request.ModelId)
		}
		if request.Temperature == nil || *request.Temperature != 1.5 {
			t.Fatalf("expected session chat temperature fallback 1.5, got %#v", request.Temperature)
		}
		if len(request.Messages) != 2 {
			t.Fatalf("expected system prompt plus user message, got %#v", request.Messages)
		}
		if request.Messages[0].Role == model.CanvasMessageRoleUser || request.Messages[0].StringContent() != systemPrompt {
			t.Fatalf("expected system prompt injection, got %#v", request.Messages[0])
		}
		if request.Messages[1].Role != model.CanvasMessageRoleUser || request.Messages[1].StringContent() != "use session config" {
			t.Fatalf("unexpected relay messages: %#v", request.Messages)
		}
		if onDelta != nil {
			if err := onDelta(canvasChatRelayDelta{Content: "assistant reply"}); err != nil {
				return nil, err
			}
		}
		return &canvasChatRelayResult{Text: "assistant reply"}, nil
	}

	temperature := 1.5
	contextCount := 2
	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{
		Mode:             model.CanvasModeChat,
		CurrentModel:     "gpt-chat-test",
		ChatTemperature:  &temperature,
		ChatContextCount: &contextCount,
		SystemPrompt:     &systemPrompt,
	})
	if err != nil {
		t.Fatalf("failed to create chat session: %v", err)
	}

	created, err := CreateCanvasMessage(1, session.Id, CreateCanvasMessageInput{
		Prompt: "use session config",
	})
	if err != nil {
		t.Fatalf("failed to create chat message with session config fallback: %v", err)
	}

	var metadata canvasChatMessageMetadata
	if err := common.UnmarshalJsonStr(created[0].Metadata, &metadata); err != nil {
		t.Fatalf("failed to decode metadata snapshot: %v", err)
	}
	if metadata.Temperature == nil || *metadata.Temperature != temperature {
		t.Fatalf("expected metadata temperature %.1f, got %#v", temperature, metadata.Temperature)
	}
	if metadata.ContextCount == nil || *metadata.ContextCount != contextCount {
		t.Fatalf("expected metadata context count %d, got %#v", contextCount, metadata.ContextCount)
	}
	if metadata.SystemPrompt != systemPrompt {
		t.Fatalf("expected metadata system prompt %q, got %q", systemPrompt, metadata.SystemPrompt)
	}
}

func TestCreateCanvasChatMessagePersistsReasoningContentFromMixedDeltas(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)
	seedCanvasChatCapability(t, db, 1, "default", "default", "default", "gpt-chat-test")

	callCanvasChatRelay = func(ctx context.Context, request canvasChatRelayRequest, onDelta func(delta canvasChatRelayDelta) error) (*canvasChatRelayResult, error) {
		deltas := []canvasChatRelayDelta{
			{ReasoningContent: "Step 1"},
			{ReasoningContent: "\nStep 2"},
			{Content: "final "},
			{Content: "answer"},
		}
		for _, delta := range deltas {
			if onDelta != nil {
				if err := onDelta(delta); err != nil {
					return nil, err
				}
			}
		}
		return &canvasChatRelayResult{
			Text:             "final answer",
			ReasoningContent: "Step 1\nStep 2",
		}, nil
	}

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeChat})
	if err != nil {
		t.Fatalf("failed to create chat session: %v", err)
	}

	created, err := CreateCanvasMessage(1, session.Id, CreateCanvasMessageInput{
		Prompt:  "show work",
		ModelId: "gpt-chat-test",
	})
	if err != nil {
		t.Fatalf("failed to create chat message: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("expected 2 chat messages, got %d", len(created))
	}
	assistant := created[1]
	if assistant.Prompt != "final answer" {
		t.Fatalf("expected final assistant prompt to persist, got %q", assistant.Prompt)
	}
	if assistant.ReasoningContent != "Step 1\nStep 2" {
		t.Fatalf("expected reasoning content to persist, got %q", assistant.ReasoningContent)
	}

	reloaded, err := ListCanvasMessages(1, session.Id)
	if err != nil {
		t.Fatalf("failed to reload chat messages: %v", err)
	}
	if len(reloaded) != 2 {
		t.Fatalf("expected 2 reloaded chat messages, got %d", len(reloaded))
	}
	if reloaded[1].Prompt != "final answer" {
		t.Fatalf("expected reloaded assistant prompt, got %q", reloaded[1].Prompt)
	}
	if reloaded[1].ReasoningContent != "Step 1\nStep 2" {
		t.Fatalf("expected reloaded reasoning content, got %q", reloaded[1].ReasoningContent)
	}
}

func TestCanvasChatSummaryDisabledSkipsBackgroundSummary(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)
	seedCanvasChatCapability(t, db, 1, "default", "default", "default", "gpt-chat-test")

	queueCanvasChatBackgroundTask = func(fn func()) {
		if fn != nil {
			fn()
		}
	}

	replyIndex := 0
	summaryCalls := 0
	callCanvasChatRelay = func(ctx context.Context, request canvasChatRelayRequest, onDelta func(delta canvasChatRelayDelta) error) (*canvasChatRelayResult, error) {
		if len(request.Messages) > 0 && strings.Contains(request.Messages[0].StringContent(), "你负责维护长对话摘要记忆") {
			summaryCalls++
			return &canvasChatRelayResult{Text: "summary memory"}, nil
		}
		replyIndex++
		reply := fmt.Sprintf("assistant-%d", replyIndex)
		if onDelta != nil {
			if err := onDelta(canvasChatRelayDelta{Content: reply}); err != nil {
				return nil, err
			}
		}
		return &canvasChatRelayResult{Text: reply}, nil
	}

	summaryEnabled := false
	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{
		Mode:           model.CanvasModeChat,
		CurrentModel:   "gpt-chat-test",
		SummaryEnabled: &summaryEnabled,
	})
	if err != nil {
		t.Fatalf("failed to create chat session: %v", err)
	}

	contextCount := 1
	for i := 1; i <= 3; i++ {
		if _, err := CreateCanvasMessage(1, session.Id, CreateCanvasMessageInput{
			Prompt:       fmt.Sprintf("user-%d", i),
			ModelId:      "gpt-chat-test",
			ContextCount: &contextCount,
		}); err != nil {
			t.Fatalf("failed to create chat message %d: %v", i, err)
		}
	}

	if summaryCalls != 0 {
		t.Fatalf("expected summary to remain disabled, got %d summary calls", summaryCalls)
	}
	reloadedSession, err := model.GetCanvasSessionByID(1, session.Id)
	if err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}
	if reloadedSession.SummaryPrompt != "" || reloadedSession.LastSummarizedMessageId != 0 {
		t.Fatalf("expected no summary state when disabled, got prompt=%q cursor=%d", reloadedSession.SummaryPrompt, reloadedSession.LastSummarizedMessageId)
	}
}

func TestBuildCanvasChatRelayMessagesSkipsSummaryWhenDisabled(t *testing.T) {
	setupCanvasSessionServiceTestDB(t)
	summaryEnabled := false
	systemPrompt := "system prompt"
	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{
		Mode:           model.CanvasModeChat,
		SummaryEnabled: &summaryEnabled,
		SystemPrompt:   &systemPrompt,
	})
	if err != nil {
		t.Fatalf("failed to create chat session: %v", err)
	}
	if err := model.UpdateCanvasSessionFields(1, session.Id, map[string]interface{}{
		"summary_prompt":             "default summary",
		"last_summarized_message_id": 11,
	}); err != nil {
		t.Fatalf("failed to seed summary state: %v", err)
	}
	reloadedSession, err := model.GetCanvasSessionByID(1, session.Id)
	if err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}
	prepared := &canvasChatPreparedRequest{
		UserId:       1,
		Session:      reloadedSession,
		Prompt:       "current user message",
		FinalModel:   "gpt-chat-test",
		ContextCount: 0,
		UserMessage:  &model.CanvasMessage{Id: 999},
	}

	messages, err := buildCanvasChatRelayMessages(prepared)
	if err != nil {
		t.Fatalf("failed to build relay messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected only system prompt and current user message, got %#v", messages)
	}
	if messages[0].StringContent() != "system prompt" {
		t.Fatalf("expected first message to be system prompt, got %#v", messages[0])
	}
	if messages[1].Role != model.CanvasMessageRoleUser || messages[1].StringContent() != "current user message" {
		t.Fatalf("unexpected user relay message: %#v", messages[1])
	}
}

func TestBuildCanvasChatRelayMessagesKeepsHistoryWhenSummaryDisabled(t *testing.T) {
	setupCanvasSessionServiceTestDB(t)

	summaryEnabled := false
	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{
		Mode:           model.CanvasModeChat,
		CurrentModel:   "gpt-chat-test",
		SummaryEnabled: &summaryEnabled,
	})
	if err != nil {
		t.Fatalf("failed to create chat session: %v", err)
	}

	now := common.GetTimestamp()
	historyMessages := []*model.CanvasMessage{
		{
			SessionId:   session.Id,
			UserId:      1,
			Mode:        model.CanvasModeChat,
			Role:        model.CanvasMessageRoleUser,
			Prompt:      "history user",
			Status:      model.CanvasMessageStatusSuccess,
			CreatedTime: now,
			UpdatedTime: now,
		},
		{
			SessionId:   session.Id,
			UserId:      1,
			Mode:        model.CanvasModeChat,
			Role:        model.CanvasMessageRoleAssistant,
			Prompt:      "history assistant",
			Status:      model.CanvasMessageStatusSuccess,
			CreatedTime: now + 1,
			UpdatedTime: now + 1,
		},
	}
	for _, message := range historyMessages {
		if err := model.CreateCanvasMessage(message); err != nil {
			t.Fatalf("failed to create history message: %v", err)
		}
	}
	if err := model.UpdateCanvasSessionFields(1, session.Id, map[string]interface{}{
		"summary_prompt":             "legacy summary",
		"last_summarized_message_id": historyMessages[1].Id,
	}); err != nil {
		t.Fatalf("failed to seed summary state: %v", err)
	}
	reloadedSession, err := model.GetCanvasSessionByID(1, session.Id)
	if err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}

	prepared := &canvasChatPreparedRequest{
		UserId:       1,
		Session:      reloadedSession,
		Prompt:       "current user message",
		FinalModel:   "gpt-chat-test",
		ContextCount: 2,
		UserMessage:  &model.CanvasMessage{Id: 999},
	}
	messages, err := buildCanvasChatRelayMessages(prepared)
	if err != nil {
		t.Fatalf("failed to build relay messages: %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("expected historical messages to remain when summary disabled, got %#v", messages)
	}
	if messages[0].Role != model.CanvasMessageRoleUser || messages[0].StringContent() != "history user" {
		t.Fatalf("expected first history user message, got %#v", messages[0])
	}
	if messages[1].Role != model.CanvasMessageRoleAssistant || messages[1].StringContent() != "history assistant" {
		t.Fatalf("expected second history assistant message, got %#v", messages[1])
	}
	if messages[2].Role != model.CanvasMessageRoleUser || messages[2].StringContent() != "current user message" {
		t.Fatalf("expected current user message last, got %#v", messages[2])
	}
}

func TestListRecentCanvasSuccessfulChatMessagesUsesCreatedTimeOrdering(t *testing.T) {
	setupCanvasSessionServiceTestDB(t)

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeChat})
	if err != nil {
		t.Fatalf("failed to create chat session: %v", err)
	}

	for _, message := range []*model.CanvasMessage{
		{SessionId: session.Id, UserId: 1, Mode: model.CanvasModeChat, Role: model.CanvasMessageRoleUser, Prompt: "created-300", Status: model.CanvasMessageStatusSuccess, CreatedTime: 300, UpdatedTime: 300},
		{SessionId: session.Id, UserId: 1, Mode: model.CanvasModeChat, Role: model.CanvasMessageRoleAssistant, Prompt: "created-100", Status: model.CanvasMessageStatusSuccess, CreatedTime: 100, UpdatedTime: 100},
		{SessionId: session.Id, UserId: 1, Mode: model.CanvasModeChat, Role: model.CanvasMessageRoleUser, Prompt: "created-200", Status: model.CanvasMessageStatusSuccess, CreatedTime: 200, UpdatedTime: 200},
	} {
		if err := model.CreateCanvasMessage(message); err != nil {
			t.Fatalf("failed to create canvas chat history: %v", err)
		}
	}

	messages, err := listRecentCanvasSuccessfulChatMessages(1, session.Id, 0, 0, 2)
	if err != nil {
		t.Fatalf("failed to list recent chat messages: %v", err)
	}
	if got := []string{messages[0].Prompt, messages[1].Prompt}; !equalStrings(got, []string{"created-200", "created-300"}) {
		t.Fatalf("expected recent chat messages by created_time order, got %#v", got)
	}
}

func TestListCanvasSuccessfulChatMessagesAfterUsesCreatedTimeOrdering(t *testing.T) {
	setupCanvasSessionServiceTestDB(t)

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeChat})
	if err != nil {
		t.Fatalf("failed to create chat session: %v", err)
	}

	for _, message := range []*model.CanvasMessage{
		{SessionId: session.Id, UserId: 1, Mode: model.CanvasModeChat, Role: model.CanvasMessageRoleUser, Prompt: "created-300", Status: model.CanvasMessageStatusSuccess, CreatedTime: 300, UpdatedTime: 300},
		{SessionId: session.Id, UserId: 1, Mode: model.CanvasModeChat, Role: model.CanvasMessageRoleAssistant, Prompt: "created-100", Status: model.CanvasMessageStatusSuccess, CreatedTime: 100, UpdatedTime: 100},
		{SessionId: session.Id, UserId: 1, Mode: model.CanvasModeChat, Role: model.CanvasMessageRoleUser, Prompt: "created-200", Status: model.CanvasMessageStatusSuccess, CreatedTime: 200, UpdatedTime: 200},
	} {
		if err := model.CreateCanvasMessage(message); err != nil {
			t.Fatalf("failed to create canvas chat history: %v", err)
		}
	}

	messages, err := listCanvasSuccessfulChatMessagesAfter(1, session.Id, 0, 3)
	if err != nil {
		t.Fatalf("failed to list chat messages after cursor: %v", err)
	}
	if got := []string{messages[0].Prompt, messages[1].Prompt, messages[2].Prompt}; !equalStrings(got, []string{"created-100", "created-200", "created-300"}) {
		t.Fatalf("expected summary chat messages by created_time order, got %#v", got)
	}
}

func TestCreateCanvasChatMessageFailureKeepsPartialText(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)
	seedCanvasChatCapability(t, db, 1, "default", "default", "default", "gpt-chat-test")

	callCanvasChatRelay = func(ctx context.Context, request canvasChatRelayRequest, onDelta func(delta canvasChatRelayDelta) error) (*canvasChatRelayResult, error) {
		if onDelta != nil {
			if err := onDelta(canvasChatRelayDelta{
				Content:          "partial reply",
				ReasoningContent: "reasoning chunk",
			}); err != nil {
				return nil, err
			}
		}
		return &canvasChatRelayResult{
			Text:             "partial reply",
			ReasoningContent: "reasoning chunk",
		}, fmt.Errorf("upstream exploded")
	}

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeChat})
	if err != nil {
		t.Fatalf("failed to create chat session: %v", err)
	}

	if _, err := CreateCanvasMessage(1, session.Id, CreateCanvasMessageInput{
		Prompt:  "please fail",
		ModelId: "gpt-chat-test",
	}); err == nil {
		t.Fatal("expected chat creation to fail")
	}

	messages, err := ListCanvasMessages(1, session.Id)
	if err != nil {
		t.Fatalf("failed to list chat messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 persisted chat messages, got %d", len(messages))
	}
	assistant := messages[1]
	if assistant.Prompt != "partial reply" {
		t.Fatalf("expected partial assistant text to persist, got %q", assistant.Prompt)
	}
	if assistant.ReasoningContent != "reasoning chunk" {
		t.Fatalf("expected partial reasoning content to persist, got %q", assistant.ReasoningContent)
	}
	if assistant.Status != model.CanvasMessageStatusFailed {
		t.Fatalf("expected failed status, got %q", assistant.Status)
	}
	if !strings.Contains(assistant.ErrorMessage, "upstream exploded") {
		t.Fatalf("expected upstream error message, got %q", assistant.ErrorMessage)
	}
}

func TestCanvasChatResponseBodyReturnsReasoningContent(t *testing.T) {
	body, err := common.Marshal(dto.OpenAITextResponse{
		Choices: []dto.OpenAITextResponseChoice{
			{
				Message: dto.Message{
					Role:             model.CanvasMessageRoleAssistant,
					Content:          "final answer",
					ReasoningContent: "reasoning summary",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to marshal response body: %v", err)
	}

	result, err := parseCanvasChatResponseBody(body)
	if err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	if result.Text != "final answer" {
		t.Fatalf("expected parsed text, got %q", result.Text)
	}
	if result.ReasoningContent != "reasoning summary" {
		t.Fatalf("expected parsed reasoning content, got %q", result.ReasoningContent)
	}
}

func TestCanvasChatStreamEmitsReasoningDeltaAndSnapshots(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupCanvasSessionServiceTestDB(t)
	seedCanvasChatCapability(t, db, 1, "default", "default", "default", "gpt-chat-test")

	callCanvasChatRelay = func(ctx context.Context, request canvasChatRelayRequest, onDelta func(delta canvasChatRelayDelta) error) (*canvasChatRelayResult, error) {
		deltas := []canvasChatRelayDelta{
			{ReasoningContent: "Step 1"},
			{ReasoningContent: "\nStep 2"},
			{Content: "Answer"},
		}
		for _, delta := range deltas {
			if onDelta != nil {
				if err := onDelta(delta); err != nil {
					return nil, err
				}
			}
		}
		return &canvasChatRelayResult{
			Text:             "Answer",
			ReasoningContent: "Step 1\nStep 2",
		}, nil
	}

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeChat})
	if err != nil {
		t.Fatalf("failed to create chat session: %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/api/canvas/sessions/1/messages", nil)

	if err := StreamCanvasChatMessage(c, 1, session.Id, CreateCanvasMessageInput{
		Prompt:  "hello",
		ModelId: "gpt-chat-test",
	}); err != nil {
		t.Fatalf("StreamCanvasChatMessage returned error: %v", err)
	}

	events := parseCanvasChatStreamTestEvents(t, recorder.Body.String())
	createdEvent := findCanvasChatStreamTestEvent(events, "canvas.message.created")
	if createdEvent == nil {
		t.Fatalf("expected created event, got %#v", events)
	}
	if !strings.Contains(createdEvent.RawData, "\"reasoning_content\":\"\"") {
		t.Fatalf("expected created snapshot to include empty reasoning_content, got %s", createdEvent.RawData)
	}
	if len(createdEvent.Payload.Messages) != 2 {
		t.Fatalf("expected created event to include user and assistant messages, got %#v", createdEvent.Payload.Messages)
	}

	deltaEvent := findCanvasChatStreamTestEvent(events, "canvas.message.delta")
	if deltaEvent == nil {
		t.Fatalf("expected delta event, got %#v", events)
	}
	if deltaEvent.Payload.Delta != "Answer" {
		t.Fatalf("expected content delta to stream, got %q", deltaEvent.Payload.Delta)
	}
	if deltaEvent.Payload.ReasoningDelta != "Step 1\nStep 2" {
		t.Fatalf("expected reasoning delta to stream, got %q", deltaEvent.Payload.ReasoningDelta)
	}
	if deltaEvent.Payload.Message == nil || deltaEvent.Payload.Message.ReasoningContent != "Step 1\nStep 2" {
		t.Fatalf("expected delta snapshot to carry reasoning content, got %#v", deltaEvent.Payload.Message)
	}

	completedEvent := findCanvasChatStreamTestEvent(events, "canvas.message.completed")
	if completedEvent == nil {
		t.Fatalf("expected completed event, got %#v", events)
	}
	if completedEvent.Payload.Message == nil || completedEvent.Payload.Message.Prompt != "Answer" {
		t.Fatalf("expected completed snapshot to carry final prompt, got %#v", completedEvent.Payload.Message)
	}
	if completedEvent.Payload.Message.ReasoningContent != "Step 1\nStep 2" {
		t.Fatalf("expected completed snapshot to carry reasoning content, got %#v", completedEvent.Payload.Message)
	}
}

func TestCanvasChatStreamErrorEventIncludesReasoningSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupCanvasSessionServiceTestDB(t)
	seedCanvasChatCapability(t, db, 1, "default", "default", "default", "gpt-chat-test")

	callCanvasChatRelay = func(ctx context.Context, request canvasChatRelayRequest, onDelta func(delta canvasChatRelayDelta) error) (*canvasChatRelayResult, error) {
		if onDelta != nil {
			if err := onDelta(canvasChatRelayDelta{
				Content:          "partial",
				ReasoningContent: "thinking",
			}); err != nil {
				return nil, err
			}
		}
		return &canvasChatRelayResult{
			Text:             "partial",
			ReasoningContent: "thinking",
		}, fmt.Errorf("upstream exploded")
	}

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeChat})
	if err != nil {
		t.Fatalf("failed to create chat session: %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/api/canvas/sessions/1/messages", nil)

	if err := StreamCanvasChatMessage(c, 1, session.Id, CreateCanvasMessageInput{
		Prompt:  "hello",
		ModelId: "gpt-chat-test",
	}); err != nil {
		t.Fatalf("StreamCanvasChatMessage returned error: %v", err)
	}

	events := parseCanvasChatStreamTestEvents(t, recorder.Body.String())
	errorEvent := findCanvasChatStreamTestEvent(events, "canvas.message.error")
	if errorEvent == nil {
		t.Fatalf("expected error event, got %#v", events)
	}
	if errorEvent.Payload.Error == "" || !strings.Contains(errorEvent.Payload.Error, "upstream exploded") {
		t.Fatalf("expected error payload to contain upstream message, got %#v", errorEvent.Payload)
	}
	if errorEvent.Payload.Message == nil || errorEvent.Payload.Message.Prompt != "partial" {
		t.Fatalf("expected error snapshot to carry partial prompt, got %#v", errorEvent.Payload.Message)
	}
	if errorEvent.Payload.Message.ReasoningContent != "thinking" {
		t.Fatalf("expected error snapshot to carry reasoning content, got %#v", errorEvent.Payload.Message)
	}
}

func TestCanvasChatSummaryBranchSelectionRestoresDefaultContext(t *testing.T) {
	session := &model.CanvasSession{
		Mode:                    model.CanvasModeChat,
		ClearContextMessageId:   0,
		SummaryPrompt:           "default summary",
		LastSummarizedMessageId: 11,
	}

	storedPrompt, storedLast, err := setCanvasChatSummary(session, 90, "branch summary", 120)
	if err != nil {
		t.Fatalf("failed to add branch summary: %v", err)
	}
	session.SummaryPrompt = storedPrompt
	session.LastSummarizedMessageId = storedLast

	session.ClearContextMessageId = 90
	branchSummary, branchCursor, err := getCanvasChatActiveSummary(session)
	if err != nil {
		t.Fatalf("failed to read branch summary: %v", err)
	}
	if branchSummary != "branch summary" || branchCursor != 120 {
		t.Fatalf("expected active branch summary/cursor to restore, got summary=%q cursor=%d", branchSummary, branchCursor)
	}

	session.ClearContextMessageId = 0
	restoredSummary, restoredCursor, err := getCanvasChatActiveSummary(session)
	if err != nil {
		t.Fatalf("failed to restore default summary: %v", err)
	}
	if restoredSummary != "default summary" || restoredCursor != 11 {
		t.Fatalf("expected default summary/cursor to be restored, got summary=%q cursor=%d", restoredSummary, restoredCursor)
	}
	if getCanvasChatSummaryCursorForClearContext(session, 90) != 120 {
		t.Fatalf("expected branch cursor lookup to remain available after restoring context")
	}
}

func TestCanvasChatSummaryWriteBackSkipsStaleBranchChanges(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)
	seedCanvasChatCapability(t, db, 1, "default", "default", "default", "gpt-chat-test")

	queueCanvasChatBackgroundTask = func(fn func()) {
		if fn != nil {
			fn()
		}
	}

	summaryRelayCalls := 0
	callCanvasChatRelay = func(ctx context.Context, request canvasChatRelayRequest, onDelta func(delta canvasChatRelayDelta) error) (*canvasChatRelayResult, error) {
		if len(request.Messages) > 0 && strings.Contains(request.Messages[0].StringContent(), "你负责维护长对话摘要记忆") {
			summaryRelayCalls++
			if summaryRelayCalls == 1 {
				if err := model.UpdateCanvasSessionFields(1, 1, map[string]interface{}{
					"clear_context_message_id": 999,
				}); err != nil {
					t.Fatalf("failed to mutate session clear context during summary: %v", err)
				}
			}
			return &canvasChatRelayResult{Text: "summary memory"}, nil
		}
		if onDelta != nil {
			if err := onDelta(canvasChatRelayDelta{Content: "assistant reply"}); err != nil {
				return nil, err
			}
		}
		return &canvasChatRelayResult{Text: "assistant reply"}, nil
	}

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{
		Mode:         model.CanvasModeChat,
		CurrentModel: "gpt-chat-test",
	})
	if err != nil {
		t.Fatalf("failed to create chat session: %v", err)
	}
	if _, err := UpdateCanvasSession(1, session.Id, UpdateCanvasSessionInput{
		SummaryTriggerMessages: common.GetPointer(0),
		SummaryRecentMessages:  common.GetPointer(0),
	}); err != nil {
		t.Fatalf("failed to reduce summary thresholds: %v", err)
	}

	contextCount := 1
	for i := 1; i <= 3; i++ {
		if _, err := CreateCanvasMessage(1, session.Id, CreateCanvasMessageInput{
			Prompt:       fmt.Sprintf("user-%d", i),
			ModelId:      "gpt-chat-test",
			ContextCount: &contextCount,
		}); err != nil {
			t.Fatalf("failed to create chat message %d: %v", i, err)
		}
	}

	reloadedSession, err := model.GetCanvasSessionByID(1, session.Id)
	if err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}
	if reloadedSession.SummaryPrompt != "" || reloadedSession.LastSummarizedMessageId != 0 {
		t.Fatalf("expected stale summary write-back to be skipped, got prompt=%q cursor=%d", reloadedSession.SummaryPrompt, reloadedSession.LastSummarizedMessageId)
	}
}

func TestCanvasChatSummaryTriggerAdvancesCursor(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)
	seedCanvasChatCapability(t, db, 1, "default", "default", "default", "gpt-chat-test")

	queueCanvasChatBackgroundTask = func(fn func()) {
		if fn != nil {
			fn()
		}
	}

	replyIndex := 0
	summaryCalls := 0
	callCanvasChatRelay = func(ctx context.Context, request canvasChatRelayRequest, onDelta func(delta canvasChatRelayDelta) error) (*canvasChatRelayResult, error) {
		if len(request.Messages) > 0 && strings.Contains(request.Messages[0].StringContent(), "你负责维护长对话摘要记忆") {
			summaryCalls++
			return &canvasChatRelayResult{Text: "summary memory"}, nil
		}
		replyIndex++
		reply := fmt.Sprintf("assistant-%d", replyIndex)
		if onDelta != nil {
			if err := onDelta(canvasChatRelayDelta{Content: reply}); err != nil {
				return nil, err
			}
		}
		return &canvasChatRelayResult{Text: reply}, nil
	}

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeChat})
	if err != nil {
		t.Fatalf("failed to create chat session: %v", err)
	}

	contextCount := 1
	assistantIDs := make([]int, 0, 3)
	if _, err := UpdateCanvasSession(1, session.Id, UpdateCanvasSessionInput{
		SummaryTriggerMessages: common.GetPointer(0),
		SummaryRecentMessages:  common.GetPointer(0),
	}); err != nil {
		t.Fatalf("failed to update summary policy: %v", err)
	}
	for i := 1; i <= 3; i++ {
		created, err := CreateCanvasMessage(1, session.Id, CreateCanvasMessageInput{
			Prompt:       fmt.Sprintf("user-%d", i),
			ModelId:      "gpt-chat-test",
			ContextCount: &contextCount,
		})
		if err != nil {
			t.Fatalf("failed to create chat message %d: %v", i, err)
		}
		assistantIDs = append(assistantIDs, created[1].Id)
	}

	if summaryCalls == 0 {
		t.Fatal("expected summary request to be triggered")
	}

	reloadedSession, err := model.GetCanvasSessionByID(1, session.Id)
	if err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}
	if reloadedSession.SummaryPrompt != "summary memory" {
		t.Fatalf("expected summary prompt to persist, got %q", reloadedSession.SummaryPrompt)
	}
	if reloadedSession.LastSummarizedMessageId != assistantIDs[1] {
		t.Fatalf("expected summary cursor to advance to second assistant message %d, got %d", assistantIDs[1], reloadedSession.LastSummarizedMessageId)
	}
}

func TestCanvasChatClearContextTruncatesRelayHistory(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)
	seedCanvasChatCapability(t, db, 1, "default", "default", "default", "gpt-chat-test")

	var capturedRequests [][]dto.Message
	replyIndex := 0
	callCanvasChatRelay = func(ctx context.Context, request canvasChatRelayRequest, onDelta func(delta canvasChatRelayDelta) error) (*canvasChatRelayResult, error) {
		cloned := make([]dto.Message, len(request.Messages))
		copy(cloned, request.Messages)
		capturedRequests = append(capturedRequests, cloned)
		replyIndex++
		reply := fmt.Sprintf("assistant-%d", replyIndex)
		if onDelta != nil {
			if err := onDelta(canvasChatRelayDelta{Content: reply}); err != nil {
				return nil, err
			}
		}
		return &canvasChatRelayResult{Text: reply}, nil
	}

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeChat})
	if err != nil {
		t.Fatalf("failed to create chat session: %v", err)
	}

	first, err := CreateCanvasMessage(1, session.Id, CreateCanvasMessageInput{
		Prompt:  "first turn",
		ModelId: "gpt-chat-test",
	})
	if err != nil {
		t.Fatalf("failed to create first chat message: %v", err)
	}
	second, err := CreateCanvasMessage(1, session.Id, CreateCanvasMessageInput{
		Prompt:  "second turn",
		ModelId: "gpt-chat-test",
	})
	if err != nil {
		t.Fatalf("failed to create second chat message: %v", err)
	}
	if err := model.UpdateCanvasSessionFields(1, session.Id, map[string]interface{}{
		"clear_context_message_id": second[1].Id,
	}); err != nil {
		t.Fatalf("failed to update clear_context_message_id: %v", err)
	}

	if _, err := CreateCanvasMessage(1, session.Id, CreateCanvasMessageInput{
		Prompt:  "fresh turn",
		ModelId: "gpt-chat-test",
	}); err != nil {
		t.Fatalf("failed to create fresh chat message: %v", err)
	}

	if len(capturedRequests) != 3 {
		t.Fatalf("expected 3 captured relay requests, got %d", len(capturedRequests))
	}
	thirdRequest := capturedRequests[2]
	if len(thirdRequest) != 1 {
		t.Fatalf("expected clear-context request to send only the current user message, got %#v", thirdRequest)
	}
	if thirdRequest[0].Role != model.CanvasMessageRoleUser || thirdRequest[0].StringContent() != "fresh turn" {
		t.Fatalf("unexpected clear-context relay payload: %#v", thirdRequest)
	}
	if first[1].Id >= second[1].Id {
		t.Fatalf("expected message ids to increase across turns, got first=%d second=%d", first[1].Id, second[1].Id)
	}
}

func TestUpdateCanvasChatSessionClearContextControls(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeChat})
	if err != nil {
		t.Fatalf("failed to create chat session: %v", err)
	}

	now := common.GetTimestamp()
	messages := []*model.CanvasMessage{
		{
			SessionId:   session.Id,
			UserId:      1,
			Mode:        model.CanvasModeChat,
			Role:        model.CanvasMessageRoleUser,
			Prompt:      "user-1",
			Status:      model.CanvasMessageStatusSuccess,
			CreatedTime: now,
			UpdatedTime: now,
		},
		{
			SessionId:   session.Id,
			UserId:      1,
			Mode:        model.CanvasModeChat,
			Role:        model.CanvasMessageRoleAssistant,
			Prompt:      "assistant-1",
			Status:      model.CanvasMessageStatusSuccess,
			CreatedTime: now + 1,
			UpdatedTime: now + 1,
		},
	}
	for _, message := range messages {
		if err := model.CreateCanvasMessage(message); err != nil {
			t.Fatalf("failed to create chat message: %v", err)
		}
	}

	cleared, err := UpdateCanvasSession(1, session.Id, UpdateCanvasSessionInput{
		ClearContextToLatest: common.GetPointer(true),
	})
	if err != nil {
		t.Fatalf("failed to clear chat context to latest: %v", err)
	}
	if cleared.ClearContextMessageId != messages[1].Id {
		t.Fatalf("expected clear_context_message_id %d, got %d", messages[1].Id, cleared.ClearContextMessageId)
	}

	restored, err := UpdateCanvasSession(1, session.Id, UpdateCanvasSessionInput{
		ClearContextMessageId: common.GetPointer(0),
	})
	if err != nil {
		t.Fatalf("failed to restore full chat context: %v", err)
	}
	if restored.ClearContextMessageId != 0 {
		t.Fatalf("expected restored clear_context_message_id to be 0, got %d", restored.ClearContextMessageId)
	}

	if _, err := UpdateCanvasSession(1, session.Id, UpdateCanvasSessionInput{
		ClearContextMessageId: common.GetPointer(messages[0].Id + 999),
	}); err == nil {
		t.Fatal("expected invalid clear_context_message_id to be rejected")
	}

	imageSession, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeImage})
	if err != nil {
		t.Fatalf("failed to create image session: %v", err)
	}
	if _, err := UpdateCanvasSession(1, imageSession.Id, UpdateCanvasSessionInput{
		ClearContextToLatest: common.GetPointer(true),
	}); err == nil {
		t.Fatal("expected clear context update on non-chat session to fail")
	}

	var sessionCount int64
	if err := db.Model(&model.CanvasSession{}).Where("id = ?", session.Id).Count(&sessionCount).Error; err != nil {
		t.Fatalf("failed to verify session remains persisted: %v", err)
	}
	if sessionCount != 1 {
		t.Fatalf("expected chat session to remain persisted, count=%d", sessionCount)
	}
}

func TestListCanvasMessagesIncludesEffectiveVideoResultURL(t *testing.T) {
	setupCanvasSessionServiceTestDB(t)

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeVideo})
	if err != nil {
		t.Fatalf("failed to create video session: %v", err)
	}

	payload, err := common.Marshal(map[string]any{
		"content": map[string]any{
			"video_url": "https://cdn.example.com/canvas.mp4",
		},
	})
	if err != nil {
		t.Fatalf("failed to marshal task payload: %v", err)
	}

	task := &model.Task{
		UserId:     1,
		TaskID:     "task_canvas_direct",
		Action:     constant.TaskActionTextGenerate,
		Status:     model.TaskStatusSuccess,
		Progress:   "100%",
		SubmitTime: common.GetTimestamp(),
		Properties: model.Properties{
			Input:             "canvas video prompt",
			OriginModelName:   "sora-compatible",
			UpstreamModelName: "sora-compatible",
			RequestParams:     `{"input_reference":"https://cdn.example.com/reference.png"}`,
		},
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://gateway.example.com/v1/videos/task_canvas_direct/content",
		},
		Data: payload,
	}
	if err := model.DB.Create(task).Error; err != nil {
		t.Fatalf("failed to create video task: %v", err)
	}

	message := &model.CanvasMessage{
		SessionId:   session.Id,
		UserId:      1,
		Mode:        model.CanvasModeVideo,
		Role:        model.CanvasMessageRoleAssistant,
		Prompt:      "canvas video prompt",
		Status:      dto.VideoStatusCompleted,
		TaskId:      strconv.FormatInt(task.ID, 10),
		TaskType:    model.CanvasTaskTypeVideo,
		CreatedTime: common.GetTimestamp(),
		UpdatedTime: common.GetTimestamp(),
	}
	if err := model.DB.Create(message).Error; err != nil {
		t.Fatalf("failed to create canvas message: %v", err)
	}

	messages, err := ListCanvasMessages(1, session.Id)
	if err != nil {
		t.Fatalf("ListCanvasMessages returned error: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 canvas message, got %d", len(messages))
	}
	if messages[0].VideoTask == nil {
		t.Fatal("expected attached video task")
	}
	if messages[0].VideoTask.ResultURL != "https://cdn.example.com/canvas.mp4" {
		t.Fatalf("expected direct canvas result url, got %q", messages[0].VideoTask.ResultURL)
	}
	if len(messages[0].ReferenceImages) != 1 || messages[0].ReferenceImages[0] != "https://cdn.example.com/reference.png" {
		t.Fatalf("expected attached video reference image, got %#v", messages[0].ReferenceImages)
	}
}

func TestListCanvasMessageTimelinePaginatesAndCarriesImageReferences(t *testing.T) {
	setupCanvasSessionServiceTestDB(t)

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeImage})
	if err != nil {
		t.Fatalf("failed to create image session: %v", err)
	}

	referenceURL := "/api/image-generation/files/image-generation/ref/20260531/reference.png"
	taskA := &model.ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-a",
		Prompt:          "first result",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusSuccess,
		Params:          `{"size":"1024x1024","reference_images":["` + referenceURL + `"]}`,
		ImageUrl:        "/api/image-generation/files/image-generation/20260531/result-a.png",
		ThumbnailUrl:    "/api/image-generation/files/image-generation/thumb/20260531/result-a.jpg",
		ImageMetadata:   `{"width":1024,"height":1024}`,
		CreatedTime:     common.GetTimestamp(),
	}
	if err := model.DB.Create(taskA).Error; err != nil {
		t.Fatalf("failed to create first image task: %v", err)
	}
	taskB := &model.ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-b",
		Prompt:          "second result",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusSuccess,
		Params:          `{"size":"1536x1024"}`,
		ImageUrl:        "/api/image-generation/files/image-generation/20260531/result-b.png",
		ThumbnailUrl:    "/api/image-generation/files/image-generation/thumb/20260531/result-b.jpg",
		ImageMetadata:   `{"width":1536,"height":1024}`,
		CreatedTime:     common.GetTimestamp(),
	}
	if err := model.DB.Create(taskB).Error; err != nil {
		t.Fatalf("failed to create second image task: %v", err)
	}

	for _, msg := range []*model.CanvasMessage{
		{SessionId: session.Id, UserId: 1, Mode: model.CanvasModeImage, Role: model.CanvasMessageRoleUser, Prompt: "first prompt", CreatedTime: 1, UpdatedTime: 1},
		{SessionId: session.Id, UserId: 1, Mode: model.CanvasModeImage, Role: model.CanvasMessageRoleAssistant, Prompt: "first result", Status: model.ImageTaskStatusSuccess, TaskId: strconv.Itoa(taskA.Id), TaskType: model.CanvasTaskTypeImage, CreatedTime: 2, UpdatedTime: 2},
		{SessionId: session.Id, UserId: 1, Mode: model.CanvasModeImage, Role: model.CanvasMessageRoleUser, Prompt: "second prompt", CreatedTime: 3, UpdatedTime: 3},
		{SessionId: session.Id, UserId: 1, Mode: model.CanvasModeImage, Role: model.CanvasMessageRoleAssistant, Prompt: "second result", Status: model.ImageTaskStatusSuccess, TaskId: strconv.Itoa(taskB.Id), TaskType: model.CanvasTaskTypeImage, CreatedTime: 4, UpdatedTime: 4},
	} {
		if err := model.CreateCanvasMessage(msg); err != nil {
			t.Fatalf("failed to create canvas message: %v", err)
		}
	}

	page, err := ListCanvasMessageTimeline(1, session.Id, 3, "")
	if err != nil {
		t.Fatalf("ListCanvasMessageTimeline returned error: %v", err)
	}
	if len(page.Items) != 3 {
		t.Fatalf("expected 3 timeline items, got %d", len(page.Items))
	}
	if !page.HasMore {
		t.Fatal("expected first timeline page to report has_more")
	}
	if page.NextCursor != encodeCanvasMessageTimelineCursor(page.Items[0].CanvasMessage) {
		t.Fatalf("expected next cursor %q, got %q", encodeCanvasMessageTimelineCursor(page.Items[0].CanvasMessage), page.NextCursor)
	}
	if len(page.Items[0].ReferenceImages) != 1 || page.Items[0].ReferenceImages[0] != referenceURL {
		t.Fatalf("expected image references on timeline item, got %#v", page.Items[0].ReferenceImages)
	}
	if page.Items[0].ImageTask == nil {
		t.Fatal("expected attached image task on timeline item")
	}
	if strings.Contains(page.Items[0].ImageTask.Params, "reference_image") {
		t.Fatalf("expected sanitized image params on timeline item, got %q", page.Items[0].ImageTask.Params)
	}

	nextPage, err := ListCanvasMessageTimeline(1, session.Id, 3, page.NextCursor)
	if err != nil {
		t.Fatalf("failed to load second timeline page: %v", err)
	}
	if len(nextPage.Items) != 1 || nextPage.Items[0].Role != model.CanvasMessageRoleUser {
		t.Fatalf("expected older single user message on second page, got %#v", nextPage.Items)
	}
	if nextPage.HasMore {
		t.Fatal("expected second timeline page to be terminal")
	}
}

func TestListCanvasMessageTimelineUsesCreatedTimeAndIDCursorOrdering(t *testing.T) {
	setupCanvasSessionServiceTestDB(t)

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeImage})
	if err != nil {
		t.Fatalf("failed to create image session: %v", err)
	}

	created := []*model.CanvasMessage{
		{SessionId: session.Id, UserId: 1, Mode: model.CanvasModeImage, Role: model.CanvasMessageRoleUser, Prompt: "time-100", CreatedTime: 100, UpdatedTime: 100},
		{SessionId: session.Id, UserId: 1, Mode: model.CanvasModeImage, Role: model.CanvasMessageRoleUser, Prompt: "time-050-a", CreatedTime: 50, UpdatedTime: 50},
		{SessionId: session.Id, UserId: 1, Mode: model.CanvasModeImage, Role: model.CanvasMessageRoleUser, Prompt: "time-050-b", CreatedTime: 50, UpdatedTime: 50},
		{SessionId: session.Id, UserId: 1, Mode: model.CanvasModeImage, Role: model.CanvasMessageRoleUser, Prompt: "time-150", CreatedTime: 150, UpdatedTime: 150},
	}
	for _, message := range created {
		if err := model.CreateCanvasMessage(message); err != nil {
			t.Fatalf("failed to create canvas message: %v", err)
		}
	}

	firstPage, err := ListCanvasMessageTimeline(1, session.Id, 2, "")
	if err != nil {
		t.Fatalf("failed to load first timeline page: %v", err)
	}
	if len(firstPage.Items) != 2 {
		t.Fatalf("expected 2 items on first page, got %d", len(firstPage.Items))
	}
	if got := []string{firstPage.Items[0].Prompt, firstPage.Items[1].Prompt}; !equalStrings(got, []string{"time-100", "time-150"}) {
		t.Fatalf("unexpected first page ordering: %#v", got)
	}
	if !firstPage.HasMore {
		t.Fatal("expected more history on first page")
	}

	secondPage, err := ListCanvasMessageTimeline(1, session.Id, 2, firstPage.NextCursor)
	if err != nil {
		t.Fatalf("failed to load second timeline page: %v", err)
	}
	if len(secondPage.Items) != 2 {
		t.Fatalf("expected 2 items on second page, got %d", len(secondPage.Items))
	}
	if got := []string{secondPage.Items[0].Prompt, secondPage.Items[1].Prompt}; !equalStrings(got, []string{"time-050-a", "time-050-b"}) {
		t.Fatalf("unexpected second page ordering: %#v", got)
	}
	if secondPage.HasMore {
		t.Fatal("expected second page to be terminal")
	}
}

func TestListCanvasMessagesBatchLoadsVideoModelMappingsOnce(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeVideo})
	if err != nil {
		t.Fatalf("failed to create video session: %v", err)
	}

	for _, mapping := range []*model.ModelMapping{
		{RequestModel: "video-alpha", ActualModel: "video-alpha", DisplayName: "Video Alpha", ModelType: 3, Status: 1, RequestEndpoint: "openai-video"},
		{RequestModel: "video-beta", ActualModel: "video-beta", DisplayName: "Video Beta", ModelType: 3, Status: 1, RequestEndpoint: "openai-video-generation"},
	} {
		if err := db.Create(mapping).Error; err != nil {
			t.Fatalf("failed to create model mapping: %v", err)
		}
	}

	var createdTasks []*model.Task
	for index, modelID := range []string{"video-alpha", "video-beta", "video-alpha"} {
		task := &model.Task{
			UserId:     1,
			TaskID:     fmt.Sprintf("task_video_batch_%d", index),
			Action:     constant.TaskActionTextGenerate,
			Status:     model.TaskStatusSuccess,
			Progress:   "100%",
			SubmitTime: common.GetTimestamp(),
			Properties: model.Properties{
				Input:             fmt.Sprintf("video prompt %d", index),
				OriginModelName:   modelID,
				UpstreamModelName: modelID,
			},
		}
		if err := db.Create(task).Error; err != nil {
			t.Fatalf("failed to create video task: %v", err)
		}
		createdTasks = append(createdTasks, task)
		if err := model.CreateCanvasMessage(&model.CanvasMessage{
			SessionId:   session.Id,
			UserId:      1,
			Mode:        model.CanvasModeVideo,
			Role:        model.CanvasMessageRoleAssistant,
			Prompt:      task.Properties.Input,
			Status:      dto.VideoStatusCompleted,
			TaskId:      strconv.FormatInt(task.ID, 10),
			TaskType:    model.CanvasTaskTypeVideo,
			CreatedTime: int64(index + 1),
			UpdatedTime: int64(index + 1),
		}); err != nil {
			t.Fatalf("failed to create canvas message: %v", err)
		}
	}

	callbackName := "count_canvas_video_model_mapping_queries"
	modelMappingQueries := 0
	if err := db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "model_mappings" {
			modelMappingQueries++
		}
	}); err != nil {
		t.Fatalf("failed to register query callback: %v", err)
	}
	defer func() {
		_ = db.Callback().Query().Remove(callbackName)
	}()

	messages, err := ListCanvasMessages(1, session.Id)
	if err != nil {
		t.Fatalf("ListCanvasMessages returned error: %v", err)
	}
	if len(messages) != len(createdTasks) {
		t.Fatalf("expected %d video messages, got %d", len(createdTasks), len(messages))
	}
	if modelMappingQueries != 1 {
		t.Fatalf("expected one batched model mapping query, got %d", modelMappingQueries)
	}
	for _, message := range messages {
		if message.VideoTask == nil || strings.TrimSpace(message.VideoTask.DisplayName) == "" {
			t.Fatalf("expected attached video task display name, got %#v", message.VideoTask)
		}
	}
}

func TestListCanvasMessagesDoesNotFallbackPerVideoTaskWhenModelMappingMissing(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeVideo})
	if err != nil {
		t.Fatalf("failed to create video session: %v", err)
	}

	if err := db.Create(&model.ModelMapping{
		RequestModel:    "video-alpha",
		ActualModel:     "video-alpha",
		DisplayName:     "Video Alpha",
		ModelType:       3,
		Status:          1,
		RequestEndpoint: "openai-video",
	}).Error; err != nil {
		t.Fatalf("failed to create model mapping: %v", err)
	}

	taskModels := []string{"video-alpha", "video-missing", "video-missing"}
	for index, modelID := range taskModels {
		task := &model.Task{
			UserId:     1,
			TaskID:     fmt.Sprintf("task_video_missing_%d", index),
			Action:     constant.TaskActionTextGenerate,
			Status:     model.TaskStatusSuccess,
			Progress:   "100%",
			SubmitTime: common.GetTimestamp(),
			Properties: model.Properties{
				Input:             fmt.Sprintf("video prompt %d", index),
				OriginModelName:   modelID,
				UpstreamModelName: modelID,
			},
		}
		if err := db.Create(task).Error; err != nil {
			t.Fatalf("failed to create video task: %v", err)
		}
		if err := model.CreateCanvasMessage(&model.CanvasMessage{
			SessionId:   session.Id,
			UserId:      1,
			Mode:        model.CanvasModeVideo,
			Role:        model.CanvasMessageRoleAssistant,
			Prompt:      task.Properties.Input,
			Status:      dto.VideoStatusCompleted,
			TaskId:      strconv.FormatInt(task.ID, 10),
			TaskType:    model.CanvasTaskTypeVideo,
			CreatedTime: int64(index + 1),
			UpdatedTime: int64(index + 1),
		}); err != nil {
			t.Fatalf("failed to create canvas message: %v", err)
		}
	}

	callbackName := "count_canvas_video_missing_model_mapping_queries"
	modelMappingQueries := 0
	if err := db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "model_mappings" {
			modelMappingQueries++
		}
	}); err != nil {
		t.Fatalf("failed to register query callback: %v", err)
	}
	defer func() {
		_ = db.Callback().Query().Remove(callbackName)
	}()

	messages, err := ListCanvasMessages(1, session.Id)
	if err != nil {
		t.Fatalf("ListCanvasMessages returned error: %v", err)
	}
	if modelMappingQueries != 1 {
		t.Fatalf("expected one batched model mapping query even with missing mappings, got %d", modelMappingQueries)
	}
	if len(messages) != len(taskModels) {
		t.Fatalf("expected %d video messages, got %d", len(taskModels), len(messages))
	}

	var missingCount int
	for _, message := range messages {
		if message.VideoTask == nil {
			t.Fatalf("expected attached video task, got %#v", message)
		}
		switch message.VideoTask.ModelID {
		case "video-alpha":
			if message.VideoTask.DisplayName != "Video Alpha" {
				t.Fatalf("expected mapped display name for video-alpha, got %#v", message.VideoTask)
			}
			if message.VideoTask.RequestEndpoint != "openai-video" {
				t.Fatalf("expected mapped request endpoint for video-alpha, got %#v", message.VideoTask)
			}
		case "video-missing":
			missingCount++
			if message.VideoTask.DisplayName != "video-missing" {
				t.Fatalf("expected fallback display name for missing mapping, got %#v", message.VideoTask)
			}
			if message.VideoTask.RequestEndpoint != "" {
				t.Fatalf("expected empty request endpoint for missing mapping, got %#v", message.VideoTask)
			}
		default:
			t.Fatalf("unexpected model id in video task summary: %#v", message.VideoTask)
		}
	}
	if missingCount != 2 {
		t.Fatalf("expected 2 missing-mapping video tasks, got %d", missingCount)
	}
}

func equalStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func TestCreateCanvasMessagesAllowEmptyClientRequestID(t *testing.T) {
	setupCanvasSessionServiceTestDB(t)

	createImageGenerationTaskForCanvas = func(userId int, modelId string, selectedGroup string, prompt string, requestEndpoint string, params string) (*model.ImageGenerationTask, error) {
		task := &model.ImageGenerationTask{
			UserId:          userId,
			ModelId:         modelId,
			SelectedGroup:   selectedGroup,
			Prompt:          prompt,
			RequestEndpoint: requestEndpoint,
			Status:          model.ImageTaskStatusPending,
			Params:          params,
			CreatedTime:     common.GetTimestamp(),
		}
		if err := task.Insert(); err != nil {
			return nil, err
		}
		return task, nil
	}

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeImage})
	if err != nil {
		t.Fatalf("failed to create image session: %v", err)
	}
	created, err := CreateCanvasMessage(1, session.Id, CreateCanvasMessageInput{
		Prompt:          "legacy image",
		ModelId:         "gpt-image-test",
		Group:           "default",
		RequestEndpoint: "openai",
		Params:          `{}`,
	})
	if err != nil {
		t.Fatalf("failed to create legacy image message: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("expected 2 created legacy messages, got %d", len(created))
	}
	for _, message := range created {
		if message.ClientRequestId != "" {
			t.Fatalf("expected empty client_request_id for legacy create, got %#v", message)
		}
	}

	reloaded, err := ListCanvasMessages(1, session.Id)
	if err != nil {
		t.Fatalf("failed to reload legacy image messages: %v", err)
	}
	for _, message := range reloaded {
		if message.ClientRequestId != "" {
			t.Fatalf("expected empty reloaded client_request_id for legacy flow, got %#v", message)
		}
	}
}

func TestCanvasMessagesAreUserIsolated(t *testing.T) {
	setupCanvasSessionServiceTestDB(t)

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeImage})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	if _, err := ListCanvasMessages(2, session.Id); err == nil {
		t.Fatal("expected other user to be denied session messages")
	}
	title := "stolen"
	if _, err := UpdateCanvasSession(2, session.Id, UpdateCanvasSessionInput{Title: &title}); err == nil {
		t.Fatal("expected other user to be denied session update")
	}
	if err := DeleteCanvasSession(2, session.Id); err == nil {
		t.Fatal("expected other user to be denied session delete")
	}
}

func TestDeleteCanvasSessionSoftDeletesSessionMessagesAndAssociatedTasks(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeImage, Title: "delete me"})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	imageTask := &model.ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-test",
		Prompt:          "image",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusSuccess,
		ImageUrl:        "/api/image-generation/files/image-generation/result/test.png",
		CreatedTime:     common.GetTimestamp(),
	}
	if err := db.Create(imageTask).Error; err != nil {
		t.Fatalf("failed to create image task: %v", err)
	}
	for _, msg := range []*model.CanvasMessage{
		{SessionId: session.Id, UserId: 1, Mode: model.CanvasModeImage, Role: model.CanvasMessageRoleUser, Prompt: "image"},
		{SessionId: session.Id, UserId: 1, Mode: model.CanvasModeImage, Role: model.CanvasMessageRoleAssistant, Prompt: "image", Status: model.ImageTaskStatusSuccess, TaskId: strconv.Itoa(imageTask.Id), TaskType: model.CanvasTaskTypeImage},
	} {
		if err := model.CreateCanvasMessage(msg); err != nil {
			t.Fatalf("failed to create canvas message: %v", err)
		}
	}

	if err := DeleteCanvasSession(1, session.Id); err != nil {
		t.Fatalf("failed to delete canvas session: %v", err)
	}
	if reloaded, err := model.GetCanvasSessionByID(1, session.Id); err != nil || reloaded != nil {
		t.Fatalf("expected session to be soft deleted, got %#v err=%v", reloaded, err)
	}
	messages, err := model.ListCanvasSessionMessageTaskRefsForMode(model.CanvasModeImage, 1, session.Id)
	if err != nil {
		t.Fatalf("failed to list message refs after delete: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("expected message refs to be soft deleted, got %d", len(messages))
	}

	var imageTaskCount int64
	if err := db.Model(&model.ImageGenerationTask{}).Where("id = ?", imageTask.Id).Count(&imageTaskCount).Error; err != nil {
		t.Fatalf("failed to count image task: %v", err)
	}
	if imageTaskCount != 0 {
		t.Fatalf("expected associated image task to be deleted, count=%d", imageTaskCount)
	}
}

func TestDeleteCanvasSessionRollsBackWhenMessageSoftDeleteFails(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeImage, Title: "rollback"})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	imageTask := &model.ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-test",
		Prompt:          "image rollback",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusSuccess,
		CreatedTime:     common.GetTimestamp(),
	}
	if err := db.Create(imageTask).Error; err != nil {
		t.Fatalf("failed to create image task: %v", err)
	}
	for _, msg := range []*model.CanvasMessage{
		{SessionId: session.Id, UserId: 1, Mode: model.CanvasModeImage, Role: model.CanvasMessageRoleUser, Prompt: "image"},
		{SessionId: session.Id, UserId: 1, Mode: model.CanvasModeImage, Role: model.CanvasMessageRoleAssistant, Prompt: "image", Status: model.ImageTaskStatusSuccess, TaskId: strconv.Itoa(imageTask.Id), TaskType: model.CanvasTaskTypeImage},
	} {
		if err := model.CreateCanvasMessage(msg); err != nil {
			t.Fatalf("failed to create canvas message: %v", err)
		}
	}

	updateCallbackName := "fail_canvas_message_soft_delete"
	if err := db.Callback().Update().Before("gorm:update").Register(updateCallbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "canvas_messages" {
			tx.AddError(fmt.Errorf("boom"))
		}
	}); err != nil {
		t.Fatalf("failed to register update callback: %v", err)
	}
	defer func() {
		_ = db.Callback().Update().Remove(updateCallbackName)
	}()

	if err := DeleteCanvasSession(1, session.Id); err == nil {
		t.Fatal("expected DeleteCanvasSession to fail when message soft delete fails")
	}

	reloadedSession, err := model.GetCanvasSessionByID(1, session.Id)
	if err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}
	if reloadedSession == nil || reloadedSession.DeletedTime != 0 {
		t.Fatalf("expected session deletion to roll back, got %#v", reloadedSession)
	}

	messages, err := model.ListCanvasSessionMessageTaskRefsForMode(model.CanvasModeImage, 1, session.Id)
	if err != nil {
		t.Fatalf("failed to list message refs after rollback: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected messages to remain visible after rollback, got %d", len(messages))
	}

	var imageTaskCount int64
	if err := db.Model(&model.ImageGenerationTask{}).Where("id = ?", imageTask.Id).Count(&imageTaskCount).Error; err != nil {
		t.Fatalf("failed to count image task after rollback: %v", err)
	}
	if imageTaskCount != 1 {
		t.Fatalf("expected image task deletion to roll back, count=%d", imageTaskCount)
	}

	var cleanupJobCount int64
	if err := db.Model(&model.CanvasAssetCleanupJob{}).Count(&cleanupJobCount).Error; err != nil {
		t.Fatalf("failed to count cleanup jobs after rollback: %v", err)
	}
	if cleanupJobCount != 0 {
		t.Fatalf("expected cleanup job creation to roll back, count=%d", cleanupJobCount)
	}
}

func TestDeleteCanvasSessionRejectsRunningAssociatedTasks(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeVideo, Title: "running"})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	videoTask := &model.Task{
		UserId:     1,
		TaskID:     "task_running_video",
		Action:     constant.TaskActionTextGenerate,
		Status:     model.TaskStatusQueued,
		SubmitTime: common.GetTimestamp(),
		Properties: model.Properties{
			Input: "video",
		},
	}
	if err := db.Create(videoTask).Error; err != nil {
		t.Fatalf("failed to create video task: %v", err)
	}
	if err := model.CreateCanvasMessage(&model.CanvasMessage{
		SessionId: session.Id,
		UserId:    1,
		Mode:      model.CanvasModeVideo,
		Role:      model.CanvasMessageRoleAssistant,
		Prompt:    "video",
		Status:    dto.VideoStatusQueued,
		TaskId:    strconv.FormatInt(videoTask.ID, 10),
		TaskType:  model.CanvasTaskTypeVideo,
	}); err != nil {
		t.Fatalf("failed to create canvas message: %v", err)
	}

	if err := DeleteCanvasSession(1, session.Id); err == nil {
		t.Fatal("expected running associated task to block session deletion")
	}
	reloaded, err := model.GetCanvasSessionByID(1, session.Id)
	if err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}
	if reloaded == nil {
		t.Fatal("session should remain when deletion is rejected")
	}
}

func TestDeleteCanvasSessionRejectsRunningChatGeneration(t *testing.T) {
	setupCanvasSessionServiceTestDB(t)

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeChat, Title: "running chat"})
	if err != nil {
		t.Fatalf("failed to create chat session: %v", err)
	}
	if err := model.CreateCanvasMessage(&model.CanvasMessage{
		SessionId:   session.Id,
		UserId:      1,
		Mode:        model.CanvasModeChat,
		Role:        model.CanvasMessageRoleAssistant,
		Prompt:      "working",
		Status:      model.CanvasMessageStatusGenerating,
		CreatedTime: common.GetTimestamp(),
		UpdatedTime: common.GetTimestamp(),
	}); err != nil {
		t.Fatalf("failed to create generating chat message: %v", err)
	}

	if err := DeleteCanvasSession(1, session.Id); err == nil {
		t.Fatal("expected running chat generation to block session deletion")
	}
	reloaded, err := model.GetCanvasSessionByID(1, session.Id)
	if err != nil {
		t.Fatalf("failed to reload chat session: %v", err)
	}
	if reloaded == nil {
		t.Fatal("chat session should remain when deletion is rejected")
	}
}

func TestDeleteCanvasSessionBatchesAssociatedTaskLookupsAndDeletes(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeImage, Title: "batched delete"})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	imageTaskIDs := make([]int, 0, 3)
	for index := 0; index < 3; index++ {
		task := &model.ImageGenerationTask{
			UserId:          1,
			ModelId:         "gpt-image-test",
			Prompt:          fmt.Sprintf("image-%d", index),
			RequestEndpoint: "openai",
			Status:          model.ImageTaskStatusSuccess,
			CreatedTime:     common.GetTimestamp(),
		}
		if err := db.Create(task).Error; err != nil {
			t.Fatalf("failed to create image task: %v", err)
		}
		imageTaskIDs = append(imageTaskIDs, task.Id)
		if err := model.CreateCanvasMessage(&model.CanvasMessage{
			SessionId: session.Id,
			UserId:    1,
			Mode:      model.CanvasModeImage,
			Role:      model.CanvasMessageRoleAssistant,
			Prompt:    task.Prompt,
			Status:    model.ImageTaskStatusSuccess,
			TaskId:    strconv.Itoa(task.Id),
			TaskType:  model.CanvasTaskTypeImage,
		}); err != nil {
			t.Fatalf("failed to create image canvas message: %v", err)
		}
	}

	queryCallbackName := "count_canvas_delete_task_queries"
	deleteCallbackName := "count_canvas_delete_task_deletes"
	imageTaskQueries := 0
	imageTaskDeletes := 0
	if err := db.Callback().Query().Before("gorm:query").Register(queryCallbackName, func(tx *gorm.DB) {
		if tx.Statement == nil {
			return
		}
		switch tx.Statement.Table {
		case "image_generation_tasks":
			imageTaskQueries++
		}
	}); err != nil {
		t.Fatalf("failed to register query callback: %v", err)
	}
	if err := db.Callback().Delete().Before("gorm:delete").Register(deleteCallbackName, func(tx *gorm.DB) {
		if tx.Statement == nil {
			return
		}
		switch tx.Statement.Table {
		case "image_generation_tasks":
			imageTaskDeletes++
		}
	}); err != nil {
		t.Fatalf("failed to register delete callback: %v", err)
	}
	defer func() {
		_ = db.Callback().Query().Remove(queryCallbackName)
		_ = db.Callback().Delete().Remove(deleteCallbackName)
	}()

	if err := DeleteCanvasSession(1, session.Id); err != nil {
		t.Fatalf("DeleteCanvasSession returned error: %v", err)
	}
	if imageTaskQueries != 1 {
		t.Fatalf("expected one batched image task query, got %d", imageTaskQueries)
	}
	if imageTaskDeletes != 1 {
		t.Fatalf("expected one batched image task delete, got %d", imageTaskDeletes)
	}

	if reloaded, err := model.GetCanvasSessionByID(1, session.Id); err != nil || reloaded != nil {
		t.Fatalf("expected session to be soft deleted, got %#v err=%v", reloaded, err)
	}
	messages, err := model.ListCanvasSessionMessageTaskRefsForMode(model.CanvasModeImage, 1, session.Id)
	if err != nil {
		t.Fatalf("failed to list message refs after delete: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("expected message refs to be soft deleted, got %d", len(messages))
	}
	var imageTaskCount int64
	if err := db.Model(&model.ImageGenerationTask{}).Where("id IN ?", imageTaskIDs).Count(&imageTaskCount).Error; err != nil {
		t.Fatalf("failed to count image tasks: %v", err)
	}
	if imageTaskCount != 0 {
		t.Fatalf("expected associated image tasks to be deleted, count=%d", imageTaskCount)
	}
}

func TestDeleteCanvasSessionEnqueuesAssetCleanupJobs(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeImage, Title: "cleanup jobs"})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	imageTask := &model.ImageGenerationTask{
		UserId:          1,
		ModelId:         "gpt-image-test",
		Prompt:          "image cleanup",
		RequestEndpoint: "openai",
		Status:          model.ImageTaskStatusSuccess,
		ImageUrl:        "/api/image-generation/files/image-generation/result/image.png",
		ThumbnailUrl:    "/api/image-generation/files/image-generation/result/image-thumb.png",
		Params:          `{"reference_images":["/api/image-generation/files/image-generation/reference/ref.png"]}`,
		CreatedTime:     common.GetTimestamp(),
	}
	if err := db.Create(imageTask).Error; err != nil {
		t.Fatalf("failed to create image task: %v", err)
	}
	for _, msg := range []*model.CanvasMessage{
		{SessionId: session.Id, UserId: 1, Mode: model.CanvasModeImage, Role: model.CanvasMessageRoleAssistant, Prompt: "image", Status: model.ImageTaskStatusSuccess, TaskId: strconv.Itoa(imageTask.Id), TaskType: model.CanvasTaskTypeImage},
	} {
		if err := model.CreateCanvasMessage(msg); err != nil {
			t.Fatalf("failed to create canvas message: %v", err)
		}
	}

	if err := DeleteCanvasSession(1, session.Id); err != nil {
		t.Fatalf("DeleteCanvasSession returned error: %v", err)
	}

	var jobs []*model.CanvasAssetCleanupJob
	if err := db.Order("id asc").Find(&jobs).Error; err != nil {
		t.Fatalf("failed to list cleanup jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 cleanup job, got %d", len(jobs))
	}
	if jobs[0].Status != model.CanvasAssetCleanupJobStatusPending {
		t.Fatalf("expected pending cleanup jobs, got %#v", jobs)
	}

	var imagePayload canvasImageAssetCleanupPayload
	if err := common.UnmarshalJsonStr(jobs[0].Payload, &imagePayload); err != nil {
		t.Fatalf("failed to unmarshal image cleanup payload: %v", err)
	}
	if imagePayload.TaskId != imageTask.Id || imagePayload.ImageURL != imageTask.ImageUrl {
		t.Fatalf("unexpected image cleanup payload: %#v", imagePayload)
	}
}

func TestDeleteCanvasVideoSessionDeletesAssociatedTaskAndEnqueuesCleanupJob(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)

	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{Mode: model.CanvasModeVideo, Title: "video cleanup"})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	videoTask := &model.Task{
		UserId:     1,
		TaskID:     "task_cleanup_video",
		Action:     constant.TaskActionTextGenerate,
		Status:     model.TaskStatusSuccess,
		SubmitTime: common.GetTimestamp(),
		Properties: model.Properties{
			RequestParams: `{"image":"/api/image-generation/files/image-generation/reference/video-ref.png"}`,
		},
		PrivateData: model.TaskPrivateData{
			ResultURL: "/api/image-generation/files/image-generation/result/video.mp4",
		},
	}
	if err := db.Create(videoTask).Error; err != nil {
		t.Fatalf("failed to create video task: %v", err)
	}
	if err := model.CreateCanvasMessage(&model.CanvasMessage{
		SessionId: session.Id,
		UserId:    1,
		Mode:      model.CanvasModeVideo,
		Role:      model.CanvasMessageRoleAssistant,
		Prompt:    "video",
		Status:    dto.VideoStatusCompleted,
		TaskId:    strconv.FormatInt(videoTask.ID, 10),
		TaskType:  model.CanvasTaskTypeVideo,
	}); err != nil {
		t.Fatalf("failed to create video canvas message: %v", err)
	}

	if err := DeleteCanvasSession(1, session.Id); err != nil {
		t.Fatalf("DeleteCanvasSession returned error: %v", err)
	}

	var jobs []*model.CanvasAssetCleanupJob
	if err := db.Order("id asc").Find(&jobs).Error; err != nil {
		t.Fatalf("failed to list cleanup jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 cleanup job, got %d", len(jobs))
	}

	var videoPayload canvasVideoAssetCleanupPayload
	if err := common.UnmarshalJsonStr(jobs[0].Payload, &videoPayload); err != nil {
		t.Fatalf("failed to unmarshal video cleanup payload: %v", err)
	}
	if videoPayload.TaskId != videoTask.ID || videoPayload.ResultURL != videoTask.GetResultURL() {
		t.Fatalf("unexpected video cleanup payload: %#v", videoPayload)
	}

	var videoTaskCount int64
	if err := db.Model(&model.Task{}).Where("id = ?", videoTask.ID).Count(&videoTaskCount).Error; err != nil {
		t.Fatalf("failed to count video task: %v", err)
	}
	if videoTaskCount != 0 {
		t.Fatalf("expected associated video task to be deleted, count=%d", videoTaskCount)
	}
}
