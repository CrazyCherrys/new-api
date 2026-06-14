package service

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

func TestBuildVideoTaskSummaryAndDetailUseEffectiveResultURL(t *testing.T) {
	setupCanvasSessionServiceTestDB(t)

	payload, err := common.Marshal(map[string]any{
		"content": map[string]any{
			"video_url": "https://cdn.example.com/detail.mp4",
		},
	})
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	task := &model.Task{
		UserId:     1,
		TaskID:     "task_video_detail",
		Action:     constant.TaskActionTextGenerate,
		Status:     model.TaskStatusSuccess,
		Progress:   "100%",
		SubmitTime: common.GetTimestamp(),
		Properties: model.Properties{
			Input:             "detail prompt",
			OriginModelName:   "sora-compatible",
			UpstreamModelName: "sora-compatible",
		},
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://gateway.example.com/v1/videos/task_video_detail/content",
		},
		Data: payload,
	}
	if err := model.DB.Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	summary := buildVideoTaskSummary(task)
	if summary == nil {
		t.Fatal("expected summary")
	}
	if summary.ResultURL != "https://cdn.example.com/detail.mp4" {
		t.Fatalf("expected direct summary result url, got %q", summary.ResultURL)
	}
	if summary.VideoURL != "/v1/videos/task_video_detail/content" {
		t.Fatalf("expected proxy video_url fallback, got %q", summary.VideoURL)
	}

	detail, err := GetVideoGenerationTaskDetail(1, fmt.Sprint(task.ID))
	if err != nil {
		t.Fatalf("GetVideoGenerationTaskDetail returned error: %v", err)
	}
	if detail.ResultURL != "https://cdn.example.com/detail.mp4" {
		t.Fatalf("expected direct detail result url, got %q", detail.ResultURL)
	}
}

func TestGetCanvasVideoGenerationTaskDetailUsesCanvasProxyFallback(t *testing.T) {
	setupCanvasSessionServiceTestDB(t)

	task := &model.Task{
		UserId:     1,
		TaskID:     "task_canvas_detail",
		Action:     constant.TaskActionTextGenerate,
		Status:     model.TaskStatusSuccess,
		Progress:   "100%",
		SubmitTime: common.GetTimestamp(),
		Properties: model.Properties{
			Input:             "canvas detail prompt",
			OriginModelName:   "sora-compatible",
			UpstreamModelName: "sora-compatible",
		},
		PrivateData: model.TaskPrivateData{
			ResultURL: "/v1/videos/task_canvas_detail/content",
		},
	}
	if err := task.InsertCanvasVideo(); err != nil {
		t.Fatalf("failed to create canvas video task: %v", err)
	}

	detail, err := GetCanvasVideoGenerationTaskDetail(1, fmt.Sprint(task.ID))
	if err != nil {
		t.Fatalf("GetCanvasVideoGenerationTaskDetail returned error: %v", err)
	}
	if detail.VideoURL != "/api/canvas/videos/task_canvas_detail/content" {
		t.Fatalf("expected canvas detail video_url fallback, got %q", detail.VideoURL)
	}
	if detail.ResultURL != "/api/canvas/videos/task_canvas_detail/content" {
		t.Fatalf("expected canvas detail result_url fallback, got %q", detail.ResultURL)
	}
}

type videoRelayRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn videoRelayRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func useVideoRelayHTTPClient(t *testing.T, fn videoRelayRoundTripFunc) {
	t.Helper()

	previousClient := videoRelayHTTPClient
	videoRelayHTTPClient = &http.Client{Transport: fn}
	t.Cleanup(func() {
		videoRelayHTTPClient = previousClient
	})
}

func videoRelayTestResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func seedVideoGenerationCreatePath(t *testing.T, db *gorm.DB, userId int, modelID string) {
	t.Helper()

	if err := db.AutoMigrate(&model.Task{}); err != nil {
		t.Fatalf("failed to migrate task table: %v", err)
	}
	user := &model.User{
		Id:       userId,
		Username: fmt.Sprintf("video-user-%d", userId),
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed video user: %v", err)
	}
	seedUserTokenWithGroup(t, db, userId, fmt.Sprintf("video-token-%d", userId), "default")
	if err := db.Create(&model.ModelMapping{
		RequestModel:      modelID,
		ActualModel:       modelID,
		ModelType:         3,
		Status:            1,
		RequestEndpoint:   "openai-video-generation",
		VideoCapabilities: `["text_to_video"]`,
		DurationOptions:   `[5]`,
	}).Error; err != nil {
		t.Fatalf("failed to seed video model mapping: %v", err)
	}
}

func TestCreateCanvasVideoGenerationTaskSetsInternalScopeHeader(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)
	userId := 61
	modelID := "video-canvas-header"
	seedVideoGenerationCreatePath(t, db, userId, modelID)

	headerSeen := false
	useVideoRelayHTTPClient(t, func(req *http.Request) (*http.Response, error) {
		headerSeen = IsCanvasVideoTaskScopeHeaderValue(req.Header.Get(constant.HeaderCanvasVideoTaskScope))
		if !headerSeen {
			return videoRelayTestResponse(http.StatusBadRequest, "missing canvas video scope header"), nil
		}
		task := &model.Task{
			TaskID:     "task_canvas_header",
			UserId:     userId,
			Action:     constant.TaskActionTextGenerate,
			Status:     model.TaskStatusQueued,
			Progress:   "0%",
			SubmitTime: common.GetTimestamp(),
			Properties: model.Properties{OriginModelName: modelID},
		}
		if err := task.InsertCanvasVideo(); err != nil {
			return videoRelayTestResponse(http.StatusInternalServerError, err.Error()), nil
		}
		return videoRelayTestResponse(http.StatusOK, `{"task_id":"task_canvas_header"}`), nil
	})

	summary, err := CreateCanvasVideoGenerationTask(userId, modelID, "canvas prompt", "openai-video-generation", `{"duration":5}`)
	if err != nil {
		t.Fatalf("CreateCanvasVideoGenerationTask returned error: %v", err)
	}
	if !headerSeen {
		t.Fatal("expected canvas creation path to send internal scope header")
	}
	if summary == nil || summary.TaskID != "task_canvas_header" {
		t.Fatalf("unexpected canvas video summary: %#v", summary)
	}
	if summary.VideoURL != "/api/canvas/videos/task_canvas_header/content" {
		t.Fatalf("expected canvas video proxy URL, got %q", summary.VideoURL)
	}
	if summary.ResultURL != "/api/canvas/videos/task_canvas_header/content" {
		t.Fatalf("expected canvas result URL fallback, got %q", summary.ResultURL)
	}
}

func TestCreateVideoGenerationTaskDoesNotSetCanvasScopeHeader(t *testing.T) {
	db := setupImageGenerationServiceTestDB(t)
	userId := 62
	modelID := "video-normal-header"
	seedVideoGenerationCreatePath(t, db, userId, modelID)

	headerSeen := false
	useVideoRelayHTTPClient(t, func(req *http.Request) (*http.Response, error) {
		headerSeen = IsCanvasVideoTaskScopeHeaderValue(req.Header.Get(constant.HeaderCanvasVideoTaskScope))
		task := &model.Task{
			TaskID:     "task_normal_header",
			UserId:     userId,
			Action:     constant.TaskActionTextGenerate,
			Status:     model.TaskStatusQueued,
			Progress:   "0%",
			SubmitTime: common.GetTimestamp(),
			Properties: model.Properties{OriginModelName: modelID},
		}
		if err := task.Insert(); err != nil {
			return videoRelayTestResponse(http.StatusInternalServerError, err.Error()), nil
		}
		return videoRelayTestResponse(http.StatusOK, `{"task_id":"task_normal_header"}`), nil
	})

	summary, err := CreateVideoGenerationTask(userId, modelID, "normal prompt", "openai-video-generation", `{"duration":5}`)
	if err != nil {
		t.Fatalf("CreateVideoGenerationTask returned error: %v", err)
	}
	if headerSeen {
		t.Fatal("normal creation path should not send canvas video scope header")
	}
	if summary == nil || summary.TaskID != "task_normal_header" {
		t.Fatalf("unexpected normal video summary: %#v", summary)
	}
	if summary.VideoURL != "/v1/videos/task_normal_header/content" {
		t.Fatalf("expected normal video proxy URL, got %q", summary.VideoURL)
	}
}

func TestBuildVideoRelaySubmitBodyUsesMultipartForOpenAIVideoReferenceImage(t *testing.T) {
	body, contentType, err := buildVideoRelaySubmitBody(
		"sora-2",
		"cat video",
		"openai-video",
		VideoGenerationParams{
			Duration:   8,
			Resolution: "720x1280",
		},
		"data:image/png;base64,YWJj",
	)
	if err != nil {
		t.Fatalf("buildVideoRelaySubmitBody returned error: %v", err)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data;") {
		t.Fatalf("expected multipart content type, got %q", contentType)
	}

	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("failed to parse multipart content type: %v", err)
	}
	payload, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("failed to read multipart body: %v", err)
	}
	form, err := multipart.NewReader(bytes.NewReader(payload), params["boundary"]).ReadForm(1024 * 1024)
	if err != nil {
		t.Fatalf("failed to read multipart form: %v", err)
	}
	defer form.RemoveAll()

	if got := form.Value["model"]; len(got) != 1 || got[0] != "sora-2" {
		t.Fatalf("expected model field sora-2, got %#v", got)
	}
	if got := form.Value["prompt"]; len(got) != 1 || got[0] != "cat video" {
		t.Fatalf("expected prompt field cat video, got %#v", got)
	}
	if got := form.Value["seconds"]; len(got) != 1 || got[0] != "8" {
		t.Fatalf("expected seconds field 8, got %#v", got)
	}
	files := form.File["input_reference"]
	if len(files) != 1 || files[0].Filename != "reference.png" {
		t.Fatalf("expected one input_reference file, got %#v", files)
	}
}

func TestBuildVideoRelaySubmitBodyUsesMultipartForOpenAIVideoTextOnly(t *testing.T) {
	body, contentType, err := buildVideoRelaySubmitBody(
		"sora-2",
		"text only video",
		"openai-video",
		VideoGenerationParams{
			Duration:   4,
			Resolution: "1280x720",
		},
		"",
	)
	if err != nil {
		t.Fatalf("buildVideoRelaySubmitBody returned error: %v", err)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data;") {
		t.Fatalf("expected multipart content type, got %q", contentType)
	}

	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("failed to parse multipart content type: %v", err)
	}
	payload, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("failed to read multipart body: %v", err)
	}
	form, err := multipart.NewReader(bytes.NewReader(payload), params["boundary"]).ReadForm(1024 * 1024)
	if err != nil {
		t.Fatalf("failed to read multipart form: %v", err)
	}
	defer form.RemoveAll()

	if got := form.Value["model"]; len(got) != 1 || got[0] != "sora-2" {
		t.Fatalf("expected model field sora-2, got %#v", got)
	}
	if got := form.Value["prompt"]; len(got) != 1 || got[0] != "text only video" {
		t.Fatalf("expected prompt field text only video, got %#v", got)
	}
	if got := form.Value["seconds"]; len(got) != 1 || got[0] != "4" {
		t.Fatalf("expected seconds field 4, got %#v", got)
	}
	if got := form.Value["size"]; len(got) != 1 || got[0] != "1280x720" {
		t.Fatalf("expected size field 1280x720, got %#v", got)
	}
	if files := form.File["input_reference"]; len(files) != 0 {
		t.Fatalf("did not expect input_reference file for text-only submit, got %#v", files)
	}
}

func TestBuildVideoRelaySubmitBodyDownloadsRemoteReferenceForOpenAIVideo(t *testing.T) {
	previousDownloader := getVideoReferenceImageFromURL
	getVideoReferenceImageFromURL = func(url string) (string, string, error) {
		if url != "https://example.com/reference.png" {
			t.Fatalf("unexpected url %q", url)
		}
		return "image/png", "cG5n", nil
	}
	defer func() {
		getVideoReferenceImageFromURL = previousDownloader
	}()

	body, contentType, err := buildVideoRelaySubmitBody(
		"sora-2",
		"remote ref video",
		"openai-video",
		VideoGenerationParams{
			Duration:   8,
			Resolution: "720x1280",
		},
		"https://example.com/reference.png",
	)
	if err != nil {
		t.Fatalf("buildVideoRelaySubmitBody returned error: %v", err)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data;") {
		t.Fatalf("expected multipart content type, got %q", contentType)
	}

	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("failed to parse multipart content type: %v", err)
	}
	payload, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("failed to read multipart body: %v", err)
	}
	form, err := multipart.NewReader(bytes.NewReader(payload), params["boundary"]).ReadForm(1024 * 1024)
	if err != nil {
		t.Fatalf("failed to read multipart form: %v", err)
	}
	defer form.RemoveAll()

	files := form.File["input_reference"]
	if len(files) != 1 || files[0].Filename != "reference.png" {
		t.Fatalf("expected one downloaded input_reference file, got %#v", files)
	}
	file, err := files[0].Open()
	if err != nil {
		t.Fatalf("failed to open multipart file: %v", err)
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("failed to read multipart file: %v", err)
	}
	if string(content) != "png" {
		t.Fatalf("expected downloaded file content to be preserved, got %q", string(content))
	}
}

func TestListVideoGenerationTasksUsesSQLFilterAndBatchedMappings(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)

	for _, mapping := range []*model.ModelMapping{
		{
			RequestModel:    "video-alpha",
			ActualModel:     "video-alpha",
			DisplayName:     "Video Alpha",
			ModelType:       3,
			Status:          1,
			RequestEndpoint: "openai-video",
		},
		{
			RequestModel:    "video-beta",
			ActualModel:     "video-beta",
			DisplayName:     "Video Beta",
			ModelType:       3,
			Status:          1,
			RequestEndpoint: "openai-video-generation",
		},
	} {
		if err := db.Create(mapping).Error; err != nil {
			t.Fatalf("failed to create model mapping: %v", err)
		}
	}

	for index, modelID := range []string{"video-beta", "video-alpha", "video-beta", "video-alpha"} {
		task := &model.Task{
			UserId:     1,
			TaskID:     fmt.Sprintf("task_video_list_%d", index),
			Action:     constant.TaskActionTextGenerate,
			Status:     model.TaskStatusSuccess,
			Progress:   "100%",
			SubmitTime: int64(100 + index),
			Properties: model.Properties{
				Input:             fmt.Sprintf("video list prompt %d", index),
				OriginModelName:   modelID,
				UpstreamModelName: modelID,
			},
		}
		if err := db.Create(task).Error; err != nil {
			t.Fatalf("failed to create video task: %v", err)
		}
	}

	callbackName := "capture_video_generation_task_sql"
	var taskQueries []string
	modelMappingQueries := 0
	if err := db.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement == nil {
			return
		}
		switch tx.Statement.Table {
		case "tasks":
			taskQueries = append(taskQueries, tx.Dialector.Explain(tx.Statement.SQL.String(), tx.Statement.Vars...))
		case "model_mappings":
			modelMappingQueries++
		}
	}); err != nil {
		t.Fatalf("failed to register query callback: %v", err)
	}
	defer func() {
		_ = db.Callback().Query().Remove(callbackName)
	}()

	page, err := ListVideoGenerationTasks(1, 1, 1, "", "", "video-alpha", "", "", "", "", 0, 0)
	if err != nil {
		t.Fatalf("ListVideoGenerationTasks returned error: %v", err)
	}
	if !page.HasTotal || page.Total != 2 {
		t.Fatalf("expected filtered total 2 on first page, got %#v", page)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected one paged item, got %d", len(page.Items))
	}
	if page.Items[0].ModelID != "video-alpha" {
		t.Fatalf("expected SQL-filtered model video-alpha, got %#v", page.Items[0])
	}
	if page.Items[0].DisplayName != "Video Alpha" {
		t.Fatalf("expected batched model mapping display name, got %#v", page.Items[0])
	}
	if page.Items[0].RequestType != "text_to_video" {
		t.Fatalf("expected request_type text_to_video, got %#v", page.Items[0])
	}
	if modelMappingQueries != 1 {
		t.Fatalf("expected one batched model mapping query, got %d", modelMappingQueries)
	}
	if len(taskQueries) != 2 {
		t.Fatalf("expected count and paged task queries only, got %d queries: %#v", len(taskQueries), taskQueries)
	}

	hasModelFilter := false
	hasPagedLimit := false
	for _, query := range taskQueries {
		if strings.Contains(query, "origin_model_name = \"video-alpha\"") || strings.Contains(query, "upstream_model_name = \"video-alpha\"") {
			hasModelFilter = true
		}
		if strings.Contains(strings.ToUpper(query), "LIMIT 2") {
			hasPagedLimit = true
		}
	}
	if !hasModelFilter {
		t.Fatalf("expected SQL task query to include model filter, got %#v", taskQueries)
	}
	if !hasPagedLimit {
		t.Fatalf("expected paged SQL task query to include LIMIT 2, got %#v", taskQueries)
	}
}

func TestListVideoGenerationTasksCursorPaginationReturnsNextCursor(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)

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

	for index, submitTime := range []int64{300, 200, 100} {
		task := &model.Task{
			UserId:     1,
			TaskID:     fmt.Sprintf("task_video_cursor_%d", index),
			Action:     constant.TaskActionTextGenerate,
			Status:     model.TaskStatusSuccess,
			Progress:   "100%",
			SubmitTime: submitTime,
			Properties: model.Properties{
				Input:             fmt.Sprintf("video cursor prompt %d", index),
				OriginModelName:   "video-alpha",
				UpstreamModelName: "video-alpha",
			},
		}
		if err := db.Create(task).Error; err != nil {
			t.Fatalf("failed to create task: %v", err)
		}
	}

	firstPage, err := ListVideoGenerationTasks(1, 1, 2, "", "", "", "", "", "", "", 0, 0)
	if err != nil {
		t.Fatalf("failed to list first page: %v", err)
	}
	if !firstPage.HasTotal || firstPage.Total != 3 {
		t.Fatalf("expected first page total 3, got %#v", firstPage)
	}
	if len(firstPage.Items) != 2 {
		t.Fatalf("expected 2 items on first page, got %d", len(firstPage.Items))
	}
	if !firstPage.HasMore || strings.TrimSpace(firstPage.NextCursor) == "" {
		t.Fatalf("expected first page to expose next cursor, got %#v", firstPage)
	}
	if got := []int64{firstPage.Items[0].CreatedTime, firstPage.Items[1].CreatedTime}; fmt.Sprint(got) != fmt.Sprint([]int64{300, 200}) {
		t.Fatalf("unexpected first page ordering: %#v", got)
	}

	secondPage, err := ListVideoGenerationTasks(1, 2, 2, firstPage.NextCursor, "", "", "", "", "", "", 0, 0)
	if err != nil {
		t.Fatalf("failed to list second page: %v", err)
	}
	if secondPage.HasTotal {
		t.Fatalf("expected cursor page to skip total recount, got %#v", secondPage)
	}
	if secondPage.HasMore {
		t.Fatalf("expected second page to be terminal, got %#v", secondPage)
	}
	if len(secondPage.Items) != 1 || secondPage.Items[0].CreatedTime != 100 {
		t.Fatalf("unexpected second page items: %#v", secondPage.Items)
	}
}

func TestListVideoGenerationTasksSupportsKeywordSeriesAndSort(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)

	for _, mapping := range []*model.ModelMapping{
		{
			RequestModel:    "video-alpha",
			ActualModel:     "video-alpha",
			DisplayName:     "Video Alpha",
			ModelSeries:     "series-a",
			ModelType:       3,
			Status:          1,
			RequestEndpoint: "openai-video",
		},
		{
			RequestModel:    "video-beta",
			ActualModel:     "video-beta",
			DisplayName:     "Video Beta",
			ModelSeries:     "series-b",
			ModelType:       3,
			Status:          1,
			RequestEndpoint: "openai-video-generation",
		},
	} {
		if err := db.Create(mapping).Error; err != nil {
			t.Fatalf("failed to create model mapping: %v", err)
		}
	}

	tasks := []*model.Task{
		{
			UserId:     1,
			TaskID:     "task_video_keyword_1",
			Action:     constant.TaskActionTextGenerate,
			Status:     model.TaskStatusSuccess,
			Progress:   "100%",
			SubmitTime: 100,
			FinishTime: 200,
			Quota:      10,
			Properties: model.Properties{
				Input:             "ocean sunrise prompt",
				OriginModelName:   "video-alpha",
				UpstreamModelName: "video-alpha",
			},
		},
		{
			UserId:     1,
			TaskID:     "task_video_keyword_2",
			Action:     constant.TaskActionTextGenerate,
			Status:     model.TaskStatusSuccess,
			Progress:   "100%",
			SubmitTime: 110,
			FinishTime: 210,
			Quota:      90,
			Properties: model.Properties{
				Input:             "forest dusk prompt",
				OriginModelName:   "video-beta",
				UpstreamModelName: "video-beta",
			},
		},
	}
	for _, task := range tasks {
		if err := db.Create(task).Error; err != nil {
			t.Fatalf("failed to create video task: %v", err)
		}
	}

	page, err := ListVideoGenerationTasks(
		1,
		1,
		10,
		"",
		"completed",
		"",
		"series-b",
		"forest",
		"cost",
		"desc",
		0,
		0,
	)
	if err != nil {
		t.Fatalf("ListVideoGenerationTasks returned error: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected one filtered task, got %d", len(page.Items))
	}
	if page.Items[0].ModelID != "video-beta" {
		t.Fatalf("expected filtered series model video-beta, got %#v", page.Items[0])
	}
	if page.Items[0].Prompt != "forest dusk prompt" {
		t.Fatalf("expected keyword matched prompt, got %#v", page.Items[0])
	}
}

func TestListVideoGenerationTasksUpdatesUsesBatchedMappingsWithoutCounts(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)

	for _, mapping := range []*model.ModelMapping{
		{
			RequestModel:    "video-alpha",
			ActualModel:     "video-alpha",
			DisplayName:     "Video Alpha",
			ModelType:       3,
			Status:          1,
			RequestEndpoint: "openai-video",
		},
		{
			RequestModel:    "video-beta",
			ActualModel:     "video-beta",
			DisplayName:     "Video Beta",
			ModelType:       3,
			Status:          1,
			RequestEndpoint: "openai-video-generation",
		},
	} {
		if err := db.Create(mapping).Error; err != nil {
			t.Fatalf("failed to create model mapping: %v", err)
		}
	}

	for _, task := range []*model.Task{
		{
			UserId:     1,
			TaskID:     "task_video_updates_active",
			Action:     constant.TaskActionTextGenerate,
			Status:     model.TaskStatusQueued,
			Progress:   "10%",
			SubmitTime: 500,
			Properties: model.Properties{
				Input:             "active video",
				OriginModelName:   "video-alpha",
				UpstreamModelName: "video-alpha",
			},
		},
		{
			UserId:     1,
			TaskID:     "task_video_updates_recent",
			Action:     constant.TaskActionTextGenerate,
			Status:     model.TaskStatusSuccess,
			Progress:   "100%",
			SubmitTime: 400,
			FinishTime: 450,
			Properties: model.Properties{
				Input:             "recent completed video",
				OriginModelName:   "video-beta",
				UpstreamModelName: "video-beta",
			},
		},
		{
			UserId:     1,
			TaskID:     "task_video_updates_old",
			Action:     constant.TaskActionTextGenerate,
			Status:     model.TaskStatusSuccess,
			Progress:   "100%",
			SubmitTime: 200,
			FinishTime: 220,
			Properties: model.Properties{
				Input:             "old completed video",
				OriginModelName:   "video-beta",
				UpstreamModelName: "video-beta",
			},
		},
		{
			UserId:     2,
			TaskID:     "task_video_updates_other_user",
			Action:     constant.TaskActionTextGenerate,
			Status:     model.TaskStatusQueued,
			Progress:   "5%",
			SubmitTime: 600,
			Properties: model.Properties{
				Input:             "other user",
				OriginModelName:   "video-alpha",
				UpstreamModelName: "video-alpha",
			},
		},
	} {
		if err := db.Create(task).Error; err != nil {
			t.Fatalf("failed to create task: %v", err)
		}
	}

	callbackName := "capture_video_generation_update_sql"
	var taskQueries []string
	modelMappingQueries := 0
	if err := db.Callback().Query().After("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement == nil {
			return
		}
		switch tx.Statement.Table {
		case "tasks":
			taskQueries = append(taskQueries, tx.Dialector.Explain(tx.Statement.SQL.String(), tx.Statement.Vars...))
		case "model_mappings":
			modelMappingQueries++
		}
	}); err != nil {
		t.Fatalf("failed to register query callback: %v", err)
	}
	defer func() {
		_ = db.Callback().Query().Remove(callbackName)
	}()

	items, err := ListVideoGenerationTaskUpdates(1, 300, 10)
	if err != nil {
		t.Fatalf("ListVideoGenerationTaskUpdates returned error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 update items, got %d", len(items))
	}
	if items[0].Status != dto.VideoStatusQueued || items[0].DisplayName != "Video Alpha" {
		t.Fatalf("expected queued mapped active task first, got %#v", items[0])
	}
	if items[0].RequestType != "text_to_video" {
		t.Fatalf("expected active update request_type text_to_video, got %#v", items[0])
	}
	if items[1].Status != dto.VideoStatusCompleted || items[1].DisplayName != "Video Beta" {
		t.Fatalf("expected recent completed mapped task second, got %#v", items[1])
	}
	if items[1].RequestType != "text_to_video" {
		t.Fatalf("expected recent update request_type text_to_video, got %#v", items[1])
	}
	if modelMappingQueries != 1 {
		t.Fatalf("expected one batched model mapping query, got %d", modelMappingQueries)
	}
	if len(taskQueries) != 2 {
		t.Fatalf("expected exactly two task update queries, got %d: %#v", len(taskQueries), taskQueries)
	}
	for _, query := range taskQueries {
		if strings.Contains(strings.ToUpper(query), "COUNT(") {
			t.Fatalf("updates query should not execute COUNT(*), got %#v", taskQueries)
		}
	}
}
