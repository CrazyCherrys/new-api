package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type createCanvasSessionRequest struct {
	Mode  string `json:"mode"`
	Title string `json:"title"`
}

type updateCanvasSessionRequest struct {
	Title  *string `json:"title"`
	Pinned *bool   `json:"pinned"`
}

type createCanvasMessageRequest struct {
	Prompt          string `json:"prompt"`
	ModelId         string `json:"model_id"`
	Group           string `json:"group"`
	RequestEndpoint string `json:"request_endpoint"`
	Params          string `json:"params"`
}

func ListCanvasSessions(c *gin.Context) {
	userId := c.GetInt("id")
	if userId == 0 {
		common.ApiErrorMsg(c, "未授权")
		return
	}

	sessions, err := service.ListCanvasSessions(userId, c.Query("mode"))
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
		Mode:  req.Mode,
		Title: req.Title,
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
		Title:  req.Title,
		Pinned: req.Pinned,
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

	messages, err := service.CreateCanvasMessage(userId, sessionId, service.CreateCanvasMessageInput{
		Prompt:          req.Prompt,
		ModelId:         req.ModelId,
		Group:           req.Group,
		RequestEndpoint: req.RequestEndpoint,
		Params:          req.Params,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, messages)
}

func parseCanvasSessionID(c *gin.Context) (int, error) {
	return strconv.Atoi(strings.TrimSpace(c.Param("id")))
}
