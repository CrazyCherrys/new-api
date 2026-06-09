package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupRelayTaskInsertTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousCanvasChatDB := model.CANVAS_CHAT_DB
	previousCanvasImageDB := model.CANVAS_IMAGE_DB
	previousCanvasVideoDB := model.CANVAS_VIDEO_DB
	previousUsingSQLite := common.UsingSQLite
	previousUsingMySQL := common.UsingMySQL
	previousUsingPostgreSQL := common.UsingPostgreSQL

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	model.InitCommonColumnNames()
	t.Setenv("CANVAS_CHAT_SQL_DSN", "")
	t.Setenv("CANVAS_IMAGE_SQL_DSN", "")
	t.Setenv("CANVAS_VIDEO_SQL_DSN", "")

	mainDSN := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	mainDB, err := gorm.Open(sqlite.Open(mainDSN), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open main sqlite db: %v", err)
	}

	model.DB = mainDB
	model.LOG_DB = mainDB
	if err := mainDB.AutoMigrate(&model.Task{}); err != nil {
		t.Fatalf("failed to migrate main task table: %v", err)
	}
	model.InitCanvasDBs()

	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		model.CANVAS_CHAT_DB = previousCanvasChatDB
		model.CANVAS_IMAGE_DB = previousCanvasImageDB
		model.CANVAS_VIDEO_DB = previousCanvasVideoDB
		common.UsingSQLite = previousUsingSQLite
		common.UsingMySQL = previousUsingMySQL
		common.UsingPostgreSQL = previousUsingPostgreSQL
		model.InitCommonColumnNames()
		if sqlDB, dbErr := mainDB.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	return mainDB
}

func newRelayTaskInsertContext(t *testing.T, configureHeader func(http.Header)) *gin.Context {
	t.Helper()

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	request := httptest.NewRequest(http.MethodPost, "/v1/videos", nil)
	if configureHeader != nil {
		configureHeader(request.Header)
	}
	context.Request = request
	return context
}

func TestInsertRelayTaskMarksCanvasVideoScopeOnlyWithValidInternalHeader(t *testing.T) {
	mainDB := setupRelayTaskInsertTestDB(t)

	cases := []struct {
		name       string
		taskID     string
		headerFunc func(http.Header)
		wantScope  string
	}{
		{
			name:      "missing header",
			taskID:    "task_missing_canvas_header",
			wantScope: "main",
		},
		{
			name:   "invalid header",
			taskID: "task_invalid_canvas_header",
			headerFunc: func(header http.Header) {
				header.Set(constant.HeaderCanvasVideoTaskScope, "invalid")
			},
			wantScope: "main",
		},
		{
			name:   "valid header",
			taskID: "task_valid_canvas_header",
			headerFunc: func(header http.Header) {
				service.SetCanvasVideoTaskScopeHeader(header)
			},
			wantScope: "canvas_video",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := &model.Task{
				TaskID:     tc.taskID,
				UserId:     51,
				Action:     constant.TaskActionTextGenerate,
				Status:     model.TaskStatusQueued,
				Progress:   "0%",
				SubmitTime: 100,
			}
			if err := insertRelayTask(newRelayTaskInsertContext(t, tc.headerFunc), task); err != nil {
				t.Fatalf("insertRelayTask returned error: %v", err)
			}
			if task.StorageScope != tc.wantScope {
				t.Fatalf("expected storage scope %q, got %q", tc.wantScope, task.StorageScope)
			}

			var mainCount int64
			if err := mainDB.Model(&model.Task{}).Where("task_id = ?", tc.taskID).Count(&mainCount).Error; err != nil {
				t.Fatalf("failed to count main tasks: %v", err)
			}
			if mainCount != 1 {
				t.Fatalf("expected one inserted task in compatibility store, got %d", mainCount)
			}
		})
	}
}
