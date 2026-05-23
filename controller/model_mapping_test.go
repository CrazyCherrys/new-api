package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
)

func TestValidateImageModelEndpointRejectsDeprecatedOpenAIMod(t *testing.T) {
	validEndpoints := []string{"openai", "openai-response", "gemini"}
	for _, endpoint := range validEndpoints {
		if err := validateModelMappingEndpoint(2, endpoint); err != nil {
			t.Fatalf("expected endpoint %q to remain valid, got %v", endpoint, err)
		}
	}

	if err := validateModelMappingEndpoint(2, "openai_mod"); err == nil {
		t.Fatal("expected deprecated openai_mod endpoint to be rejected")
	}
}

func TestNormalizeVideoCapabilitiesIncludesTextToVideo(t *testing.T) {
	normalized, err := model.NormalizeVideoCapabilities(`["image_to_video","text_to_video"]`)
	if err != nil {
		t.Fatalf("expected video capabilities to normalize, got %v", err)
	}

	effective, err := model.EffectiveVideoCapabilities(normalized)
	if err != nil {
		t.Fatalf("expected effective video capabilities, got %v", err)
	}
	expected := []string{"image_to_video", "text_to_video"}
	if len(effective) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, effective)
	}
	for i := range expected {
		if effective[i] != expected[i] {
			t.Fatalf("expected %v, got %v", expected, effective)
		}
	}
}

func TestEffectiveVideoCapabilitiesLegacyDefaultStaysImageToVideo(t *testing.T) {
	effective, err := model.EffectiveVideoCapabilities("")
	if err != nil {
		t.Fatalf("expected effective video capabilities, got %v", err)
	}
	expected := []string{"image_to_video"}
	if len(effective) != len(expected) || effective[0] != expected[0] {
		t.Fatalf("expected %v, got %v", expected, effective)
	}
}

func TestNormalizeDurationOptionsAcceptsSecondLabels(t *testing.T) {
	normalized, err := model.NormalizeDurationOptions(`["5s","10秒","15"]`)
	if err != nil {
		t.Fatalf("expected duration options to normalize, got %v", err)
	}
	if normalized != `[5,10,15]` {
		t.Fatalf("expected [5,10,15], got %s", normalized)
	}

	if _, err := model.NormalizeDurationOptions(`["5ms"]`); err == nil {
		t.Fatal("expected invalid duration label to be rejected")
	}
}
