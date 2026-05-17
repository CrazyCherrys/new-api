package controller

import "testing"

func TestNormalizeModelMappingEndpoint(t *testing.T) {
	if got := normalizeModelMappingEndpoint("  Dalle  "); got != "openai" {
		t.Fatalf("expected dalle to normalize to openai, got %q", got)
	}
	if got := normalizeModelMappingEndpoint("OPENAI-VIDEO-GENERATIONS"); got != "openai-video-generation" {
		t.Fatalf("expected openai video generations to normalize, got %q", got)
	}
	if got := normalizeModelMappingEndpoint("Sora"); got != "openai-video" {
		t.Fatalf("expected sora to normalize to openai-video, got %q", got)
	}
}

func TestValidateModelMappingEndpoint(t *testing.T) {
	tests := []struct {
		name      string
		modelType int
		endpoint  string
		wantErr   bool
	}{
		{name: "image ok", modelType: 2, endpoint: "openai", wantErr: false},
		{name: "image invalid video", modelType: 2, endpoint: "openai-video", wantErr: true},
		{name: "video generation ok", modelType: 3, endpoint: "openai-video-generation", wantErr: false},
		{name: "video videos ok", modelType: 3, endpoint: "openai-video", wantErr: false},
		{name: "video image invalid", modelType: 3, endpoint: "openai", wantErr: true},
		{name: "other type ok", modelType: 1, endpoint: "openai", wantErr: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateModelMappingEndpoint(tc.modelType, tc.endpoint)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %s: %v", tc.name, err)
			}
		})
	}
}
