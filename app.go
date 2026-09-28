package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	StageName = "阶段 3：统计图表 + 按年 / 月 / 周分析"
)

const (
	monitorEnabledSetting = "monitor_enabled"
	gameDirectorySetting  = "game_scan_directory"
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
	a.svc.Tracker.SetErrorHandler(func(err error) {
		runtime.LogErrorf(ctx, "进程监控失败: %v", err)
	})
	a.svc.Tracker.SetUpdateHandler(func(status services.MonitorStatus) {
		runtime.EventsEmit(ctx, "tracker:update", status)
	})
	if err := a.svc.Tracker.Recover(); err != nil {
		runtime.LogErrorf(ctx, "恢复未结束的游玩记录失败: %v", err)
	} else if enabled, err := a.loadMonitorEnabled(); err != nil {
		a.startupErr = err
		runtime.LogErrorf(ctx, "读取监控设置失败: %v", err)
	} else if enabled {
		a.svc.Tracker.Start()
	}
	runtime.LogInfof(ctx, "数据库已就绪: %s", paths.DBPath)
}

// shutdown 应用退出：关闭数据库连接，确保数据落盘。
func (a *App) shutdown(ctx context.Context) {
	if a.svc != nil && a.svc.Tracker != nil {
		if err := a.svc.Tracker.Stop(); err != nil {
			runtime.LogErrorf(ctx, "收尾游玩记录失败: %v", err)
		}
	}
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

func (a *App) loadMonitorEnabled() (bool, error) {
	var setting models.Setting
	err := a.db.First(&setting, "key = ?", monitorEnabledSetting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := a.saveSetting(monitorEnabledSetting, "true"); err != nil {
			return false, err
		}
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("查询监控设置失败: %w", err)
	}
	return setting.Value == "true", nil
}

func (a *App) saveSetting(key, value string) error {
	setting := models.Setting{Key: key, Value: value}
	if err := a.db.Save(&setting).Error; err != nil {
		return fmt.Errorf("保存设置失败: %w", err)
	}
	return nil
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

// SelectGameDirectory 打开系统目录选择框，供用户选择待扫描的游戏目录。
func (a *App) SelectGameDirectory() (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择游戏目录",
	})
	if err != nil {
		return "", fmt.Errorf("选择游戏目录失败: %w", err)
	}
	return dir, nil
}

// AddGameDirectory 校验并保存默认扫描目录。
func (a *App) AddGameDirectory(dir string) error {
	if err := a.ready(); err != nil {
		return err
	}
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return errors.New("游戏目录不能为空")
	}
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("解析游戏目录失败: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return fmt.Errorf("读取游戏目录失败: %w", err)
	}
	if !info.IsDir() {
		return errors.New("所选路径不是目录")
	}
	return a.saveSetting(gameDirectorySetting, absolute)
}

// GetGameDirectory 返回上次保存的扫描目录。
func (a *App) GetGameDirectory() (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	var setting models.Setting
	err := a.db.First(&setting, "key = ?", gameDirectorySetting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("读取游戏扫描目录失败: %w", err)
	}
	return setting.Value, nil
}

// ScanGamesInDirectory 扫描目录并返回可预览的游戏列表；深度为 0 时使用默认深度。
func (a *App) ScanGamesInDirectory(dir string, depth int) ([]models.ScannedGame, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.Scanner.ScanGamesInDirectory(dir, depth)
}

// ImportScannedGames 批量导入扫描结果，重复的可执行文件路径会被跳过。
func (a *App) ImportScannedGames(games []models.ScannedGame) (services.ImportResult, error) {
	if err := a.ready(); err != nil {
		return services.ImportResult{}, err
	}
	result, err := a.svc.Game.ImportScannedGames(games)
	if err == nil {
		runtime.EventsEmit(a.ctx, "tracker:update", a.svc.Tracker.Status())
	}
	return result, err
}

// StartMonitor 开启进程监控，并持久化用户的开关选择。
func (a *App) StartMonitor() error {
	if err := a.ready(); err != nil {
		return err
	}
	if err := a.saveSetting(monitorEnabledSetting, "true"); err != nil {
		return err
	}
	a.svc.Tracker.Start()
	return nil
}

// StopMonitor 暂停进程监控并收尾当前游戏会话。
func (a *App) StopMonitor() error {
	if err := a.ready(); err != nil {
		return err
	}
	if err := a.svc.Tracker.Stop(); err != nil {
		return err
	}
	if err := a.saveSetting(monitorEnabledSetting, "false"); err != nil {
		return err
	}
	return nil
}

// GetMonitorStatus 返回监控开关与正在运行的游戏。
func (a *App) GetMonitorStatus() (services.MonitorStatus, error) {
	if err := a.ready(); err != nil {
		return services.MonitorStatus{}, err
	}
	return a.svc.Tracker.Status(), nil
}

// ------------------------------------------------------------- 统计与图表

// GetStatsByPeriod 按年 / 月 / 周聚合游玩数据（供统计报告页的图表使用）。
// start 与 end 为空时统计当前周期，同时传入时按自定义范围统计。
func (a *App) GetStatsByPeriod(periodType string, start string, end string) (services.StatsResult, error) {
	if err := a.ready(); err != nil {
		return services.StatsResult{}, err
	}
	return a.svc.Stats.GetStatsByPeriod(periodType, start, end)
}

// GetDashboardStats 返回仪表盘汇总统计（总时长、今日 / 本周 / 本月、Top 游戏、分类占比）。
func (a *App) GetDashboardStats() (services.StatsOverview, error) {
	if err := a.ready(); err != nil {
		return services.StatsOverview{}, err
	}
	return a.svc.Stats.GetDashboardStats()
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
