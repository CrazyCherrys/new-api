package dto

type VideoGenerationModel struct {
	RequestModel      string   `json:"request_model"`
	DisplayName       string   `json:"display_name"`
	ModelSeries       string   `json:"model_series"`
	RequestEndpoint   string   `json:"request_endpoint"`
	VideoCapabilities []string `json:"video_capabilities"`
	DurationOptions   []int    `json:"duration_options"`
	Resolutions       []string `json:"resolutions"`
	AspectRatios      []string `json:"aspect_ratios"`
}

type VideoGenerationTaskSummary struct {
	ID              int64  `json:"id"`
	TaskID          string `json:"task_id"`
	Status          string `json:"status"`
	Progress        string `json:"progress"`
	Prompt          string `json:"prompt"`
	ModelID         string `json:"model_id"`
	DisplayName     string `json:"display_name"`
	RequestEndpoint string `json:"request_endpoint"`
	RequestType     string `json:"request_type"`
	Duration        int    `json:"duration"`
	Resolution      string `json:"resolution"`
	AspectRatio     string `json:"aspect_ratio"`
	CreatedTime     int64  `json:"created_time"`
	StartedTime     int64  `json:"started_time"`
	CompletedTime   int64  `json:"completed_time"`
	ThumbnailURL    string `json:"thumbnail_url"`
	VideoURL        string `json:"video_url"`
	ResultURL       string `json:"result_url"`
	FailReason      string `json:"fail_reason,omitempty"`
}

type VideoGenerationTaskDetail struct {
	VideoGenerationTaskSummary
	Quota int `json:"quota"`
}
