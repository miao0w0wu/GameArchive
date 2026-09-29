package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image/png"
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
	StageName = "阶段 6：Steam 平台数据接入"
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
	a.svc = services.New(db, paths.DataDir)
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

// ------------------------------------------------------------- 本地存档与备份

// SelectSaveArchiveFile 打开文件选择框，选择要登记的存档文件。
func (a *App) SelectSaveArchiveFile() (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择存档文件",
	})
	if err != nil {
		return "", fmt.Errorf("选择存档文件失败: %w", err)
	}
	return path, nil
}

// SelectSaveArchiveDirectory 打开目录选择框，选择要登记的存档文件夹。
func (a *App) SelectSaveArchiveDirectory() (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择存档文件夹",
	})
	if err != nil {
		return "", fmt.Errorf("选择存档文件夹失败: %w", err)
	}
	return path, nil
}

// SelectSaveBackupDirectory 打开目录选择框，选择备份目标文件夹。
func (a *App) SelectSaveBackupDirectory(defaultDirectory string) (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	options := runtime.OpenDialogOptions{Title: "选择备份目标文件夹"}
	if strings.TrimSpace(defaultDirectory) != "" {
		options.DefaultDirectory = defaultDirectory
	}
	path, err := runtime.OpenDirectoryDialog(a.ctx, options)
	if err != nil {
		return "", fmt.Errorf("选择备份目标文件夹失败: %w", err)
	}
	return path, nil
}

// AddSaveArchive 将用户选择的文件或文件夹登记为某个游戏的存档。
// 该操作只记录路径，不会移动、复制或删除原始存档。
func (a *App) AddSaveArchive(gameID uint, name, sourcePath string, isDir bool, backupDir, note string) (*models.SaveArchive, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.Archive.AddArchive(gameID, name, sourcePath, isDir, backupDir, note)
}

// ListSaveArchives 返回指定游戏的全部存档记录。
func (a *App) ListSaveArchives(gameID uint) ([]models.SaveArchive, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.Archive.ListArchives(gameID)
}

// BackupSaveArchive 将存档复制到指定目录并记录备份时间、路径与备注。
func (a *App) BackupSaveArchive(archiveID uint, targetDir, note string) (*models.SaveBackup, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.Archive.BackupArchive(archiveID, targetDir, note)
}

// ListSaveBackups 返回指定存档的全部备份历史。
func (a *App) ListSaveBackups(archiveID uint) ([]models.SaveBackup, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.Archive.ListBackups(archiveID)
}

// DeleteSaveArchive 删除存档记录；只有用户确认时才删除原始存档文件。
func (a *App) DeleteSaveArchive(id uint, deleteOriginal bool) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.svc.Archive.DeleteArchive(id, deleteOriginal)
}

// DeleteSaveBackup 删除备份历史；只有用户确认时才删除对应备份文件。
func (a *App) DeleteSaveBackup(id uint, deleteFile bool) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.svc.Archive.DeleteBackup(id, deleteFile)
}

// ------------------------------------------------------------- Steam 平台数据

// GetSteamSettings 返回保存在本机的 Steam API Key 和 SteamID。
func (a *App) GetSteamSettings() (services.SteamConfig, error) {
	if err := a.ready(); err != nil {
		return services.SteamConfig{}, err
	}
	return a.svc.Steam.GetConfig()
}

// SaveSteamSettings 保存 Steam API Key 和 SteamID。
func (a *App) SaveSteamSettings(config services.SteamConfig) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.svc.Steam.SaveConfig(config)
}

// GetSteamSyncStatus 返回上一次 Steam 游戏库同步结果。
func (a *App) GetSteamSyncStatus() (services.SteamSyncStatus, error) {
	if err := a.ready(); err != nil {
		return services.SteamSyncStatus{}, err
	}
	return a.svc.Steam.GetSyncStatus()
}

// SyncSteamLibrary 手动获取 Steam 游戏库并匹配或导入本地游戏记录。
func (a *App) SyncSteamLibrary() (services.SteamSyncResult, error) {
	if err := a.ready(); err != nil {
		return services.SteamSyncResult{}, err
	}
	result, err := a.svc.Steam.SyncLibrary(a.ctx)
	if err != nil {
		return services.SteamSyncResult{}, err
	}
	games, listErr := a.svc.Game.List()
	if listErr != nil {
		runtime.LogWarningf(a.ctx, "查询 Steam 游戏图片任务失败: %v", listErr)
	} else {
		missingCovers := make([]uint, 0)
		missingIcons := make([]uint, 0)
		for _, game := range games {
			if game.SteamAppID == 0 {
				continue
			}
			if game.CoverPath == "" {
				missingCovers = append(missingCovers, game.ID)
			}
			if game.IconPath == "" {
				missingIcons = append(missingIcons, game.ID)
			}
		}
		for start := 0; start < len(missingCovers); start += 100 {
			end := min(start+100, len(missingCovers))
			if _, coverErr := a.svc.Cover.BatchFetchMissing(missingCovers[start:end], "cover"); coverErr != nil {
				runtime.LogWarningf(a.ctx, "自动获取 Steam 游戏封面失败: %v", coverErr)
			}
		}
		for start := 0; start < len(missingIcons); start += 100 {
			end := min(start+100, len(missingIcons))
			if _, coverErr := a.svc.Cover.BatchFetchMissing(missingIcons[start:end], "icon"); coverErr != nil {
				runtime.LogWarningf(a.ctx, "自动获取 Steam 游戏图标失败: %v", coverErr)
			}
		}
	}
	runtime.EventsEmit(a.ctx, "tracker:update", a.svc.Tracker.Status())
	return result, nil
}

// ------------------------------------------------------------- 游戏封面与展示

// SelectAndSetCover 选择图片并将其复制到应用本地缓存，target 为 cover 或 icon。
func (a *App) SelectAndSetCover(gameID uint, target string) (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	if target != "cover" && target != "icon" {
		return "", errors.New("图片类型必须为 cover 或 icon")
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择游戏" + map[string]string{"cover": "封面", "icon": "图标"}[target],
		Filters: []runtime.FileFilter{
			{DisplayName: "图片文件 (*.png;*.jpg;*.jpeg;*.gif;*.ico)", Pattern: "*.png;*.jpg;*.jpeg;*.gif;*.ico"},
		},
	})
	if err != nil {
		return "", fmt.Errorf("选择游戏图片失败: %w", err)
	}
	if strings.TrimSpace(path) == "" {
		return "", nil
	}
	return a.svc.Cover.SetImage(gameID, target, path)
}

// ClearCover 清除图片字段，之后可点击自动获取来恢复图片。
func (a *App) ClearCover(gameID uint, target string) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.svc.Cover.ClearImage(gameID, target)
}

// AutoFetchCover 手动触发封面自动获取，允许用户显式替换已有图片。
func (a *App) AutoFetchCover(gameID uint) (services.CoverResult, error) {
	if err := a.ready(); err != nil {
		return services.CoverResult{}, err
	}
	return a.svc.Cover.AutoFetch(gameID, "cover", true)
}

// AutoFetchIcon 手动触发图标自动获取。
func (a *App) AutoFetchIcon(gameID uint) (services.CoverResult, error) {
	if err := a.ready(); err != nil {
		return services.CoverResult{}, err
	}
	return a.svc.Cover.AutoFetch(gameID, "icon", true)
}

// BatchFetchCovers 批量自动获取封面，单次最多支持 100 个游戏。
func (a *App) BatchFetchCovers(gameIDs []uint) (services.BatchCoverResult, error) {
	if err := a.ready(); err != nil {
		return services.BatchCoverResult{}, err
	}
	return a.svc.Cover.BatchFetch(gameIDs, "cover")
}

// BatchFetchIcons 批量自动获取图标，单次最多支持 100 个游戏。
func (a *App) BatchFetchIcons(gameIDs []uint) (services.BatchCoverResult, error) {
	if err := a.ready(); err != nil {
		return services.BatchCoverResult{}, err
	}
	return a.svc.Cover.BatchFetch(gameIDs, "icon")
}

// GetGameCoverInfo 返回封面 / 图标来源、更新时间和缓存位置。
func (a *App) GetGameCoverInfo(gameID uint) (services.GameCoverInfo, error) {
	if err := a.ready(); err != nil {
		return services.GameCoverInfo{}, err
	}
	return a.svc.Cover.GetInfo(gameID)
}

// GetGameImageData 返回游戏图片的本地 data URL，所有文件读取均在 Go 侧完成。
func (a *App) GetGameImageData(gameID uint, target string) (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	return a.svc.Cover.GetImageData(gameID, target)
}

// GetCoverSettings 返回在线封面搜索配置和缓存目录。
func (a *App) GetCoverSettings() (services.CoverSettings, error) {
	if err := a.ready(); err != nil {
		return services.CoverSettings{}, err
	}
	return a.svc.Cover.GetSettings()
}

// SaveCoverSettings 保存在线封面搜索配置。
func (a *App) SaveCoverSettings(settings services.CoverSettings) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.svc.Cover.SaveSettings(settings)
}

// GetLibraryDisplaySettings 返回游戏库展示偏好。
func (a *App) GetLibraryDisplaySettings() (services.LibraryDisplaySettings, error) {
	if err := a.ready(); err != nil {
		return services.LibraryDisplaySettings{}, err
	}
	return a.svc.Cover.GetLibraryDisplaySettings()
}

// SaveLibraryDisplaySettings 保存游戏库展示偏好。
func (a *App) SaveLibraryDisplaySettings(settings services.LibraryDisplaySettings) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.svc.Cover.SaveLibraryDisplaySettings(settings)
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

// ------------------------------------------------------------- AI 报告与导入导出

// GetAISettings 返回 AI 接口配置（API Key 保存在本机数据库，不会写入日志）。
func (a *App) GetAISettings() (services.AIConfig, error) {
	if err := a.ready(); err != nil {
		return services.AIConfig{}, err
	}
	return a.svc.Report.GetAIConfig()
}

// SaveAISettings 保存 AI 接口配置。
func (a *App) SaveAISettings(config services.AIConfig) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.svc.Report.SaveAIConfig(config)
}

// TestAIConnection 用当前配置请求一次接口，验证地址、密钥与模型是否可用。
func (a *App) TestAIConnection() error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.svc.Report.TestConnection()
}

// GenerateAIReport 依据统计数据调用 OpenAI 兼容接口生成 Markdown 报告并保存。
func (a *App) GenerateAIReport(req services.AIReportRequest) (*models.Report, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.Report.GenerateAIReport(req)
}

// ListReports 返回报告列表（不含正文）。
func (a *App) ListReports() ([]models.Report, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.Report.List()
}

// GetReport 返回报告详情（含 Markdown 正文）。
func (a *App) GetReport(id uint) (*models.Report, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.Report.Get(id)
}

// DeleteReport 删除报告。
func (a *App) DeleteReport(id uint) error {
	if err := a.ready(); err != nil {
		return err
	}
	return a.svc.Report.Delete(id)
}

// SaveReport 保存报告正文（用于编辑后的保存或手动新建）。
func (a *App) SaveReport(report models.Report) (*models.Report, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	return a.svc.Report.Save(report)
}

// ImportReportFromFile 弹出文件选择框，从 .json / .md / .txt 导入外部报告。
func (a *App) ImportReportFromFile() (*models.Report, error) {
	if err := a.ready(); err != nil {
		return nil, err
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "导入报告",
		Filters: []runtime.FileFilter{
			{DisplayName: "报告文件 (*.md;*.markdown;*.txt;*.json)", Pattern: "*.md;*.markdown;*.txt;*.json"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("选择报告文件失败: %w", err)
	}
	if strings.TrimSpace(path) == "" {
		return nil, nil // 用户取消选择
	}
	report, err := a.svc.Report.ImportFromFile(path)
	if err != nil {
		return nil, err
	}
	return report, nil
}

// ExportReportMarkdown 弹出保存对话框，把报告正文导出为 Markdown 文件。
// 返回保存路径；用户取消时返回空字符串。
func (a *App) ExportReportMarkdown(id uint) (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	content, filename, err := a.svc.Report.ExportMarkdown(id)
	if err != nil {
		return "", err
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出报告",
		DefaultFilename: filename,
		Filters: []runtime.FileFilter{
			{DisplayName: "Markdown 文件 (*.md)", Pattern: "*.md"},
		},
	})
	if err != nil {
		return "", fmt.Errorf("选择保存位置失败: %w", err)
	}
	if strings.TrimSpace(path) == "" {
		return "", nil // 用户取消保存
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("写入报告文件失败: %w", err)
	}
	return path, nil
}

// SaveImage 由前端传入 base64 图片数据（不含 dataURL 前缀），弹出保存对话框写入 PNG。
// 返回保存路径；用户取消时返回空字符串。
func (a *App) SaveImage(base64Data string, defaultName string) (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	data := strings.TrimSpace(base64Data)
	if data == "" {
		return "", errors.New("图片数据为空")
	}
	if len(data) > 32<<20 {
		return "", errors.New("图片数据超过 24 MiB 限制")
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("解析图片数据失败: %w", err)
	}
	if len(raw) > 24<<20 {
		return "", errors.New("图片数据超过 24 MiB 限制")
	}
	config, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("图片数据不是有效的 PNG: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > 20000 || config.Height > 20000 ||
		int64(config.Width)*int64(config.Height) > 100_000_000 {
		return "", errors.New("图片尺寸无效或超过支持范围")
	}

	name := strings.TrimSpace(defaultName)
	if name == "" {
		name = "game-archive"
	}
	if !strings.HasSuffix(strings.ToLower(name), ".png") {
		name += ".png"
	}

	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出图片",
		DefaultFilename: name,
		Filters: []runtime.FileFilter{
			{DisplayName: "PNG 图片 (*.png)", Pattern: "*.png"},
		},
	})
	if err != nil {
		return "", fmt.Errorf("选择保存位置失败: %w", err)
	}
	if strings.TrimSpace(path) == "" {
		return "", nil // 用户取消保存
	}

	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return "", fmt.Errorf("写入图片失败: %w", err)
	}
	return path, nil
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
