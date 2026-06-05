package dto

type CanvasChatAttachment struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type CanvasChatModelCatalogItem struct {
	RequestModel     string   `json:"request_model"`
	DisplayName      string   `json:"display_name"`
	ModelSeries      string   `json:"model_series"`
	RequestEndpoint  string   `json:"request_endpoint"`
	Description      string   `json:"description,omitempty"`
	VendorIcon       string   `json:"vendor_icon,omitempty"`
	ChatCapabilities []string `json:"chat_capabilities"`
}
