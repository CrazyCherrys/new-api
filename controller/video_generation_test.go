package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

func TestGetVideoGenerationModelsIncludesCatalogMetadata(t *testing.T) {
	db := setupImageGenerationControllerTestDB(t)

	if err := db.Create(&model.ModelMapping{
		RequestModel:      "video-alpha",
		ActualModel:       "video-alpha",
		DisplayName:       "Video Alpha",
		ModelSeries:       "openai",
		ModelType:         3,
		Status:            1,
		RequestEndpoint:   "openai-video",
		VideoCapabilities: `["text_to_video"]`,
		DurationOptions:   `[5]`,
	}).Error; err != nil {
		t.Fatalf("failed to create video mapping: %v", err)
	}
	vendor := &model.Vendor{
		Name:   "OpenAI",
		Icon:   "OpenAI",
		Status: 1,
	}
	if err := db.Create(vendor).Error; err != nil {
		t.Fatalf("failed to create vendor: %v", err)
	}
	if err := db.Create(&model.Model{
		ModelName:    "alpha",
		Description:  "Video catalog description",
		VendorID:     vendor.Id,
		NameRule:     model.NameRuleSuffix,
		Status:       1,
		SyncOfficial: 1,
	}).Error; err != nil {
		t.Fatalf("failed to create video model metadata: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/video-generation/models", nil, 1)
	GetVideoGenerationModels(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var items []dto.VideoGenerationModel
	if err := common.Unmarshal(response.Data, &items); err != nil {
		t.Fatalf("failed to decode video model response: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one video model, got %#v", items)
	}
	if items[0].Description != "Video catalog description" || items[0].VendorIcon != "OpenAI" {
		t.Fatalf("expected catalog metadata in video model response, got %#v", items[0])
	}
}
