package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"

	"GameArchive/internal/database"
	"GameArchive/internal/models"
	"GameArchive/internal/services"
)

const (
	// AppVersion 应用版本号。
	AppVersion = "0.1.0"
	// StageName 当前实现阶段，展示在设置页以便区分尚未实现的功能。
	StageName = "阶段 1：项目初始化 + SQLite + 游戏 / 分类 / 标签 CRUD"
)

// App 是绑定给前端的应用对象。
// 这里的导出方法会被 Wails 生成到 frontend/wailsjs/go/main/App 中供前端调用。
type App struct {
	ctx        context.Context
	db         *gorm.DB
	paths      database.Paths
	svc        *services.Services
	startupErr error
}

// NewApp 创建应用对象。
func NewApp() *App {
	return &App{}
}

// startup 应用启动：初始化数据库并构建服务层。
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	db, paths, err := database.Open()
	if err != nil {
		a.startupErr = err
		runtime.LogErrorf(ctx, "初始化数据库失败: %v", err)
		return
	}

	a.db = db
	a.paths = paths
	a.svc = services.New(db)
	runtime.LogInfof(ctx, "数据库已就绪: %s", paths.DBPath)
}

// shutdown 应用退出：关闭数据库连接，确保数据落盘。
func (a *App) shutdown(ctx context.Context) {
	if a.db == nil {
		return
	}
	sqlDB, err := a.db.DB()
	if err != nil {
		runtime.LogErrorf(ctx, "获取数据库连接失败: %v", err)
		return
	}
	if err := sqlDB.Close(); err != nil {
		runtime.LogErrorf(ctx, "关闭数据库失败: %v", err)
	}
}

// ready 确认数据库已就绪，未就绪时返回可读的错误信息。
func (a *App) ready() error {
	if a.startupErr != nil {
		return fmt.Errorf("数据库初始化失败: %w", a.startupErr)
	}
	if a.svc == nil {
		return errors.New("数据库尚未就绪，请重启应用")
	}
	return nil
}

// AppInfo 应用信息，供设置页展示。
type AppInfo struct {
	AppName       string `json:"appName"`
	Version       string `json:"version"`
	Stage         string `json:"stage"`
	DataDir       string `json:"dataDir"`
	DBPath        string `json:"dbPath"`
	Ready         bool   `json:"ready"`
	StartupError  string `json:"startupError"`
	GameCount     int64  `json:"gameCount"`
	CategoryCount int64  `json:"categoryCount"`
	TagCount      int64  `json:"tagCount"`
}

// GetAppInfo 返回应用运行环境与基础统计信息。
// 即使数据库初始化失败也会正常返回，便于前端展示错误原因。
func (a *App) GetAppInfo() AppInfo {
	info := AppInfo{
		AppName: "游戏档案",
		Version: AppVersion,
		Stage:   StageName,
		DataDir: a.paths.DataDir,
		DBPath:  a.paths.DBPath,
	}
	if err := a.ready(); err != nil {
		info.StartupError = err.Error()
		return info
	}

	info.Ready = true
	counters := []struct {
		model any
		dst   *int64
	}{
		{&models.Game{}, &info.GameCount},
		{&models.Category{}, &info.CategoryCount},
		{&models.Tag{}, &info.TagCount},
	}
	for _, c := range counters {
		if err := a.db.Model(c.model).Count(c.dst).Error; err != nil {
			runtime.LogErrorf(a.ctx, "统计数量失败: %v", err)
		}
	}
	return info
}

// ------------------------------------------------------------- 游戏 CRUD

// ListGames 返回全部游戏（含分类与标签）。
func (a *App) ListGames() ([]models.Game, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.Game.List()
}

// GetGameDetail 返回单个游戏的详情。
func (a *App) GetGameDetail(id uint) (*services.GameDetail, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.Game.Detail(id)
}

// AddGame 新增游戏，返回创建后的完整记录。
func (a *App) AddGame(game models.Game) (*models.Game, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.Game.Add(game)
}

// UpdateGame 更新游戏基础信息与标签，返回更新后的完整记录。
func (a *App) UpdateGame(game models.Game) (*models.Game, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.Game.Update(game)
}

// DeleteGame 删除游戏（不会删除磁盘上的游戏文件或存档文件）。
func (a *App) DeleteGame(id uint) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.svc.Game.Delete(id)
}

// ------------------------------------------------------------- 分类 CRUD

// ListCategories 返回全部分类。
func (a *App) ListCategories() ([]models.Category, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.Category.List()
}

// AddCategory 新增分类。
func (a *App) AddCategory(category models.Category) (*models.Category, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.Category.Add(category)
}

// UpdateCategory 更新分类。
func (a *App) UpdateCategory(category models.Category) (*models.Category, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.Category.Update(category)
}

// DeleteCategory 删除分类，原属该分类的游戏会变为「未分类」。
func (a *App) DeleteCategory(id uint) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.svc.Category.Delete(id)
}

// ------------------------------------------------------------- 标签 CRUD

// ListTags 返回全部标签。
func (a *App) ListTags() ([]models.Tag, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.Tag.List()
}

// AddTag 新增标签。
func (a *App) AddTag(tag models.Tag) (*models.Tag, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.Tag.Add(tag)
}

// UpdateTag 更新标签。
func (a *App) UpdateTag(tag models.Tag) (*models.Tag, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.Tag.Update(tag)
}

// DeleteTag 删除标签，并解除其与所有游戏的关联。
func (a *App) DeleteTag(id uint) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.svc.Tag.Delete(id)
}
