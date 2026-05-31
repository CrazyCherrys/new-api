package service

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"strings"
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

func TestBuildVideoRelaySubmitBodyUsesMultipartForOpenAIVideoReferenceImage(t *testing.T) {
	body, contentType, err := buildVideoRelaySubmitBody(
		"sora-2",
		"cat video",
		"openai-video",
		VideoGenerationParams{
			Duration:   8,
			Resolution: "720x1280",
		},
		"data:image/png;base64,YWJj",
	)
	if err != nil {
		t.Fatalf("buildVideoRelaySubmitBody returned error: %v", err)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data;") {
		t.Fatalf("expected multipart content type, got %q", contentType)
	}

	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("failed to parse multipart content type: %v", err)
	}
	payload, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("failed to read multipart body: %v", err)
	}
	form, err := multipart.NewReader(bytes.NewReader(payload), params["boundary"]).ReadForm(1024 * 1024)
	if err != nil {
		t.Fatalf("failed to read multipart form: %v", err)
	}
	defer form.RemoveAll()

	if got := form.Value["model"]; len(got) != 1 || got[0] != "sora-2" {
		t.Fatalf("expected model field sora-2, got %#v", got)
	}
	if got := form.Value["prompt"]; len(got) != 1 || got[0] != "cat video" {
		t.Fatalf("expected prompt field cat video, got %#v", got)
	}
	if got := form.Value["seconds"]; len(got) != 1 || got[0] != "8" {
		t.Fatalf("expected seconds field 8, got %#v", got)
	}
	files := form.File["input_reference"]
	if len(files) != 1 || files[0].Filename != "reference.png" {
		t.Fatalf("expected one input_reference file, got %#v", files)
	}
}

func TestBuildVideoRelaySubmitBodyUsesMultipartForOpenAIVideoTextOnly(t *testing.T) {
	body, contentType, err := buildVideoRelaySubmitBody(
		"sora-2",
		"text only video",
		"openai-video",
		VideoGenerationParams{
			Duration:   4,
			Resolution: "1280x720",
		},
		"",
	)
	if err != nil {
		t.Fatalf("buildVideoRelaySubmitBody returned error: %v", err)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data;") {
		t.Fatalf("expected multipart content type, got %q", contentType)
	}

	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("failed to parse multipart content type: %v", err)
	}
	payload, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("failed to read multipart body: %v", err)
	}
	form, err := multipart.NewReader(bytes.NewReader(payload), params["boundary"]).ReadForm(1024 * 1024)
	if err != nil {
		t.Fatalf("failed to read multipart form: %v", err)
	}
	defer form.RemoveAll()

	if got := form.Value["model"]; len(got) != 1 || got[0] != "sora-2" {
		t.Fatalf("expected model field sora-2, got %#v", got)
	}
	if got := form.Value["prompt"]; len(got) != 1 || got[0] != "text only video" {
		t.Fatalf("expected prompt field text only video, got %#v", got)
	}
	if got := form.Value["seconds"]; len(got) != 1 || got[0] != "4" {
		t.Fatalf("expected seconds field 4, got %#v", got)
	}
	if got := form.Value["size"]; len(got) != 1 || got[0] != "1280x720" {
		t.Fatalf("expected size field 1280x720, got %#v", got)
	}
	if files := form.File["input_reference"]; len(files) != 0 {
		t.Fatalf("did not expect input_reference file for text-only submit, got %#v", files)
	}
}

func TestBuildVideoRelaySubmitBodyDownloadsRemoteReferenceForOpenAIVideo(t *testing.T) {
	previousDownloader := getVideoReferenceImageFromURL
	getVideoReferenceImageFromURL = func(url string) (string, string, error) {
		if url != "https://example.com/reference.png" {
			t.Fatalf("unexpected url %q", url)
		}
		return "image/png", "cG5n", nil
	}
	defer func() {
		getVideoReferenceImageFromURL = previousDownloader
	}()

	body, contentType, err := buildVideoRelaySubmitBody(
		"sora-2",
		"remote ref video",
		"openai-video",
		VideoGenerationParams{
			Duration:   8,
			Resolution: "720x1280",
		},
		"https://example.com/reference.png",
	)
	if err != nil {
		t.Fatalf("buildVideoRelaySubmitBody returned error: %v", err)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data;") {
		t.Fatalf("expected multipart content type, got %q", contentType)
	}

	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("failed to parse multipart content type: %v", err)
	}
	payload, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("failed to read multipart body: %v", err)
	}
	form, err := multipart.NewReader(bytes.NewReader(payload), params["boundary"]).ReadForm(1024 * 1024)
	if err != nil {
		t.Fatalf("failed to read multipart form: %v", err)
	}
	defer form.RemoveAll()

	files := form.File["input_reference"]
	if len(files) != 1 || files[0].Filename != "reference.png" {
		t.Fatalf("expected one downloaded input_reference file, got %#v", files)
	}
	file, err := files[0].Open()
	if err != nil {
		t.Fatalf("failed to open multipart file: %v", err)
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("failed to read multipart file: %v", err)
	}
	if string(content) != "png" {
		t.Fatalf("expected downloaded file content to be preserved, got %q", string(content))
	}
}
