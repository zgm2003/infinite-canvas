package handler

import (
	"testing"

	"github.com/basketikun/infinite-canvas/config"
	"github.com/basketikun/infinite-canvas/repository"
)

func setupHandlerTestDB(t *testing.T) {
	t.Helper()
	config.Cfg = config.Config{
		StorageDriver: "sqlite",
		DatabaseDSN:   "file:handler_tests?mode=memory&cache=shared",
		AutoMigrate:   true,
	}
	db, err := repository.DB()
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"credit_logs", "users", "settings", "prompts", "assets"} {
		if err := db.Exec("DELETE FROM " + table).Error; err != nil {
			t.Fatal(err)
		}
	}
}
