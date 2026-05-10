package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type optionAPIResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Data    []model.Option `json:"data"`
}

type optionMutationResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func withOptionMap(t *testing.T, options map[string]string) {
	t.Helper()

	common.OptionMapRWMutex.Lock()
	previous := common.OptionMap
	common.OptionMap = make(map[string]string, len(options))
	for key, value := range options {
		common.OptionMap[key] = value
	}
	common.OptionMapRWMutex.Unlock()

	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previous
		common.OptionMapRWMutex.Unlock()
	})
}

func TestGetOptionsMasksWorkerS3SecretFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withOptionMap(t, map[string]string{
		"worker_setting.s3_endpoint":   "https://s3.example.com",
		"worker_setting.s3_bucket":     "bucket",
		"worker_setting.s3_region":     "us-east-1",
		"worker_setting.s3_path_prefix": "worker/output",
		"worker_setting.s3_access_key": "raw-access-key",
		"worker_setting.s3_secret_key": "raw-secret-key",
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/option/", nil)

	GetOptions(ctx)

	var response optionAPIResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	values := make(map[string]string, len(response.Data))
	for _, option := range response.Data {
		values[option.Key] = option.Value
	}
	if values["worker_setting.s3_access_key"] != maskedWorkerS3OptionValue {
		t.Fatalf("expected access key to be masked, got %q", values["worker_setting.s3_access_key"])
	}
	if values["worker_setting.s3_secret_key"] != maskedWorkerS3OptionValue {
		t.Fatalf("expected secret key to be masked, got %q", values["worker_setting.s3_secret_key"])
	}
	if values["worker_setting.s3_bucket"] != "bucket" {
		t.Fatalf("expected bucket to be returned normally, got %q", values["worker_setting.s3_bucket"])
	}
	if values["worker_setting.s3_endpoint"] != "https://s3.example.com" {
		t.Fatalf("expected endpoint to be returned normally, got %q", values["worker_setting.s3_endpoint"])
	}
	if values["worker_setting.s3_region"] != "us-east-1" {
		t.Fatalf("expected region to be returned normally, got %q", values["worker_setting.s3_region"])
	}
	if values["worker_setting.s3_path_prefix"] != "worker/output" {
		t.Fatalf("expected path prefix to be returned normally, got %q", values["worker_setting.s3_path_prefix"])
	}
	if string(recorder.Body.Bytes()) == "" ||
		containsAny(recorder.Body.String(), "raw-access-key", "raw-secret-key") {
		t.Fatalf("response leaked raw S3 credentials: %s", recorder.Body.String())
	}
}

func TestGetOptionsLeavesUnsetWorkerS3SecretsBlank(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withOptionMap(t, map[string]string{
		"worker_setting.s3_access_key": "",
		"worker_setting.s3_secret_key": "",
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/option/", nil)

	GetOptions(ctx)

	var response optionAPIResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	values := make(map[string]string, len(response.Data))
	for _, option := range response.Data {
		values[option.Key] = option.Value
	}
	if values["worker_setting.s3_access_key"] != "" {
		t.Fatalf("expected unset access key to stay blank, got %q", values["worker_setting.s3_access_key"])
	}
	if values["worker_setting.s3_secret_key"] != "" {
		t.Fatalf("expected unset secret key to stay blank, got %q", values["worker_setting.s3_secret_key"])
	}
}

func TestUpdateOptionKeepsWorkerS3SecretWhenMaskedValueSubmitted(t *testing.T) {
	for _, maskedValue := range []string{maskedWorkerS3OptionValue, "***"} {
		t.Run(maskedValue, func(t *testing.T) {
			withOptionMap(t, map[string]string{
				"worker_setting.s3_secret_key": "raw-secret-key",
			})

			ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/option/", map[string]string{
				"key":   "worker_setting.s3_secret_key",
				"value": maskedValue,
			}, 1)

			UpdateOption(ctx)

			var response optionMutationResponse
			if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if !response.Success {
				t.Fatalf("expected success response, got message: %s", response.Message)
			}

			common.OptionMapRWMutex.RLock()
			stored := common.OptionMap["worker_setting.s3_secret_key"]
			common.OptionMapRWMutex.RUnlock()
			if stored != "raw-secret-key" {
				t.Fatalf("expected masked submit to keep existing secret, got %q", stored)
			}
		})
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
