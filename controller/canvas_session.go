package controller

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
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
	SystemPrompt           *string  `json:"system_prompt"`
	SummaryEnabled         *bool    `json:"summary_enabled"`
	SummaryTriggerMessages *int     `json:"summary_trigger_messages"`
	SummaryRecentMessages  *int     `json:"summary_recent_messages"`
	ClearContextMessageId  *int     `json:"clear_context_message_id"`
	ClearContextToLatest   *bool    `json:"clear_context_to_latest"`
}

type createCanvasMessageRequest struct {
	Prompt          string   `json:"prompt"`
	ModelId         string   `json:"model_id"`
	Group           string   `json:"group"`
	RequestEndpoint string   `json:"request_endpoint"`
	Params          string   `json:"params"`
	Stream          *bool    `json:"stream"`
	Temperature     *float64 `json:"temperature"`
	ContextCount    *int     `json:"context_count"`
	ClientRequestId string   `json:"client_request_id"`
}

var (
	listCanvasSessionsForController      = service.ListCanvasSessions
	getCanvasSessionByIDForController    = model.GetCanvasSessionByID
	createCanvasMessageForController     = service.CreateCanvasMessageWithContext
	streamCanvasChatMessageForController = service.StreamCanvasChatMessage
)

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
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, sessions)
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
		SystemPrompt:           req.SystemPrompt,
		SummaryEnabled:         req.SummaryEnabled,
		SummaryTriggerMessages: req.SummaryTriggerMessages,
		SummaryRecentMessages:  req.SummaryRecentMessages,
	})
	if err != nil {
		common.ApiError(c, err)
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
	sessionId, err := parseCanvasSessionID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var req updateCanvasSessionRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}

	session, err := service.UpdateCanvasSession(userId, sessionId, service.UpdateCanvasSessionInput{
		Title:                  req.Title,
		Pinned:                 req.Pinned,
		CurrentModel:           req.CurrentModel,
		ChatTemperature:        req.ChatTemperature,
		ChatContextCount:       req.ChatContextCount,
		SystemPrompt:           req.SystemPrompt,
		SummaryEnabled:         req.SummaryEnabled,
		SummaryTriggerMessages: req.SummaryTriggerMessages,
		SummaryRecentMessages:  req.SummaryRecentMessages,
		ClearContextMessageId:  req.ClearContextMessageId,
		ClearContextToLatest:   req.ClearContextToLatest,
	})
	if err != nil {
		common.ApiError(c, err)
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
	sessionId, err := parseCanvasSessionID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	if err := service.DeleteCanvasSession(userId, sessionId); err != nil {
		common.ApiError(c, err)
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
	sessionId, err := parseCanvasSessionID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	messages, err := service.ListCanvasMessages(userId, sessionId)
	if err != nil {
		common.ApiError(c, err)
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
	sessionId, err := parseCanvasSessionID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var req createCanvasMessageRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiError(c, err)
		return
	}

	input := service.CreateCanvasMessageInput{
		Prompt:          req.Prompt,
		ModelId:         req.ModelId,
		Group:           req.Group,
		RequestEndpoint: req.RequestEndpoint,
		Params:          req.Params,
		Stream:          req.Stream,
		Temperature:     req.Temperature,
		ContextCount:    req.ContextCount,
		ClientRequestId: req.ClientRequestId,
	}
	if req.Stream != nil && *req.Stream {
		session, err := getCanvasSessionByIDForController(userId, sessionId)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if session == nil {
			common.ApiError(c, fmt.Errorf("canvas session not found"))
			return
		}
		if session.Mode == model.CanvasModeChat {
			if err := streamCanvasChatMessageForController(c, userId, sessionId, input); err != nil {
				common.ApiError(c, err)
			}
			return
		}
	}

	messages, err := createCanvasMessageForController(c.Request.Context(), userId, sessionId, input)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, messages)
}

func parseCanvasSessionID(c *gin.Context) (int, error) {
	return strconv.Atoi(strings.TrimSpace(c.Param("id")))
}
