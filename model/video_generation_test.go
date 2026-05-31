package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestEffectiveVideoResultURL(t *testing.T) {
	mustMarshal := func(value any) []byte {
		t.Helper()
		data, err := common.Marshal(value)
		if err != nil {
			t.Fatalf("failed to marshal payload: %v", err)
		}
		return data
	}

	tests := []struct {
		name string
		task *Task
		want string
	}{
		{
			name: "prefers stored direct result url",
			task: &Task{
				TaskID: "task_direct",
				PrivateData: TaskPrivateData{
					ResultURL: "https://cdn.example.com/stored.mp4",
				},
				Data: mustMarshal(map[string]any{
					"content": map[string]any{
						"video_url": "https://cdn.example.com/payload.mp4",
					},
				}),
			},
			want: "https://cdn.example.com/stored.mp4",
		},
		{
			name: "recovers direct result url from wrapped payload when stored proxy",
			task: &Task{
				TaskID: "task_payload",
				PrivateData: TaskPrivateData{
					ResultURL: "https://gateway.example.com/v1/videos/task_payload/content",
				},
				Data: mustMarshal(map[string]any{
					"data": map[string]any{
						"content": map[string]any{
							"video_url": "https://cdn.example.com/payload.mp4",
						},
					},
				}),
			},
			want: "https://cdn.example.com/payload.mp4",
		},
		{
			name: "falls back to proxy when no direct url exists",
			task: &Task{
				TaskID: "task_proxy",
			},
			want: "/v1/videos/task_proxy/content",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := EffectiveVideoResultURL(tc.task); got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}
