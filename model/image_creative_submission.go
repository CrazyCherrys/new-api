package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ImageCreativeSubmission records user-submitted image assets awaiting or passing review.
type ImageCreativeSubmission struct {
	Id            int    `json:"id" gorm:"primaryKey"`
	TaskId        int    `json:"task_id" gorm:"uniqueIndex;not null"`
	UserId        int    `json:"user_id" gorm:"index;not null"`
	Status        string `json:"status" gorm:"size:20;not null;index;default:'pending'"`
	SubmittedTime int64  `json:"submitted_time" gorm:"bigint;index"`
	ReviewedTime  int64  `json:"reviewed_time" gorm:"bigint"`
	ReviewerId    int    `json:"reviewer_id" gorm:"index"`
	RejectReason  string `json:"reject_reason" gorm:"type:text"`
}

const (
	CreativeSubmissionStatusPending  = "pending"
	CreativeSubmissionStatusApproved = "approved"
	CreativeSubmissionStatusRejected = "rejected"
)

type ImageCreativeAsset struct {
	Id          int    `json:"id"`
	ModelId     string `json:"model_id"`
	DisplayName string `json:"display_name"`
	ModelSeries string `json:"model_series"`
	Prompt      string `json:"prompt"`
	Params      string `json:"params"`
	ImageUrl    string `json:"image_url"`
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

func GetImageCreativeSubmissionByTaskID(taskId int) (*ImageCreativeSubmission, error) {
	var submission ImageCreativeSubmission
	err := DB.Where("task_id = ?", taskId).First(&submission).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &submission, err
}

func SubmitImageAssetToCreativeSpace(userId int, taskId int) (*ImageCreativeSubmission, error) {
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
		existing, err := GetImageCreativeSubmissionByTaskID(taskId)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
		return nil, errors.New("投稿已存在")
	}
	if submission.Id == 0 {
		existing, err := GetImageCreativeSubmissionByTaskID(taskId)
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

func publicCreativeAssetsBaseQuery() *gorm.DB {
	return DB.Table("image_creative_submissions AS s").
		Select("s.id, t.model_id, COALESCE(m.display_name, '') AS display_name, COALESCE(m.model_series, '') AS model_series, t.prompt, t.params, t.image_url").
		Joins("JOIN image_generation_tasks AS t ON t.id = s.task_id").
		Joins("LEFT JOIN model_mappings AS m ON m.request_model = t.model_id").
		Where("s.status = ? AND t.status = ? AND t.image_url <> ?", CreativeSubmissionStatusApproved, ImageTaskStatusSuccess, "")
}

func GetApprovedCreativeAssets(startIdx int, num int) ([]*ImageCreativeAsset, int64, error) {
	var assets []*ImageCreativeAsset
	var total int64

	if err := publicCreativeAssetsBaseQuery().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := publicCreativeAssetsBaseQuery().
		Order("s.reviewed_time DESC, s.submitted_time DESC, s.id DESC").
		Limit(num).
		Offset(startIdx).
		Scan(&assets).Error; err != nil {
		return nil, 0, err
	}

	return assets, total, nil
}

func GetApprovedCreativeAssetByID(id int) (*ImageCreativeAsset, error) {
	var asset ImageCreativeAsset
	err := publicCreativeAssetsBaseQuery().Where("s.id = ?", id).Scan(&asset).Error
	if err != nil {
		return nil, err
	}
	if asset.Id == 0 {
		return nil, nil
	}
	return &asset, nil
}

func adminCreativeSubmissionsBaseQuery() *gorm.DB {
	return DB.Table("image_creative_submissions AS s").
		Select("s.id, s.id AS submission_id, s.task_id, s.user_id, COALESCE(u.username, '') AS username, COALESCE(u.username, '') AS user_name, COALESCE(u.display_name, '') AS user_display_name, s.status, s.reject_reason, s.reviewer_id, s.submitted_time, s.reviewed_time, t.model_id, COALESCE(m.display_name, '') AS display_name, COALESCE(m.model_series, '') AS model_series, t.prompt, t.params, t.image_url, t.image_metadata, t.created_time, t.completed_time").
		Joins("JOIN image_generation_tasks AS t ON t.id = s.task_id").
		Joins("LEFT JOIN model_mappings AS m ON m.request_model = t.model_id").
		Joins("LEFT JOIN users AS u ON u.id = s.user_id")
}

func GetImageCreativeSubmissions(startIdx int, num int, status string) ([]*ImageCreativeAdminSubmission, int64, error) {
	status = strings.TrimSpace(status)
	if status == "" {
		status = CreativeSubmissionStatusPending
	}
	if !isValidCreativeSubmissionStatus(status) {
		return nil, 0, errors.New("无效的审核状态")
	}

	query := adminCreativeSubmissionsBaseQuery().Where("s.status = ?", status)
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

func GetImageCreativeAdminSubmissionByID(id int) (*ImageCreativeAdminSubmission, error) {
	var submission ImageCreativeAdminSubmission
	err := adminCreativeSubmissionsBaseQuery().Where("s.id = ?", id).Scan(&submission).Error
	if err != nil {
		return nil, err
	}
	if submission.Id == 0 {
		return nil, nil
	}
	return &submission, nil
}

func ReviewImageCreativeSubmission(id int, reviewerId int, status string, rejectReason string) (*ImageCreativeAdminSubmission, error) {
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

	return GetImageCreativeAdminSubmissionByID(id)
}
