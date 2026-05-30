package main

import (
	"strings"
	"testing"
)

func TestRunChecksMigrationBeforeServing(t *testing.T) {
	t.Setenv("STORAGE_DRIVER", "sqlite")
	t.Setenv("DATABASE_DSN", "file:main_run_migration_ready?mode=memory&cache=shared")
	t.Setenv("DB_AUTO_MIGRATE", "false")
	t.Setenv("ADMIN_USERNAME", "")
	t.Setenv("ADMIN_PASSWORD", "")
	t.Setenv("JWT_SECRET", "test-secret")

	started := false
	err := runWithServer(nil, func() error {
		started = true
		return nil
	})

	if err == nil {
		t.Fatal("expected startup to fail before serving when database has not been migrated")
	}
	if started {
		t.Fatal("server should not start when database has not been migrated")
	}
	if !strings.Contains(err.Error(), "数据库未完成迁移") {
		t.Fatalf("expected migration hint, got %v", err)
	}

	if err := run([]string{"migrate"}); err != nil {
		t.Fatal(err)
	}

	err = runWithServer(nil, func() error {
		started = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !started {
		t.Fatal("server should start after explicit migration")
	}
}
