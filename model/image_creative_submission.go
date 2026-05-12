package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/samber/hot"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ImageCreativeSubmission records user-submitted image assets awaiting or passing review.
type ImageCreativeSubmission struct {
	Id            int    `json:"id" gorm:"primaryKey"`
	TaskId        int    `json:"task_id" gorm:"uniqueIndex;not null"`
	UserId        int    `json:"user_id" gorm:"index;not null"`
	Status        string `json:"status" gorm:"size:20;not null;index;index:idx_image_creative_public_order,priority:1;default:'pending'"`
	SubmittedTime int64  `json:"submitted_time" gorm:"bigint;index;index:idx_image_creative_public_order,priority:3"`
	ReviewedTime  int64  `json:"reviewed_time" gorm:"bigint;index:idx_image_creative_public_order,priority:2"`
	ReviewerId    int    `json:"reviewer_id" gorm:"index"`
	RejectReason  string `json:"reject_reason" gorm:"type:text"`
}

const (
	CreativeSubmissionStatusPending  = "pending"
	CreativeSubmissionStatusApproved = "approved"
	CreativeSubmissionStatusRejected = "rejected"
)

type ImageCreativeAsset struct {
	Id           int    `json:"id"`
	ModelId      string `json:"model_id"`
	DisplayName  string `json:"display_name"`
	ModelSeries  string `json:"model_series"`
	Prompt       string `json:"prompt"`
	Params       string `json:"params"`
	ImageUrl     string `json:"image_url"`
	ThumbnailUrl string `json:"thumbnail_url"`
}

type ImageCreativeAdminSubmission struct {
	Id              int    `json:"id"`
	SubmissionId    int    `json:"submission_id"`
	TaskId          int    `json:"task_id"`
	UserId          int    `json:"user_id"`
	Username        string `json:"username"`
	UserName        string `json:"user_name"`
	UserDisplayName string `json:"user_display_name"`
	Status          string `json:"status"`
	RejectReason    string `json:"reject_reason"`
	ReviewerId      int    `json:"reviewer_id"`
	SubmittedTime   int64  `json:"submitted_time"`
	ReviewedTime    int64  `json:"reviewed_time"`
	ModelId         string `json:"model_id"`
	DisplayName     string `json:"display_name"`
	ModelSeries     string `json:"model_series"`
	Prompt          string `json:"prompt"`
	Params          string `json:"params"`
	ImageUrl        string `json:"image_url"`
	ImageMetadata   string `json:"image_metadata"`
	CreatedTime     int64  `json:"created_time"`
	CompletedTime   int64  `json:"completed_time"`
}

type InspirationAssetPage struct {
	Items []*ImageCreativeAsset `json:"items"`
	Total int64                 `json:"total"`
}

const (
	inspirationAssetListCacheNamespace   = "new-api:inspiration_assets:v1"
	inspirationAssetDetailCacheNamespace = "new-api:inspiration_asset:v1"
)

var (
	inspirationAssetListCacheOnce   sync.Once
	inspirationAssetDetailCacheOnce sync.Once

	inspirationAssetListCache   *cachex.HybridCache[InspirationAssetPage]
	inspirationAssetDetailCache *cachex.HybridCache[ImageCreativeAsset]
)

func inspirationAssetListCacheTTL() time.Duration {
	ttlSeconds := common.GetEnvOrDefault("INSPIRATION_ASSET_LIST_CACHE_TTL", 60)
	if ttlSeconds <= 0 {
		ttlSeconds = 60
	}
	return time.Duration(ttlSeconds) * time.Second
}

func inspirationAssetDetailCacheTTL() time.Duration {
	ttlSeconds := common.GetEnvOrDefault("INSPIRATION_ASSET_DETAIL_CACHE_TTL", 300)
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}
	return time.Duration(ttlSeconds) * time.Second
}

func inspirationAssetListCacheCapacity() int {
	capacity := common.GetEnvOrDefault("INSPIRATION_ASSET_LIST_CACHE_CAP", 200)
	if capacity <= 0 {
		capacity = 200
	}
	return capacity
}

func inspirationAssetDetailCacheCapacity() int {
	capacity := common.GetEnvOrDefault("INSPIRATION_ASSET_DETAIL_CACHE_CAP", 2000)
	if capacity <= 0 {
		capacity = 2000
	}
	return capacity
}

func getInspirationAssetListCache() *cachex.HybridCache[InspirationAssetPage] {
	inspirationAssetListCacheOnce.Do(func() {
		ttl := inspirationAssetListCacheTTL()
		inspirationAssetListCache = cachex.NewHybridCache[InspirationAssetPage](cachex.HybridCacheConfig[InspirationAssetPage]{
			Namespace: cachex.Namespace(inspirationAssetListCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[InspirationAssetPage]{},
			Memory: func() *hot.HotCache[string, InspirationAssetPage] {
				return hot.NewHotCache[string, InspirationAssetPage](hot.LRU, inspirationAssetListCacheCapacity()).
					WithTTL(ttl).
					WithJanitor().
					Build()
			},
		})
	})
	return inspirationAssetListCache
}

func getInspirationAssetDetailCache() *cachex.HybridCache[ImageCreativeAsset] {
	inspirationAssetDetailCacheOnce.Do(func() {
		ttl := inspirationAssetDetailCacheTTL()
		inspirationAssetDetailCache = cachex.NewHybridCache[ImageCreativeAsset](cachex.HybridCacheConfig[ImageCreativeAsset]{
			Namespace: cachex.Namespace(inspirationAssetDetailCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[ImageCreativeAsset]{},
			Memory: func() *hot.HotCache[string, ImageCreativeAsset] {
				return hot.NewHotCache[string, ImageCreativeAsset](hot.LRU, inspirationAssetDetailCacheCapacity()).
					WithTTL(ttl).
					WithJanitor().
					Build()
			},
		})
	})
	return inspirationAssetDetailCache
}

func inspirationAssetListCacheKey(startIdx int, num int) string {
	if startIdx < 0 || num <= 0 {
		return ""
	}
	return fmt.Sprintf("%d:%d", startIdx, num)
}

func inspirationAssetDetailCacheKey(id int) string {
	if id <= 0 {
		return ""
	}
	return strconv.Itoa(id)
}

func cloneImageCreativeAsset(asset *ImageCreativeAsset) *ImageCreativeAsset {
	if asset == nil {
		return nil
	}
	next := *asset
	return &next
}

func cloneImageCreativeAssets(assets []*ImageCreativeAsset) []*ImageCreativeAsset {
	if len(assets) == 0 {
		return assets
	}
	next := make([]*ImageCreativeAsset, 0, len(assets))
	for _, asset := range assets {
		next = append(next, cloneImageCreativeAsset(asset))
	}
	return next
}

func InvalidateInspirationAssetCache() {
	_ = getInspirationAssetListCache().Purge()
	_ = getInspirationAssetDetailCache().Purge()
}

func isValidCreativeSubmissionStatus(status string) bool {
	switch status {
	case CreativeSubmissionStatusPending, CreativeSubmissionStatusApproved, CreativeSubmissionStatusRejected:
		return true
	default:
		return false
	}
}

func isValidCreativeReviewStatus(status string) bool {
	switch status {
	case CreativeSubmissionStatusApproved, CreativeSubmissionStatusRejected:
		return true
	default:
		return false
	}
}

func GetImageInspirationSubmissionByTaskID(taskId int) (*ImageCreativeSubmission, error) {
	var submission ImageCreativeSubmission
	err := DB.Where("task_id = ?", taskId).First(&submission).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &submission, err
}

func GetImageCreativeSubmissionByTaskID(taskId int) (*ImageCreativeSubmission, error) {
	return GetImageInspirationSubmissionByTaskID(taskId)
}

func SubmitImageAssetToInspiration(userId int, taskId int) (*ImageCreativeSubmission, error) {
	asset, err := GetImageAssetByID(userId, taskId)
	if err != nil {
		return nil, err
	}
	if asset == nil {
		return nil, errors.New("资产不存在或不可提交")
	}

	submission := &ImageCreativeSubmission{
		TaskId:        taskId,
		UserId:        userId,
		Status:        CreativeSubmissionStatusPending,
		SubmittedTime: common.GetTimestamp(),
	}
	result := DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "task_id"}},
		DoNothing: true,
	}).Create(submission)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		existing, err := GetImageInspirationSubmissionByTaskID(taskId)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
		return nil, errors.New("投稿已存在")
	}
	if submission.Id == 0 {
		existing, err := GetImageInspirationSubmissionByTaskID(taskId)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
		return nil, errors.New("投稿创建失败")
	}
	return submission, nil
}

func SubmitImageAssetToCreativeSpace(userId int, taskId int) (*ImageCreativeSubmission, error) {
	return SubmitImageAssetToInspiration(userId, taskId)
}

func publicInspirationAssetsBaseQuery() *gorm.DB {
	return DB.Table("image_creative_submissions AS s").
		Select("s.id, t.model_id, COALESCE(m.display_name, '') AS display_name, COALESCE(m.model_series, '') AS model_series, t.prompt, t.params, t.image_url, t.thumbnail_url").
		Joins("JOIN image_generation_tasks AS t ON t.id = s.task_id").
		Joins("LEFT JOIN model_mappings AS m ON m.request_model = t.model_id").
		Where("s.status = ? AND t.status = ? AND t.image_url <> ?", CreativeSubmissionStatusApproved, ImageTaskStatusSuccess, "")
}

func GetApprovedInspirationAssets(startIdx int, num int) ([]*ImageCreativeAsset, int64, error) {
	cacheKey := inspirationAssetListCacheKey(startIdx, num)
	if cacheKey != "" {
		cached, ok, err := getInspirationAssetListCache().Get(cacheKey)
		if err == nil && ok {
			return cloneImageCreativeAssets(cached.Items), cached.Total, nil
		}
	}

	var assets []*ImageCreativeAsset
	var total int64

	if err := publicInspirationAssetsBaseQuery().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := publicInspirationAssetsBaseQuery().
		Order("s.reviewed_time DESC, s.submitted_time DESC, s.id DESC").
		Limit(num).
		Offset(startIdx).
		Scan(&assets).Error; err != nil {
		return nil, 0, err
	}

	if cacheKey != "" {
		_ = getInspirationAssetListCache().SetWithTTL(cacheKey, InspirationAssetPage{
			Items: cloneImageCreativeAssets(assets),
			Total: total,
		}, inspirationAssetListCacheTTL())
	}

	return cloneImageCreativeAssets(assets), total, nil
}

func GetApprovedCreativeAssets(startIdx int, num int) ([]*ImageCreativeAsset, int64, error) {
	return GetApprovedInspirationAssets(startIdx, num)
}

func GetApprovedInspirationAssetByID(id int) (*ImageCreativeAsset, error) {
	cacheKey := inspirationAssetDetailCacheKey(id)
	if cacheKey != "" {
		cached, ok, err := getInspirationAssetDetailCache().Get(cacheKey)
		if err == nil && ok {
			return cloneImageCreativeAsset(&cached), nil
		}
	}

	var asset ImageCreativeAsset
	err := publicInspirationAssetsBaseQuery().Where("s.id = ?", id).Scan(&asset).Error
	if err != nil {
		return nil, err
	}
	if asset.Id == 0 {
		return nil, nil
	}
	if cacheKey != "" {
		_ = getInspirationAssetDetailCache().SetWithTTL(cacheKey, asset, inspirationAssetDetailCacheTTL())
	}
	return cloneImageCreativeAsset(&asset), nil
}

func GetApprovedCreativeAssetByID(id int) (*ImageCreativeAsset, error) {
	return GetApprovedInspirationAssetByID(id)
}

func adminInspirationSubmissionsBaseQuery() *gorm.DB {
	return DB.Table("image_creative_submissions AS s").
		Select("s.id, s.id AS submission_id, s.task_id, s.user_id, COALESCE(u.username, '') AS username, COALESCE(u.username, '') AS user_name, COALESCE(u.display_name, '') AS user_display_name, s.status, s.reject_reason, s.reviewer_id, s.submitted_time, s.reviewed_time, t.model_id, COALESCE(m.display_name, '') AS display_name, COALESCE(m.model_series, '') AS model_series, t.prompt, t.params, t.image_url, t.image_metadata, t.created_time, t.completed_time").
		Joins("JOIN image_generation_tasks AS t ON t.id = s.task_id").
		Joins("LEFT JOIN model_mappings AS m ON m.request_model = t.model_id").
		Joins("LEFT JOIN users AS u ON u.id = s.user_id")
}

func GetImageInspirationSubmissions(startIdx int, num int, status string) ([]*ImageCreativeAdminSubmission, int64, error) {
	status = strings.TrimSpace(status)
	if status == "" {
		status = CreativeSubmissionStatusPending
	}
	if !isValidCreativeSubmissionStatus(status) {
		return nil, 0, errors.New("无效的审核状态")
	}

	query := adminInspirationSubmissionsBaseQuery().Where("s.status = ?", status)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var submissions []*ImageCreativeAdminSubmission
	if err := query.
		Order("s.submitted_time DESC, s.id DESC").
		Limit(num).
		Offset(startIdx).
		Scan(&submissions).Error; err != nil {
		return nil, 0, err
	}

	return submissions, total, nil
}

func GetImageCreativeSubmissions(startIdx int, num int, status string) ([]*ImageCreativeAdminSubmission, int64, error) {
	return GetImageInspirationSubmissions(startIdx, num, status)
}

func GetImageInspirationAdminSubmissionByID(id int) (*ImageCreativeAdminSubmission, error) {
	var submission ImageCreativeAdminSubmission
	err := adminInspirationSubmissionsBaseQuery().Where("s.id = ?", id).Scan(&submission).Error
	if err != nil {
		return nil, err
	}
	if submission.Id == 0 {
		return nil, nil
	}
	return &submission, nil
}

func GetImageCreativeAdminSubmissionByID(id int) (*ImageCreativeAdminSubmission, error) {
	return GetImageInspirationAdminSubmissionByID(id)
}

func ReviewImageInspirationSubmission(id int, reviewerId int, status string, rejectReason string) (*ImageCreativeAdminSubmission, error) {
	status = strings.TrimSpace(status)
	if !isValidCreativeReviewStatus(status) {
		return nil, errors.New("审核状态必须为 approved 或 rejected")
	}

	rejectReason = strings.TrimSpace(rejectReason)
	if status == CreativeSubmissionStatusApproved {
		rejectReason = ""
	}

	updates := map[string]interface{}{
		"status":        status,
		"reviewed_time": common.GetTimestamp(),
		"reviewer_id":   reviewerId,
		"reject_reason": rejectReason,
	}
	result := DB.Model(&ImageCreativeSubmission{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, errors.New("投稿不存在")
	}

	InvalidateInspirationAssetCache()
	return GetImageInspirationAdminSubmissionByID(id)
}

func ReviewImageCreativeSubmission(id int, reviewerId int, status string, rejectReason string) (*ImageCreativeAdminSubmission, error) {
	return ReviewImageInspirationSubmission(id, reviewerId, status, rejectReason)
}

func DeleteImageInspirationSubmission(id int) error {
	result := DB.Delete(&ImageCreativeSubmission{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("投稿不存在")
	}
	InvalidateInspirationAssetCache()
	return nil
}

func DeleteImageCreativeSubmission(id int) error {
	return DeleteImageInspirationSubmission(id)
}
