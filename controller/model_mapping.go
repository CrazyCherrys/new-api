package controller

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func normalizeModelMappingEndpoint(endpoint string) string {
	switch strings.ToLower(strings.TrimSpace(endpoint)) {
	case "dalle":
		return "openai"
	case "openai-video-generations", "video-generation", "video-generations":
		return string(constant.EndpointTypeOpenAIVideoGeneration)
	case "openai-videos", "sora", "video":
		return string(constant.EndpointTypeOpenAIVideo)
	default:
		return strings.ToLower(strings.TrimSpace(endpoint))
	}
}

// validateModelMappingEndpoint 校验模型映射的请求端点
func validateModelMappingEndpoint(modelType int, endpoint string) error {
	if endpoint == "" {
		return errors.New("请求端点不能为空")
	}

	switch modelType {
	case 2:
		validEndpoints := []string{"openai", "openai-response", "gemini", "openai_mod"}
		for _, valid := range validEndpoints {
			if endpoint == valid {
				return nil
			}
		}
		return errors.New("请求端点必须是 openai、openai-response、gemini 或 openai_mod 之一")
	case 3:
		validEndpoints := []string{string(constant.EndpointTypeOpenAIVideoGeneration), string(constant.EndpointTypeOpenAIVideo)}
		for _, valid := range validEndpoints {
			if endpoint == valid {
				return nil
			}
		}
		return errors.New("请求端点必须是 openai-video-generation 或 openai-video 之一")
	default:
		return nil
	}
}

func defaultModelMappingEndpoint(modelType int) string {
	switch modelType {
	case 3:
		return string(constant.EndpointTypeOpenAIVideoGeneration)
	default:
		return "openai"
	}
}

func normalizeOrDefaultModelMappingEndpoint(modelType int, endpoint string) string {
	normalized := normalizeModelMappingEndpoint(endpoint)
	if normalized == "" {
		return defaultModelMappingEndpoint(modelType)
	}
	if err := validateModelMappingEndpoint(modelType, normalized); err == nil {
		return normalized
	}
	return defaultModelMappingEndpoint(modelType)
}

func validateImageModelCapabilities(modelType int, raw string) (string, error) {
	if modelType != 2 {
		return "", nil
	}

	normalized, err := model.NormalizeImageCapabilities(raw)
	if err != nil {
		return "", err
	}
	if normalized == "" {
		return "", errors.New("绘画模型必须至少选择一个模型能力")
	}
	return normalized, nil
}

// GetAllModelMappings 获取模型映射列表（分页）
func GetAllModelMappings(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	mappings, err := model.GetAllModelMappings(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var total int64
	model.DB.Model(&model.ModelMapping{}).Count(&total)

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(mappings)
	common.ApiSuccess(c, pageInfo)
}

// SearchModelMappings 搜索模型映射
func SearchModelMappings(c *gin.Context) {
	keyword := c.Query("keyword")
	modelTypeStr := c.Query("model_type")
	modelType := 0
	if modelTypeStr != "" {
		modelType, _ = strconv.Atoi(modelTypeStr)
	}

	pageInfo := common.GetPageQuery(c)
	mappings, total, err := model.SearchModelMappings(keyword, modelType, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(mappings)
	common.ApiSuccess(c, pageInfo)
}

// GetModelMapping 获取单个模型映射
func GetModelMapping(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	mapping, err := model.GetModelMapping(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, mapping)
}

// CreateModelMapping 创建模型映射
func CreateModelMapping(c *gin.Context) {
	var mm model.ModelMapping
	if err := c.ShouldBindJSON(&mm); err != nil {
		common.ApiError(c, err)
		return
	}

	if mm.RequestModel == "" {
		common.ApiErrorMsg(c, "请求模型ID不能为空")
		return
	}

	if mm.DisplayName == "" {
		common.ApiErrorMsg(c, "模型显示名称不能为空")
		return
	}

	mm.RequestEndpoint = normalizeOrDefaultModelMappingEndpoint(mm.ModelType, mm.RequestEndpoint)

	// 校验模型的请求端点
	if err := validateModelMappingEndpoint(mm.ModelType, mm.RequestEndpoint); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	normalizedCapabilities, err := validateImageModelCapabilities(mm.ModelType, mm.ImageCapabilities)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	mm.ImageCapabilities = normalizedCapabilities

	// 如果 ActualModel 为空，使用 RequestModel 作为默认值
	if mm.ActualModel == "" {
		mm.ActualModel = mm.RequestModel
	}

	// 检查是否已存在
	existing, _ := model.GetModelMappingByRequestModel(mm.RequestModel)
	if existing != nil {
		common.ApiErrorMsg(c, "该请求模型ID已存在")
		return
	}

	if err := mm.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, mm)
}

// UpdateModelMapping 更新模型映射
func UpdateModelMapping(c *gin.Context) {
	var mm model.ModelMapping
	if err := c.ShouldBindJSON(&mm); err != nil {
		common.ApiError(c, err)
		return
	}

	if mm.Id == 0 {
		common.ApiErrorMsg(c, "ID不能为空")
		return
	}

	if mm.RequestModel == "" {
		common.ApiErrorMsg(c, "请求模型ID不能为空")
		return
	}

	if mm.DisplayName == "" {
		common.ApiErrorMsg(c, "模型显示名称不能为空")
		return
	}

	mm.RequestEndpoint = normalizeOrDefaultModelMappingEndpoint(mm.ModelType, mm.RequestEndpoint)

	// 校验模型的请求端点
	if err := validateModelMappingEndpoint(mm.ModelType, mm.RequestEndpoint); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	normalizedCapabilities, err := validateImageModelCapabilities(mm.ModelType, mm.ImageCapabilities)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	mm.ImageCapabilities = normalizedCapabilities

	// 如果 ActualModel 为空，使用 RequestModel 作为默认值
	if mm.ActualModel == "" {
		mm.ActualModel = mm.RequestModel
	}

	if err := mm.Update(); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, mm)
}

// DeleteModelMapping 删除模型映射
func DeleteModelMapping(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	if err := model.DeleteModelMapping(id); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, nil)
}
