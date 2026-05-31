package dto

type CanvasChatModelCatalogItem struct {
	RequestModel    string `json:"request_model"`
	DisplayName     string `json:"display_name"`
	ModelSeries     string `json:"model_series"`
	RequestEndpoint string `json:"request_endpoint"`
}
