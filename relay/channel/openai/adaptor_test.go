package openai

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
)

func TestConvertImageRequestOpenAIEditUsesMultipartForReferenceImagesAndMask(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/v1/images/edits", nil)

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesEdits,
	}
	request := dto.ImageRequest{
		RequestEndpoint: "openai",
		Model:           "gpt-image-1",
		Prompt:          "edit prompt",
		Size:            "1024x1024",
		ReferenceImages: []string{"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGP4z8DwHwAFAAH/e+m+7wAAAABJRU5ErkJggg=="},
		Mask:            "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGP4z8DwHwAFAAH/e+m+7wAAAABJRU5ErkJggg==",
	}

	converted, err := adaptor.ConvertImageRequest(ctx, info, request)
	if err != nil {
		t.Fatalf("ConvertImageRequest returned error: %v", err)
	}

	bodyBuffer, ok := converted.(*bytes.Buffer)
	if !ok {
		t.Fatalf("expected multipart buffer, got %T", converted)
	}

	contentType := ctx.Request.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "multipart/form-data;") {
		t.Fatalf("expected multipart content type, got %q", contentType)
	}
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("failed to parse content type: %v", err)
	}
	boundary := params["boundary"]
	if boundary == "" {
		t.Fatal("expected multipart boundary")
	}

	reader := multipart.NewReader(bytes.NewReader(bodyBuffer.Bytes()), boundary)
	parts := map[string][]string{}
	fileCounts := map[string]int{}

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read multipart body: %v", err)
		}
		data, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("failed to read part %q: %v", part.FormName(), err)
		}
		if part.FileName() != "" {
			fileCounts[part.FormName()]++
		}
		parts[part.FormName()] = append(parts[part.FormName()], string(data))
	}

	if got := parts["model"]; len(got) != 1 || got[0] != "gpt-image-1" {
		t.Fatalf("unexpected model fields: %#v", got)
	}
	if got := parts["prompt"]; len(got) != 1 || got[0] != "edit prompt" {
		t.Fatalf("unexpected prompt fields: %#v", got)
	}
	if got := parts["size"]; len(got) != 1 || got[0] != "1024x1024" {
		t.Fatalf("unexpected size fields: %#v", got)
	}
	if fileCounts["image"] != 1 {
		t.Fatalf("expected 1 image file part, got %d", fileCounts["image"])
	}
	if fileCounts["mask"] != 1 {
		t.Fatalf("expected 1 mask file part, got %d", fileCounts["mask"])
	}
}
