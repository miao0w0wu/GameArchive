// Package services 是业务逻辑层：所有数据库读写都集中在这里，
// 由 main 包的 App 绑定层暴露给前端调用。
package services

import (
	"errors"

	"gorm.io/gorm"
)

// ErrNotFound 记录不存在的统一错误，前端可直接展示。
var ErrNotFound = errors.New("记录不存在")

// Services 聚合各业务服务，便于在绑定层统一持有。
type Services struct {
	Game     *GameService
	Category *CategoryService
	Tag      *TagService
	Scanner  *ScannerService
	Tracker  *Tracker
	Stats    *StatsService
	Report   *ReportService
	Archive  *ArchiveService
	Steam    *SteamService
}

// New 创建业务服务集合。
func New(db *gorm.DB) *Services {
	return &Services{
		Game:     &GameService{db: db},
		Category: &CategoryService{db: db},
		Tag:      &TagService{db: db},
		Scanner:  &ScannerService{},
		Tracker:  NewTracker(db, nil, nil),
		Stats:    &StatsService{db: db},
		Report:   NewReportService(db),
		Archive:  &ArchiveService{db: db},
		Steam:    NewSteamService(db),
	}
}
