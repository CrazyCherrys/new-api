package controller

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func normalizeImageModelEndpoint(endpoint string) string {
	switch strings.ToLower(strings.TrimSpace(endpoint)) {
	case "dalle":
		return "openai"
	default:
		return strings.ToLower(strings.TrimSpace(endpoint))
	}
}

// validateImageModelEndpoint 校验绘画模型的请求端点
func validateImageModelEndpoint(modelType int, endpoint string) error {
	if modelType == 2 {
		if endpoint == "" {
			return errors.New("绘画模型必须指定请求端点")
		}
		validEndpoints := []string{"openai", "gemini", "openai_mod"}
		for _, valid := range validEndpoints {
			if endpoint == valid {
				return nil
			}
		}
		return errors.New("请求端点必须是 openai、gemini 或 openai_mod 之一")
	}
	return nil
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

	if mm.RequestEndpoint == "" {
		common.ApiErrorMsg(c, "请求端点不能为空")
		return
	}
	mm.RequestEndpoint = normalizeImageModelEndpoint(mm.RequestEndpoint)

	// 校验绘画模型的请求端点
	if err := validateImageModelEndpoint(mm.ModelType, mm.RequestEndpoint); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

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

	if mm.RequestEndpoint == "" {
		common.ApiErrorMsg(c, "请求端点不能为空")
		return
	}
	mm.RequestEndpoint = normalizeImageModelEndpoint(mm.RequestEndpoint)

	// 校验绘画模型的请求端点
	if err := validateImageModelEndpoint(mm.ModelType, mm.RequestEndpoint); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

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
