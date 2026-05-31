package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

func seedCanvasChatModelAbility(
	t *testing.T,
	channelId int,
	group string,
	modelName string,
) {
	t.Helper()

	channel := &model.Channel{
		Id:     channelId,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "channel-key-" + modelName,
		Status: common.ChannelStatusEnabled,
		Name:   "channel-" + modelName,
		Group:  group,
		Models: modelName,
	}
	if err := model.DB.Create(channel).Error; err != nil {
		t.Fatalf("failed to create channel %s: %v", modelName, err)
	}

	ability := &model.Ability{
		Group:     group,
		Model:     modelName,
		ChannelId: channel.Id,
		Enabled:   true,
		Weight:    0,
	}
	if err := model.DB.Create(ability).Error; err != nil {
		t.Fatalf("failed to create ability %s: %v", modelName, err)
	}
}

func TestListUserCanvasChatModelsFiltersNonChatMappings(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)

	const (
		userID     = 41
		userGroup  = "default"
		chatModel  = "gpt-4.1"
		imageModel = "gpt-image-1"
		videoModel = "sora-video"
	)

	seedCanvasChatCapability(t, db, userID, userGroup, userGroup, userGroup, chatModel)
	seedCanvasChatModelAbility(t, userID+1001, userGroup, imageModel)
	seedCanvasChatModelAbility(t, userID+1002, userGroup, videoModel)

	mappings := []*model.ModelMapping{
		{
			RequestModel:    chatModel,
			ActualModel:     chatModel,
			DisplayName:     "Chat Model",
			ModelSeries:     "openai",
			ModelType:       1,
			Status:          1,
			RequestEndpoint: "openai",
		},
		{
			RequestModel:      imageModel,
			ActualModel:       imageModel,
			DisplayName:       "Image Model",
			ModelSeries:       "openai",
			ModelType:         2,
			Status:            1,
			RequestEndpoint:   "openai",
			ImageCapabilities: `["image_generation"]`,
		},
		{
			RequestModel:      videoModel,
			ActualModel:       videoModel,
			DisplayName:       "Video Model",
			ModelSeries:       "openai",
			ModelType:         3,
			Status:            1,
			RequestEndpoint:   "openai-video",
			VideoCapabilities: `["text_to_video"]`,
		},
	}
	for _, mapping := range mappings {
		if err := db.Create(mapping).Error; err != nil {
			t.Fatalf("failed to create model mapping %s: %v", mapping.RequestModel, err)
		}
	}

	models, err := ListUserCanvasChatModels(userID)
	if err != nil {
		t.Fatalf("expected chat model list to load: %v", err)
	}
	if len(models) != 1 || models[0] != chatModel {
		t.Fatalf("expected only chat model %q, got %#v", chatModel, models)
	}

	resolved, err := resolveCanvasChatModel(userID, &model.CanvasSession{}, "")
	if err != nil {
		t.Fatalf("expected chat model resolve to succeed: %v", err)
	}
	if resolved != chatModel {
		t.Fatalf("expected chat model fallback %q, got %q", chatModel, resolved)
	}
}
