package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupModelMetaTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	previousDB := DB
	previousLogDB := LOG_DB
	previousUsingSQLite := common.UsingSQLite
	previousUsingMySQL := common.UsingMySQL
	previousUsingPostgreSQL := common.UsingPostgreSQL

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	DB = db
	LOG_DB = db

	if err := db.AutoMigrate(&Vendor{}, &Model{}); err != nil {
		t.Fatalf("failed to migrate model metadata tables: %v", err)
	}

	t.Cleanup(func() {
		DB = previousDB
		LOG_DB = previousLogDB
		common.UsingSQLite = previousUsingSQLite
		common.UsingMySQL = previousUsingMySQL
		common.UsingPostgreSQL = previousUsingPostgreSQL

		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestGetCatalogDisplayMetadataByModelNamesMatchesNameRules(t *testing.T) {
	db := setupModelMetaTestDB(t)

	vendors := []*Vendor{
		{Name: "OpenAI", Icon: "OpenAI", Status: 1},
		{Name: "Anthropic", Icon: "Claude.Color", Status: 1},
		{Name: "Google", Icon: "Gemini.Color", Status: 1},
	}
	for _, vendor := range vendors {
		if err := db.Create(vendor).Error; err != nil {
			t.Fatalf("failed to create vendor %q: %v", vendor.Name, err)
		}
	}

	entries := []*Model{
		{
			ModelName:    "gpt-4o",
			Description:  "prefix description",
			VendorID:     vendors[0].Id,
			NameRule:     NameRulePrefix,
			Status:       1,
			SyncOfficial: 1,
		},
		{
			ModelName:    "gpt-4o-mini",
			Description:  "exact description",
			VendorID:     vendors[0].Id,
			NameRule:     NameRuleExact,
			Status:       1,
			SyncOfficial: 1,
		},
		{
			ModelName:    "alpha",
			Description:  "suffix description",
			VendorID:     vendors[1].Id,
			NameRule:     NameRuleSuffix,
			Status:       1,
			SyncOfficial: 1,
		},
		{
			ModelName:    "vision",
			Description:  "contains description",
			VendorID:     vendors[2].Id,
			NameRule:     NameRuleContains,
			Status:       1,
			SyncOfficial: 1,
		},
	}
	for _, entry := range entries {
		if err := db.Create(entry).Error; err != nil {
			t.Fatalf("failed to create model metadata %q: %v", entry.ModelName, err)
		}
	}

	metadataByModel, err := GetCatalogDisplayMetadataByModelNames([]string{
		"gpt-4o-mini",
		"gpt-4o-realtime",
		"video-alpha",
		"my-vision-model",
		"unmatched-model",
	})
	if err != nil {
		t.Fatalf("expected metadata lookup to succeed: %v", err)
	}

	if metadataByModel["gpt-4o-mini"].Description != "exact description" || metadataByModel["gpt-4o-mini"].VendorIcon != "OpenAI" {
		t.Fatalf("expected exact rule to win, got %#v", metadataByModel["gpt-4o-mini"])
	}
	if metadataByModel["gpt-4o-realtime"].Description != "prefix description" || metadataByModel["gpt-4o-realtime"].VendorIcon != "OpenAI" {
		t.Fatalf("expected prefix rule match, got %#v", metadataByModel["gpt-4o-realtime"])
	}
	if metadataByModel["video-alpha"].Description != "suffix description" || metadataByModel["video-alpha"].VendorIcon != "Claude.Color" {
		t.Fatalf("expected suffix rule match, got %#v", metadataByModel["video-alpha"])
	}
	if metadataByModel["my-vision-model"].Description != "contains description" || metadataByModel["my-vision-model"].VendorIcon != "Gemini.Color" {
		t.Fatalf("expected contains rule match, got %#v", metadataByModel["my-vision-model"])
	}
	if _, ok := metadataByModel["unmatched-model"]; ok {
		t.Fatalf("expected unmatched model to be omitted, got %#v", metadataByModel["unmatched-model"])
	}
}
