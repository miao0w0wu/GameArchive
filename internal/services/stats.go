package services

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"GameArchive/internal/models"
)

// 统计周期类型，对应前端「统计报告」页的 年 / 月 / 周 切换。
const (
	PeriodYear  = "year"
	PeriodMonth = "month"
	PeriodWeek  = "week"
)

// 趋势分桶粒度。
const (
	bucketDay   = "day"
	bucketMonth = "month"
)

const (
	// maxTopGames Top 游戏排行榜的最大条数。
	maxTopGames = 10
	// uncategorizedName 游戏没有分类时的展示名称。
	uncategorizedName = "未分类"
	// uncategorizedColor 游戏没有分类时的默认颜色。
	uncategorizedColor = "#64748b"
	// dateLayout PlaySession.date 与请求参数使用的日期格式。
	dateLayout = "2006-01-02"
)

// StatsService 负责按年 / 月 / 周聚合游玩数据，供统计报告与仪表盘使用。
//
// 聚合数据来源是阶段 2 写入的 play_sessions；
// 跨天会话按开始时间归属到开始那一天，与 games.total_seconds 的口径一致。
type StatsService struct{ db *gorm.DB }

// TrendPoint 是趋势图上的一个数据点（天或月）。
type TrendPoint struct {
	// Label 分桶标签：按天为 2006-01-02，按月为 2006-01。
	Label string `json:"label"`
	// Seconds 该分桶内的游玩秒数。
	Seconds int64 `json:"seconds"`
	// Sessions 该分桶内的游玩会话数。
	Sessions int64 `json:"sessions"`
}

// BreakdownItem 表示某个分类或标签的时长及其占比。
type BreakdownItem struct {
	Name string `json:"name"`
	// Color 分类 / 标签颜色，未分类时使用默认灰色。
	Color   string  `json:"color"`
	Seconds int64   `json:"seconds"`
	Percent float64 `json:"percent"`
	// GameCount 该分类 / 标签下产生过时长的游戏数量。
	GameCount int64 `json:"gameCount"`
}

// GameRankItem 表示单个游戏在统计范围内的时长排行。
type GameRankItem struct {
	GameID        uint       `json:"gameId"`
	Name          string     `json:"name"`
	CategoryName  string     `json:"categoryName"`
	CategoryColor string     `json:"categoryColor"`
	Seconds       int64      `json:"seconds"`
	Percent       float64    `json:"percent"`
	LastPlayedAt  *time.Time `json:"lastPlayedAt"`
}

// StatsResult 是某个统计周期的完整结果。
type StatsResult struct {
	PeriodType string `json:"periodType"`
	// BucketUnit 趋势分桶粒度：day 或 month。
	BucketUnit string `json:"bucketUnit"`
	// Start / End 统计范围（含首尾两天），格式 2006-01-02。
	Start string `json:"start"`
	End   string `json:"end"`

	TotalSeconds    int64 `json:"totalSeconds"`
	SessionCount    int64 `json:"sessionCount"`
	PlayedGameCount int64 `json:"playedGameCount"`
	ActiveDays      int64 `json:"activeDays"`
	// DailyAverage 平均每个游玩日的秒数。
	DailyAverage int64 `json:"dailyAverage"`

	Trend      []TrendPoint    `json:"trend"`
	Categories []BreakdownItem `json:"categories"`
	Tags       []BreakdownItem `json:"tags"`
	TopGames   []GameRankItem  `json:"topGames"`
}

// StatsOverview 是仪表盘使用的汇总数据（全部时间维度）。
type StatsOverview struct {
	TotalSeconds int64 `json:"totalSeconds"`
	TodaySeconds int64 `json:"todaySeconds"`
	WeekSeconds  int64 `json:"weekSeconds"`
	MonthSeconds int64 `json:"monthSeconds"`
	ActiveDays   int64 `json:"activeDays"`
	GameCount    int64 `json:"gameCount"`

	TopGames   []GameRankItem  `json:"topGames"`
	Categories []BreakdownItem `json:"categories"`
}

// periodWindow 是一次统计查询解析后的时间窗口。
type periodWindow struct {
	periodType string
	// start 为包含的起点，end 为不包含的终点。
	start time.Time
	end   time.Time
	unit  string
}

// statsSession 是聚合用的最小会话信息。
type statsSession struct {
	GameID          uint
	StartAt         time.Time
	DurationSeconds int64
}

// statsGame 是聚合用的游戏信息（含分类与标签）。
type statsGame struct {
	ID            uint
	Name          string
	CategoryName  string
	CategoryColor string
	Tags          []BreakdownItem
	LastPlayedAt  *time.Time
}

// breakdownBuilder 在聚合过程中累计某个分类 / 标签的时长与游戏集合。
type breakdownBuilder struct {
	name    string
	color   string
	seconds int64
	games   map[uint]struct{}
}

// GetStatsByPeriod 按指定周期聚合游玩数据。
//
// periodType 必须是 year / month / week；start 与 end 为空时取当前周期，
// 同时提供时按自定义范围统计（end 含当天），此时分桶粒度按跨度自动选择。
func (s *StatsService) GetStatsByPeriod(periodType, start, end string) (StatsResult, error) {
	window, err := resolvePeriodWindow(periodType, start, end, time.Now())
	if err != nil {
		return StatsResult{}, err
	}

	sessions, err := s.loadSessions(window.start, window.end)
	if err != nil {
		return StatsResult{}, err
	}
	games, err := s.loadGameIndex()
	if err != nil {
		return StatsResult{}, err
	}

	result := StatsResult{
		PeriodType: window.periodType,
		BucketUnit: window.unit,
		Start:      window.start.Format(dateLayout),
		End:        window.end.AddDate(0, 0, -1).Format(dateLayout),
		Trend:      emptyTrend(window),
		Categories: []BreakdownItem{},
		Tags:       []BreakdownItem{},
		TopGames:   []GameRankItem{},
	}

	buckets := make(map[string]*TrendPoint, len(result.Trend))
	for i := range result.Trend {
		buckets[result.Trend[i].Label] = &result.Trend[i]
	}

	gameTotals := make(map[uint]int64)
	playedGames := make(map[uint]struct{})
	activeDays := make(map[string]struct{})
	categoryAcc := make(map[string]*breakdownBuilder)
	tagAcc := make(map[string]*breakdownBuilder)

	for _, session := range sessions {
		result.TotalSeconds += session.DurationSeconds
		result.SessionCount++
		playedGames[session.GameID] = struct{}{}
		activeDays[session.StartAt.Format(dateLayout)] = struct{}{}
		gameTotals[session.GameID] += session.DurationSeconds

		if point, ok := buckets[bucketLabel(session.StartAt, window.unit)]; ok {
			point.Seconds += session.DurationSeconds
			point.Sessions++
		}

		game, ok := games[session.GameID]
		if !ok {
			// 游戏已被删除时仍保留时长统计，只是不参与分类 / 标签聚合。
			continue
		}
		addBreakdown(categoryAcc, game.CategoryName, game.CategoryColor, session.GameID, session.DurationSeconds)
		for _, tag := range game.Tags {
			addBreakdown(tagAcc, tag.Name, tag.Color, session.GameID, session.DurationSeconds)
		}
	}

	result.PlayedGameCount = int64(len(playedGames))
	result.ActiveDays = int64(len(activeDays))
	if result.ActiveDays > 0 {
		result.DailyAverage = result.TotalSeconds / result.ActiveDays
	}
	result.Categories = buildBreakdown(categoryAcc, result.TotalSeconds)
	result.Tags = buildBreakdown(tagAcc, result.TotalSeconds)
	result.TopGames = rankGames(gameTotals, games, result.TotalSeconds)
	return result, nil
}

// GetDashboardStats 返回仪表盘需要的汇总数据：总时长、今日 / 本周 / 本月时长、
// 活跃天数以及全部时间的 Top 游戏与分类占比。
func (s *StatsService) GetDashboardStats() (StatsOverview, error) {
	var games []models.Game
	if err := s.db.Preload("Category").Find(&games).Error; err != nil {
		return StatsOverview{}, fmt.Errorf("查询游戏统计信息失败: %w", err)
	}

	now := time.Now()
	overview := StatsOverview{
		GameCount:  int64(len(games)),
		TopGames:   []GameRankItem{},
		Categories: []BreakdownItem{},
	}

	categoryAcc := make(map[string]*breakdownBuilder)
	gameTotals := make(map[uint]int64)
	index := make(map[uint]statsGame, len(games))
	for _, game := range games {
		info := newStatsGame(game)
		index[game.ID] = info
		overview.TotalSeconds += game.TotalSeconds
		if game.TotalSeconds <= 0 {
			continue
		}
		gameTotals[game.ID] = game.TotalSeconds
		addBreakdown(categoryAcc, info.CategoryName, info.CategoryColor, game.ID, game.TotalSeconds)
	}
	overview.Categories = buildBreakdown(categoryAcc, overview.TotalSeconds)
	overview.TopGames = rankGames(gameTotals, index, overview.TotalSeconds)

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	windows := []struct {
		start time.Time
		end   time.Time
		dst   *int64
	}{
		{today, today.AddDate(0, 0, 1), &overview.TodaySeconds},
		{startOfWeek(now), startOfWeek(now).AddDate(0, 0, 7), &overview.WeekSeconds},
		{time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()), time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, 1, 0), &overview.MonthSeconds},
	}
	for _, w := range windows {
		total, err := s.sumSessions(w.start, w.end)
		if err != nil {
			return StatsOverview{}, err
		}
		*w.dst = total
	}

	if err := s.db.Model(&models.PlaySession{}).
		Where("duration_seconds > 0 AND date <> ''").
		Select("COUNT(DISTINCT date)").
		Scan(&overview.ActiveDays).Error; err != nil {
		return StatsOverview{}, fmt.Errorf("统计游玩天数失败: %w", err)
	}
	return overview, nil
}

// resolvePeriodWindow 把周期类型与自定义范围解析成查询窗口。
func resolvePeriodWindow(periodType, start, end string, now time.Time) (periodWindow, error) {
	period := strings.ToLower(strings.TrimSpace(periodType))
	if period == "" {
		period = PeriodWeek
	}
	switch period {
	case PeriodYear, PeriodMonth, PeriodWeek:
	default:
		return periodWindow{}, errors.New("统计周期必须是 year、month 或 week")
	}

	startTime, hasStart, err := parseDay(start)
	if err != nil {
		return periodWindow{}, err
	}
	endTime, hasEnd, err := parseDay(end)
	if err != nil {
		return periodWindow{}, err
	}

	window := periodWindow{periodType: period}
	if hasStart || hasEnd {
		if !hasStart || !hasEnd {
			return periodWindow{}, errors.New("自定义统计范围需要同时提供开始与结束日期")
		}
		if endTime.Before(startTime) {
			return periodWindow{}, errors.New("统计范围的结束日期不能早于开始日期")
		}
		window.start = startTime
		window.end = endTime.AddDate(0, 0, 1) // 结束日期含当天
		window.unit = bucketForSpan(window.start, window.end)
		return window, nil
	}

	switch period {
	case PeriodYear:
		window.start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		window.end = window.start.AddDate(1, 0, 0)
		window.unit = bucketMonth
	case PeriodMonth:
		window.start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		window.end = window.start.AddDate(0, 1, 0)
		window.unit = bucketDay
	case PeriodWeek:
		window.start = startOfWeek(now)
		window.end = window.start.AddDate(0, 0, 7)
		window.unit = bucketDay
	}
	return window, nil
}

// parseDay 解析 YYYY-MM-DD；空字符串表示未提供。
func parseDay(value string) (time.Time, bool, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, false, nil
	}
	parsed, err := time.ParseInLocation(dateLayout, trimmed, time.Local)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("日期格式应为 YYYY-MM-DD：%s", trimmed)
	}
	return parsed, true, nil
}

// startOfWeek 返回给定时间所在周的周一零点。
func startOfWeek(t time.Time) time.Time {
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	offset := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -offset)
}

// bucketForSpan 在自定义范围下按跨度选择分桶粒度，避免趋势点过多。
func bucketForSpan(start, end time.Time) string {
	if end.Sub(start) <= 62*24*time.Hour {
		return bucketDay
	}
	return bucketMonth
}

// emptyTrend 预先生成范围内所有分桶，保证图表上时间轴连续。
func emptyTrend(window periodWindow) []TrendPoint {
	points := make([]TrendPoint, 0)
	cursor := window.start
	if window.unit == bucketMonth {
		cursor = time.Date(cursor.Year(), cursor.Month(), 1, 0, 0, 0, 0, cursor.Location())
	}
	for cursor.Before(window.end) {
		points = append(points, TrendPoint{Label: bucketLabel(cursor, window.unit)})
		if window.unit == bucketMonth {
			cursor = cursor.AddDate(0, 1, 0)
		} else {
			cursor = cursor.AddDate(0, 0, 1)
		}
	}
	return points
}

// bucketLabel 返回某个时间点所属分桶的标签。
func bucketLabel(t time.Time, unit string) string {
	if unit == bucketMonth {
		return t.Format("2006-01")
	}
	return t.Format(dateLayout)
}

// loadSessions 读取范围内有效（时长大于 0）的游玩记录。
func (s *StatsService) loadSessions(start, end time.Time) ([]statsSession, error) {
	sessions := make([]statsSession, 0)
	err := s.db.Model(&models.PlaySession{}).
		Select("game_id, start_at, duration_seconds").
		Where("start_at >= ? AND start_at < ? AND duration_seconds > 0", start, end).
		Order("start_at").
		Scan(&sessions).Error
	if err != nil {
		return nil, fmt.Errorf("查询游玩记录失败: %w", err)
	}
	return sessions, nil
}

// sumSessions 统计范围内游玩秒数。
func (s *StatsService) sumSessions(start, end time.Time) (int64, error) {
	var total int64
	err := s.db.Model(&models.PlaySession{}).
		Select("COALESCE(SUM(duration_seconds), 0)").
		Where("start_at >= ? AND start_at < ?", start, end).
		Scan(&total).Error
	if err != nil {
		return 0, fmt.Errorf("统计周期时长失败: %w", err)
	}
	return total, nil
}

// loadGameIndex 读取全部游戏（含分类与标签）用于聚合。
func (s *StatsService) loadGameIndex() (map[uint]statsGame, error) {
	var games []models.Game
	if err := s.db.Preload("Category").Preload("Tags").Find(&games).Error; err != nil {
		return nil, fmt.Errorf("查询游戏统计信息失败: %w", err)
	}
	index := make(map[uint]statsGame, len(games))
	for _, game := range games {
		index[game.ID] = newStatsGame(game)
	}
	return index, nil
}

// newStatsGame 把数据模型转换成聚合用的结构，并补全未分类的默认名称与颜色。
func newStatsGame(game models.Game) statsGame {
	info := statsGame{
		ID:            game.ID,
		Name:          game.Name,
		CategoryName:  uncategorizedName,
		CategoryColor: uncategorizedColor,
		Tags:          make([]BreakdownItem, 0, len(game.Tags)),
		LastPlayedAt:  game.LastPlayedAt,
	}
	if game.Category != nil {
		info.CategoryName = game.Category.Name
		if game.Category.Color != "" {
			info.CategoryColor = game.Category.Color
		}
	}
	for _, tag := range game.Tags {
		color := tag.Color
		if color == "" {
			color = uncategorizedColor
		}
		info.Tags = append(info.Tags, BreakdownItem{Name: tag.Name, Color: color})
	}
	return info
}

// addBreakdown 累加分类 / 标签的时长并记录游戏集合。
func addBreakdown(acc map[string]*breakdownBuilder, name, color string, gameID uint, seconds int64) {
	if name == "" {
		name = uncategorizedName
	}
	item, ok := acc[name]
	if !ok {
		if color == "" {
			color = uncategorizedColor
		}
		item = &breakdownBuilder{name: name, color: color, games: make(map[uint]struct{})}
		acc[name] = item
	}
	item.seconds += seconds
	item.games[gameID] = struct{}{}
}

// buildBreakdown 把累计结果转成按时长倒序的列表。
func buildBreakdown(acc map[string]*breakdownBuilder, total int64) []BreakdownItem {
	items := make([]BreakdownItem, 0, len(acc))
	for _, item := range acc {
		items = append(items, BreakdownItem{
			Name:      item.name,
			Color:     item.color,
			Seconds:   item.seconds,
			Percent:   percentOf(item.seconds, total),
			GameCount: int64(len(item.games)),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Seconds != items[j].Seconds {
			return items[i].Seconds > items[j].Seconds
		}
		return items[i].Name < items[j].Name
	})
	return items
}

// rankGames 生成按时长倒序的 Top 游戏列表。
func rankGames(totals map[uint]int64, games map[uint]statsGame, total int64) []GameRankItem {
	ranked := make([]GameRankItem, 0, len(totals))
	for gameID, seconds := range totals {
		game, ok := games[gameID]
		if !ok {
			// 游戏记录已被删除，但游玩时长仍在，此时只展示 ID 便于排查。
			ranked = append(ranked, GameRankItem{
				GameID:        gameID,
				Name:          fmt.Sprintf("已删除的游戏 #%d", gameID),
				CategoryName:  uncategorizedName,
				CategoryColor: uncategorizedColor,
				Seconds:       seconds,
				Percent:       percentOf(seconds, total),
			})
			continue
		}
		ranked = append(ranked, GameRankItem{
			GameID:        gameID,
			Name:          game.Name,
			CategoryName:  game.CategoryName,
			CategoryColor: game.CategoryColor,
			Seconds:       seconds,
			Percent:       percentOf(seconds, total),
			LastPlayedAt:  game.LastPlayedAt,
		})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Seconds != ranked[j].Seconds {
			return ranked[i].Seconds > ranked[j].Seconds
		}
		return ranked[i].Name < ranked[j].Name
	})
	if len(ranked) > maxTopGames {
		ranked = ranked[:maxTopGames]
	}
	return ranked
}

// percentOf 计算占比（百分比，保留一位小数）。
func percentOf(part, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return math.Round(float64(part)/float64(total)*1000) / 10
}
