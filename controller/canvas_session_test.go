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

	previousGetSession := getCanvasSessionByIdentifierForController
	previousCreateCanvasMessage := createCanvasMessageForController
	previousStreamCanvasChat := streamCanvasChatMessageForController
	t.Cleanup(func() {
		getCanvasSessionByIdentifierForController = previousGetSession
		createCanvasMessageForController = previousCreateCanvasMessage
		streamCanvasChatMessageForController = previousStreamCanvasChat
	})

	getCanvasSessionByIdentifierForController = func(userId int, identifier string) (*model.CanvasSession, error) {
		return &model.CanvasSession{
			Id:     7,
			UserId: userId,
			Mode:   model.CanvasModeChat,
		}, nil
	}

	streamCalled := false
	createCalled := false
	streamCanvasChatMessageForController = func(c *gin.Context, userId int, session *model.CanvasSession, input service.CreateCanvasMessageInput) error {
		streamCalled = true
		if session == nil || session.Id != 7 {
			t.Fatalf("unexpected resolved session: %#v", session)
		}
		if input.Stream == nil || !*input.Stream {
			t.Fatalf("expected stream flag to be forwarded, got %#v", input.Stream)
		}
		if input.Prompt != "hello stream" || input.ModelId != "gpt-chat-test" {
			t.Fatalf("unexpected stream input: %#v", input)
		}
		if input.WebSearchEnabled == nil || !*input.WebSearchEnabled {
			t.Fatalf("expected web search flag to be forwarded, got %#v", input.WebSearchEnabled)
		}
		if len(input.Attachments) != 1 || input.Attachments[0].Kind != "image" {
			t.Fatalf("expected chat attachments to be forwarded, got %#v", input.Attachments)
		}
		c.Status(http.StatusOK)
		return nil
	}
	createCanvasMessageForController = func(ctx context.Context, userId int, session *model.CanvasSession, input service.CreateCanvasMessageInput) ([]*service.CanvasMessageWithTask, error) {
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
			"web_search_enabled":true,
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

func TestCanvasControllerResolvesSessionByPublicID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousGetSession := getCanvasSessionByIdentifierForController
	previousListMessages := listCanvasMessageTimelineForSessionForController
	t.Cleanup(func() {
		getCanvasSessionByIdentifierForController = previousGetSession
		listCanvasMessageTimelineForSessionForController = previousListMessages
	})

	resolvedIdentifier := ""
	getCanvasSessionByIdentifierForController = func(userId int, identifier string) (*model.CanvasSession, error) {
		resolvedIdentifier = identifier
		return &model.CanvasSession{
			Id:       42,
			PublicId: "cs_public_demo",
			UserId:   userId,
			Mode:     model.CanvasModeChat,
		}, nil
	}

	listCanvasMessageTimelineForSessionForController = func(userId int, session *model.CanvasSession, limit int, cursor string) (*service.CanvasMessageTimelinePage, error) {
		if userId != 7 || session == nil || session.Id != 42 || limit != 20 {
			t.Fatalf("unexpected list call: user=%d session=%#v limit=%d cursor=%q", userId, session, limit, cursor)
		}
		return &service.CanvasMessageTimelinePage{
			Items: []*service.CanvasMessageWithTask{
				{
					CanvasMessage: &model.CanvasMessage{
						Id:        1,
						SessionId: session.Id,
						UserId:    userId,
						Mode:      model.CanvasModeChat,
						Role:      model.CanvasMessageRoleUser,
						Prompt:    "hello",
					},
				},
			},
		}, nil
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("id", 7)
	c.Params = gin.Params{{Key: "id", Value: "cs_public_demo"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/canvas/sessions/cs_public_demo/messages?limit=20", nil)

	ListCanvasMessages(c)

	if resolvedIdentifier != "cs_public_demo" {
		t.Fatalf("expected public_id resolver to receive cs_public_demo, got %q", resolvedIdentifier)
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
				Description:      "Canvas chat description",
				VendorIcon:       "OpenAI",
				ChatCapabilities: []string{"image_upload", "file_upload", "web_search"},
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
	if len(response.Data[0].ChatCapabilities) != 3 {
		t.Fatalf("expected chat capabilities to be returned, got %#v", response.Data[0])
	}
	if response.Data[0].ChatCapabilities[2] != "web_search" {
		t.Fatalf("expected web_search capability to be returned, got %#v", response.Data[0].ChatCapabilities)
	}
	if response.Data[0].Description != "Canvas chat description" || response.Data[0].VendorIcon != "OpenAI" {
		t.Fatalf("expected catalog display metadata to be returned, got %#v", response.Data[0])
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
				Description:     "Canvas option description",
				VendorIcon:      "OpenAI",
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
	if response.Data[0].Description != "Canvas option description" || response.Data[0].VendorIcon != "OpenAI" {
		t.Fatalf("expected option display metadata, got %#v", response.Data[0])
	}
}

func TestCanvasControllersKeepSessionListAvailableWhileModeSpecificErrorsReturn503(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousListCanvasSessions := listCanvasSessionsForController
	previousGetCanvasChatModels := getCanvasChatModelsForController
	t.Cleanup(func() {
		listCanvasSessionsForController = previousListCanvasSessions
		getCanvasChatModelsForController = previousGetCanvasChatModels
	})

	listCanvasSessionsForController = func(userId int, mode string, limit int, offset int) (*service.CanvasSessionListPage, error) {
		return &service.CanvasSessionListPage{
			Items: []*model.CanvasSession{
				{Id: 1, PublicId: "cs_demo", Mode: model.CanvasModeImage, Title: "image"},
			},
		}, nil
	}
	getCanvasChatModelsForController = func(userId int) ([]*dto.CanvasChatModelCatalogItem, error) {
		return nil, &model.CanvasModeUnavailableError{
			Mode:   model.CanvasModeChat,
			Reason: "boom",
		}
	}

	sessionRecorder := httptest.NewRecorder()
	sessionCtx, _ := gin.CreateTestContext(sessionRecorder)
	sessionCtx.Set("id", 7)
	sessionCtx.Request = httptest.NewRequest(http.MethodGet, "/api/canvas/sessions", nil)

	ListCanvasSessions(sessionCtx)

	if sessionRecorder.Code != http.StatusOK {
		t.Fatalf("expected session list to stay available, got %d", sessionRecorder.Code)
	}

	modelRecorder := httptest.NewRecorder()
	modelCtx, _ := gin.CreateTestContext(modelRecorder)
	modelCtx.Set("id", 7)
	modelCtx.Request = httptest.NewRequest(http.MethodGet, "/api/canvas/chat-models", nil)

	GetCanvasChatModels(modelCtx)

	if modelRecorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected mode-specific outage to return HTTP 503, got %d", modelRecorder.Code)
	}

	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := common.DecodeJson(modelRecorder.Body, &response); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if response.Success {
		t.Fatalf("expected failure response, got success payload %#v", response)
	}
	if !strings.Contains(response.Message, "boom") {
		t.Fatalf("unexpected 503 message: %q", response.Message)
	}
}
