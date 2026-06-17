package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

type canvasChatRelayRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn canvasChatRelayRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

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
		chatEndpoint      = "openai-response"
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
			"display_name":      "Chat Model",
			"model_series":      "openai",
			"request_endpoint":  chatEndpoint,
			"chat_capabilities": `["image_upload","file_upload","web_search"]`,
		}).Error; err != nil {
		t.Fatalf("failed to update seeded chat model mapping: %v", err)
	}
	vendor := &model.Vendor{
		Name:   "OpenAI",
		Icon:   "OpenAI",
		Status: 1,
	}
	if err := db.Create(vendor).Error; err != nil {
		t.Fatalf("failed to create chat model vendor: %v", err)
	}
	if err := db.Create(&model.Model{
		ModelName:    chatModel,
		Description:  "Chat model catalog description",
		VendorID:     vendor.Id,
		NameRule:     model.NameRuleExact,
		Status:       1,
		SyncOfficial: 1,
	}).Error; err != nil {
		t.Fatalf("failed to create chat model metadata: %v", err)
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
	if catalog[0].ModelSeries != "openai" || catalog[0].RequestEndpoint != chatEndpoint {
		t.Fatalf("expected catalog metadata to be preserved, got %#v", catalog[0])
	}
	if len(catalog[0].ChatCapabilities) != 3 || catalog[0].ChatCapabilities[0] != "image_upload" || catalog[0].ChatCapabilities[1] != "file_upload" || catalog[0].ChatCapabilities[2] != "web_search" {
		t.Fatalf("expected catalog chat capabilities to be preserved, got %#v", catalog[0])
	}
	if catalog[0].Description != "Chat model catalog description" || catalog[0].VendorIcon != "OpenAI" {
		t.Fatalf("expected catalog display metadata to be preserved, got %#v", catalog[0])
	}

	resolved, err := resolveCanvasChatModel(userID, &model.CanvasSession{}, "")
	if err != nil {
		t.Fatalf("expected chat model resolve to succeed: %v", err)
	}
	if resolved != chatModel {
		t.Fatalf("expected chat model fallback %q, got %q", chatModel, resolved)
	}
}

func TestApplyCanvasChatRelayWebSearchByRequestEndpoint(t *testing.T) {
	tests := []struct {
		name            string
		requestEndpoint string
		enabled         bool
		expectInjected  bool
	}{
		{name: "openai enabled", requestEndpoint: "openai", enabled: true, expectInjected: true},
		{name: "openai-response enabled", requestEndpoint: "openai-response", enabled: true, expectInjected: true},
		{name: "anthropic enabled", requestEndpoint: "anthropic", enabled: true, expectInjected: true},
		{name: "openai disabled", requestEndpoint: "openai", enabled: false, expectInjected: false},
		{name: "gemini safe downgrade", requestEndpoint: "gemini", enabled: true, expectInjected: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := &dto.GeneralOpenAIRequest{}
			applyCanvasChatRelayWebSearch(request, tc.requestEndpoint, tc.enabled)
			if tc.expectInjected {
				if request.WebSearchOptions == nil {
					t.Fatalf("expected web search options to be injected for %q", tc.requestEndpoint)
				}
				if request.WebSearchOptions.SearchContextSize != "medium" {
					t.Fatalf("expected medium search context size, got %#v", request.WebSearchOptions)
				}
				return
			}
			if request.WebSearchOptions != nil {
				t.Fatalf("expected web search options to stay nil, got %#v", request.WebSearchOptions)
			}
		})
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
			"display_name":      displayName,
			"model_series":      "openai",
			"chat_capabilities": `["image_upload"]`,
		}).Error; err != nil {
		t.Fatalf("failed to update seeded structured model mapping: %v", err)
	}
	vendor := &model.Vendor{
		Name:   "OpenAI",
		Icon:   "OpenAI",
		Status: 1,
	}
	if err := db.Create(vendor).Error; err != nil {
		t.Fatalf("failed to create structured model vendor: %v", err)
	}
	if err := db.Create(&model.Model{
		ModelName:    usableModel,
		Description:  "Structured catalog description",
		VendorID:     vendor.Id,
		NameRule:     model.NameRuleExact,
		Status:       1,
		SyncOfficial: 1,
	}).Error; err != nil {
		t.Fatalf("failed to create structured model metadata: %v", err)
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
	if len(options[0].ChatCapabilities) != 1 || options[0].ChatCapabilities[0] != "image_upload" {
		t.Fatalf("expected chat capabilities to be preserved, got %#v", options[0])
	}
	if options[0].Description != "Structured catalog description" || options[0].VendorIcon != "OpenAI" {
		t.Fatalf("expected display metadata to be preserved, got %#v", options[0])
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

func TestCanvasChatResponseMetadataUpstreamResponseIDRoundTrip(t *testing.T) {
	metadata, err := buildCanvasChatResponseMetadata(nil, "gpt-4.1", "default", nil, 8, true, nil, "resp_123")
	if err != nil {
		t.Fatalf("failed to build response metadata: %v", err)
	}
	if got := extractCanvasChatUpstreamResponseID(metadata); got != "resp_123" {
		t.Fatalf("expected upstream response id round trip, got %q", got)
	}
}

func TestBuildCanvasChatResponsesRequest(t *testing.T) {
	setupCanvasSessionServiceTestDB(t)

	systemPrompt := "system prompt"
	session, err := CreateCanvasSession(1, CreateCanvasSessionInput{
		Mode:         model.CanvasModeChat,
		SystemPrompt: &systemPrompt,
	})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	buildPrepared := func(prompt string, metadata string, webSearch bool) *canvasChatPreparedRequest {
		temperature := 0.25
		return &canvasChatPreparedRequest{
			UserId:             1,
			Session:            session,
			Prompt:             prompt,
			FinalModel:         "gpt-4.1",
			RequestEndpoint:    "openai-response",
			FinalGroup:         "default",
			Temperature:        &temperature,
			ContextCount:       8,
			WebSearchEnabled:   webSearch,
			PreviousResponseID: "resp_prev_001",
			UserMessage: &model.CanvasMessage{
				Id:       11,
				Role:     model.CanvasMessageRoleUser,
				Prompt:   prompt,
				Metadata: metadata,
			},
		}
	}

	t.Run("plain text", func(t *testing.T) {
		req, err := buildCanvasChatResponsesRequest(buildPrepared("hello", "", false))
		if err != nil {
			t.Fatalf("failed to build responses request: %v", err)
		}
		if req.Model != "gpt-4.1" || req.PreviousResponseID != "resp_prev_001" {
			t.Fatalf("unexpected request envelope: %#v", req)
		}
		if req.Stream == nil || !*req.Stream {
			t.Fatalf("expected stream true, got %#v", req.Stream)
		}
		if len(req.ToolChoice) != 0 {
			t.Fatalf("expected tool_choice empty without web search, got %s", string(req.ToolChoice))
		}
		var input []map[string]any
		if err := common.Unmarshal(req.Input, &input); err != nil {
			t.Fatalf("failed to decode input: %v", err)
		}
		if len(input) != 1 {
			t.Fatalf("expected one input item, got %#v", input)
		}
		content := input[0]["content"].([]any)
		if len(content) != 1 || content[0].(map[string]any)["type"] != "input_text" || content[0].(map[string]any)["text"] != "hello" {
			t.Fatalf("expected plain text input, got %#v", input)
		}
	})

	t.Run("text plus image", func(t *testing.T) {
		req, err := buildCanvasChatResponsesRequest(buildPrepared(
			"look",
			`{"attachments":[{"kind":"image","name":"ref.png","mime_type":"image/png","data":"data:image/png;base64,Zm9v"}]}`,
			false,
		))
		if err != nil {
			t.Fatalf("failed to build responses request: %v", err)
		}
		var input []map[string]any
		if err := common.Unmarshal(req.Input, &input); err != nil {
			t.Fatalf("failed to decode input: %v", err)
		}
		if len(input) != 1 {
			t.Fatalf("expected one input item, got %#v", input)
		}
		content := input[0]["content"].([]any)
		if len(content) != 2 {
			t.Fatalf("expected text + image content, got %#v", content)
		}
		if content[0].(map[string]any)["type"] != "input_text" || content[1].(map[string]any)["type"] != "input_image" {
			t.Fatalf("unexpected content shape: %#v", content)
		}
	})

	t.Run("text plus file", func(t *testing.T) {
		req, err := buildCanvasChatResponsesRequest(buildPrepared(
			"read this",
			`{"attachments":[{"kind":"file","name":"notes.txt","mime_type":"text/plain","data":"data:text/plain;base64,YmFy"}]}`,
			false,
		))
		if err != nil {
			t.Fatalf("failed to build responses request: %v", err)
		}
		var input []map[string]any
		if err := common.Unmarshal(req.Input, &input); err != nil {
			t.Fatalf("failed to decode input: %v", err)
		}
		if len(input) != 1 {
			t.Fatalf("expected one input item, got %#v", input)
		}
		content := input[0]["content"].([]any)
		if len(content) != 2 {
			t.Fatalf("expected text + file content, got %#v", content)
		}
		if content[0].(map[string]any)["type"] != "input_text" || content[1].(map[string]any)["type"] != "input_file" {
			t.Fatalf("unexpected content shape: %#v", content)
		}
	})

	t.Run("web search", func(t *testing.T) {
		req, err := buildCanvasChatResponsesRequest(buildPrepared("search", "", true))
		if err != nil {
			t.Fatalf("failed to build responses request: %v", err)
		}
		if string(req.ToolChoice) != "\"auto\"" {
			t.Fatalf("expected tool_choice auto, got %s", string(req.ToolChoice))
		}
		tools := req.GetToolsMap()
		if len(tools) != 1 || tools[0]["type"] != dto.BuildInToolWebSearch {
			t.Fatalf("expected official web_search tool, got %#v", tools)
		}
		var instructions string
		if err := common.Unmarshal(req.Instructions, &instructions); err != nil {
			t.Fatalf("failed to decode instructions: %v", err)
		}
		if instructions != systemPrompt {
			t.Fatalf("expected system prompt in instructions, got %q", instructions)
		}
	})

	t.Run("reasoning and max_output_tokens from params", func(t *testing.T) {
		prepared := buildPrepared("think", "", false)
		prepared.ResponsesOptions = &canvasChatResponsesOptions{
			Reasoning: &dto.Reasoning{
				Effort:  "medium",
				Summary: "auto",
			},
			MaxOutputTokens:   common.GetPointer(uint(256)),
			Store:             common.GetPointer(true),
			ParallelToolCalls: common.GetPointer(false),
		}
		req, err := buildCanvasChatResponsesRequest(prepared)
		if err != nil {
			t.Fatalf("failed to build responses request: %v", err)
		}
		if req.Reasoning == nil || req.Reasoning.Effort != "medium" || req.Reasoning.Summary != "auto" {
			t.Fatalf("expected reasoning to persist, got %#v", req.Reasoning)
		}
		if req.MaxOutputTokens == nil || *req.MaxOutputTokens != 256 {
			t.Fatalf("expected max_output_tokens 256, got %#v", req.MaxOutputTokens)
		}
		var store bool
		if err := common.Unmarshal(req.Store, &store); err != nil {
			t.Fatalf("failed to decode store: %v", err)
		}
		if !store {
			t.Fatalf("expected store=true, got %v", store)
		}
		var parallel bool
		if err := common.Unmarshal(req.ParallelToolCalls, &parallel); err != nil {
			t.Fatalf("failed to decode parallel_tool_calls: %v", err)
		}
		if parallel {
			t.Fatalf("expected parallel_tool_calls=false, got %v", parallel)
		}
	})
}

func TestParseCanvasResponsesBody(t *testing.T) {
	body, err := common.Marshal(dto.OpenAIResponsesResponse{
		ID:    "resp_abc",
		Model: "gpt-4.1",
		Output: []dto.ResponsesOutput{
			{
				Type: "message",
				Role: "assistant",
				Content: []dto.ResponsesOutputContent{
					{Type: "output_text", Text: "final answer"},
				},
			},
		},
		Reasoning: &dto.Reasoning{
			Summary: "reasoning summary",
		},
	})
	if err != nil {
		t.Fatalf("failed to marshal responses body: %v", err)
	}
	result, err := parseCanvasResponsesBody(body, nil)
	if err != nil {
		t.Fatalf("failed to parse responses body: %v", err)
	}
	if result.Text != "final answer" {
		t.Fatalf("expected output text, got %q", result.Text)
	}
	if result.ReasoningContent != "reasoning summary" {
		t.Fatalf("expected reasoning summary, got %q", result.ReasoningContent)
	}
	if result.UpstreamResponseID != "resp_abc" {
		t.Fatalf("expected upstream response id, got %q", result.UpstreamResponseID)
	}
}

func TestParseCanvasResponsesStreamResponse(t *testing.T) {
	tests := []struct {
		name            string
		payload         string
		wantContent     string
		wantReasoning   string
		wantResponseID  string
		wantCompletedID string
		wantEmit        bool
	}{
		{
			name:           "created",
			payload:        `{"type":"response.created","response":{"id":"resp_1"}}`,
			wantResponseID: "resp_1",
		},
		{
			name:        "text delta",
			payload:     `{"type":"response.output_text.delta","delta":"hello"}`,
			wantContent: "hello",
			wantEmit:    true,
		},
		{
			name:          "reasoning delta",
			payload:       `{"type":"response.reasoning_summary_text.delta","delta":"step 1"}`,
			wantReasoning: "step 1",
			wantEmit:      true,
		},
		{
			name:            "completed",
			payload:         `{"type":"response.completed","response":{"id":"resp_done"}}`,
			wantResponseID:  "resp_done",
			wantCompletedID: "resp_done",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			delta, responseID, completedID, emitDelta, err := parseCanvasResponsesStreamResponse(tc.payload)
			if err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}
			if delta.Content != tc.wantContent || delta.ReasoningContent != tc.wantReasoning {
				t.Fatalf("unexpected delta: %#v", delta)
			}
			if responseID != tc.wantResponseID {
				t.Fatalf("expected response id %q, got %q", tc.wantResponseID, responseID)
			}
			if completedID != tc.wantCompletedID {
				t.Fatalf("expected completed id %q, got %q", tc.wantCompletedID, completedID)
			}
			if emitDelta != tc.wantEmit {
				t.Fatalf("expected emit=%v, got %v", tc.wantEmit, emitDelta)
			}
		})
	}
}

func TestDefaultCallCanvasChatRelayRoutesByRequestEndpoint(t *testing.T) {
	db := setupCanvasSessionServiceTestDB(t)
	seedCanvasChatCapability(t, db, 1, "default", "default", "default", "gpt-chat-test")

	previousClient := canvasChatHTTPClient
	t.Cleanup(func() {
		canvasChatHTTPClient = previousClient
	})

	var capturedPath string
	var capturedCompatHeader string
	var capturedBody string
	canvasChatHTTPClient = &http.Client{
		Transport: canvasChatRelayRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(req.Body)
			capturedPath = req.URL.Path
			capturedCompatHeader = req.Header.Get(constant.HeaderCanvasChatResponsesCompat)
			capturedBody = string(body)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"text/event-stream"},
				},
				Body: io.NopCloser(bytes.NewBufferString("data: [DONE]\n\n")),
			}, nil
		}),
	}

	t.Run("openai chat completions", func(t *testing.T) {
		capturedPath = ""
		capturedCompatHeader = ""
		capturedBody = ""
		_, err := defaultCallCanvasChatRelay(context.Background(), canvasChatRelayRequest{
			UserId:           1,
			UserGroup:        "default",
			ModelId:          "gpt-chat-test",
			RequestEndpoint:  "openai",
			Group:            "default",
			Messages:         []dto.Message{{Role: "user", Content: "hello"}},
			WebSearchEnabled: false,
		}, nil)
		if err != nil {
			t.Fatalf("defaultCallCanvasChatRelay returned error: %v", err)
		}
		if capturedPath != "/v1/chat/completions" {
			t.Fatalf("expected chat completions path, got %q", capturedPath)
		}
		if capturedCompatHeader != "" {
			t.Fatalf("expected no compat header for ordinary chat, got %q", capturedCompatHeader)
		}
	})

	t.Run("openai responses", func(t *testing.T) {
		capturedPath = ""
		capturedCompatHeader = ""
		capturedBody = ""
		_, err := defaultCallCanvasChatRelay(context.Background(), canvasChatRelayRequest{
			UserId:             1,
			UserGroup:          "default",
			ModelId:            "gpt-chat-test",
			RequestEndpoint:    "openai-response",
			Group:              "default",
			Messages:           []dto.Message{{Role: "user", Content: "hello"}},
			WebSearchEnabled:   true,
			PreviousResponseID: "resp_prev_123",
		}, nil)
		if err != nil {
			t.Fatalf("defaultCallCanvasChatRelay returned error: %v", err)
		}
		if capturedPath != "/v1/responses" {
			t.Fatalf("expected responses path, got %q", capturedPath)
		}
		if capturedCompatHeader != "" {
			t.Fatalf("expected no compat header for native responses path, got %q", capturedCompatHeader)
		}
		if !strings.Contains(capturedBody, `"previous_response_id":"resp_prev_123"`) {
			t.Fatalf("expected previous_response_id in request body, got %s", capturedBody)
		}
		if !strings.Contains(capturedBody, `"type":"web_search"`) {
			t.Fatalf("expected official web_search tool in request body, got %s", capturedBody)
		}
	})

	t.Run("openai responses forwards responses options", func(t *testing.T) {
		capturedPath = ""
		capturedCompatHeader = ""
		capturedBody = ""
		_, err := defaultCallCanvasChatRelay(context.Background(), canvasChatRelayRequest{
			UserId:             1,
			UserGroup:          "default",
			ModelId:            "gpt-chat-test",
			RequestEndpoint:    "openai-response",
			Group:              "default",
			Messages:           []dto.Message{{Role: "user", Content: "hello"}},
			PreviousResponseID: "resp_prev_456",
			ResponsesOptions: &canvasChatResponsesOptions{
				Reasoning: &dto.Reasoning{
					Effort:  "high",
					Summary: "detailed",
				},
				MaxOutputTokens:   common.GetPointer(uint(128)),
				Store:             common.GetPointer(true),
				ParallelToolCalls: common.GetPointer(false),
			},
		}, nil)
		if err != nil {
			t.Fatalf("defaultCallCanvasChatRelay returned error: %v", err)
		}
		if !strings.Contains(capturedBody, `"max_output_tokens":128`) {
			t.Fatalf("expected max_output_tokens in request body, got %s", capturedBody)
		}
		if !strings.Contains(capturedBody, `"store":true`) {
			t.Fatalf("expected store in request body, got %s", capturedBody)
		}
		if !strings.Contains(capturedBody, `"parallel_tool_calls":false`) {
			t.Fatalf("expected parallel_tool_calls in request body, got %s", capturedBody)
		}
		if !strings.Contains(capturedBody, `"reasoning":{"effort":"high","summary":"detailed"}`) {
			t.Fatalf("expected reasoning in request body, got %s", capturedBody)
		}
	})
}
