package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
