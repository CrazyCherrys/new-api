package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type createCanvasSessionRequest struct {
	Mode                   string   `json:"mode"`
	Title                  string   `json:"title"`
	CurrentModel           string   `json:"current_model"`
	ChatTemperature        *float64 `json:"chat_temperature"`
	ChatContextCount       *int     `json:"chat_context_count"`
	WebSearchEnabled       *bool    `json:"web_search_enabled"`
	SystemPrompt           *string  `json:"system_prompt"`
	SummaryEnabled         *bool    `json:"summary_enabled"`
	SummaryTriggerMessages *int     `json:"summary_trigger_messages"`
	SummaryRecentMessages  *int     `json:"summary_recent_messages"`
}

type updateCanvasSessionRequest struct {
	Title                  *string  `json:"title"`
	Pinned                 *bool    `json:"pinned"`
	CurrentModel           *string  `json:"current_model"`
	ChatTemperature        *float64 `json:"chat_temperature"`
	ChatContextCount       *int     `json:"chat_context_count"`
	WebSearchEnabled       *bool    `json:"web_search_enabled"`
	SystemPrompt           *string  `json:"system_prompt"`
	SummaryEnabled         *bool    `json:"summary_enabled"`
	SummaryTriggerMessages *int     `json:"summary_trigger_messages"`
	SummaryRecentMessages  *int     `json:"summary_recent_messages"`
	ClearContextMessageId  *int     `json:"clear_context_message_id"`
	ClearContextToLatest   *bool    `json:"clear_context_to_latest"`
}

type createCanvasMessageRequest struct {
	Prompt          string                     `json:"prompt"`
	ModelId         string                     `json:"model_id"`
	Group           string                     `json:"group"`
	RequestEndpoint string                     `json:"request_endpoint"`
	Params          string                     `json:"params"`
	Attachments     []dto.CanvasChatAttachment `json:"attachments"`
	dto.CanvasChatRequestSettings
	Stream          *bool    `json:"stream"`
	Temperature     *float64 `json:"temperature"`
	ContextCount    *int     `json:"context_count"`
	ClientRequestId string   `json:"client_request_id"`
}

var (
	listCanvasSessionsForController                  = service.ListCanvasSessions
	listCanvasChatModelsForController                = service.ListUserCanvasChatModelOptions
	getCanvasChatModelsForController                 = service.ListUserCanvasChatModelCatalog
	getCanvasSessionByIdentifierForController        = model.GetCanvasSessionByIdentifier
	listCanvasMessageTimelineForSessionForController = service.ListCanvasMessageTimelineWithResolvedSession
	updateCanvasSessionForController                 = service.UpdateCanvasSessionWithResolvedSession
	deleteCanvasSessionForController                 = service.DeleteCanvasSessionWithResolvedSession
	createCanvasMessageForController                 = service.CreateCanvasMessageWithResolvedSession
	streamCanvasChatMessageForController             = service.StreamCanvasChatMessageWithResolvedSession
)

func writeCanvasError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	if model.IsCanvasModeUnavailableError(err) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	common.ApiError(c, err)
}

func ListCanvasSessions(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		common.ApiErrorMsg(c, "未授权")
		return
	}

	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	sessions, err := listCanvasSessionsForController(userId, c.Query("mode"), limit, offset)
	if err != nil {
		writeCanvasError(c, err)
		return
	}
	common.ApiSuccess(c, sessions)
}

func GetCanvasSession(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		common.ApiErrorMsg(c, "未授权")
		return
	}

	session, err := resolveCanvasSessionForController(c, userId)
	if err != nil {
		writeCanvasError(c, err)
		return
	}
	if session == nil {
		writeCanvasError(c, fmt.Errorf("canvas session not found"))
		return
	}

	common.ApiSuccess(c, session)
}

func ListCanvasChatModels(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		common.ApiErrorMsg(c, "未授权")
		return
	}

	models, err := listCanvasChatModelsForController(userId)
	if err != nil {
		writeCanvasError(c, err)
		return
	}
	common.ApiSuccess(c, models)
}

func GetCanvasChatModels(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		common.ApiErrorMsg(c, "未授权")
		return
	}

	models, err := getCanvasChatModelsForController(userId)
	if err != nil {
		writeCanvasError(c, err)
		return
	}
	common.ApiSuccess(c, models)
}

func CreateCanvasSession(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		common.ApiErrorMsg(c, "未授权")
		return
	}

	var req createCanvasSessionRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}

	session, err := service.CreateCanvasSession(userId, service.CreateCanvasSessionInput{
		Mode:                   req.Mode,
		Title:                  req.Title,
		CurrentModel:           req.CurrentModel,
		ChatTemperature:        req.ChatTemperature,
		ChatContextCount:       req.ChatContextCount,
		WebSearchEnabled:       req.WebSearchEnabled,
		SystemPrompt:           req.SystemPrompt,
		SummaryEnabled:         req.SummaryEnabled,
		SummaryTriggerMessages: req.SummaryTriggerMessages,
		SummaryRecentMessages:  req.SummaryRecentMessages,
	})
	if err != nil {
		writeCanvasError(c, err)
		return
	}
	common.ApiSuccess(c, session)
}

func UpdateCanvasSession(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		common.ApiErrorMsg(c, "未授权")
		return
	}
	session, err := resolveCanvasSessionForController(c, userId)
	if err != nil {
		writeCanvasError(c, err)
		return
	}
	if session == nil {
		writeCanvasError(c, fmt.Errorf("canvas session not found"))
		return
	}

	var req updateCanvasSessionRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}

	session, err = updateCanvasSessionForController(userId, session, service.UpdateCanvasSessionInput{
		Title:                  req.Title,
		Pinned:                 req.Pinned,
		CurrentModel:           req.CurrentModel,
		ChatTemperature:        req.ChatTemperature,
		ChatContextCount:       req.ChatContextCount,
		WebSearchEnabled:       req.WebSearchEnabled,
		SystemPrompt:           req.SystemPrompt,
		SummaryEnabled:         req.SummaryEnabled,
		SummaryTriggerMessages: req.SummaryTriggerMessages,
		SummaryRecentMessages:  req.SummaryRecentMessages,
		ClearContextMessageId:  req.ClearContextMessageId,
		ClearContextToLatest:   req.ClearContextToLatest,
	})
	if err != nil {
		writeCanvasError(c, err)
		return
	}
	common.ApiSuccess(c, session)
}

func DeleteCanvasSession(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		common.ApiErrorMsg(c, "未授权")
		return
	}
	session, err := resolveCanvasSessionForController(c, userId)
	if err != nil {
		writeCanvasError(c, err)
		return
	}
	if session == nil {
		writeCanvasError(c, fmt.Errorf("canvas session not found"))
		return
	}

	if err := deleteCanvasSessionForController(userId, session); err != nil {
		writeCanvasError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func ListCanvasMessages(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		common.ApiErrorMsg(c, "未授权")
		return
	}
	session, err := resolveCanvasSessionForController(c, userId)
	if err != nil {
		writeCanvasError(c, err)
		return
	}
	if session == nil {
		writeCanvasError(c, fmt.Errorf("canvas session not found"))
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))
	messages, err := listCanvasMessageTimelineForSessionForController(userId, session, limit, c.Query("cursor"))
	if err != nil {
		writeCanvasError(c, err)
		return
	}
	common.ApiSuccess(c, messages)
}

func CreateCanvasMessage(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		common.ApiErrorMsg(c, "未授权")
		return
	}
	session, err := resolveCanvasSessionForController(c, userId)
	if err != nil {
		writeCanvasError(c, err)
		return
	}
	if session == nil {
		writeCanvasError(c, fmt.Errorf("canvas session not found"))
		return
	}

	var req createCanvasMessageRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}

	input := service.CreateCanvasMessageInput{
		Prompt:           req.Prompt,
		ModelId:          req.ModelId,
		Group:            req.Group,
		RequestEndpoint:  req.RequestEndpoint,
		Params:           req.Params,
		Attachments:      req.Attachments,
		WebSearchEnabled: req.WebSearchEnabled,
		Stream:           req.Stream,
		Temperature:      req.Temperature,
		ContextCount:     req.ContextCount,
		ClientRequestId:  req.ClientRequestId,
	}
	if req.Stream != nil && *req.Stream {
		if session.Mode == model.CanvasModeChat {
			if err := streamCanvasChatMessageForController(c, userId, session, input); err != nil {
				writeCanvasError(c, err)
			}
			return
		}
	}

	messages, err := createCanvasMessageForController(c.Request.Context(), userId, session, input)
	if err != nil {
		writeCanvasError(c, err)
		return
	}
	common.ApiSuccess(c, messages)
}

func parseCanvasSessionIdentifier(c *gin.Context) string {
	return strings.TrimSpace(c.Param("id"))
}

func resolveCanvasSessionForController(c *gin.Context, userId int) (*model.CanvasSession, error) {
	identifier := parseCanvasSessionIdentifier(c)
	if identifier == "" {
		return nil, fmt.Errorf("canvas session identifier is required")
	}
	return getCanvasSessionByIdentifierForController(userId, identifier)
}
