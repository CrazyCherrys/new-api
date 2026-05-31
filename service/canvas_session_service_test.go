package service

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
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
	previousCreateImage := createImageGenerationTaskForCanvas
	previousCreateVideo := createVideoGenerationTaskForCanvas

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(
		&model.User{},
		&model.ModelMapping{},
		&model.CanvasSession{},
		&model.CanvasMessage{},
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

		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
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

	sessions, err := ListCanvasSessions(1, model.CanvasModeImage)
	if err != nil {
		t.Fatalf("failed to list image sessions: %v", err)
	}
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

	videoSessions, err := ListCanvasSessions(1, model.CanvasModeVideo)
	if err != nil {
		t.Fatalf("failed to list video sessions: %v", err)
	}
	if len(videoSessions) != 1 || videoSessions[0].Id != videoSession.Id {
		t.Fatalf("expected only the video session, got %#v", videoSessions)
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
	videoTask := &model.Task{
		UserId:     1,
		TaskID:     "task_delete_video",
		Action:     constant.TaskActionTextGenerate,
		Status:     model.TaskStatusSuccess,
		SubmitTime: common.GetTimestamp(),
		Properties: model.Properties{
			Input: "video",
		},
	}
	if err := db.Create(videoTask).Error; err != nil {
		t.Fatalf("failed to create video task: %v", err)
	}
	for _, msg := range []*model.CanvasMessage{
		{SessionId: session.Id, UserId: 1, Mode: model.CanvasModeImage, Role: model.CanvasMessageRoleUser, Prompt: "image"},
		{SessionId: session.Id, UserId: 1, Mode: model.CanvasModeImage, Role: model.CanvasMessageRoleAssistant, Prompt: "image", Status: model.ImageTaskStatusSuccess, TaskId: strconv.Itoa(imageTask.Id), TaskType: model.CanvasTaskTypeImage},
		{SessionId: session.Id, UserId: 1, Mode: model.CanvasModeVideo, Role: model.CanvasMessageRoleAssistant, Prompt: "video", Status: dto.VideoStatusCompleted, TaskId: strconv.FormatInt(videoTask.ID, 10), TaskType: model.CanvasTaskTypeVideo},
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
	messages, err := model.ListCanvasMessages(1, session.Id)
	if err != nil {
		t.Fatalf("failed to list messages after delete: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("expected messages to be soft deleted, got %d", len(messages))
	}

	var imageTaskCount int64
	if err := db.Model(&model.ImageGenerationTask{}).Where("id = ?", imageTask.Id).Count(&imageTaskCount).Error; err != nil {
		t.Fatalf("failed to count image task: %v", err)
	}
	if imageTaskCount != 0 {
		t.Fatalf("expected associated image task to be deleted, count=%d", imageTaskCount)
	}
	var videoTaskCount int64
	if err := db.Model(&model.Task{}).Where("id = ?", videoTask.ID).Count(&videoTaskCount).Error; err != nil {
		t.Fatalf("failed to count video task: %v", err)
	}
	if videoTaskCount != 0 {
		t.Fatalf("expected associated video task to be deleted, count=%d", videoTaskCount)
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
