package service

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

func TestBuildVideoTaskSummaryAndDetailUseEffectiveResultURL(t *testing.T) {
	setupCanvasSessionServiceTestDB(t)

	payload, err := common.Marshal(map[string]any{
		"content": map[string]any{
			"video_url": "https://cdn.example.com/detail.mp4",
		},
	})
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	task := &model.Task{
		UserId:     1,
		TaskID:     "task_video_detail",
		Action:     constant.TaskActionTextGenerate,
		Status:     model.TaskStatusSuccess,
		Progress:   "100%",
		SubmitTime: common.GetTimestamp(),
		Properties: model.Properties{
			Input:             "detail prompt",
			OriginModelName:   "sora-compatible",
			UpstreamModelName: "sora-compatible",
		},
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://gateway.example.com/v1/videos/task_video_detail/content",
		},
		Data: payload,
	}
	if err := model.DB.Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	summary := buildVideoTaskSummary(task)
	if summary == nil {
		t.Fatal("expected summary")
	}
	if summary.ResultURL != "https://cdn.example.com/detail.mp4" {
		t.Fatalf("expected direct summary result url, got %q", summary.ResultURL)
	}
	if summary.VideoURL != "/v1/videos/task_video_detail/content" {
		t.Fatalf("expected proxy video_url fallback, got %q", summary.VideoURL)
	}

	detail, err := GetVideoGenerationTaskDetail(1, fmt.Sprint(task.ID))
	if err != nil {
		t.Fatalf("GetVideoGenerationTaskDetail returned error: %v", err)
	}
	if detail.ResultURL != "https://cdn.example.com/detail.mp4" {
		t.Fatalf("expected direct detail result url, got %q", detail.ResultURL)
	}
}
