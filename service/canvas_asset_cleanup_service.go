package service

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/worker_setting"
)

type canvasImageAssetCleanupPayload struct {
	TaskId       int    `json:"task_id"`
	ImageURL     string `json:"image_url"`
	ThumbnailURL string `json:"thumbnail_url"`
	Params       string `json:"params"`
}

type canvasVideoAssetCleanupPayload struct {
	TaskId        int64  `json:"task_id"`
	RequestParams string `json:"request_params"`
	ResultURL     string `json:"result_url"`
	ThumbnailURL  string `json:"thumbnail_url"`
}

const (
	canvasAssetCleanupTickInterval = 30 * time.Second
	canvasAssetCleanupBatchSize    = 50
	canvasAssetCleanupLeaseTTL     = 2 * time.Minute
	canvasAssetCleanupRetryDelay   = 1 * time.Minute
)

var (
	canvasAssetCleanupTaskOnce    sync.Once
	canvasAssetCleanupTaskRunning atomic.Bool
	handleCanvasAssetCleanupJobFn = handleCanvasAssetCleanupJob
)

func buildCanvasAssetCleanupJobs(imageTasks []*model.ImageGenerationTask, videoTasks []*model.Task) ([]*model.CanvasAssetCleanupJob, error) {
	jobs := make([]*model.CanvasAssetCleanupJob, 0, len(imageTasks)+len(videoTasks))

	for _, task := range imageTasks {
		if task == nil || task.Id <= 0 {
			continue
		}
		payloadBytes, err := common.Marshal(canvasImageAssetCleanupPayload{
			TaskId:       task.Id,
			ImageURL:     strings.TrimSpace(task.ImageUrl),
			ThumbnailURL: strings.TrimSpace(task.ThumbnailUrl),
			Params:       task.Params,
		})
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, &model.CanvasAssetCleanupJob{
			UserId:       task.UserId,
			TaskType:     model.CanvasTaskTypeImage,
			TaskRecordId: strconv.Itoa(task.Id),
			Payload:      string(payloadBytes),
		})
	}

	for _, task := range videoTasks {
		if task == nil || task.ID <= 0 {
			continue
		}
		payloadBytes, err := common.Marshal(canvasVideoAssetCleanupPayload{
			TaskId:        task.ID,
			RequestParams: task.Properties.RequestParams,
			ResultURL:     strings.TrimSpace(task.GetResultURL()),
			ThumbnailURL:  strings.TrimSpace(model.ExtractTaskThumbnailURL(task)),
		})
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, &model.CanvasAssetCleanupJob{
			UserId:       task.UserId,
			TaskType:     model.CanvasTaskTypeVideo,
			TaskRecordId: strconv.FormatInt(task.ID, 10),
			Payload:      string(payloadBytes),
		})
	}

	return jobs, nil
}

func StartCanvasAssetCleanupTask() {
	canvasAssetCleanupTaskOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}

		common.SysLog(fmt.Sprintf("canvas asset cleanup worker started: tick=%s batch=%d", canvasAssetCleanupTickInterval, canvasAssetCleanupBatchSize))
		go func() {
			ticker := time.NewTicker(canvasAssetCleanupTickInterval)
			defer ticker.Stop()

			runCanvasAssetCleanupJobsOnce()
			for range ticker.C {
				runCanvasAssetCleanupJobsOnce()
			}
		}()
	})
}

func runCanvasAssetCleanupJobsOnce() {
	if !canvasAssetCleanupTaskRunning.CompareAndSwap(false, true) {
		return
	}
	defer canvasAssetCleanupTaskRunning.Store(false)

	now := common.GetTimestamp()
	jobs, err := model.ListClaimableCanvasAssetCleanupJobs(canvasAssetCleanupBatchSize, now)
	if err != nil {
		common.SysLog(fmt.Sprintf("Failed to list canvas asset cleanup jobs: %v", err))
		return
	}
	for _, job := range jobs {
		if job == nil || job.Id <= 0 {
			continue
		}
		claimed, err := model.ClaimCanvasAssetCleanupJob(job.TaskType, job.Id, now, now+int64(canvasAssetCleanupLeaseTTL/time.Second))
		if err != nil {
			common.SysLog(fmt.Sprintf("Failed to claim canvas asset cleanup job %d: %v", job.Id, err))
			continue
		}
		if !claimed {
			continue
		}
		if err := handleCanvasAssetCleanupJobFn(job); err != nil {
			retryCount := job.RetryCount + 1
			nextRunTime := common.GetTimestamp() + int64(canvasAssetCleanupRetryDelay/time.Second)
			if markErr := model.MarkCanvasAssetCleanupJobFailed(job.TaskType, job.Id, retryCount, nextRunTime, err.Error()); markErr != nil {
				common.SysLog(fmt.Sprintf("Failed to mark canvas asset cleanup job %d failed: %v", job.Id, markErr))
			}
			continue
		}
		if err := model.DeleteCanvasAssetCleanupJob(job.TaskType, job.Id); err != nil {
			common.SysLog(fmt.Sprintf("Failed to delete completed canvas asset cleanup job %d: %v", job.Id, err))
		}
	}
}

func handleCanvasAssetCleanupJob(job *model.CanvasAssetCleanupJob) error {
	if job == nil {
		return nil
	}
	switch job.TaskType {
	case model.CanvasTaskTypeImage:
		var payload canvasImageAssetCleanupPayload
		if err := common.UnmarshalJsonStr(job.Payload, &payload); err != nil {
			return err
		}
		return cleanupCanvasImageAssets(payload)
	case model.CanvasTaskTypeVideo:
		var payload canvasVideoAssetCleanupPayload
		if err := common.UnmarshalJsonStr(job.Payload, &payload); err != nil {
			return err
		}
		return cleanupCanvasVideoAssets(payload)
	default:
		return fmt.Errorf("unsupported canvas asset cleanup task type: %s", job.TaskType)
	}
}

func cleanupCanvasImageAssets(payload canvasImageAssetCleanupPayload) error {
	cfg := worker_setting.GetWorkerSetting()
	if cfg == nil {
		return nil
	}
	if strings.TrimSpace(payload.ImageURL) != "" {
		if err := deleteImageFileByKind(payload.ImageURL, cfg, imageGenerationAssetKindResult); err != nil {
			return err
		}
	}
	if strings.TrimSpace(payload.ThumbnailURL) != "" && payload.ThumbnailURL != payload.ImageURL {
		if err := deleteImageFileByKind(payload.ThumbnailURL, cfg, imageGenerationAssetKindResult); err != nil {
			return err
		}
	}
	refs, err := collectStoredReferenceImages(payload.Params)
	if err != nil {
		return err
	}
	return releaseTaskReferenceAssets(payload.TaskId, refs, cfg, false)
}

func cleanupCanvasVideoAssets(payload canvasVideoAssetCleanupPayload) error {
	cfg := worker_setting.GetWorkerSetting()
	if cfg == nil {
		return nil
	}
	if strings.TrimSpace(payload.RequestParams) != "" {
		var req relaycommon.TaskSubmitReq
		if err := common.UnmarshalJsonStr(payload.RequestParams, &req); err == nil {
			imageURL := strings.TrimSpace(req.Image)
			if imageURL == "" {
				imageURL = strings.TrimSpace(req.InputReference)
			}
			if imageURL != "" {
				if err := deleteImageFile(imageURL, cfg); err != nil {
					return err
				}
			}
		}
	}
	if resultURL := strings.TrimSpace(payload.ResultURL); resultURL != "" && isImageGenerationStoredAssetURL(resultURL) {
		if err := deleteImageFile(resultURL, cfg); err != nil {
			return err
		}
	}
	if thumbnailURL := strings.TrimSpace(payload.ThumbnailURL); thumbnailURL != "" && thumbnailURL != strings.TrimSpace(payload.ResultURL) && isImageGenerationStoredAssetURL(thumbnailURL) {
		if err := deleteImageFile(thumbnailURL, cfg); err != nil {
			return err
		}
	}
	return nil
}
