package gemini

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

func TestConvertImageRequestMapsResolutionAndAspectRatioToImageConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-2.5-flash-image",
		},
	}
	request := dto.ImageRequest{
		Model:       "gemini-image",
		Prompt:      "draw prompt",
		Resolution:  "2K",
		AspectRatio: "16:9",
	}

	converted, err := adaptor.ConvertImageRequest(nil, info, request)
	if err != nil {
		t.Fatalf("ConvertImageRequest returned error: %v", err)
	}

	out, ok := converted.(dto.GeminiChatRequest)
	if !ok {
		t.Fatalf("expected GeminiChatRequest, got %T", converted)
	}
	if len(out.GenerationConfig.ImageConfig) == 0 {
		t.Fatal("expected imageConfig")
	}

	var imageConfig map[string]string
	if err := common.Unmarshal(out.GenerationConfig.ImageConfig, &imageConfig); err != nil {
		t.Fatalf("failed to decode imageConfig: %v", err)
	}
	if imageConfig["aspectRatio"] != "16:9" {
		t.Fatalf("unexpected aspectRatio: %#v", imageConfig["aspectRatio"])
	}
	if imageConfig["imageSize"] != "2K" {
		t.Fatalf("unexpected imageSize: %#v", imageConfig["imageSize"])
	}
}
