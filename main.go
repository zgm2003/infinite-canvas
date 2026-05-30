package main

import (
	"fmt"
	"log"
	"os"

	"github.com/basketikun/infinite-canvas/config"
	"github.com/basketikun/infinite-canvas/repository"
	"github.com/basketikun/infinite-canvas/router"
	"github.com/basketikun/infinite-canvas/service"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	return runWithServer(args, func() error {
		return router.New().Run(":" + config.Cfg.Port)
	})
}

func runWithServer(args []string, startServer func() error) error {
	if err := config.Load(); err != nil {
		return err
	}
	if len(args) > 0 && args[0] == "migrate" {
		return repository.Migrate()
	}
	if err := repository.EnsureMigrated(); err != nil {
		if !config.Cfg.AutoMigrate {
			return fmt.Errorf("数据库未完成迁移，请先执行 go run . migrate，或在开发环境设置 DB_AUTO_MIGRATE=true: %w", err)
		}
		return err
	}
	if err := service.EnsureDefaultAdmin(); err != nil {
		return err
	}
	service.StartPromptSyncScheduler()
	defer service.StopPromptSyncScheduler()
	return startServer()
}
