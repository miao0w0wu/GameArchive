package services

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"gorm.io/gorm"

	"GameArchive/internal/models"
)

const monitorInterval = 5 * time.Second

// ProcessInfo 是进程匹配所需的最小信息集合。
type ProcessInfo struct {
	ExePath string
	Name    string
}

// ProcessLister 返回当前系统进程快照。
type ProcessLister func() ([]ProcessInfo, error)

// ActiveGame 描述当前被监控到的游戏进程。
type ActiveGame struct {
	GameID          uint      `json:"gameId"`
	Name            string    `json:"name"`
	StartedAt       time.Time `json:"startedAt"`
	DurationSeconds int64     `json:"durationSeconds"`
}

// MonitorStatus 是监控开关和当前游戏状态。
type MonitorStatus struct {
	Running     bool         `json:"running"`
	ActiveGames []ActiveGame `json:"activeGames"`
}

type activeSession struct {
	gameID          uint
	name            string
	sessionID       uint
	startedAt       time.Time
	accountedSecond int64
}

// Tracker 每 5 秒检查进程，并将游玩时长增量写入 play_sessions 与 games。
type Tracker struct {
	db       *gorm.DB
	interval time.Duration
	list     ProcessLister
	onError  func(error)
	onUpdate func(MonitorStatus)

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	done    chan struct{}
	stopErr error
	active  map[uint]*activeSession
}

// NewTracker 创建进程监控器。nil 参数使用默认轮询器、间隔和事件回调。
func NewTracker(db *gorm.DB, list ProcessLister, onError func(error)) *Tracker {
	if list == nil {
		list = listSystemProcesses
	}
	return &Tracker{
		db:       db,
		interval: monitorInterval,
		list:     list,
		onError:  onError,
		active:   make(map[uint]*activeSession),
	}
}

// SetUpdateHandler 设置每次成功轮询后触发的状态通知。
func (t *Tracker) SetUpdateHandler(handler func(MonitorStatus)) {
	t.mu.Lock()
	t.onUpdate = handler
	t.mu.Unlock()
}

// SetErrorHandler 设置监控运行错误的日志回调。
func (t *Tracker) SetErrorHandler(handler func(error)) {
	t.mu.Lock()
	t.onError = handler
	t.mu.Unlock()
}

// Recover 收尾上次异常退出留下的未结束会话，不把应用关闭期间计入时长。
func (t *Tracker) Recover() error {
	var sessions []models.PlaySession
	if err := t.db.Where("end_at IS NULL").Find(&sessions).Error; err != nil {
		return fmt.Errorf("查询未结束的游玩记录失败: %w", err)
	}
	return t.db.Transaction(func(tx *gorm.DB) error {
		for _, session := range sessions {
			endAt := session.StartAt.Add(time.Duration(session.DurationSeconds) * time.Second)
			if err := tx.Model(&models.PlaySession{}).Where("id = ?", session.ID).
				Updates(map[string]any{"end_at": endAt, "duration_seconds": session.DurationSeconds}).Error; err != nil {
				return fmt.Errorf("收尾上次未结束的游玩记录失败: %w", err)
			}
			if err := tx.Model(&models.Game{}).Where("id = ?", session.GameID).
				Update("last_played_at", endAt).Error; err != nil {
				return fmt.Errorf("更新游戏最后游玩时间失败: %w", err)
			}
		}
		return nil
	})
}

// Start 启动后台轮询；重复调用不会创建额外监控 goroutine。
func (t *Tracker) Start() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.running {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel
	t.done = make(chan struct{})
	t.stopErr = nil
	t.running = true
	go t.run(ctx, t.done)
}

// Stop 停止轮询并结束所有进行中的游玩记录。
func (t *Tracker) Stop() error {
	t.mu.Lock()
	if !t.running {
		t.mu.Unlock()
		return nil
	}
	cancel, done := t.cancel, t.done
	t.mu.Unlock()

	cancel()
	<-done

	t.mu.Lock()
	t.running = false
	t.cancel = nil
	err := t.stopErr
	t.mu.Unlock()
	if err != nil {
		return fmt.Errorf("停止进程监控时保存游玩记录失败: %w", err)
	}
	return nil
}

// Status 返回当前监控状态的快照。
func (t *Tracker) Status() MonitorStatus {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.statusLocked(time.Now())
}

func (t *Tracker) run(ctx context.Context, done chan struct{}) {
	defer close(done)
	t.scan(time.Now())
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if err := t.finishAll(time.Now()); err != nil {
				t.mu.Lock()
				t.stopErr = err
				t.mu.Unlock()
				t.reportError(err)
			}
			return
		case now := <-ticker.C:
			t.scan(now)
		}
	}
}

func (t *Tracker) scan(now time.Time) {
	processes, err := t.list()
	if err != nil {
		t.reportError(fmt.Errorf("读取系统进程失败: %w", err))
		return
	}

	var games []models.Game
	if err := t.db.Select("id", "name", "exe_path", "process_name").
		Find(&games).Error; err != nil {
		t.reportError(fmt.Errorf("读取游戏监控配置失败: %w", err))
		return
	}

	present := make(map[uint]models.Game)
	for _, game := range games {
		if isGameRunning(game, processes) {
			present[game.ID] = game
		}
	}

	t.mu.Lock()
	before := cloneActiveSessions(t.active)
	err = t.db.Transaction(func(tx *gorm.DB) error {
		for gameID, game := range present {
			current := t.active[gameID]
			if current == nil {
				session := models.PlaySession{
					GameID:  gameID,
					StartAt: now,
					Date:    now.Local().Format("2006-01-02"),
					Source:  "monitor",
				}
				if err := tx.Create(&session).Error; err != nil {
					return fmt.Errorf("创建游戏 %q 的游玩记录失败: %w", game.Name, err)
				}
				current = &activeSession{
					gameID:    gameID,
					name:      game.Name,
					sessionID: session.ID,
					startedAt: now,
				}
				t.active[gameID] = current
				if err := tx.Model(&models.Game{}).Where("id = ?", gameID).
					Update("last_played_at", now).Error; err != nil {
					return fmt.Errorf("更新游戏最后游玩时间失败: %w", err)
				}
			}
			if err := t.accountElapsed(tx, current, now, false); err != nil {
				return err
			}
		}
		for gameID, current := range t.active {
			if _, running := present[gameID]; running {
				continue
			}
			if err := t.accountElapsed(tx, current, now, true); err != nil {
				return err
			}
			delete(t.active, gameID)
		}
		return nil
	})
	if err != nil {
		t.active = before
	}
	status := t.statusLocked(now)
	handler := t.onUpdate
	t.mu.Unlock()

	if err != nil {
		t.reportError(err)
		return
	}
	if handler != nil {
		handler(status)
	}
}

func (t *Tracker) accountElapsed(tx *gorm.DB, current *activeSession, now time.Time, finish bool) error {
	elapsed := int64(now.Sub(current.startedAt).Seconds())
	if elapsed < current.accountedSecond {
		elapsed = current.accountedSecond
	}
	delta := elapsed - current.accountedSecond
	updates := map[string]any{"duration_seconds": elapsed}
	if finish {
		updates["end_at"] = now
	}
	sessionResult := tx.Model(&models.PlaySession{}).Where("id = ?", current.sessionID).Updates(updates)
	if sessionResult.Error != nil {
		return fmt.Errorf("更新游戏 %q 的游玩记录失败: %w", current.name, sessionResult.Error)
	}
	if sessionResult.RowsAffected == 0 {
		current.accountedSecond = elapsed
		return nil
	}
	if delta > 0 {
		if err := tx.Model(&models.Game{}).Where("id = ?", current.gameID).
			Update("total_seconds", gorm.Expr("total_seconds + ?", delta)).Error; err != nil {
			return fmt.Errorf("累计游戏 %q 的游玩时长失败: %w", current.name, err)
		}
	}
	if finish {
		if err := tx.Model(&models.Game{}).Where("id = ?", current.gameID).
			Update("last_played_at", now).Error; err != nil {
			return fmt.Errorf("更新游戏最后游玩时间失败: %w", err)
		}
	}
	current.accountedSecond = elapsed
	return nil
}

func (t *Tracker) finishAll(now time.Time) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	before := cloneActiveSessions(t.active)
	err := t.db.Transaction(func(tx *gorm.DB) error {
		for gameID, current := range t.active {
			if err := t.accountElapsed(tx, current, now, true); err != nil {
				return err
			}
			delete(t.active, gameID)
		}
		return nil
	})
	if err != nil {
		t.active = before
		return err
	}
	return nil
}

func cloneActiveSessions(active map[uint]*activeSession) map[uint]*activeSession {
	cloned := make(map[uint]*activeSession, len(active))
	for gameID, session := range active {
		copied := *session
		cloned[gameID] = &copied
	}
	return cloned
}

func (t *Tracker) statusLocked(now time.Time) MonitorStatus {
	status := MonitorStatus{
		Running:     t.running,
		ActiveGames: make([]ActiveGame, 0, len(t.active)),
	}
	for _, current := range t.active {
		duration := int64(now.Sub(current.startedAt).Seconds())
		if duration < 0 {
			duration = 0
		}
		status.ActiveGames = append(status.ActiveGames, ActiveGame{
			GameID:          current.gameID,
			Name:            current.name,
			StartedAt:       current.startedAt,
			DurationSeconds: duration,
		})
	}
	sort.Slice(status.ActiveGames, func(i, j int) bool {
		return status.ActiveGames[i].Name < status.ActiveGames[j].Name
	})
	return status
}

func (t *Tracker) reportError(err error) {
	if t.onError != nil {
		t.onError(err)
	}
}

func listSystemProcesses() ([]ProcessInfo, error) {
	processes, err := process.Processes()
	if err != nil {
		return nil, err
	}
	result := make([]ProcessInfo, 0, len(processes))
	for _, p := range processes {
		name, nameErr := p.Name()
		exePath, pathErr := p.Exe()
		if nameErr != nil && pathErr != nil {
			continue
		}
		result = append(result, ProcessInfo{Name: name, ExePath: exePath})
	}
	return result, nil
}

func isGameRunning(game models.Game, processes []ProcessInfo) bool {
	expectedPath := normalizePath(game.ExePath)
	expectedName := normalizeProcessName(game.ProcessName)
	for _, candidate := range processes {
		if expectedPath != "" && normalizePath(candidate.ExePath) == expectedPath {
			return true
		}
	}
	if expectedName == "" {
		return false
	}
	for _, candidate := range processes {
		if normalizeProcessName(candidate.Name) == expectedName ||
			normalizeProcessName(filepath.Base(candidate.ExePath)) == expectedName {
			return true
		}
	}
	return false
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return strings.ToLower(filepath.Clean(path))
}

func normalizeProcessName(name string) string {
	name = strings.TrimSpace(filepath.Base(name))
	name = strings.TrimSuffix(name, filepath.Ext(name))
	return strings.ToLower(name)
}
