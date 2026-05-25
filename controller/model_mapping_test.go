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

func TestSanitizeModelMappingSettingsKeepsOpenAIImageResolutions(t *testing.T) {
	for _, endpoint := range []string{"openai", "openai-response"} {
		t.Run(endpoint, func(t *testing.T) {
			mapping := &model.ModelMapping{
				ModelType:       2,
				RequestEndpoint: endpoint,
				Resolutions:     `["1K","2K"]`,
				AspectRatios:    `["1:1","16:9"]`,
			}

			sanitizeModelMappingSettings(mapping)

			if mapping.Resolutions != `["1K","2K"]` {
				t.Fatalf("expected resolutions to be preserved, got %q", mapping.Resolutions)
			}
			if mapping.AspectRatios != `["1:1","16:9"]` {
				t.Fatalf("expected aspect ratios to be preserved, got %q", mapping.AspectRatios)
			}
		})
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

func TestValidateReferenceImageLimit(t *testing.T) {
	cases := []struct {
		name      string
		mapping   model.ModelMapping
		wantLimit int
		wantErr   bool
	}{
		{
			name: "unlimited editing model remains zero",
			mapping: model.ModelMapping{
				ModelType:           2,
				ImageCapabilities:   `["image_generation","image_editing"]`,
				ReferenceImageLimit: 0,
			},
			wantLimit: 0,
		},
		{
			name: "limited editing model keeps positive limit",
			mapping: model.ModelMapping{
				ModelType:           2,
				ImageCapabilities:   `["image_generation","image_editing"]`,
				ReferenceImageLimit: 4,
			},
			wantLimit: 4,
		},
		{
			name: "image model without editing clears limit",
			mapping: model.ModelMapping{
				ModelType:           2,
				ImageCapabilities:   `["image_generation"]`,
				ReferenceImageLimit: 4,
			},
			wantLimit: 0,
		},
		{
			name: "non image model clears limit",
			mapping: model.ModelMapping{
				ModelType:           1,
				ReferenceImageLimit: 4,
			},
			wantLimit: 0,
		},
		{
			name: "negative limit is rejected",
			mapping: model.ModelMapping{
				ModelType:           2,
				ImageCapabilities:   `["image_editing"]`,
				ReferenceImageLimit: -1,
			},
			wantErr: true,
		},
		{
			name: "over max limit is rejected",
			mapping: model.ModelMapping{
				ModelType:           2,
				ImageCapabilities:   `["image_editing"]`,
				ReferenceImageLimit: maxReferenceImageLimit + 1,
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mapping := tc.mapping
			err := validateReferenceImageLimit(&mapping)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
			if mapping.ReferenceImageLimit != tc.wantLimit {
				t.Fatalf("expected limit %d, got %d", tc.wantLimit, mapping.ReferenceImageLimit)
			}
		})
	}
}
