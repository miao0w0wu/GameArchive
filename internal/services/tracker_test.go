package services

import (
	"sync/atomic"
	"testing"
	"time"

	"GameArchive/internal/models"
)

func TestTrackerAccountsAndClosesSession(t *testing.T) {
	db := newTestDB(t)
	game := models.Game{Name: "Sample Game", ProcessName: "sample.exe"}
	if err := db.Create(&game).Error; err != nil {
		t.Fatal(err)
	}

	var running atomic.Bool
	running.Store(true)
	tracker := NewTracker(db, func() ([]ProcessInfo, error) {
		if running.Load() {
			return []ProcessInfo{{Name: "sample.exe"}}, nil
		}
		return nil, nil
	}, nil)
	tracker.interval = 10 * time.Millisecond
	tracker.Start()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var stored models.Game
		if err := db.First(&stored, game.ID).Error; err != nil {
			t.Fatal(err)
		}
		if stored.TotalSeconds > 0 {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	var stored models.Game
	if err := db.First(&stored, game.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.TotalSeconds == 0 {
		t.Fatal("tracker did not accumulate elapsed time")
	}

	running.Store(false)
	time.Sleep(25 * time.Millisecond)
	if err := tracker.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	var session models.PlaySession
	if err := db.First(&session, "game_id = ?", game.ID).Error; err != nil {
		t.Fatal(err)
	}
	if session.EndAt == nil {
		t.Fatal("session was not closed when monitoring stopped")
	}
	if err := db.First(&stored, game.ID).Error; err != nil {
		t.Fatal(err)
	}
	if session.DurationSeconds != stored.TotalSeconds {
		t.Fatalf("session duration %d differs from game total %d", session.DurationSeconds, stored.TotalSeconds)
	}
}

func TestIsGameRunningMatchesPathBeforeNameFallback(t *testing.T) {
	game := models.Game{
		ExePath:     "C:\\Games\\Target\\target.exe",
		ProcessName: "target.exe",
	}
	if !isGameRunning(game, []ProcessInfo{{ExePath: "C:\\Games\\Target\\target.exe", Name: "different.exe"}}) {
		t.Fatal("expected executable path match")
	}
	if !isGameRunning(game, []ProcessInfo{{ExePath: "D:\\Other\\target.exe", Name: "target.exe"}}) {
		t.Fatal("expected process name fallback")
	}
	if isGameRunning(game, []ProcessInfo{{ExePath: "D:\\Other\\different.exe", Name: "different.exe"}}) {
		t.Fatal("unexpected match")
	}
}

func TestRecoverClosesAbandonedSessionWithoutCountingDowntime(t *testing.T) {
	db := newTestDB(t)
	game := models.Game{Name: "Sample Game", TotalSeconds: 12}
	if err := db.Create(&game).Error; err != nil {
		t.Fatal(err)
	}
	started := time.Now().Add(-time.Hour).Truncate(time.Second)
	session := models.PlaySession{
		GameID:          game.ID,
		StartAt:         started,
		DurationSeconds: 7,
		Date:            started.Format("2006-01-02"),
		Source:          "monitor",
	}
	if err := db.Create(&session).Error; err != nil {
		t.Fatal(err)
	}
	if err := NewTracker(db, nil, nil).Recover(); err != nil {
		t.Fatalf("Recover() error = %v", err)
	}
	if err := db.First(&session, session.ID).Error; err != nil {
		t.Fatal(err)
	}
	wantEnd := started.Add(7 * time.Second)
	if session.EndAt == nil || !session.EndAt.Equal(wantEnd) {
		t.Fatalf("recovered end time = %v, want %v", session.EndAt, wantEnd)
	}
	if err := db.First(&game, game.ID).Error; err != nil {
		t.Fatal(err)
	}
	if game.TotalSeconds != 12 {
		t.Fatalf("recovery changed total seconds to %d", game.TotalSeconds)
	}
}
