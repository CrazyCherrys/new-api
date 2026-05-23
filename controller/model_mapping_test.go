package controller

import "testing"

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
