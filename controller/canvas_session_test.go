package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func TestCreateCanvasMessageStreamsChatRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

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

func TestListCanvasSessionsSupportsPaginationResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

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
