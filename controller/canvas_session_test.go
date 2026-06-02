package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func TestCreateCanvasMessageStreamsChatRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("CANVAS_CHAT_SQL_DSN", "")
	t.Setenv("CANVAS_IMAGE_SQL_DSN", "")
	t.Setenv("CANVAS_VIDEO_SQL_DSN", "")
	model.InitCanvasDBs()

	previousGetSession := getCanvasSessionByIDForController
	previousCreateCanvasMessage := createCanvasMessageForController
	previousStreamCanvasChat := streamCanvasChatMessageForController
	t.Cleanup(func() {
		getCanvasSessionByIDForController = previousGetSession
		createCanvasMessageForController = previousCreateCanvasMessage
		streamCanvasChatMessageForController = previousStreamCanvasChat
	})

	getCanvasSessionByIDForController = func(userId int, sessionId int) (*model.CanvasSession, error) {
		return &model.CanvasSession{
			Id:     sessionId,
			UserId: userId,
			Mode:   model.CanvasModeChat,
		}, nil
	}

	streamCalled := false
	createCalled := false
	streamCanvasChatMessageForController = func(c *gin.Context, userId int, sessionId int, input service.CreateCanvasMessageInput) error {
		streamCalled = true
		if input.Stream == nil || !*input.Stream {
			t.Fatalf("expected stream flag to be forwarded, got %#v", input.Stream)
		}
		if input.Prompt != "hello stream" || input.ModelId != "gpt-chat-test" {
			t.Fatalf("unexpected stream input: %#v", input)
		}
		if len(input.Attachments) != 1 || input.Attachments[0].Kind != "image" {
			t.Fatalf("expected chat attachments to be forwarded, got %#v", input.Attachments)
		}
		c.Status(http.StatusOK)
		return nil
	}
	createCanvasMessageForController = func(ctx context.Context, userId int, sessionId int, input service.CreateCanvasMessageInput) ([]*service.CanvasMessageWithTask, error) {
		createCalled = true
		return nil, nil
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("id", 1)
	c.Params = gin.Params{{Key: "id", Value: "7"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/canvas/sessions/7/messages", strings.NewReader(`{
			"prompt":"hello stream",
			"model_id":"gpt-chat-test",
			"attachments":[{"kind":"image","name":"ref.png","mime_type":"image/png","data":"data:image/png;base64,Zm9v"}],
			"stream":true
		}`))

	CreateCanvasMessage(c)

	if !streamCalled {
		t.Fatal("expected stream canvas chat handler to be called")
	}
	if createCalled {
		t.Fatal("did not expect blocking create handler to be called for stream chat request")
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", recorder.Code)
	}
}

func TestGetCanvasChatModelsReturnsCatalogCapabilities(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("CANVAS_CHAT_SQL_DSN", "")
	t.Setenv("CANVAS_IMAGE_SQL_DSN", "")
	t.Setenv("CANVAS_VIDEO_SQL_DSN", "")
	model.InitCanvasDBs()

	previousGetCanvasChatModels := getCanvasChatModelsForController
	getCanvasChatModelsForController = func(userId int) ([]*dto.CanvasChatModelCatalogItem, error) {
		return []*dto.CanvasChatModelCatalogItem{
			{
				RequestModel:     "gpt-4.1",
				DisplayName:      "GPT 4.1",
				ModelSeries:      "openai",
				RequestEndpoint:  "openai",
				ChatCapabilities: []string{"image_upload", "file_upload"},
			},
		}, nil
	}
	t.Cleanup(func() {
		getCanvasChatModelsForController = previousGetCanvasChatModels
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("id", 9)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/canvas/chat-models", nil)

	GetCanvasChatModels(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", recorder.Code)
	}

	var response struct {
		Success bool                             `json:"success"`
		Message string                           `json:"message"`
		Data    []dto.CanvasChatModelCatalogItem `json:"data"`
	}
	if err := common.DecodeJson(recorder.Body, &response); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if !response.Success || len(response.Data) != 1 {
		t.Fatalf("unexpected response payload: %#v", response)
	}
	if len(response.Data[0].ChatCapabilities) != 2 {
		t.Fatalf("expected chat capabilities to be returned, got %#v", response.Data[0])
	}
}

func TestListCanvasSessionsSupportsPaginationResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("CANVAS_CHAT_SQL_DSN", "")
	t.Setenv("CANVAS_IMAGE_SQL_DSN", "")
	t.Setenv("CANVAS_VIDEO_SQL_DSN", "")
	model.InitCanvasDBs()

	previousListCanvasSessions := listCanvasSessionsForController
	t.Cleanup(func() {
		listCanvasSessionsForController = previousListCanvasSessions
	})

	var capturedUserID int
	var capturedMode string
	var capturedLimit int
	var capturedOffset int
	listCanvasSessionsForController = func(userId int, mode string, limit int, offset int) (*service.CanvasSessionListPage, error) {
		capturedUserID = userId
		capturedMode = mode
		capturedLimit = limit
		capturedOffset = offset
		return &service.CanvasSessionListPage{
			Items: []*model.CanvasSession{
				{Id: 11, Mode: model.CanvasModeChat, Title: "chat recent"},
				{Id: 12, Mode: model.CanvasModeImage, Title: "image recent"},
			},
			HasMore: true,
		}, nil
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("id", 7)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/canvas/sessions?limit=20&offset=40", nil)

	ListCanvasSessions(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", recorder.Code)
	}
	if capturedUserID != 7 || capturedMode != "" || capturedLimit != 20 || capturedOffset != 40 {
		t.Fatalf("unexpected forwarded list params: user=%d mode=%q limit=%d offset=%d", capturedUserID, capturedMode, capturedLimit, capturedOffset)
	}

	var response struct {
		Success bool                          `json:"success"`
		Message string                        `json:"message"`
		Data    service.CanvasSessionListPage `json:"data"`
	}
	if err := common.DecodeJson(recorder.Body, &response); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if !response.Success {
		t.Fatalf("expected success response, got message %q", response.Message)
	}
	if len(response.Data.Items) != 2 || !response.Data.HasMore {
		t.Fatalf("unexpected response payload: %#v", response.Data)
	}
	if response.Data.Items[0].Id != 11 || response.Data.Items[1].Id != 12 {
		t.Fatalf("unexpected response items: %#v", response.Data.Items)
	}
}

func TestListCanvasChatModelsReturnsStructuredOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("CANVAS_CHAT_SQL_DSN", "")
	t.Setenv("CANVAS_IMAGE_SQL_DSN", "")
	t.Setenv("CANVAS_VIDEO_SQL_DSN", "")
	model.InitCanvasDBs()

	previousListCanvasChatModels := listCanvasChatModelsForController
	t.Cleanup(func() {
		listCanvasChatModelsForController = previousListCanvasChatModels
	})

	var capturedUserID int
	listCanvasChatModelsForController = func(userId int) ([]service.CanvasChatModelOption, error) {
		capturedUserID = userId
		return []service.CanvasChatModelOption{
			{
				RequestModel:    "gpt-4.1",
				DisplayName:     "GPT 4.1",
				ModelSeries:     "openai",
				RequestEndpoint: "openai",
				Usable:          true,
				AvailableGroups: []string{"default"},
			},
		}, nil
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("id", 9)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/canvas/chat/models", nil)

	ListCanvasChatModels(c)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", recorder.Code)
	}
	if capturedUserID != 9 {
		t.Fatalf("expected user id 9, got %d", capturedUserID)
	}

	var response struct {
		Success bool                            `json:"success"`
		Message string                          `json:"message"`
		Data    []service.CanvasChatModelOption `json:"data"`
	}
	if err := common.DecodeJson(recorder.Body, &response); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if !response.Success || len(response.Data) != 1 {
		t.Fatalf("unexpected response payload: %#v", response)
	}
	if response.Data[0].RequestModel != "gpt-4.1" || response.Data[0].DisplayName != "GPT 4.1" {
		t.Fatalf("unexpected model option: %#v", response.Data[0])
	}
}

func TestCanvasControllersReturn503WhenCanvasDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("CANVAS_CHAT_SQL_DSN", "postgres://chat")
	t.Setenv("CANVAS_IMAGE_SQL_DSN", "")
	t.Setenv("CANVAS_VIDEO_SQL_DSN", "postgres://video")
	model.InitCanvasDBs()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("id", 7)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/canvas/sessions", nil)

	ListCanvasSessions(c)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected HTTP 503, got %d", recorder.Code)
	}

	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := common.DecodeJson(recorder.Body, &response); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if response.Success {
		t.Fatalf("expected failure response, got success payload %#v", response)
	}
	if !strings.Contains(response.Message, "must be configured together") {
		t.Fatalf("unexpected 503 message: %q", response.Message)
	}
}
