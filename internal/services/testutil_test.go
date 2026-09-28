package services

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"GameArchive/internal/models"
)

// newTestDB 创建一个内存 SQLite 数据库并建好业务表，供各服务的单元测试共用。
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	err = db.AutoMigrate(
		&models.Category{},
		&models.Tag{},
		&models.Game{},
		&models.PlaySession{},
		&models.Setting{},
		&models.Report{},
		&models.SaveArchive{},
		&models.SaveBackup{},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}
