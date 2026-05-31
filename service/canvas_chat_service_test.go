package service

import (
	"strings"
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

func seedCanvasChatUser(t *testing.T, userId int, userGroup string) {
	t.Helper()

	user := &model.User{
		Id:       userId,
		Username: "canvas-chat-user-" + userGroup,
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    userGroup,
	}
	if err := model.DB.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
}

func TestListUserCanvasChatModelsFiltersNonChatMappings(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)

	const (
		userID            = 41
		userGroup         = "default"
		chatModel         = "gpt-4.1"
		imageModel        = "gpt-image-1"
		videoModel        = "sora-video"
		unmappedModel     = "legacy-freeform-model"
		disabledChatModel = "gpt-disabled-chat"
		noTokenChatModel  = "gpt-no-token-chat"
	)

	seedCanvasChatCapability(t, db, userID, userGroup, userGroup, userGroup, chatModel)
	seedCanvasChatModelAbility(t, userID+1001, userGroup, imageModel)
	seedCanvasChatModelAbility(t, userID+1002, userGroup, videoModel)
	seedCanvasChatModelAbility(t, userID+1003, userGroup, unmappedModel)
	seedCanvasChatModelAbility(t, userID+1004, userGroup, disabledChatModel)
	seedCanvasChatModelAbility(t, userID+1005, "vip", noTokenChatModel)
	if err := db.Model(&model.ModelMapping{}).
		Where("request_model = ?", chatModel).
		Updates(map[string]any{
			"display_name": "Chat Model",
			"model_series": "openai",
		}).Error; err != nil {
		t.Fatalf("failed to update seeded chat model mapping: %v", err)
	}

	mappings := []*model.ModelMapping{
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
		{
			RequestModel:    disabledChatModel,
			ActualModel:     disabledChatModel,
			DisplayName:     "Disabled Chat Model",
			ModelSeries:     "openai",
			ModelType:       1,
			Status:          0,
			RequestEndpoint: "openai",
		},
		{
			RequestModel:    noTokenChatModel,
			ActualModel:     noTokenChatModel,
			DisplayName:     "No Token Chat Model",
			ModelSeries:     "claude",
			ModelType:       1,
			Status:          1,
			RequestEndpoint: "anthropic",
		},
	}
	for _, mapping := range mappings {
		if err := db.Create(mapping).Error; err != nil {
			t.Fatalf("failed to create model mapping %s: %v", mapping.RequestModel, err)
		}
	}
	if err := db.Model(&model.ModelMapping{}).
		Where("request_model = ?", disabledChatModel).
		Update("status", 0).Error; err != nil {
		t.Fatalf("failed to disable chat model mapping: %v", err)
	}

	models, err := ListUserCanvasChatModels(userID)
	if err != nil {
		t.Fatalf("expected chat model list to load: %v", err)
	}
	if len(models) != 1 || models[0] != chatModel {
		t.Fatalf("expected only chat model %q, got %#v", chatModel, models)
	}
	catalog, err := ListUserCanvasChatModelCatalog(userID)
	if err != nil {
		t.Fatalf("expected chat model catalog to load: %v", err)
	}
	if len(catalog) != 1 {
		t.Fatalf("expected only one catalog item, got %#v", catalog)
	}
	if catalog[0].RequestModel != chatModel {
		t.Fatalf("expected catalog request model %q, got %#v", chatModel, catalog[0])
	}
	if catalog[0].DisplayName != "Chat Model" {
		t.Fatalf("expected catalog display name to be preserved, got %#v", catalog[0])
	}
	if catalog[0].ModelSeries != "openai" || catalog[0].RequestEndpoint != "openai" {
		t.Fatalf("expected catalog metadata to be preserved, got %#v", catalog[0])
	}

	resolved, err := resolveCanvasChatModel(userID, &model.CanvasSession{}, "")
	if err != nil {
		t.Fatalf("expected chat model resolve to succeed: %v", err)
	}
	if resolved != chatModel {
		t.Fatalf("expected chat model fallback %q, got %q", chatModel, resolved)
	}
}

func TestListUserCanvasChatModelCatalogRequiresAvailableToken(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)

	const (
		userID    = 52
		userGroup = "default"
		modelName = "gpt-no-token"
	)

	user := &model.User{
		Id:       userID,
		Username: "catalog-no-token-user",
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    userGroup,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	channel := &model.Channel{
		Id:     userID + 2000,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "catalog-no-token-channel",
		Status: common.ChannelStatusEnabled,
		Name:   "catalog-no-token-channel",
		Group:  userGroup,
		Models: modelName,
	}
	if err := db.Create(channel).Error; err != nil {
		t.Fatalf("failed to create channel: %v", err)
	}

	ability := &model.Ability{
		Group:     userGroup,
		Model:     modelName,
		ChannelId: channel.Id,
		Enabled:   true,
		Weight:    0,
	}
	if err := db.Create(ability).Error; err != nil {
		t.Fatalf("failed to create ability: %v", err)
	}

	mapping := &model.ModelMapping{
		RequestModel:    modelName,
		ActualModel:     modelName,
		DisplayName:     "No Token Model",
		ModelSeries:     "openai",
		ModelType:       1,
		Status:          1,
		RequestEndpoint: "openai",
	}
	if err := db.Create(mapping).Error; err != nil {
		t.Fatalf("failed to create model mapping: %v", err)
	}

	catalog, err := ListUserCanvasChatModelCatalog(userID)
	if err != nil {
		t.Fatalf("expected empty catalog without token, got error: %v", err)
	}
	if len(catalog) != 0 {
		t.Fatalf("expected no catalog items without available token, got %#v", catalog)
	}

	models, err := ListUserCanvasChatModels(userID)
	if err != nil {
		t.Fatalf("expected empty model list without token, got error: %v", err)
	}
	if len(models) != 0 {
		t.Fatalf("expected no model ids without available token, got %#v", models)
	}
}

func TestListUserCanvasChatModelOptionsReturnsStructuredData(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)

	const (
		userID          = 61
		userGroup       = "default"
		usableModel     = "gpt-structured"
		hiddenModel     = "gpt-hidden"
		displayName     = "Structured Chat Model"
		requestEndpoint = "openai"
	)

	seedCanvasChatCapability(t, db, userID, userGroup, userGroup, userGroup, usableModel)
	if err := db.Model(&model.ModelMapping{}).
		Where("request_model = ?", usableModel).
		Updates(map[string]any{
			"display_name": displayName,
			"model_series": "openai",
		}).Error; err != nil {
		t.Fatalf("failed to update seeded structured model mapping: %v", err)
	}

	mappings := []*model.ModelMapping{
		{
			RequestModel:    hiddenModel,
			ActualModel:     hiddenModel,
			DisplayName:     "Hidden Chat Model",
			ModelSeries:     "openai",
			ModelType:       1,
			Status:          1,
			RequestEndpoint: requestEndpoint,
		},
	}
	for _, mapping := range mappings {
		if err := db.Create(mapping).Error; err != nil {
			t.Fatalf("failed to create model mapping %s: %v", mapping.RequestModel, err)
		}
	}

	options, err := ListUserCanvasChatModelOptions(userID)
	if err != nil {
		t.Fatalf("expected structured chat model options: %v", err)
	}
	if len(options) != 1 {
		t.Fatalf("expected only visible model option, got %#v", options)
	}
	if options[0].RequestModel != usableModel || options[0].DisplayName != displayName {
		t.Fatalf("unexpected model option payload: %#v", options[0])
	}
	if !options[0].Usable || len(options[0].AvailableGroups) != 1 || options[0].AvailableGroups[0] != userGroup {
		t.Fatalf("expected usable model with available group %q, got %#v", userGroup, options[0])
	}
	if options[0].RequestEndpoint != requestEndpoint {
		t.Fatalf("expected request endpoint %q, got %#v", requestEndpoint, options[0])
	}
}

func TestListUserCanvasChatModelOptionsMarksUnreachableModel(t *testing.T) {
	setupCanvasSessionServiceTestDB(t)

	const (
		userID      = 71
		userGroup   = "default"
		modelID     = "gpt-no-token"
		displayName = "No Token Chat Model"
	)

	seedCanvasChatUser(t, userID, userGroup)
	seedCanvasChatModelAbility(t, userID+2000, userGroup, modelID)
	if err := model.DB.Create(&model.ModelMapping{
		RequestModel:    modelID,
		ActualModel:     modelID,
		DisplayName:     displayName,
		ModelSeries:     "openai",
		ModelType:       1,
		Status:          1,
		RequestEndpoint: "openai",
	}).Error; err != nil {
		t.Fatalf("failed to create model mapping: %v", err)
	}

	options, err := ListUserCanvasChatModelOptions(userID)
	if err != nil {
		t.Fatalf("expected structured chat model options: %v", err)
	}
	if len(options) != 1 {
		t.Fatalf("expected one visible but unreachable model, got %#v", options)
	}
	if options[0].Usable {
		t.Fatalf("expected model to be marked unusable, got %#v", options[0])
	}
	if options[0].UnavailableReason != "当前没有可达分组/令牌支持该模型" {
		t.Fatalf("unexpected unavailable reason: %#v", options[0])
	}
}

func TestResolveCanvasChatModelRejectsUnavailableCurrentSessionModel(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)

	const (
		userID       = 81
		userGroup    = "default"
		usableModel  = "gpt-usable"
		invalidModel = "gpt-orphaned"
	)

	seedCanvasChatCapability(t, db, userID, userGroup, userGroup, userGroup, usableModel)
	seedCanvasChatModelAbility(t, userID+3000, "vip", invalidModel)
	if err := db.Model(&model.ModelMapping{}).
		Where("request_model = ?", usableModel).
		Updates(map[string]any{
			"display_name": "Usable",
			"model_series": "openai",
		}).Error; err != nil {
		t.Fatalf("failed to update seeded usable model mapping: %v", err)
	}

	mappings := []*model.ModelMapping{
		{
			RequestModel:    invalidModel,
			ActualModel:     invalidModel,
			DisplayName:     "Unavailable",
			ModelSeries:     "openai",
			ModelType:       1,
			Status:          1,
			RequestEndpoint: "openai",
		},
	}
	for _, mapping := range mappings {
		if err := db.Create(mapping).Error; err != nil {
			t.Fatalf("failed to create model mapping %s: %v", mapping.RequestModel, err)
		}
	}

	_, err := resolveCanvasChatModel(userID, &model.CanvasSession{CurrentModel: invalidModel}, "")
	if err == nil {
		t.Fatal("expected unavailable current session model to be rejected")
	}
	if !strings.Contains(err.Error(), invalidModel) {
		t.Fatalf("expected error to mention invalid model %q, got %v", invalidModel, err)
	}
}

func TestDiagnoseCanvasChatModelMappingWarnings(t *testing.T) {
	t.Run("warns without ability", func(t *testing.T) {
		setupCanvasSessionServiceTestDB(t)
		const modelID = "gpt-no-ability"
		if err := model.DB.Create(&model.ModelMapping{
			RequestModel:    modelID,
			ActualModel:     modelID,
			DisplayName:     "No Ability",
			ModelSeries:     "openai",
			ModelType:       1,
			Status:          1,
			RequestEndpoint: "openai",
		}).Error; err != nil {
			t.Fatalf("failed to create model mapping: %v", err)
		}

		diagnostic, err := DiagnoseCanvasChatModelMapping(modelID)
		if err != nil {
			t.Fatalf("expected diagnostic result: %v", err)
		}
		if diagnostic == nil || len(diagnostic.WarningMessages) != 1 {
			t.Fatalf("expected a single warning, got %#v", diagnostic)
		}
		if diagnostic.WarningMessages[0] != "映射已保存，但没有任何启用能力声明该模型，Canvas 不会显示" {
			t.Fatalf("unexpected warning: %#v", diagnostic.WarningMessages)
		}
	})

	t.Run("warns without reachable token", func(t *testing.T) {
		setupCanvasSessionServiceTestDB(t)
		const (
			userID    = 91
			userGroup = "default"
			modelID   = "gpt-no-route"
		)
		seedCanvasChatUser(t, userID, userGroup)
		seedCanvasChatModelAbility(t, userID+4000, userGroup, modelID)
		if err := model.DB.Create(&model.ModelMapping{
			RequestModel:    modelID,
			ActualModel:     modelID,
			DisplayName:     "No Route",
			ModelSeries:     "openai",
			ModelType:       1,
			Status:          1,
			RequestEndpoint: "openai",
		}).Error; err != nil {
			t.Fatalf("failed to create model mapping: %v", err)
		}

		diagnostic, err := DiagnoseCanvasChatModelMapping(modelID)
		if err != nil {
			t.Fatalf("expected diagnostic result: %v", err)
		}
		if diagnostic == nil || len(diagnostic.WarningMessages) != 1 {
			t.Fatalf("expected a single warning, got %#v", diagnostic)
		}
		if diagnostic.WarningMessages[0] != "映射已保存，但当前没有可达分组/令牌支持该模型" {
			t.Fatalf("unexpected warning: %#v", diagnostic.WarningMessages)
		}
	})

	t.Run("no warning when mapping is reachable", func(t *testing.T) {
		db := setupCanvasSessionServiceTestDB(t)
		const (
			userID    = 101
			userGroup = "default"
			modelID   = "gpt-reachable"
		)
		seedCanvasChatCapability(t, db, userID, userGroup, userGroup, userGroup, modelID)
		if err := db.Model(&model.ModelMapping{}).
			Where("request_model = ?", modelID).
			Updates(map[string]any{
				"display_name": "Reachable",
				"model_series": "openai",
			}).Error; err != nil {
			t.Fatalf("failed to update seeded model mapping: %v", err)
		}

		diagnostic, err := DiagnoseCanvasChatModelMapping(modelID)
		if err != nil {
			t.Fatalf("expected diagnostic result: %v", err)
		}
		if diagnostic == nil {
			t.Fatal("expected diagnostic payload")
		}
		if len(diagnostic.WarningMessages) != 0 || !diagnostic.VisibleInCanvas {
			t.Fatalf("expected reachable mapping without warnings, got %#v", diagnostic)
		}
	})
}
