package controller

import (
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

func TestGetVideoGenerationModelsIncludesCatalogMetadata(t *testing.T) {
	db := setupImageGenerationControllerTestDB(t)

	if err := db.Create(&model.ModelMapping{
		RequestModel:      "video-alpha",
		ActualModel:       "video-alpha",
		DisplayName:       "Video Alpha",
		ModelSeries:       "openai",
		ModelType:         3,
		Status:            1,
		RequestEndpoint:   "openai-video",
		VideoCapabilities: `["text_to_video"]`,
		DurationOptions:   `[5]`,
	}).Error; err != nil {
		t.Fatalf("failed to create video mapping: %v", err)
	}
	vendor := &model.Vendor{
		Name:   "OpenAI",
		Icon:   "OpenAI",
		Status: 1,
	}
	if err := db.Create(vendor).Error; err != nil {
		t.Fatalf("failed to create vendor: %v", err)
	}
	if err := db.Create(&model.Model{
		ModelName:    "alpha",
		Description:  "Video catalog description",
		VendorID:     vendor.Id,
		NameRule:     model.NameRuleSuffix,
		Status:       1,
		SyncOfficial: 1,
	}).Error; err != nil {
		t.Fatalf("failed to create video model metadata: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/video-generation/models", nil, 1)
	GetVideoGenerationModels(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var items []dto.VideoGenerationModel
	if err := common.Unmarshal(response.Data, &items); err != nil {
		t.Fatalf("failed to decode video model response: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one video model, got %#v", items)
	}
	if items[0].Description != "Video catalog description" || items[0].VendorIcon != "OpenAI" {
		t.Fatalf("expected catalog metadata in video model response, got %#v", items[0])
	}
}

func TestVideoGenerationTaskDetailControllersUseScopedHandlers(t *testing.T) {
	previousOrdinaryDetail := getVideoGenerationTaskDetailForController
	previousCanvasDetail := getCanvasVideoGenerationTaskDetailForController
	t.Cleanup(func() {
		getVideoGenerationTaskDetailForController = previousOrdinaryDetail
		getCanvasVideoGenerationTaskDetailForController = previousCanvasDetail
	})

	ordinaryCalls := 0
	canvasCalls := 0
	getVideoGenerationTaskDetailForController = func(userId int, identifier string) (*dto.VideoGenerationTaskDetail, error) {
		ordinaryCalls++
		if userId != 7 || identifier != "1" {
			t.Fatalf("unexpected ordinary detail request user=%d identifier=%q", userId, identifier)
		}
		return &dto.VideoGenerationTaskDetail{
			VideoGenerationTaskSummary: dto.VideoGenerationTaskSummary{
				ID:     1,
				TaskID: "task_main_collision",
			},
		}, nil
	}
	getCanvasVideoGenerationTaskDetailForController = func(userId int, identifier string) (*dto.VideoGenerationTaskDetail, error) {
		canvasCalls++
		if userId != 7 || identifier != "1" {
			t.Fatalf("unexpected canvas detail request user=%d identifier=%q", userId, identifier)
		}
		return &dto.VideoGenerationTaskDetail{
			VideoGenerationTaskSummary: dto.VideoGenerationTaskSummary{
				ID:     1,
				TaskID: "task_canvas_collision",
			},
		}, nil
	}

	ordinaryCtx, ordinaryRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/video-generation/tasks/1", nil, 7)
	ordinaryCtx.AddParam("id", "1")
	GetVideoGenerationTaskDetail(ordinaryCtx)

	if ordinaryCalls != 1 || canvasCalls != 0 {
		t.Fatalf("ordinary detail handler used wrong service hooks, ordinary=%d canvas=%d", ordinaryCalls, canvasCalls)
	}
	ordinaryResponse := decodeAPIResponse(t, ordinaryRecorder)
	if !ordinaryResponse.Success {
		t.Fatalf("expected ordinary detail success, got %s", ordinaryResponse.Message)
	}
	var ordinaryDetail dto.VideoGenerationTaskDetail
	if err := common.Unmarshal(ordinaryResponse.Data, &ordinaryDetail); err != nil {
		t.Fatalf("failed to decode ordinary detail: %v", err)
	}
	if ordinaryDetail.TaskID != "task_main_collision" {
		t.Fatalf("ordinary detail returned wrong task: %#v", ordinaryDetail)
	}

	canvasCtx, canvasRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/canvas/video-generation/tasks/1", nil, 7)
	canvasCtx.AddParam("id", "1")
	GetCanvasVideoGenerationTaskDetail(canvasCtx)

	if ordinaryCalls != 1 || canvasCalls != 1 {
		t.Fatalf("canvas detail handler used wrong service hooks, ordinary=%d canvas=%d", ordinaryCalls, canvasCalls)
	}
	canvasResponse := decodeAPIResponse(t, canvasRecorder)
	if !canvasResponse.Success {
		t.Fatalf("expected canvas detail success, got %s", canvasResponse.Message)
	}
	var canvasDetail dto.VideoGenerationTaskDetail
	if err := common.Unmarshal(canvasResponse.Data, &canvasDetail); err != nil {
		t.Fatalf("failed to decode canvas detail: %v", err)
	}
	if canvasDetail.TaskID != "task_canvas_collision" {
		t.Fatalf("canvas detail returned wrong task: %#v", canvasDetail)
	}
}

func TestGetCanvasVideoGenerationTaskDetailReturns503WhenCanvasVideoUnavailable(t *testing.T) {
	previousCanvasDetail := getCanvasVideoGenerationTaskDetailForController
	t.Cleanup(func() {
		getCanvasVideoGenerationTaskDetailForController = previousCanvasDetail
	})

	getCanvasVideoGenerationTaskDetailForController = func(userId int, identifier string) (*dto.VideoGenerationTaskDetail, error) {
		return nil, &model.CanvasModeUnavailableError{
			Mode:   model.CanvasModeVideo,
			Reason: "child database unavailable",
		}
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/canvas/video-generation/tasks/1", nil, 7)
	ctx.AddParam("id", "1")
	GetCanvasVideoGenerationTaskDetail(ctx)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected HTTP 503 for unavailable canvas video detail, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	response := decodeAPIResponse(t, recorder)
	if response.Success || !strings.Contains(response.Message, "child database unavailable") {
		t.Fatalf("unexpected unavailable response: %#v", response)
	}
}
