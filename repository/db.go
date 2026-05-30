package repository

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/basketikun/infinite-canvas/config"
	"github.com/basketikun/infinite-canvas/model"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var promptCategories = []model.PromptCategory{
	{Category: "system", Name: "系统", Description: "系统提示词分类"},
	{Category: "gpt-image-2-prompts", Name: "GPT Image 2 Prompts", Description: "EvoLinkAI 的 GPT Image 2 案例提示词分类", GithubURL: "https://github.com/EvoLinkAI/awesome-gpt-image-2-API-and-Prompts", Remote: true},
	{Category: "awesome-gpt-image", Name: "Awesome GPT Image", Description: "ZeroLu 的中文 GPT Image 提示词分类", GithubURL: "https://github.com/ZeroLu/awesome-gpt-image", Remote: true},
	{Category: "awesome-gpt4o-image-prompts", Name: "Awesome GPT4o Image Prompts", Description: "ImgEdify 的 GPT-4o 图像提示词分类", GithubURL: "https://github.com/ImgEdify/Awesome-GPT4o-Image-Prompts", Remote: true},
	{Category: "youmind-gpt-image-2", Name: "YouMind GPT Image 2", Description: "YouMind OpenLab 的 GPT Image 2 中文提示词分类", GithubURL: "https://github.com/YouMind-OpenLab/awesome-gpt-image-2", Remote: true},
	{Category: "youmind-nano-banana-pro", Name: "YouMind Nano Banana Pro", Description: "YouMind OpenLab 的 Nano Banana Pro 中文提示词分类", GithubURL: "https://github.com/YouMind-OpenLab/awesome-nano-banana-pro-prompts", Remote: true},
	{Category: "davidwu-gpt-image2-prompts", Name: "awesome-gpt-image2-prompts", Description: "davidwuw0811-boop 整理的 GPT Image 2 提示词分类", GithubURL: "https://github.com/davidwuw0811-boop/awesome-gpt-image2-prompts", Remote: true},
}

var (
	db    *gorm.DB
	dbKey string
	dbMu  sync.Mutex
)

// DB 初始化并返回全局数据库连接。
func DB() (*gorm.DB, error) {
	dbMu.Lock()
	defer dbMu.Unlock()
	driver := strings.ToLower(strings.TrimSpace(config.Cfg.StorageDriver))
	if driver == "" {
		driver = "sqlite"
	}
	dsn := config.Cfg.DatabaseDSN
	key := fmt.Sprintf("%s\x00%s\x00%t\x00%d\x00%d\x00%s", driver, dsn, config.Cfg.AutoMigrate, config.Cfg.MySQLMaxOpenConns, config.Cfg.MySQLMaxIdleConns, config.Cfg.MySQLConnMaxLifetime)
	if db != nil && dbKey == key {
		return db, nil
	}
	if db != nil && dbKey != key {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		db = nil
		dbKey = ""
	}
	if driver == "sqlite" && dsn != ":memory:" {
		_ = os.MkdirAll(filepath.Dir(dsn), 0755)
	}
	database, err := gorm.Open(dialector(driver, dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := applyPoolConfig(driver, database); err != nil {
		if sqlDB, dbErr := database.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
		return nil, err
	}
	if config.Cfg.AutoMigrate {
		if err := migrate(database); err != nil {
			if sqlDB, dbErr := database.DB(); dbErr == nil {
				_ = sqlDB.Close()
			}
			return nil, err
		}
	}
	db = database
	dbKey = key
	return db, nil
}

func Migrate() error {
	database, err := DB()
	if err != nil {
		return err
	}
	return migrate(database)
}

func EnsureMigrated() error {
	database, err := DB()
	if err != nil {
		return err
	}
	for _, item := range migratedModels() {
		if !database.Migrator().HasTable(item) {
			return fmt.Errorf("missing table for %T", item)
		}
		statement := &gorm.Statement{DB: database}
		if err := statement.Parse(item); err != nil {
			return err
		}
		for _, field := range statement.Schema.Fields {
			if field.DBName == "" {
				continue
			}
			if !database.Migrator().HasColumn(item, field.DBName) {
				return fmt.Errorf("missing column %s.%s", statement.Schema.Table, field.DBName)
			}
		}
	}
	return nil
}

func migrate(database *gorm.DB) error {
	return database.AutoMigrate(migratedModels()...)
}

func migratedModels() []any {
	return []any{
		&model.User{},
		&model.CreditLog{},
		&model.Prompt{},
		&model.Asset{},
		&model.Setting{},
	}
}

func applyPoolConfig(driver string, database *gorm.DB) error {
	if driver != "mysql" {
		return nil
	}
	sqlDB, err := database.DB()
	if err != nil {
		return err
	}
	if config.Cfg.MySQLMaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(config.Cfg.MySQLMaxOpenConns)
	}
	if config.Cfg.MySQLMaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(config.Cfg.MySQLMaxIdleConns)
	}
	if config.Cfg.MySQLConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(config.Cfg.MySQLConnMaxLifetime)
	}
	return nil
}

func dialector(driver string, dsn string) gorm.Dialector {
	switch driver {
	case "mysql":
		return mysql.Open(dsn)
	case "postgres", "postgresql":
		return postgres.Open(dsn)
	default:
		return sqlite.Open(dsn)
	}
}
