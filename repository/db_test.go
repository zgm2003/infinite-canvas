package repository

import (
	"path/filepath"
	"testing"

	"github.com/basketikun/infinite-canvas/config"
	"github.com/basketikun/infinite-canvas/model"
)

func TestDBCanOpenWithoutAutoMigrateAndMigrateExplicitly(t *testing.T) {
	resetDBForTest()
	tempDir := t.TempDir()
	t.Cleanup(resetDBForTest)
	config.Cfg = config.Config{
		StorageDriver: "sqlite",
		DatabaseDSN:   filepath.Join(tempDir, "test.db"),
		AutoMigrate:   false,
	}

	database, err := DB()
	if err != nil {
		t.Fatal(err)
	}
	if database.Migrator().HasTable(&model.User{}) {
		t.Fatal("DB() should not create tables when AutoMigrate is disabled")
	}

	if err := Migrate(); err != nil {
		t.Fatal(err)
	}
	if !database.Migrator().HasTable(&model.User{}) {
		t.Fatal("Migrate() should create application tables")
	}
}

func TestEnsureMigratedRejectsStaleSchema(t *testing.T) {
	resetDBForTest()
	t.Cleanup(resetDBForTest)
	config.Cfg = config.Config{
		StorageDriver: "sqlite",
		DatabaseDSN:   "file:repository_stale_schema?mode=memory&cache=shared",
		AutoMigrate:   false,
	}
	database, err := DB()
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		"CREATE TABLE users (id text primary key)",
		"CREATE TABLE credit_logs (id text primary key)",
		"CREATE TABLE prompts (id text primary key)",
		"CREATE TABLE assets (id text primary key)",
		"CREATE TABLE settings (key text primary key)",
	} {
		if err := database.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := EnsureMigrated(); err == nil {
		t.Fatal("expected stale schema with missing columns to fail migration check")
	}
}

func TestDBRetriesAfterInitialOpenFailure(t *testing.T) {
	resetDBForTest()
	t.Cleanup(resetDBForTest)
	config.Cfg = config.Config{
		StorageDriver: "mysql",
		DatabaseDSN:   "bad-dsn",
		AutoMigrate:   false,
	}
	if _, err := DB(); err == nil {
		t.Fatal("expected initial DB open with bad DSN to fail")
	}

	config.Cfg = config.Config{
		StorageDriver: "sqlite",
		DatabaseDSN:   ":memory:",
		AutoMigrate:   false,
	}
	if _, err := DB(); err != nil {
		t.Fatalf("expected DB() to retry after initial failure, got %v", err)
	}
}

func TestDBReopensWhenConfigChanges(t *testing.T) {
	resetDBForTest()
	tempDir := t.TempDir()
	t.Cleanup(resetDBForTest)
	config.Cfg = config.Config{
		StorageDriver: "sqlite",
		DatabaseDSN:   filepath.Join(tempDir, "first.db"),
		AutoMigrate:   false,
	}
	first, err := DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Exec("CREATE TABLE marker (id text primary key)").Error; err != nil {
		t.Fatal(err)
	}

	config.Cfg = config.Config{
		StorageDriver: "sqlite",
		DatabaseDSN:   filepath.Join(tempDir, "second.db"),
		AutoMigrate:   false,
	}
	second, err := DB()
	if err != nil {
		t.Fatal(err)
	}
	if second.Migrator().HasTable("marker") {
		t.Fatal("expected DB() to reopen when database config changes")
	}
}

func resetDBForTest() {
	dbMu.Lock()
	defer dbMu.Unlock()
	if db != nil {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
	db = nil
	dbKey = ""
}
