package dto

// CreateImageTaskRequest 创建图像生成任务请求
type CreateImageTaskRequest struct {
	Model          string `json:"model" binding:"required"`
	Prompt         string `json:"prompt" binding:"required"`
	Resolution     string `json:"resolution"`      // 可选，默认 "1K"
	AspectRatio    string `json:"aspect_ratio"`    // 可选，默认 "1:1"
	ReferenceImage string `json:"reference_image"` // 可选
	Count          int    `json:"count"`           // 可选，默认 1，最大 4
}

// ImageTaskResponse 图像任务响应
type ImageTaskResponse struct {
	TaskID         string   `json:"task_id"`
	UserID         int      `json:"user_id"`
	Model          string   `json:"model"`
	Prompt         string   `json:"prompt"`
	Resolution     string   `json:"resolution"`
	AspectRatio    string   `json:"aspect_ratio"`
	ReferenceImage string   `json:"reference_image,omitempty"`
	Count          int      `json:"count"`
	Status         string   `json:"status"`
	ErrorMessage   string   `json:"error_message,omitempty"`
	ImageURLs      []string `json:"image_urls,omitempty"`
	Attempts       int      `json:"attempts"`
	CreatedAt      int64    `json:"created_at"`
	UpdatedAt      int64    `json:"updated_at"`
	CompletedAt    *int64   `json:"completed_at,omitempty"`
}

// ListImageTasksRequest 列表查询请求
type ListImageTasksRequest struct {
	Page      int    `form:"page"`
	PageSize  int    `form:"page_size"`
	Status    string `form:"status"`
	Model     string `form:"model"`
	StartTime int64  `form:"start_time"`
	EndTime   int64  `form:"end_time"`
}

// ListImageTasksResponse 列表响应
type ListImageTasksResponse struct {
	Data     []ImageTaskResponse `json:"data"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}
