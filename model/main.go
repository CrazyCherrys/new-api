package model

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var commonGroupCol string
var commonKeyCol string
var commonTrueVal string
var commonFalseVal string

var logKeyCol string
var logGroupCol string

func initCol() {
	// init common column names
	if common.UsingPostgreSQL {
		commonGroupCol = `"group"`
		commonKeyCol = `"key"`
		commonTrueVal = "true"
		commonFalseVal = "false"
	} else {
		commonGroupCol = "`group`"
		commonKeyCol = "`key`"
		commonTrueVal = "1"
		commonFalseVal = "0"
	}
	if os.Getenv("LOG_SQL_DSN") != "" {
		switch common.LogSqlType {
		case common.DatabaseTypePostgreSQL:
			logGroupCol = `"group"`
			logKeyCol = `"key"`
		default:
			logGroupCol = commonGroupCol
			logKeyCol = commonKeyCol
		}
	} else {
		// LOG_SQL_DSN 为空时，日志数据库与主数据库相同
		if common.UsingPostgreSQL {
			logGroupCol = `"group"`
			logKeyCol = `"key"`
		} else {
			logGroupCol = commonGroupCol
			logKeyCol = commonKeyCol
		}
	}
	// log sql type and database type
	//common.SysLog("Using Log SQL Type: " + common.LogSqlType)
}

func InitCommonColumnNames() {
	initCol()
}

var DB *gorm.DB

var LOG_DB *gorm.DB

var CANVAS_CHAT_DB *gorm.DB
var CANVAS_IMAGE_DB *gorm.DB
var CANVAS_VIDEO_DB *gorm.DB

type canvasMessageStorageMode string

const (
	canvasMessageStorageModeCompatibility canvasMessageStorageMode = "compatibility"
	canvasMessageStorageModeSplit         canvasMessageStorageMode = "split"
	canvasMessageStorageModeDisabled      canvasMessageStorageMode = "disabled"
)

type canvasModeStorageState struct {
	storageMode canvasMessageStorageMode
	available   bool
	reason      string
}

type CanvasModeUnavailableError struct {
	Mode   string
	Reason string
}

func (e *CanvasModeUnavailableError) Error() string {
	mode := NormalizeCanvasMode(e.Mode)
	reason := strings.TrimSpace(e.Reason)
	if mode == "" {
		if reason == "" {
			return "canvas is unavailable"
		}
		return reason
	}
	if reason == "" {
		return fmt.Sprintf("canvas %s mode is unavailable", mode)
	}
	return fmt.Sprintf("canvas %s mode is unavailable: %s", mode, reason)
}

var (
	canvasStateMu    = sync.RWMutex{}
	canvasModeStates = map[string]canvasModeStorageState{
		CanvasModeChat:  {storageMode: canvasMessageStorageModeCompatibility, available: true},
		CanvasModeImage: {storageMode: canvasMessageStorageModeCompatibility, available: true},
		CanvasModeVideo: {storageMode: canvasMessageStorageModeCompatibility, available: true},
	}
	openCanvasMessagePostgresDBFunc = openCanvasMessagePostgresDB
)

func CanvasAvailabilityStatus() (bool, string) {
	canvasStateMu.RLock()
	defer canvasStateMu.RUnlock()
	reasons := make([]string, 0, len(canvasModeStates))
	for _, mode := range []string{CanvasModeChat, CanvasModeImage, CanvasModeVideo} {
		state := canvasModeStates[mode]
		if state.available {
			return true, ""
		}
		if reason := strings.TrimSpace(state.reason); reason != "" {
			reasons = append(reasons, fmt.Sprintf("%s: %s", mode, reason))
		}
	}
	if len(reasons) == 0 {
		return false, "canvas is unavailable"
	}
	return false, strings.Join(reasons, "; ")
}

func CanvasUsesDedicatedMessageDBs() bool {
	canvasStateMu.RLock()
	defer canvasStateMu.RUnlock()
	for _, mode := range []string{CanvasModeChat, CanvasModeImage, CanvasModeVideo} {
		if canvasModeStates[mode].storageMode == canvasMessageStorageModeSplit {
			return true
		}
	}
	return false
}

func newCanvasModeUnavailableError(mode string, reason string) error {
	return &CanvasModeUnavailableError{
		Mode:   NormalizeCanvasMode(mode),
		Reason: strings.TrimSpace(reason),
	}
}

func IsCanvasModeUnavailableError(err error) bool {
	var target *CanvasModeUnavailableError
	return errors.As(err, &target)
}

func CanvasModeAvailabilityStatus(mode string) (bool, string) {
	mode = NormalizeCanvasMode(mode)
	if mode == "" {
		return false, "invalid canvas mode"
	}
	canvasStateMu.RLock()
	defer canvasStateMu.RUnlock()
	state, ok := canvasModeStates[mode]
	if !ok {
		return false, "invalid canvas mode"
	}
	return state.available, strings.TrimSpace(state.reason)
}

func CanvasModeUsesDedicatedMessageDB(mode string) bool {
	mode = NormalizeCanvasMode(mode)
	if mode == "" {
		return false
	}
	canvasStateMu.RLock()
	defer canvasStateMu.RUnlock()
	state, ok := canvasModeStates[mode]
	return ok && state.storageMode == canvasMessageStorageModeSplit
}

func EnsureCanvasModeAvailable(mode string) error {
	if available, reason := CanvasModeAvailabilityStatus(mode); available {
		return nil
	} else {
		return newCanvasModeUnavailableError(mode, reason)
	}
}

func setCanvasModeStorageState(mode string, storageMode canvasMessageStorageMode, available bool, reason string) {
	mode = NormalizeCanvasMode(mode)
	if mode == "" {
		return
	}
	canvasStateMu.Lock()
	defer canvasStateMu.Unlock()
	canvasModeStates[mode] = canvasModeStorageState{
		storageMode: storageMode,
		available:   available,
		reason:      strings.TrimSpace(reason),
	}
}

func canvasModeDSNEnvName(mode string) string {
	switch NormalizeCanvasMode(mode) {
	case CanvasModeChat:
		return "CANVAS_CHAT_SQL_DSN"
	case CanvasModeImage:
		return "CANVAS_IMAGE_SQL_DSN"
	case CanvasModeVideo:
		return "CANVAS_VIDEO_SQL_DSN"
	default:
		return ""
	}
}

func setCanvasModeDBHandle(mode string, db *gorm.DB) {
	switch NormalizeCanvasMode(mode) {
	case CanvasModeChat:
		CANVAS_CHAT_DB = db
	case CanvasModeImage:
		CANVAS_IMAGE_DB = db
	case CanvasModeVideo:
		CANVAS_VIDEO_DB = db
	}
}

func canvasModeDedicatedDB(mode string) *gorm.DB {
	switch NormalizeCanvasMode(mode) {
	case CanvasModeChat:
		return CANVAS_CHAT_DB
	case CanvasModeImage:
		return CANVAS_IMAGE_DB
	case CanvasModeVideo:
		return CANVAS_VIDEO_DB
	default:
		return nil
	}
}

func canvasModeDataDB(mode string) (*gorm.DB, error) {
	mode = NormalizeCanvasMode(mode)
	if mode == "" {
		return nil, fmt.Errorf("invalid canvas mode")
	}
	if err := EnsureCanvasModeAvailable(mode); err != nil {
		return nil, err
	}
	if CanvasModeUsesDedicatedMessageDB(mode) {
		db := canvasModeDedicatedDB(mode)
		if db == nil {
			return nil, fmt.Errorf("canvas %s database is not initialized", mode)
		}
		return db, nil
	}
	if DB == nil {
		return nil, fmt.Errorf("canvas main database is not initialized")
	}
	return DB, nil
}

func closeCanvasMessageDBs() error {
	seen := map[*gorm.DB]struct{}{}
	for _, db := range []*gorm.DB{CANVAS_CHAT_DB, CANVAS_IMAGE_DB, CANVAS_VIDEO_DB} {
		if db == nil || db == DB || db == LOG_DB {
			continue
		}
		if _, ok := seen[db]; ok {
			continue
		}
		seen[db] = struct{}{}
		if err := closeDB(db); err != nil {
			return err
		}
	}
	CANVAS_CHAT_DB = nil
	CANVAS_IMAGE_DB = nil
	CANVAS_VIDEO_DB = nil
	return nil
}

func openCanvasMessagePostgresDB(envName string, dsn string) (*gorm.DB, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, fmt.Errorf("%s is empty", envName)
	}
	if !strings.HasPrefix(dsn, "postgres://") && !strings.HasPrefix(dsn, "postgresql://") {
		return nil, fmt.Errorf("%s only supports PostgreSQL DSN", envName)
	}
	common.SysLog("using PostgreSQL as " + envName + " canvas database")
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		PrepareStmt: true,
	})
	if err != nil {
		return nil, err
	}
	if common.DebugEnabled {
		db = db.Debug()
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(common.GetEnvOrDefault("SQL_MAX_IDLE_CONNS", 100))
	sqlDB.SetMaxOpenConns(common.GetEnvOrDefault("SQL_MAX_OPEN_CONNS", 1000))
	sqlDB.SetConnMaxLifetime(time.Second * time.Duration(common.GetEnvOrDefault("SQL_MAX_LIFETIME", 60)))
	return db, nil
}

func migrateCanvasModeDB(mode string, db *gorm.DB) error {
	if db == nil {
		return nil
	}
	switch NormalizeCanvasMode(mode) {
	case CanvasModeChat:
		if err := db.AutoMigrate(&CanvasSession{}, &CanvasChatMessage{}, &CanvasChatMessageAttachment{}); err != nil {
			return fmt.Errorf("failed to migrate chat canvas tables: %w", err)
		}
	case CanvasModeImage:
		if err := db.AutoMigrate(&CanvasSession{}, &CanvasImageMessage{}, &ImageGenerationTask{}, &ImageGenerationReferenceAsset{}, &ImageGenerationTaskReferenceAsset{}, &CanvasAssetCleanupJob{}); err != nil {
			return fmt.Errorf("failed to migrate image canvas tables: %w", err)
		}
	case CanvasModeVideo:
		if err := db.AutoMigrate(&CanvasSession{}, &CanvasVideoMessage{}, &CanvasAssetCleanupJob{}, &Task{}); err != nil {
			return fmt.Errorf("failed to migrate video canvas tables: %w", err)
		}
	}
	return nil
}

func MigrateCanvasDedicatedDBs() error {
	errs := make([]string, 0, 3)
	for _, mode := range []string{CanvasModeChat, CanvasModeImage, CanvasModeVideo} {
		if !CanvasModeUsesDedicatedMessageDB(mode) {
			continue
		}
		db := canvasModeDedicatedDB(mode)
		if db == nil {
			errs = append(errs, fmt.Sprintf("%s: dedicated database is not initialized", mode))
			continue
		}
		if err := migrateCanvasModeDB(mode, db); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", mode, err))
			continue
		}
		setCanvasModeStorageState(mode, canvasMessageStorageModeSplit, true, "")
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.New(strings.Join(errs, "; "))
}

type CanvasDedicatedBackfillStats struct {
	Mode                     string
	Sessions                 int64
	Messages                 int64
	ChatAttachments          int64
	ImageTasks               int64
	ImageReferenceAssets     int64
	ImageTaskReferenceAssets int64
	AssetCleanupJobs         int64
	VideoTasks               int64
}

const canvasDedicatedBackfillBatchSize = 500

func BackfillCanvasDedicatedDBs() (map[string]CanvasDedicatedBackfillStats, error) {
	if DB == nil {
		return nil, fmt.Errorf("canvas main database is not initialized")
	}
	if err := MigrateCanvasDedicatedDBs(); err != nil {
		return nil, err
	}
	result := make(map[string]CanvasDedicatedBackfillStats, 3)
	errs := make([]string, 0, 3)
	for _, mode := range []string{CanvasModeChat, CanvasModeImage, CanvasModeVideo} {
		if !CanvasModeUsesDedicatedMessageDB(mode) {
			continue
		}
		db := canvasModeDedicatedDB(mode)
		if db == nil {
			errs = append(errs, fmt.Sprintf("%s: dedicated database is not initialized", mode))
			continue
		}
		stats, err := backfillCanvasDedicatedModeDB(mode, db)
		result[mode] = stats
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", mode, err))
		}
	}
	if len(errs) > 0 {
		return result, errors.New(strings.Join(errs, "; "))
	}
	return result, nil
}

func backfillCanvasDedicatedModeDB(mode string, db *gorm.DB) (CanvasDedicatedBackfillStats, error) {
	stats := CanvasDedicatedBackfillStats{Mode: NormalizeCanvasMode(mode)}
	if stats.Mode == "" {
		return stats, fmt.Errorf("invalid canvas mode")
	}
	if db == nil {
		return stats, fmt.Errorf("canvas %s database is not initialized", stats.Mode)
	}

	var err error
	if canvasBackfillTableExists(DB, "canvas_sessions") {
		stats.Sessions, err = backfillCanvasRows[CanvasSession](
			DB.Table("canvas_sessions").Where("mode = ?", stats.Mode).Order("id ASC"),
			db.Table("canvas_sessions"),
			func(session *CanvasSession) {
				if session != nil && strings.TrimSpace(session.PublicId) == "" {
					session.PublicId = generateCanvasSessionPublicID(session.Mode)
				}
			},
		)
		if err != nil {
			return stats, err
		}
	}
	if canvasBackfillTableExists(DB, "canvas_messages") {
		stats.Messages, err = backfillCanvasRows[CanvasMessage](
			DB.Table("canvas_messages").Where("mode = ?", stats.Mode).Order("id ASC"),
			db.Table(canvasDedicatedMessageTableName(stats.Mode)),
			nil,
		)
		if err != nil {
			return stats, err
		}
	}

	switch stats.Mode {
	case CanvasModeChat:
		if canvasBackfillTableExists(DB, canvasChatMessageAttachmentsTable) {
			stats.ChatAttachments, err = backfillCanvasRows[CanvasChatMessageAttachment](
				DB.Table(canvasChatMessageAttachmentsTable).Order("id ASC"),
				db.Table(canvasChatMessageAttachmentsTable),
				nil,
			)
			if err != nil {
				return stats, err
			}
		}
	case CanvasModeImage:
		if canvasBackfillTableExists(DB, "image_generation_tasks") {
			stats.ImageTasks, err = backfillCanvasRows[ImageGenerationTask](
				DB.Table("image_generation_tasks").Order("id ASC"),
				db.Table("image_generation_tasks"),
				nil,
			)
			if err != nil {
				return stats, err
			}
		}
		if canvasBackfillTableExists(DB, "image_generation_reference_assets") {
			stats.ImageReferenceAssets, err = backfillCanvasRows[ImageGenerationReferenceAsset](
				DB.Table("image_generation_reference_assets").Order("id ASC"),
				db.Table("image_generation_reference_assets"),
				nil,
			)
			if err != nil {
				return stats, err
			}
		}
		if canvasBackfillTableExists(DB, "image_generation_task_reference_assets") {
			stats.ImageTaskReferenceAssets, err = backfillCanvasRows[ImageGenerationTaskReferenceAsset](
				DB.Table("image_generation_task_reference_assets").Order("id ASC"),
				db.Table("image_generation_task_reference_assets"),
				nil,
			)
			if err != nil {
				return stats, err
			}
		}
		if canvasBackfillTableExists(DB, "canvas_asset_cleanup_jobs") {
			stats.AssetCleanupJobs, err = backfillCanvasRows[CanvasAssetCleanupJob](
				DB.Table("canvas_asset_cleanup_jobs").Where("task_type = ?", CanvasTaskTypeImage).Order("id ASC"),
				db.Table("canvas_asset_cleanup_jobs"),
				nil,
			)
			if err != nil {
				return stats, err
			}
		}
	case CanvasModeVideo:
		if canvasBackfillTableExists(DB, "tasks") {
			stats.VideoTasks, err = backfillCanvasRows[Task](
				DB.Table("tasks").Where("action IN ?", DefaultVideoTaskActions()).Order("id ASC"),
				db.Table("tasks"),
				nil,
			)
			if err != nil {
				return stats, err
			}
		}
		if canvasBackfillTableExists(DB, "canvas_asset_cleanup_jobs") {
			stats.AssetCleanupJobs, err = backfillCanvasRows[CanvasAssetCleanupJob](
				DB.Table("canvas_asset_cleanup_jobs").Where("task_type = ?", CanvasTaskTypeVideo).Order("id ASC"),
				db.Table("canvas_asset_cleanup_jobs"),
				nil,
			)
			if err != nil {
				return stats, err
			}
		}
	}

	if err := resetCanvasDedicatedPostgresSequences(stats.Mode, db); err != nil {
		return stats, err
	}
	return stats, nil
}

func canvasDedicatedMessageTableName(mode string) string {
	switch NormalizeCanvasMode(mode) {
	case CanvasModeChat:
		return canvasChatMessagesTable
	case CanvasModeImage:
		return canvasImageMessagesTable
	case CanvasModeVideo:
		return canvasVideoMessagesTable
	default:
		return ""
	}
}

func canvasBackfillTableExists(db *gorm.DB, tableName string) bool {
	return db != nil && strings.TrimSpace(tableName) != "" && db.Migrator().HasTable(tableName)
}

func backfillCanvasRows[T any](source *gorm.DB, destination *gorm.DB, prepare func(*T)) (int64, error) {
	if source == nil || destination == nil {
		return 0, fmt.Errorf("canvas backfill source and destination are required")
	}
	var total int64
	var rows []T
	err := source.FindInBatches(&rows, canvasDedicatedBackfillBatchSize, func(tx *gorm.DB, batch int) error {
		if len(rows) == 0 {
			return nil
		}
		if prepare != nil {
			for index := range rows {
				prepare(&rows[index])
			}
		}
		result := destination.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows)
		if result.Error != nil {
			return result.Error
		}
		total += result.RowsAffected
		return nil
	}).Error
	return total, err
}

type canvasDedicatedPostgresSequenceTarget struct {
	Table  string
	Column string
}

func canvasDedicatedPostgresSequenceTargets(mode string) []canvasDedicatedPostgresSequenceTarget {
	switch NormalizeCanvasMode(mode) {
	case CanvasModeChat:
		return []canvasDedicatedPostgresSequenceTarget{
			{Table: "canvas_sessions", Column: "id"},
			{Table: canvasChatMessagesTable, Column: "id"},
			{Table: canvasChatMessageAttachmentsTable, Column: "id"},
		}
	case CanvasModeImage:
		return []canvasDedicatedPostgresSequenceTarget{
			{Table: "canvas_sessions", Column: "id"},
			{Table: canvasImageMessagesTable, Column: "id"},
			{Table: "image_generation_tasks", Column: "id"},
			{Table: "image_generation_reference_assets", Column: "id"},
			{Table: "image_generation_task_reference_assets", Column: "id"},
			{Table: "canvas_asset_cleanup_jobs", Column: "id"},
		}
	case CanvasModeVideo:
		return []canvasDedicatedPostgresSequenceTarget{
			{Table: "canvas_sessions", Column: "id"},
			{Table: canvasVideoMessagesTable, Column: "id"},
			{Table: "tasks", Column: "id"},
			{Table: "canvas_asset_cleanup_jobs", Column: "id"},
		}
	default:
		return nil
	}
}

func isAllowedCanvasDedicatedPostgresSequenceTarget(target canvasDedicatedPostgresSequenceTarget) bool {
	for _, mode := range []string{CanvasModeChat, CanvasModeImage, CanvasModeVideo} {
		for _, allowed := range canvasDedicatedPostgresSequenceTargets(mode) {
			if target == allowed {
				return true
			}
		}
	}
	return false
}

func quotePostgresIdentifier(identifier string) (string, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return "", fmt.Errorf("empty PostgreSQL identifier")
	}
	if strings.Contains(identifier, `"`) {
		return "", fmt.Errorf("invalid PostgreSQL identifier %q", identifier)
	}
	return `"` + identifier + `"`, nil
}

func canvasDedicatedPostgresSequenceResetSQL(target canvasDedicatedPostgresSequenceTarget) (string, []any, error) {
	if !isAllowedCanvasDedicatedPostgresSequenceTarget(target) {
		return "", nil, fmt.Errorf("canvas sequence reset target is not allowed: %s.%s", target.Table, target.Column)
	}
	quotedTable, err := quotePostgresIdentifier(target.Table)
	if err != nil {
		return "", nil, err
	}
	quotedColumn, err := quotePostgresIdentifier(target.Column)
	if err != nil {
		return "", nil, err
	}
	query := fmt.Sprintf(
		`SELECT setval(pg_get_serial_sequence(?, ?), COALESCE((SELECT MAX(%s) FROM %s), 0) + 1, false)`,
		quotedColumn,
		quotedTable,
	)
	return query, []any{target.Table, target.Column}, nil
}

func resetCanvasDedicatedPostgresSequences(mode string, db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("canvas %s database is not initialized", NormalizeCanvasMode(mode))
	}
	if db.Dialector == nil || db.Dialector.Name() != "postgres" {
		return nil
	}
	for _, target := range canvasDedicatedPostgresSequenceTargets(mode) {
		if !db.Migrator().HasTable(target.Table) {
			continue
		}
		query, args, err := canvasDedicatedPostgresSequenceResetSQL(target)
		if err != nil {
			return err
		}
		if err := db.Exec(query, args...).Error; err != nil {
			return fmt.Errorf("failed to reset %s.%s sequence: %w", target.Table, target.Column, err)
		}
	}
	return nil
}

func InitCanvasDBs() {
	if err := initCanvasDBs(); err != nil {
		common.SysError("canvas initialization failed: " + err.Error())
	}
}

func initCanvasDBs() error {
	if err := closeCanvasMessageDBs(); err != nil {
		return err
	}
	openedByDSN := make(map[string]*gorm.DB, 3)
	openCanvasDB := func(envName string, dsn string) (*gorm.DB, error) {
		if db, ok := openedByDSN[dsn]; ok {
			return db, nil
		}
		db, err := openCanvasMessagePostgresDBFunc(envName, dsn)
		if err != nil {
			return nil, err
		}
		openedByDSN[dsn] = db
		return db, nil
	}
	for _, mode := range []string{CanvasModeChat, CanvasModeImage, CanvasModeVideo} {
		envName := canvasModeDSNEnvName(mode)
		dsn := strings.TrimSpace(os.Getenv(envName))
		if dsn == "" {
			if DB == nil {
				setCanvasModeStorageState(mode, canvasMessageStorageModeCompatibility, false, "canvas main database is not initialized")
				continue
			}
			setCanvasModeStorageState(mode, canvasMessageStorageModeCompatibility, true, "")
			common.SysLog("canvas " + mode + " message storage using main database compatibility mode")
			continue
		}
		db, err := openCanvasDB(envName, dsn)
		if err != nil {
			setCanvasModeStorageState(mode, canvasMessageStorageModeSplit, false, err.Error())
			common.SysError("canvas " + mode + " mode unavailable: " + err.Error())
			continue
		}
		setCanvasModeDBHandle(mode, db)
		if err := migrateCanvasModeDB(mode, db); err != nil {
			setCanvasModeStorageState(mode, canvasMessageStorageModeSplit, false, err.Error())
			common.SysError("canvas " + mode + " mode unavailable: " + err.Error())
			continue
		}
		setCanvasModeStorageState(mode, canvasMessageStorageModeSplit, true, "")
		common.SysLog("canvas " + mode + " message storage using dedicated database")
	}
	return nil
}

func createRootAccountIfNeed() error {
	var user User
	//if user.Status != common.UserStatusEnabled {
	if err := DB.First(&user).Error; err != nil {
		common.SysLog("no user exists, create a root user for you: username is root, password is 123456")
		hashedPassword, err := common.Password2Hash("123456")
		if err != nil {
			return err
		}
		rootUser := User{
			Username:    "root",
			Password:    hashedPassword,
			Role:        common.RoleRootUser,
			Status:      common.UserStatusEnabled,
			DisplayName: "Root User",
			AccessToken: nil,
			Quota:       100000000,
		}
		DB.Create(&rootUser)
	}
	return nil
}

func CheckSetup() {
	setup := GetSetup()
	if setup == nil {
		// No setup record exists, check if we have a root user
		if RootUserExists() {
			common.SysLog("system is not initialized, but root user exists")
			// Create setup record
			newSetup := Setup{
				Version:       common.Version,
				InitializedAt: time.Now().Unix(),
			}
			err := DB.Create(&newSetup).Error
			if err != nil {
				common.SysLog("failed to create setup record: " + err.Error())
			}
			constant.Setup = true
		} else {
			common.SysLog("system is not initialized and no root user exists")
			constant.Setup = false
		}
	} else {
		// Setup record exists, system is initialized
		common.SysLog("system is already initialized at: " + time.Unix(setup.InitializedAt, 0).String())
		constant.Setup = true
	}
}

func chooseDB(envName string, isLog bool) (*gorm.DB, error) {
	defer func() {
		initCol()
	}()
	dsn := os.Getenv(envName)
	if dsn != "" {
		if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
			// Use PostgreSQL
			common.SysLog("using PostgreSQL as database")
			if !isLog {
				common.UsingPostgreSQL = true
			} else {
				common.LogSqlType = common.DatabaseTypePostgreSQL
			}
			return gorm.Open(postgres.New(postgres.Config{
				DSN:                  dsn,
				PreferSimpleProtocol: true, // disables implicit prepared statement usage
			}), &gorm.Config{
				PrepareStmt: true, // precompile SQL
			})
		}
		if strings.HasPrefix(dsn, "local") {
			common.SysLog("SQL_DSN not set, using SQLite as database")
			if !isLog {
				common.UsingSQLite = true
			} else {
				common.LogSqlType = common.DatabaseTypeSQLite
			}
			return gorm.Open(sqlite.Open(common.SQLitePath), &gorm.Config{
				PrepareStmt: true, // precompile SQL
			})
		}
		// Use MySQL
		common.SysLog("using MySQL as database")
		// check parseTime
		if !strings.Contains(dsn, "parseTime") {
			if strings.Contains(dsn, "?") {
				dsn += "&parseTime=true"
			} else {
				dsn += "?parseTime=true"
			}
		}
		if !isLog {
			common.UsingMySQL = true
		} else {
			common.LogSqlType = common.DatabaseTypeMySQL
		}
		return gorm.Open(mysql.Open(dsn), &gorm.Config{
			PrepareStmt: true, // precompile SQL
		})
	}
	// Use SQLite
	common.SysLog("SQL_DSN not set, using SQLite as database")
	common.UsingSQLite = true
	return gorm.Open(sqlite.Open(common.SQLitePath), &gorm.Config{
		PrepareStmt: true, // precompile SQL
	})
}

func InitDB() (err error) {
	db, err := chooseDB("SQL_DSN", false)
	if err == nil {
		if common.DebugEnabled {
			db = db.Debug()
		}
		DB = db
		// MySQL charset/collation startup check: ensure Chinese-capable charset
		if common.UsingMySQL {
			if err := checkMySQLChineseSupport(DB); err != nil {
				panic(err)
			}
		}
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}
		sqlDB.SetMaxIdleConns(common.GetEnvOrDefault("SQL_MAX_IDLE_CONNS", 100))
		sqlDB.SetMaxOpenConns(common.GetEnvOrDefault("SQL_MAX_OPEN_CONNS", 1000))
		sqlDB.SetConnMaxLifetime(time.Second * time.Duration(common.GetEnvOrDefault("SQL_MAX_LIFETIME", 60)))

		if !common.IsMasterNode {
			return nil
		}
		if common.UsingMySQL {
			//_, _ = sqlDB.Exec("ALTER TABLE channels MODIFY model_mapping TEXT;") // TODO: delete this line when most users have upgraded
		}
		common.SysLog("database migration started")
		err = migrateDB()
		return err
	} else {
		common.FatalLog(err)
	}
	return err
}

func InitLogDB() (err error) {
	if os.Getenv("LOG_SQL_DSN") == "" {
		LOG_DB = DB
		return
	}
	db, err := chooseDB("LOG_SQL_DSN", true)
	if err == nil {
		if common.DebugEnabled {
			db = db.Debug()
		}
		LOG_DB = db
		// If log DB is MySQL, also ensure Chinese-capable charset
		if common.LogSqlType == common.DatabaseTypeMySQL {
			if err := checkMySQLChineseSupport(LOG_DB); err != nil {
				panic(err)
			}
		}
		sqlDB, err := LOG_DB.DB()
		if err != nil {
			return err
		}
		sqlDB.SetMaxIdleConns(common.GetEnvOrDefault("SQL_MAX_IDLE_CONNS", 100))
		sqlDB.SetMaxOpenConns(common.GetEnvOrDefault("SQL_MAX_OPEN_CONNS", 1000))
		sqlDB.SetConnMaxLifetime(time.Second * time.Duration(common.GetEnvOrDefault("SQL_MAX_LIFETIME", 60)))

		if !common.IsMasterNode {
			return nil
		}
		common.SysLog("database migration started")
		err = migrateLOGDB()
		return err
	} else {
		common.FatalLog(err)
	}
	return err
}

func migrateDB() error {
	// Migrate price_amount column from float/double to decimal for existing tables
	migrateSubscriptionPlanPriceAmount()
	// Migrate model_limits column from varchar to text for existing tables
	if err := migrateTokenModelLimitsToText(); err != nil {
		return err
	}

	err := DB.AutoMigrate(
		&Channel{},
		&Token{},
		&User{},
		&PasskeyCredential{},
		&Option{},
		&Redemption{},
		&Ability{},
		&Log{},
		&Midjourney{},
		&TopUp{},
		&QuotaData{},
		&Task{},
		&Model{},
		&Vendor{},
		&PrefillGroup{},
		&Setup{},
		&TwoFA{},
		&TwoFABackupCode{},
		&Checkin{},
		&SubscriptionOrder{},
		&UserSubscription{},
		&SubscriptionPreConsumeRecord{},
		&CustomOAuthProvider{},
		&UserOAuthBinding{},
		&ModelMapping{},
		&CanvasSession{},
		&CanvasMessage{},
		&CanvasChatMessageAttachment{},
		&CanvasAssetCleanupJob{},
		&ImageGenerationTask{},
		&ImageGenerationReferenceAsset{},
		&ImageGenerationTaskReferenceAsset{},
		&ImageCreativeSubmission{},
	)
	if err != nil {
		return err
	}
	if common.UsingSQLite {
		if err := ensureSubscriptionPlanTableSQLite(); err != nil {
			return err
		}
	} else {
		if err := DB.AutoMigrate(&SubscriptionPlan{}); err != nil {
			return err
		}
	}
	return nil
}

func migrateDBFast() error {

	var wg sync.WaitGroup

	migrations := []struct {
		model interface{}
		name  string
	}{
		{&Channel{}, "Channel"},
		{&Token{}, "Token"},
		{&User{}, "User"},
		{&PasskeyCredential{}, "PasskeyCredential"},
		{&Option{}, "Option"},
		{&Redemption{}, "Redemption"},
		{&Ability{}, "Ability"},
		{&Log{}, "Log"},
		{&Midjourney{}, "Midjourney"},
		{&TopUp{}, "TopUp"},
		{&QuotaData{}, "QuotaData"},
		{&Task{}, "Task"},
		{&Model{}, "Model"},
		{&Vendor{}, "Vendor"},
		{&PrefillGroup{}, "PrefillGroup"},
		{&Setup{}, "Setup"},
		{&TwoFA{}, "TwoFA"},
		{&TwoFABackupCode{}, "TwoFABackupCode"},
		{&Checkin{}, "Checkin"},
		{&SubscriptionOrder{}, "SubscriptionOrder"},
		{&UserSubscription{}, "UserSubscription"},
		{&SubscriptionPreConsumeRecord{}, "SubscriptionPreConsumeRecord"},
		{&CustomOAuthProvider{}, "CustomOAuthProvider"},
		{&UserOAuthBinding{}, "UserOAuthBinding"},
		{&ModelMapping{}, "ModelMapping"},
		{&CanvasSession{}, "CanvasSession"},
		{&CanvasMessage{}, "CanvasMessage"},
		{&CanvasChatMessageAttachment{}, "CanvasChatMessageAttachment"},
		{&CanvasAssetCleanupJob{}, "CanvasAssetCleanupJob"},
		{&ImageGenerationTask{}, "ImageGenerationTask"},
		{&ImageGenerationReferenceAsset{}, "ImageGenerationReferenceAsset"},
		{&ImageGenerationTaskReferenceAsset{}, "ImageGenerationTaskReferenceAsset"},
		{&ImageCreativeSubmission{}, "ImageCreativeSubmission"},
	}
	// 动态计算migration数量，确保errChan缓冲区足够大
	errChan := make(chan error, len(migrations))

	for _, m := range migrations {
		wg.Add(1)
		go func(model interface{}, name string) {
			defer wg.Done()
			if err := DB.AutoMigrate(model); err != nil {
				errChan <- fmt.Errorf("failed to migrate %s: %v", name, err)
			}
		}(m.model, m.name)
	}

	// Wait for all migrations to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		if err != nil {
			return err
		}
	}
	if common.UsingSQLite {
		if err := ensureSubscriptionPlanTableSQLite(); err != nil {
			return err
		}
	} else {
		if err := DB.AutoMigrate(&SubscriptionPlan{}); err != nil {
			return err
		}
	}
	common.SysLog("database migrated")
	return nil
}

func migrateLOGDB() error {
	var err error
	if err = LOG_DB.AutoMigrate(&Log{}); err != nil {
		return err
	}
	return nil
}

type sqliteColumnDef struct {
	Name string
	DDL  string
}

func ensureSubscriptionPlanTableSQLite() error {
	if !common.UsingSQLite {
		return nil
	}
	tableName := "subscription_plans"
	if !DB.Migrator().HasTable(tableName) {
		createSQL := `CREATE TABLE ` + "`" + tableName + "`" + ` (
` + "`id`" + ` integer,
` + "`title`" + ` varchar(128) NOT NULL,
` + "`subtitle`" + ` varchar(255) DEFAULT '',
` + "`price_amount`" + ` decimal(10,6) NOT NULL,
` + "`currency`" + ` varchar(8) NOT NULL DEFAULT 'USD',
` + "`duration_unit`" + ` varchar(16) NOT NULL DEFAULT 'month',
` + "`duration_value`" + ` integer NOT NULL DEFAULT 1,
` + "`custom_seconds`" + ` bigint NOT NULL DEFAULT 0,
` + "`enabled`" + ` numeric DEFAULT 1,
` + "`sort_order`" + ` integer DEFAULT 0,
` + "`stripe_price_id`" + ` varchar(128) DEFAULT '',
` + "`creem_product_id`" + ` varchar(128) DEFAULT '',
` + "`max_purchase_per_user`" + ` integer DEFAULT 0,
` + "`upgrade_group`" + ` varchar(64) DEFAULT '',
` + "`total_amount`" + ` bigint NOT NULL DEFAULT 0,
` + "`quota_reset_period`" + ` varchar(16) DEFAULT 'never',
` + "`quota_reset_custom_seconds`" + ` bigint DEFAULT 0,
` + "`created_at`" + ` bigint,
` + "`updated_at`" + ` bigint,
PRIMARY KEY (` + "`id`" + `)
)`
		return DB.Exec(createSQL).Error
	}
	var cols []struct {
		Name string `gorm:"column:name"`
	}
	if err := DB.Raw("PRAGMA table_info(`" + tableName + "`)").Scan(&cols).Error; err != nil {
		return err
	}
	existing := make(map[string]struct{}, len(cols))
	for _, c := range cols {
		existing[c.Name] = struct{}{}
	}
	required := []sqliteColumnDef{
		{Name: "title", DDL: "`title` varchar(128) NOT NULL"},
		{Name: "subtitle", DDL: "`subtitle` varchar(255) DEFAULT ''"},
		{Name: "price_amount", DDL: "`price_amount` decimal(10,6) NOT NULL"},
		{Name: "currency", DDL: "`currency` varchar(8) NOT NULL DEFAULT 'USD'"},
		{Name: "duration_unit", DDL: "`duration_unit` varchar(16) NOT NULL DEFAULT 'month'"},
		{Name: "duration_value", DDL: "`duration_value` integer NOT NULL DEFAULT 1"},
		{Name: "custom_seconds", DDL: "`custom_seconds` bigint NOT NULL DEFAULT 0"},
		{Name: "enabled", DDL: "`enabled` numeric DEFAULT 1"},
		{Name: "sort_order", DDL: "`sort_order` integer DEFAULT 0"},
		{Name: "stripe_price_id", DDL: "`stripe_price_id` varchar(128) DEFAULT ''"},
		{Name: "creem_product_id", DDL: "`creem_product_id` varchar(128) DEFAULT ''"},
		{Name: "max_purchase_per_user", DDL: "`max_purchase_per_user` integer DEFAULT 0"},
		{Name: "upgrade_group", DDL: "`upgrade_group` varchar(64) DEFAULT ''"},
		{Name: "total_amount", DDL: "`total_amount` bigint NOT NULL DEFAULT 0"},
		{Name: "quota_reset_period", DDL: "`quota_reset_period` varchar(16) DEFAULT 'never'"},
		{Name: "quota_reset_custom_seconds", DDL: "`quota_reset_custom_seconds` bigint DEFAULT 0"},
		{Name: "created_at", DDL: "`created_at` bigint"},
		{Name: "updated_at", DDL: "`updated_at` bigint"},
	}
	for _, col := range required {
		if _, ok := existing[col.Name]; ok {
			continue
		}
		if err := DB.Exec("ALTER TABLE `" + tableName + "` ADD COLUMN " + col.DDL).Error; err != nil {
			return err
		}
	}
	return nil
}

// migrateTokenModelLimitsToText migrates model_limits column from varchar(1024) to text
// This is safe to run multiple times - it checks the column type first
func migrateTokenModelLimitsToText() error {
	// SQLite uses type affinity, so TEXT and VARCHAR are effectively the same — no migration needed
	if common.UsingSQLite {
		return nil
	}

	tableName := "tokens"
	columnName := "model_limits"

	if !DB.Migrator().HasTable(tableName) {
		return nil
	}

	if !DB.Migrator().HasColumn(&Token{}, columnName) {
		return nil
	}

	var alterSQL string
	if common.UsingPostgreSQL {
		var dataType string
		if err := DB.Raw(`SELECT data_type FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&dataType).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to query metadata for %s.%s: %v", tableName, columnName, err))
		} else if dataType == "text" {
			return nil
		}
		alterSQL = fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s TYPE text`, tableName, columnName)
	} else if common.UsingMySQL {
		var columnType string
		if err := DB.Raw(`SELECT COLUMN_TYPE FROM information_schema.columns
				WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&columnType).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to query metadata for %s.%s: %v", tableName, columnName, err))
		} else if strings.ToLower(columnType) == "text" {
			return nil
		}
		alterSQL = fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s text", tableName, columnName)
	} else {
		return nil
	}

	if alterSQL != "" {
		if err := DB.Exec(alterSQL).Error; err != nil {
			return fmt.Errorf("failed to migrate %s.%s to text: %w", tableName, columnName, err)
		}
		common.SysLog(fmt.Sprintf("Successfully migrated %s.%s to text", tableName, columnName))
	}
	return nil
}

// migrateSubscriptionPlanPriceAmount migrates price_amount column from float/double to decimal(10,6)
// This is safe to run multiple times - it checks the column type first
func migrateSubscriptionPlanPriceAmount() {
	// SQLite doesn't support ALTER COLUMN, and its type affinity handles this automatically
	// Skip early to avoid GORM parsing the existing table DDL which may cause issues
	if common.UsingSQLite {
		return
	}

	tableName := "subscription_plans"
	columnName := "price_amount"

	// Check if table exists first
	if !DB.Migrator().HasTable(tableName) {
		return
	}

	// Check if column exists
	if !DB.Migrator().HasColumn(&SubscriptionPlan{}, columnName) {
		return
	}

	var alterSQL string
	if common.UsingPostgreSQL {
		// PostgreSQL: Check if already decimal/numeric
		var dataType string
		if err := DB.Raw(`SELECT data_type FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&dataType).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to query metadata for %s.%s: %v", tableName, columnName, err))
		} else if dataType == "numeric" {
			return // Already decimal/numeric
		}
		alterSQL = fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s TYPE decimal(10,6) USING %s::decimal(10,6)`,
			tableName, columnName, columnName)
	} else if common.UsingMySQL {
		// MySQL: Check if already decimal
		var columnType string
		if err := DB.Raw(`SELECT COLUMN_TYPE FROM information_schema.columns
				WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&columnType).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to query metadata for %s.%s: %v", tableName, columnName, err))
		} else if strings.HasPrefix(strings.ToLower(columnType), "decimal") {
			return // Already decimal
		}
		alterSQL = fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s decimal(10,6) NOT NULL DEFAULT 0",
			tableName, columnName)
	} else {
		return
	}

	if alterSQL != "" {
		if err := DB.Exec(alterSQL).Error; err != nil {
			common.SysLog(fmt.Sprintf("Warning: failed to migrate %s.%s to decimal: %v", tableName, columnName, err))
		} else {
			common.SysLog(fmt.Sprintf("Successfully migrated %s.%s to decimal(10,6)", tableName, columnName))
		}
	}
}

func closeDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	err = sqlDB.Close()
	return err
}

func CloseDB() error {
	if err := closeCanvasMessageDBs(); err != nil {
		return err
	}
	if LOG_DB != DB {
		err := closeDB(LOG_DB)
		if err != nil {
			return err
		}
	}
	return closeDB(DB)
}

// checkMySQLChineseSupport ensures the MySQL connection and current schema
// default charset/collation can store Chinese characters. It allows common
// Chinese-capable charsets (utf8mb4, utf8, gbk, big5, gb18030) and panics otherwise.
func checkMySQLChineseSupport(db *gorm.DB) error {
	// 仅检测：当前库默认字符集/排序规则 + 各表的排序规则（隐含字符集）

	// Read current schema defaults
	var schemaCharset, schemaCollation string
	err := db.Raw("SELECT DEFAULT_CHARACTER_SET_NAME, DEFAULT_COLLATION_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = DATABASE()").Row().Scan(&schemaCharset, &schemaCollation)
	if err != nil {
		return fmt.Errorf("读取当前库默认字符集/排序规则失败 / Failed to read schema default charset/collation: %v", err)
	}

	toLower := func(s string) string { return strings.ToLower(s) }
	// Allowed charsets that can store Chinese text
	allowedCharsets := map[string]string{
		"utf8mb4": "utf8mb4_",
		"utf8":    "utf8_",
		"gbk":     "gbk_",
		"big5":    "big5_",
		"gb18030": "gb18030_",
	}
	isChineseCapable := func(cs, cl string) bool {
		csLower := toLower(cs)
		clLower := toLower(cl)
		if prefix, ok := allowedCharsets[csLower]; ok {
			if clLower == "" {
				return true
			}
			return strings.HasPrefix(clLower, prefix)
		}
		// 如果仅提供了排序规则，尝试按排序规则前缀判断
		for _, prefix := range allowedCharsets {
			if strings.HasPrefix(clLower, prefix) {
				return true
			}
		}
		return false
	}

	// 1) 当前库默认值必须支持中文
	if !isChineseCapable(schemaCharset, schemaCollation) {
		return fmt.Errorf("当前库默认字符集/排序规则不支持中文：schema(%s/%s)。请将库设置为 utf8mb4/utf8/gbk/big5/gb18030 / Schema default charset/collation is not Chinese-capable: schema(%s/%s). Please set to utf8mb4/utf8/gbk/big5/gb18030",
			schemaCharset, schemaCollation, schemaCharset, schemaCollation)
	}

	// 2) 所有物理表的排序规则（隐含字符集）必须支持中文
	type tableInfo struct {
		Name      string
		Collation *string
	}
	var tables []tableInfo
	if err := db.Raw("SELECT TABLE_NAME, TABLE_COLLATION FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE'").Scan(&tables).Error; err != nil {
		return fmt.Errorf("读取表排序规则失败 / Failed to read table collations: %v", err)
	}

	var badTables []string
	for _, t := range tables {
		// NULL 或空表示继承库默认设置，已在上面校验库默认，视为通过
		if t.Collation == nil || *t.Collation == "" {
			continue
		}
		cl := *t.Collation
		// 仅凭排序规则判断是否中文可用
		ok := false
		lower := strings.ToLower(cl)
		for _, prefix := range allowedCharsets {
			if strings.HasPrefix(lower, prefix) {
				ok = true
				break
			}
		}
		if !ok {
			badTables = append(badTables, fmt.Sprintf("%s(%s)", t.Name, cl))
		}
	}

	if len(badTables) > 0 {
		// 限制输出数量以避免日志过长
		maxShow := 20
		shown := badTables
		if len(shown) > maxShow {
			shown = shown[:maxShow]
		}
		return fmt.Errorf(
			"存在不支持中文的表，请修复其排序规则/字符集。示例（最多展示 %d 项）：%v / Found tables not Chinese-capable. Please fix their collation/charset. Examples (showing up to %d): %v",
			maxShow, shown, maxShow, shown,
		)
	}
	return nil
}

var (
	lastPingTime time.Time
	pingMutex    sync.Mutex
)

func PingDB() error {
	pingMutex.Lock()
	defer pingMutex.Unlock()

	if time.Since(lastPingTime) < time.Second*10 {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		log.Printf("Error getting sql.DB from GORM: %v", err)
		return err
	}

	err = sqlDB.Ping()
	if err != nil {
		log.Printf("Error pinging DB: %v", err)
		return err
	}

	lastPingTime = time.Now()
	common.SysLog("Database pinged successfully")
	return nil
}
