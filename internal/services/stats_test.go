package services

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"GameArchive/internal/models"
)

func TestResolvePeriodWindowDefaults(t *testing.T) {
	now := time.Date(2026, 9, 28, 15, 4, 5, 0, time.Local)

	year, err := resolvePeriodWindow(PeriodYear, "", "", now)
	if err != nil {
		t.Fatalf("year window error = %v", err)
	}
	if !year.start.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)) ||
		!year.end.Equal(time.Date(2027, 1, 1, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("year range = %v ~ %v", year.start, year.end)
	}
	if year.unit != bucketMonth {
		t.Fatalf("year bucket unit = %q, want %q", year.unit, bucketMonth)
	}
	if len(emptyTrend(year)) != 12 {
		t.Fatalf("year trend buckets = %d, want 12", len(emptyTrend(year)))
	}

	month, err := resolvePeriodWindow(PeriodMonth, "", "", now)
	if err != nil {
		t.Fatalf("month window error = %v", err)
	}
	if !month.start.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local)) ||
		!month.end.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("month range = %v ~ %v", month.start, month.end)
	}
	if month.unit != bucketDay {
		t.Fatalf("month bucket unit = %q, want %q", month.unit, bucketDay)
	}
	if len(emptyTrend(month)) != 30 {
		t.Fatalf("month trend buckets = %d, want 30", len(emptyTrend(month)))
	}

	week, err := resolvePeriodWindow(PeriodWeek, "", "", now)
	if err != nil {
		t.Fatalf("week window error = %v", err)
	}
	if week.start.Weekday() != time.Monday {
		t.Fatalf("week should start on Monday, got %v", week.start.Weekday())
	}
	if week.start.Hour() != 0 || week.start.Minute() != 0 || week.start.Second() != 0 {
		t.Fatalf("week start is not midnight: %v", week.start)
	}
	if !week.end.Equal(week.start.AddDate(0, 0, 7)) {
		t.Fatalf("week end = %v, want %v", week.end, week.start.AddDate(0, 0, 7))
	}
	if now.Before(week.start) || !now.Before(week.end) {
		t.Fatalf("week range %v ~ %v does not contain now %v", week.start, week.end, now)
	}
	if len(emptyTrend(week)) != 7 {
		t.Fatalf("week trend buckets = %d, want 7", len(emptyTrend(week)))
	}
}

func TestResolvePeriodWindowCustomRange(t *testing.T) {
	now := time.Date(2026, 9, 28, 15, 4, 5, 0, time.Local)

	short, err := resolvePeriodWindow(PeriodWeek, "2026-03-02", "2026-03-08", now)
	if err != nil {
		t.Fatalf("custom week error = %v", err)
	}
	if !short.start.Equal(time.Date(2026, 3, 2, 0, 0, 0, 0, time.Local)) ||
		!short.end.Equal(time.Date(2026, 3, 9, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("custom week range = %v ~ %v", short.start, short.end)
	}
	if short.unit != bucketDay {
		t.Fatalf("custom week unit = %q, want %q", short.unit, bucketDay)
	}

	long, err := resolvePeriodWindow(PeriodYear, "2026-01-01", "2026-06-30", now)
	if err != nil {
		t.Fatalf("custom year error = %v", err)
	}
	if long.unit != bucketMonth {
		t.Fatalf("long range unit = %q, want %q", long.unit, bucketMonth)
	}
	if got := len(emptyTrend(long)); got != 6 {
		t.Fatalf("long range buckets = %d, want 6", got)
	}
}

func TestResolvePeriodWindowRejectsInvalidInput(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name       string
		periodType string
		start      string
		end        string
	}{
		{"未知周期", "day", "", ""},
		{"缺少结束日期", PeriodWeek, "2026-03-02", ""},
		{"缺少开始日期", PeriodWeek, "", "2026-03-08"},
		{"结束早于开始", PeriodMonth, "2026-03-08", "2026-03-02"},
		{"日期格式错误", PeriodMonth, "2026/03/02", "2026/03/08"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := resolvePeriodWindow(tc.periodType, tc.start, tc.end, now); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

func TestGetStatsByPeriodAggregatesSessions(t *testing.T) {
	db := newTestDB(t)
	service := &StatsService{db: db}

	action := models.Category{Name: "动作", Color: "#ef4444"}
	if err := db.Create(&action).Error; err != nil {
		t.Fatal(err)
	}
	tag := models.Tag{Name: "耐玩", Color: "#22c55e"}
	if err := db.Create(&tag).Error; err != nil {
		t.Fatal(err)
	}

	alpha := models.Game{Name: "Alpha", CategoryID: &action.ID, Tags: []models.Tag{tag}}
	if err := db.Create(&alpha).Error; err != nil {
		t.Fatal(err)
	}
	beta := models.Game{Name: "Beta"}
	if err := db.Create(&beta).Error; err != nil {
		t.Fatal(err)
	}

	createSession(t, db, alpha.ID, time.Date(2026, 3, 2, 10, 0, 0, 0, time.Local), 3600)
	createSession(t, db, beta.ID, time.Date(2026, 3, 3, 20, 0, 0, 0, time.Local), 1800)
	createSession(t, db, alpha.ID, time.Date(2026, 3, 3, 22, 0, 0, 0, time.Local), 7200)
	// 范围外与零时长的记录都不应参与统计。
	createSession(t, db, alpha.ID, time.Date(2026, 3, 20, 10, 0, 0, 0, time.Local), 99999)
	createSession(t, db, alpha.ID, time.Date(2026, 3, 4, 10, 0, 0, 0, time.Local), 0)

	result, err := service.GetStatsByPeriod(PeriodWeek, "2026-03-02", "2026-03-08")
	if err != nil {
		t.Fatalf("GetStatsByPeriod() error = %v", err)
	}

	if result.TotalSeconds != 12600 {
		t.Fatalf("TotalSeconds = %d, want 12600", result.TotalSeconds)
	}
	if result.SessionCount != 3 {
		t.Fatalf("SessionCount = %d, want 3", result.SessionCount)
	}
	if result.PlayedGameCount != 2 {
		t.Fatalf("PlayedGameCount = %d, want 2", result.PlayedGameCount)
	}
	if result.ActiveDays != 2 {
		t.Fatalf("ActiveDays = %d, want 2", result.ActiveDays)
	}
	if result.DailyAverage != 6300 {
		t.Fatalf("DailyAverage = %d, want 6300", result.DailyAverage)
	}
	if result.BucketUnit != bucketDay {
		t.Fatalf("BucketUnit = %q, want %q", result.BucketUnit, bucketDay)
	}
	if result.Start != "2026-03-02" || result.End != "2026-03-08" {
		t.Fatalf("range = %s ~ %s", result.Start, result.End)
	}

	if len(result.Trend) != 7 {
		t.Fatalf("Trend length = %d, want 7", len(result.Trend))
	}
	wantTrend := map[string]TrendPoint{
		"2026-03-02": {Label: "2026-03-02", Seconds: 3600, Sessions: 1},
		"2026-03-03": {Label: "2026-03-03", Seconds: 9000, Sessions: 2},
	}
	for _, point := range result.Trend {
		expected, ok := wantTrend[point.Label]
		if !ok {
			if point.Seconds != 0 || point.Sessions != 0 {
				t.Fatalf("unexpected trend bucket %#v", point)
			}
			continue
		}
		if point != expected {
			t.Fatalf("trend bucket = %#v, want %#v", point, expected)
		}
	}

	if len(result.Categories) != 2 {
		t.Fatalf("categories = %#v, want 2 items", result.Categories)
	}
	if result.Categories[0].Name != "动作" || result.Categories[0].Seconds != 10800 ||
		result.Categories[0].Percent != 85.7 || result.Categories[0].GameCount != 1 {
		t.Fatalf("top category = %#v", result.Categories[0])
	}
	if result.Categories[0].Color != "#ef4444" {
		t.Fatalf("category color = %q, want #ef4444", result.Categories[0].Color)
	}
	if result.Categories[1].Name != uncategorizedName || result.Categories[1].Seconds != 1800 {
		t.Fatalf("second category = %#v", result.Categories[1])
	}

	if len(result.Tags) != 1 || result.Tags[0].Name != "耐玩" || result.Tags[0].Seconds != 10800 {
		t.Fatalf("tags = %#v", result.Tags)
	}

	if len(result.TopGames) != 2 {
		t.Fatalf("top games = %#v, want 2 items", result.TopGames)
	}
	if result.TopGames[0].Name != "Alpha" || result.TopGames[0].Seconds != 10800 ||
		result.TopGames[0].CategoryName != "动作" {
		t.Fatalf("top game = %#v", result.TopGames[0])
	}
	if result.TopGames[1].Name != "Beta" || result.TopGames[1].Seconds != 1800 {
		t.Fatalf("second game = %#v", result.TopGames[1])
	}
}

func TestGetStatsByPeriodEmptyResultHasEmptySlices(t *testing.T) {
	db := newTestDB(t)
	result, err := (&StatsService{db: db}).GetStatsByPeriod(PeriodMonth, "", "")
	if err != nil {
		t.Fatalf("GetStatsByPeriod() error = %v", err)
	}
	if result.TotalSeconds != 0 || result.SessionCount != 0 || result.ActiveDays != 0 {
		t.Fatalf("expected zero totals, got %#v", result)
	}
	if result.Categories == nil || result.Tags == nil || result.TopGames == nil {
		t.Fatal("expected non-nil slices so the UI can render empty states")
	}
	if len(result.Trend) == 0 {
		t.Fatal("expected trend buckets even without data")
	}
}

func TestGetDashboardStatsWindows(t *testing.T) {
	db := newTestDB(t)
	service := &StatsService{db: db}

	alpha := models.Game{Name: "Alpha", TotalSeconds: 5000}
	if err := db.Create(&alpha).Error; err != nil {
		t.Fatal(err)
	}
	idle := models.Game{Name: "Idle"}
	if err := db.Create(&idle).Error; err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	weekStart := startOfWeek(now)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)

	createSession(t, db, alpha.ID, todayStart.Add(12*time.Hour), 600)
	createSession(t, db, alpha.ID, weekStart.Add(9*time.Hour), 300)

	overview, err := service.GetDashboardStats()
	if err != nil {
		t.Fatalf("GetDashboardStats() error = %v", err)
	}

	if overview.TotalSeconds != 5000 {
		t.Fatalf("TotalSeconds = %d, want 5000", overview.TotalSeconds)
	}
	if overview.GameCount != 2 {
		t.Fatalf("GameCount = %d, want 2", overview.GameCount)
	}
	sameDay := weekStart.Year() == now.Year() && weekStart.YearDay() == now.YearDay()

	wantToday := int64(600)
	if sameDay {
		// 今天恰好是周一，两条记录都落在今天。
		wantToday += 300
	}
	if overview.TodaySeconds != wantToday {
		t.Fatalf("TodaySeconds = %d, want %d", overview.TodaySeconds, wantToday)
	}

	wantWeek := int64(600 + 300)
	wantActiveDays := int64(2)
	if sameDay {
		wantActiveDays = 1
	}
	if overview.WeekSeconds != wantWeek {
		t.Fatalf("WeekSeconds = %d, want %d", overview.WeekSeconds, wantWeek)
	}

	wantMonth := int64(600)
	if !weekStart.Before(monthStart) {
		wantMonth += 300
	}
	if overview.MonthSeconds != wantMonth {
		t.Fatalf("MonthSeconds = %d, want %d", overview.MonthSeconds, wantMonth)
	}
	if overview.ActiveDays != wantActiveDays {
		t.Fatalf("ActiveDays = %d, want %d", overview.ActiveDays, wantActiveDays)
	}

	if len(overview.TopGames) != 1 || overview.TopGames[0].Name != "Alpha" ||
		overview.TopGames[0].Seconds != 5000 || overview.TopGames[0].Percent != 100 {
		t.Fatalf("top games = %#v", overview.TopGames)
	}
	if len(overview.Categories) != 1 || overview.Categories[0].Name != uncategorizedName ||
		overview.Categories[0].Seconds != 5000 {
		t.Fatalf("categories = %#v", overview.Categories)
	}
}

func createSession(t *testing.T, db *gorm.DB, gameID uint, start time.Time, duration int64) {
	t.Helper()
	session := models.PlaySession{
		GameID:          gameID,
		StartAt:         start,
		DurationSeconds: duration,
		Date:            start.Format("2006-01-02"),
		Source:          "monitor",
	}
	if err := db.Create(&session).Error; err != nil {
		t.Fatal(err)
	}
}
