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
