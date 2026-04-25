package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// ModelMapping 模型映射配置
type ModelMapping struct {
	Id              int    `json:"id"`
	RequestModel    string `json:"request_model" gorm:"size:128;not null;uniqueIndex:uk_request_model"`
	ActualModel     string `json:"actual_model" gorm:"size:128;not null"`
	DisplayName     string `json:"display_name" gorm:"size:255;not null"`
	ModelSeries     string `json:"model_series" gorm:"size:64;default:'';index"`
	ModelType       int    `json:"model_type" gorm:"default:1;index"` // 1=对话 2=绘画 3=视频 4=音频
	Description     string `json:"description" gorm:"type:text"`
	Status          int    `json:"status" gorm:"default:1;index"`
	Priority        int    `json:"priority" gorm:"default:0"`
	RequestEndpoint string `json:"request_endpoint" gorm:"size:32;default:''"` // openai, gemini, dalle
	Resolutions     string `json:"resolutions" gorm:"type:text"`                // JSON array: ["1K","2K","4K"]
	AspectRatios    string `json:"aspect_ratios" gorm:"type:text"`              // JSON array: ["1:1","16:9",...]
	CreatedTime     int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime     int64  `json:"updated_time" gorm:"bigint"`
}

func (mm *ModelMapping) Insert() error {
	now := common.GetTimestamp()
	mm.CreatedTime = now
	mm.UpdatedTime = now
	return DB.Create(mm).Error
}

func (mm *ModelMapping) Update() error {
	mm.UpdatedTime = common.GetTimestamp()
	// Use Select to explicitly update all fields including zero values
	return DB.Model(mm).Select("*").Updates(mm).Error
}

func GetModelMapping(id int) (*ModelMapping, error) {
	var mm ModelMapping
	err := DB.First(&mm, id).Error
	return &mm, err
}

func GetAllModelMappings(startIdx int, num int) ([]*ModelMapping, error) {
	var mappings []*ModelMapping
	err := DB.Order("priority DESC, id DESC").Limit(num).Offset(startIdx).Find(&mappings).Error
	return mappings, err
}

func SearchModelMappings(keyword string, modelType int, startIdx int, num int) ([]*ModelMapping, int64, error) {
	var mappings []*ModelMapping
	var total int64

	query := DB.Model(&ModelMapping{})

	// 只返回启用状态的模型
	query = query.Where("status = ?", 1)

	if keyword != "" {
		query = query.Where("request_model LIKE ? OR display_name LIKE ? OR model_series LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}

	if modelType > 0 {
		query = query.Where("model_type = ?", modelType)
	}

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Order("priority DESC, id DESC").Limit(num).Offset(startIdx).Find(&mappings).Error
	return mappings, total, err
}

func DeleteModelMapping(id int) error {
	return DB.Delete(&ModelMapping{}, id).Error
}

func GetModelMappingByRequestModel(requestModel string) (*ModelMapping, error) {
	var mm ModelMapping
	err := DB.Where("request_model = ? AND status = 1", requestModel).First(&mm).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &mm, err
}
