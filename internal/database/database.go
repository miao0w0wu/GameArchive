// Package database 负责 SQLite 数据库的连接、建表迁移与初始化数据。
//
// 数据库文件位于用户配置目录：os.UserConfigDir()/GameArchive/game_archive.db
// 例如 Windows 下为 %AppData%\GameArchive\game_archive.db。
package database

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite" // 纯 Go SQLite 驱动，无需 CGO
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"GameArchive/internal/models"
)

const (
	// AppDirName 应用数据目录名。
	AppDirName = "GameArchive"
	// DBFileName 数据库文件名。
	DBFileName = "game_archive.db"
)

// Paths 记录应用运行期使用到的关键路径，供设置页展示。
type Paths struct {
	DataDir string
	DBPath  string
}

// Open 打开（必要时创建）数据库，完成建表迁移与初始化数据。
func Open() (*gorm.DB, Paths, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return nil, Paths{}, fmt.Errorf("获取用户配置目录失败: %w", err)
	}

	dataDir := filepath.Join(cfgDir, AppDirName)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, Paths{}, fmt.Errorf("创建数据目录失败: %w", err)
	}

	dbPath := filepath.Join(dataDir, DBFileName)
	paths := Paths{DataDir: dataDir, DBPath: dbPath}

	// 通过 DSN 设置连接级 pragma：超时等待、WAL 日志、外键约束。
	dsn := fmt.Sprintf(
		"file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)",
		filepath.ToSlash(dbPath),
	)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, paths, fmt.Errorf("打开数据库失败: %w", err)
	}

	if err := migrate(db); err != nil {
		return nil, paths, err
	}
	if err := seed(db); err != nil {
		return nil, paths, err
	}
	return db, paths, nil
}

// migrate 建表 / 补齐字段，可重复执行。
func migrate(db *gorm.DB) error {
	err := db.AutoMigrate(
		&models.Category{},
		&models.Tag{},
		&models.Genre{},
		&models.GameGenre{},
		&models.Game{}, // 同时会创建多对多关联表 game_tags
		&models.PlaySession{},
		&models.Setting{},
		&models.Report{},
		&models.SaveArchive{},
		&models.SaveBackup{},
	)
	if err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}
	if err := db.Exec("UPDATE games SET cover_source = 'manual' WHERE cover_path <> '' AND cover_source = 'none'").Error; err != nil {
		return fmt.Errorf("迁移既有游戏封面来源失败: %w", err)
	}
	if err := db.Exec("UPDATE games SET icon_source = 'manual' WHERE icon_path <> '' AND icon_source = 'none'").Error; err != nil {
		return fmt.Errorf("迁移既有游戏图标来源失败: %w", err)
	}
	return nil
}

// seed 首次运行时写入一批常用分类，方便用户直接使用；已有数据时不覆盖。
func seed(db *gorm.DB) error {
	var count int64
	if err := db.Model(&models.Category{}).Count(&count).Error; err != nil {
		return fmt.Errorf("统计分类数量失败: %w", err)
	}
	if count > 0 {
		return nil
	}

	presets := []models.Category{
		{Name: "动作", Color: "#ef4444", SortOrder: 10},
		{Name: "角色扮演", Color: "#8b5cf6", SortOrder: 20},
		{Name: "射击", Color: "#f97316", SortOrder: 30},
		{Name: "策略", Color: "#0ea5e9", SortOrder: 40},
		{Name: "模拟经营", Color: "#14b8a6", SortOrder: 50},
		{Name: "冒险", Color: "#22c55e", SortOrder: 60},
		{Name: "竞速体育", Color: "#eab308", SortOrder: 70},
		{Name: "独立游戏", Color: "#ec4899", SortOrder: 80},
		{Name: "休闲", Color: "#a3a3a3", SortOrder: 90},
	}
	if err := db.Create(&presets).Error; err != nil {
		return fmt.Errorf("写入默认分类失败: %w", err)
	}
	return nil
}
