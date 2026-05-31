package common

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

type multipartTaskProbe struct {
	Prompt         string `json:"prompt"`
	Model          string `json:"model"`
	InputReference string `json:"input_reference"`
}

func TestUnmarshalBodyReusableMultipartIncludesFileFieldPlaceholder(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("prompt", "video prompt"); err != nil {
		t.Fatalf("failed to write prompt field: %v", err)
	}
	if err := writer.WriteField("model", "sora-2"); err != nil {
		t.Fatalf("failed to write model field: %v", err)
	}
	part, err := writer.CreateFormFile("input_reference", "reference.png")
	if err != nil {
		t.Fatalf("failed to create file part: %v", err)
	}
	if _, err := part.Write([]byte("png")); err != nil {
		t.Fatalf("failed to write file part: %v", err)
	}
	contentType := writer.FormDataContentType()
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(body.Bytes()))
	c.Request.Header.Set("Content-Type", contentType)

	var parsed multipartTaskProbe
	if err := UnmarshalBodyReusable(c, &parsed); err != nil {
		t.Fatalf("UnmarshalBodyReusable returned error: %v", err)
	}
	if parsed.Prompt != "video prompt" {
		t.Fatalf("expected prompt to be parsed, got %q", parsed.Prompt)
	}
	if parsed.InputReference != "reference.png" {
		t.Fatalf("expected multipart file placeholder to preserve filename, got %q", parsed.InputReference)
	}
}
