package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
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

func TestValidateChatModelEndpointAllowsOpenAIResponse(t *testing.T) {
	validEndpoints := []string{"openai", "openai-response", "anthropic", "gemini"}
	for _, endpoint := range validEndpoints {
		if err := validateModelMappingEndpoint(1, endpoint); err != nil {
			t.Fatalf("expected chat endpoint %q to remain valid, got %v", endpoint, err)
		}
	}
}

func TestValidateChatModelEndpointRejectsUnknownEndpoint(t *testing.T) {
	if err := validateModelMappingEndpoint(1, "responses"); err == nil {
		t.Fatal("expected unknown chat endpoint to be rejected")
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

func TestNormalizeChatCapabilitiesIncludesImageAndFileUpload(t *testing.T) {
	normalized, err := model.NormalizeChatCapabilities(`[" image_upload ","file_upload","web_search","image_upload"]`)
	if err != nil {
		t.Fatalf("expected chat capabilities to normalize, got %v", err)
	}
	if normalized != `["image_upload","file_upload","web_search"]` {
		t.Fatalf("expected normalized chat capabilities, got %s", normalized)
	}

	effective, err := model.EffectiveChatCapabilities(normalized)
	if err != nil {
		t.Fatalf("expected effective chat capabilities, got %v", err)
	}
	expected := []string{"image_upload", "file_upload", "web_search"}
	if len(effective) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, effective)
	}
	for i := range expected {
		if effective[i] != expected[i] {
			t.Fatalf("expected %v, got %v", expected, effective)
		}
	}

	if _, err := model.NormalizeChatCapabilities(`["image_upload","binary_upload"]`); err == nil {
		t.Fatal("expected unsupported chat capability to be rejected")
	}
}

func TestSanitizeModelMappingSettingsClearsChatCapabilitiesForNonChatModels(t *testing.T) {
	mapping := &model.ModelMapping{
		ModelType:        2,
		ChatCapabilities: `["web_search"]`,
	}

	sanitizeModelMappingSettings(mapping)

	if mapping.ChatCapabilities != "" {
		t.Fatalf("expected non-chat model chat capabilities to be cleared, got %q", mapping.ChatCapabilities)
	}
}

func TestValidateChatModelCapabilitiesAllowsEmptyForChatModels(t *testing.T) {
	normalized, err := validateChatModelCapabilities(1, "")
	if err != nil {
		t.Fatalf("expected empty chat capabilities to be allowed, got %v", err)
	}
	if normalized != "" {
		t.Fatalf("expected empty normalized chat capabilities, got %q", normalized)
	}

	normalized, err = validateChatModelCapabilities(1, `["file_upload"]`)
	if err != nil {
		t.Fatalf("expected file upload capability to validate, got %v", err)
	}
	if normalized != `["file_upload"]` {
		t.Fatalf("unexpected normalized chat capabilities: %q", normalized)
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

func TestSanitizeModelMappingSettingsClearsChatCapabilitiesOutsideChatModels(t *testing.T) {
	mapping := &model.ModelMapping{
		ModelType:         2,
		ChatCapabilities:  `["image_upload","file_upload"]`,
		ImageCapabilities: `["image_generation"]`,
	}

	sanitizeModelMappingSettings(mapping)

	if mapping.ChatCapabilities != "" {
		t.Fatalf("expected chat capabilities to be cleared, got %q", mapping.ChatCapabilities)
	}
	if mapping.ImageCapabilities == "" {
		t.Fatal("expected image capabilities to be preserved for image models")
	}
}

func TestModelMappingInsertAndUpdatePersistChatCapabilities(t *testing.T) {
	previousDB := model.DB
	previousUsingSQLite := common.UsingSQLite
	previousUsingMySQL := common.UsingMySQL
	previousUsingPostgreSQL := common.UsingPostgreSQL

	db, err := gorm.Open(
		sqlite.Open("file:model_mapping_chat_caps?mode=memory&cache=shared"),
		&gorm.Config{},
	)
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	model.InitCommonColumnNames()
	t.Cleanup(func() {
		model.DB = previousDB
		common.UsingSQLite = previousUsingSQLite
		common.UsingMySQL = previousUsingMySQL
		common.UsingPostgreSQL = previousUsingPostgreSQL
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	if err := db.AutoMigrate(&model.ModelMapping{}); err != nil {
		t.Fatalf("failed to migrate model mappings: %v", err)
	}

	mapping := &model.ModelMapping{
		RequestModel:     "gpt-chat-test",
		ActualModel:      "gpt-chat-test",
		DisplayName:      "GPT Chat Test",
		ModelSeries:      "openai",
		ModelType:        1,
		Status:           1,
		RequestEndpoint:  "openai",
		ChatCapabilities: `["image_upload","file_upload"]`,
	}
	if err := mapping.Insert(); err != nil {
		t.Fatalf("failed to insert model mapping: %v", err)
	}
	if mapping.ChatCapabilities != `["image_upload","file_upload"]` {
		t.Fatalf("expected inserted chat capabilities to persist, got %q", mapping.ChatCapabilities)
	}

	mapping.ChatCapabilities = `["file_upload"]`
	if err := mapping.Update(); err != nil {
		t.Fatalf("failed to update model mapping: %v", err)
	}

	reloaded, err := model.GetModelMapping(mapping.Id)
	if err != nil {
		t.Fatalf("failed to reload model mapping: %v", err)
	}
	if reloaded.ChatCapabilities != `["file_upload"]` {
		t.Fatalf("expected updated chat capabilities to persist, got %q", reloaded.ChatCapabilities)
	}
}
