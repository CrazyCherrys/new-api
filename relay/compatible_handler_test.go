package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func TestShouldForceCanvasChatResponsesCompat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	service.SetCanvasChatResponsesCompatHeader(request.Header)
	context.Request = request

	if !shouldForceCanvasChatResponsesCompat(context) {
		t.Fatal("expected canvas loopback header to force responses compatibility")
	}
}

func TestShouldForceCanvasChatResponsesCompatRejectsNonLoopback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	request.Header.Set("X-NewAPI-Canvas-Chat-Responses-Compat", "1")
	context.Request = request

	if shouldForceCanvasChatResponsesCompat(context) {
		t.Fatal("expected invalid canvas responses compatibility header to be ignored")
	}
}
