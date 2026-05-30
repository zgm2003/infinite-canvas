package service

import (
	"testing"

	"github.com/basketikun/infinite-canvas/config"
	"github.com/basketikun/infinite-canvas/repository"
)

func setupServiceTestDB(t *testing.T) {
	t.Helper()
	config.Cfg = config.Config{
		StorageDriver: "sqlite",
		DatabaseDSN:   "file:service_tests?mode=memory&cache=shared",
		AutoMigrate:   true,
		JWTSecret:     "test-secret",
	}
	db, err := repository.DB()
	if err != nil {
		t.Fatal(err)
	}
	for _, trigger := range []string{
		"block_admin_adjust_credit_log_insert",
		"block_credit_log_insert",
		"block_refund_credit_log_insert",
		"block_video_refund_credit_log_insert",
	} {
		if err := db.Exec("DROP TRIGGER IF EXISTS " + trigger).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, table := range []string{"credit_logs", "users", "settings", "prompts", "assets"} {
		if err := db.Exec("DELETE FROM " + table).Error; err != nil {
			t.Fatal(err)
		}
	}
}
