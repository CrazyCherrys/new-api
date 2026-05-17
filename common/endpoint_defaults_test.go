package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
)

func TestGetDefaultEndpointInfoIncludesVideoEndpoints(t *testing.T) {
	info, ok := GetDefaultEndpointInfo(constant.EndpointTypeOpenAIVideoGeneration)
	if !ok {
		t.Fatalf("expected openai video generation endpoint info to exist")
	}
	if info.Path != "/v1/video/generations" || info.Method != "POST" {
		t.Fatalf("unexpected openai video generation endpoint info: %+v", info)
	}

	info, ok = GetDefaultEndpointInfo(constant.EndpointTypeOpenAIVideo)
	if !ok {
		t.Fatalf("expected openai video endpoint info to exist")
	}
	if info.Path != "/v1/videos" || info.Method != "POST" {
		t.Fatalf("unexpected openai video endpoint info: %+v", info)
	}
}
