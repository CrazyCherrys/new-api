package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type ImageGenerationReferenceAsset struct {
	Id            int    `json:"id" gorm:"primaryKey"`
	ContentHash   string `json:"content_hash" gorm:"size:64;uniqueIndex;not null"`
	StorageType   string `json:"storage_type" gorm:"size:16;not null;default:'local'"`
	StoragePath   string `json:"storage_path" gorm:"type:text;not null"`
	ContentType   string `json:"content_type" gorm:"size:128;default:''"`
	FileSizeBytes int64  `json:"file_size_bytes" gorm:"bigint;default:0"`
	RefCount      int    `json:"ref_count" gorm:"default:0;index"`
	CreatedTime   int64  `json:"created_time" gorm:"bigint;index"`
	LastUsedTime  int64  `json:"last_used_time" gorm:"bigint;index"`
}

type ImageGenerationTaskReferenceAsset struct {
	Id          int   `json:"id" gorm:"primaryKey"`
	TaskId      int   `json:"task_id" gorm:"index:idx_img_task_ref_asset_task,priority:1;index"`
	AssetId     int   `json:"asset_id" gorm:"index:idx_img_task_ref_asset_asset,priority:1;index"`
	CreatedTime int64 `json:"created_time" gorm:"bigint;index"`
}

func (ImageGenerationTaskReferenceAsset) TableName() string {
	return "image_generation_task_reference_assets"
}

func GetImageGenerationReferenceAssetByHash(contentHash string) (*ImageGenerationReferenceAsset, error) {
	var asset ImageGenerationReferenceAsset
	err := DB.Where("content_hash = ?", contentHash).First(&asset).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &asset, nil
}

func GetImageGenerationReferenceAssetByID(id int) (*ImageGenerationReferenceAsset, error) {
	var asset ImageGenerationReferenceAsset
	err := DB.First(&asset, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &asset, nil
}

func CreateImageGenerationReferenceAsset(asset *ImageGenerationReferenceAsset) error {
	if asset == nil {
		return nil
	}
	now := common.GetTimestamp()
	if asset.CreatedTime == 0 {
		asset.CreatedTime = now
	}
	if asset.LastUsedTime == 0 {
		asset.LastUsedTime = now
	}
	return DB.Create(asset).Error
}

func TouchImageGenerationReferenceAsset(assetId int, ts int64) error {
	updates := map[string]interface{}{
		"last_used_time": ts,
	}
	return DB.Model(&ImageGenerationReferenceAsset{}).Where("id = ?", assetId).Updates(updates).Error
}

func IncrementImageGenerationReferenceAssetRefCount(assetId int, ts int64) error {
	return DB.Model(&ImageGenerationReferenceAsset{}).
		Where("id = ?", assetId).
		Updates(map[string]interface{}{
			"ref_count":      gorm.Expr("ref_count + ?", 1),
			"last_used_time": ts,
		}).Error
}

func DecrementImageGenerationReferenceAssetRefCount(assetId int) error {
	return DB.Model(&ImageGenerationReferenceAsset{}).
		Where("id = ? AND ref_count > 0", assetId).
		Update("ref_count", gorm.Expr("ref_count - ?", 1)).Error
}

func CreateTaskReferenceAssetLink(taskId int, assetId int) error {
	link := &ImageGenerationTaskReferenceAsset{
		TaskId:      taskId,
		AssetId:     assetId,
		CreatedTime: common.GetTimestamp(),
	}
	return DB.Create(link).Error
}

func ListTaskReferenceAssetLinks(taskId int) ([]*ImageGenerationTaskReferenceAsset, error) {
	var links []*ImageGenerationTaskReferenceAsset
	if err := DB.Where("task_id = ?", taskId).Find(&links).Error; err != nil {
		return nil, err
	}
	return links, nil
}

func DeleteTaskReferenceAssetLinks(taskId int) error {
	return DB.Where("task_id = ?", taskId).Delete(&ImageGenerationTaskReferenceAsset{}).Error
}

func DeleteTaskReferenceAssetLink(taskId int, assetId int) error {
	return DB.Where("task_id = ? AND asset_id = ?", taskId, assetId).Delete(&ImageGenerationTaskReferenceAsset{}).Error
}

func DeleteImageGenerationReferenceAsset(assetId int) error {
	return DB.Delete(&ImageGenerationReferenceAsset{}, assetId).Error
}

func ListExpiredUnusedReferenceAssets(expirationTime int64) ([]*ImageGenerationReferenceAsset, error) {
	var assets []*ImageGenerationReferenceAsset
	err := DB.Where("ref_count <= 0 AND last_used_time > 0 AND last_used_time < ?", expirationTime).
		Find(&assets).Error
	if err != nil {
		return nil, err
	}
	return assets, nil
}
