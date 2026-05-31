package sora

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func TestParseTaskResultExtractsCompletedDirectVideoURL(t *testing.T) {
	payload, err := common.Marshal(map[string]any{
		"id":     "vid_123",
		"status": "completed",
		"content": map[string]any{
			"video_url": "https://cdn.example.com/result.mp4",
		},
	})
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	taskInfo, err := (&TaskAdaptor{}).ParseTaskResult(payload)
	if err != nil {
		t.Fatalf("ParseTaskResult returned error: %v", err)
	}
	if taskInfo.Status != string(model.TaskStatusSuccess) {
		t.Fatalf("expected status %q, got %q", model.TaskStatusSuccess, taskInfo.Status)
	}
	if taskInfo.Url != "https://cdn.example.com/result.mp4" {
		t.Fatalf("expected direct url to be extracted, got %q", taskInfo.Url)
	}
}

func TestParseTaskResultKeepsProxyFallbackWhenCompletedPayloadHasNoDirectURL(t *testing.T) {
	payload, err := common.Marshal(map[string]any{
		"id":     "vid_456",
		"status": "completed",
	})
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	taskInfo, err := (&TaskAdaptor{}).ParseTaskResult(payload)
	if err != nil {
		t.Fatalf("ParseTaskResult returned error: %v", err)
	}
	if taskInfo.Status != string(model.TaskStatusSuccess) {
		t.Fatalf("expected status %q, got %q", model.TaskStatusSuccess, taskInfo.Status)
	}
	if taskInfo.Url != "" {
		t.Fatalf("expected empty direct url for proxy-only payload, got %q", taskInfo.Url)
	}
}
