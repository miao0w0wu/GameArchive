package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"GameArchive/internal/models"
)

func TestSteamSyncMatchesAndImportsGames(t *testing.T) {
	db := newTestDB(t)
	existingByAppID := models.Game{
		Name:         "Renamed Locally",
		TotalSeconds: 900,
		SteamAppID:   100,
	}
	existingByName := models.Game{
		Name:         "case sensitive game",
		TotalSeconds: 300,
	}
	if err := db.Create(&existingByAppID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&existingByName).Error; err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("key") != "test-api-key" || query.Get("steamid") != "76561198000000000" {
			t.Errorf("unexpected Steam credentials in request: %v", query)
		}
		if query.Get("include_appinfo") != "1" || query.Get("include_played_free_games") != "1" {
			t.Errorf("expected app info and free games in request, got %v", query)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"response": map[string]any{
				"game_count": 3,
				"games": []map[string]any{
					{"appid": 100, "name": "Steam Renamed Game", "playtime_forever": 120},
					{"appid": 200, "name": "Case Sensitive Game", "playtime_forever": 30},
					{"appid": 300, "name": "Steam Only Game", "playtime_forever": 10},
				},
			},
		})
	}))
	defer server.Close()

	service := NewSteamService(db)
	service.endpoint = server.URL
	if err := service.SaveConfig(SteamConfig{APIKey: " test-api-key ", SteamID: "76561198000000000"}); err != nil {
		t.Fatal(err)
	}

	result, err := service.SyncLibrary(context.Background())
	if err != nil {
		t.Fatalf("SyncLibrary() error = %v", err)
	}
	if result.FetchedGames != 3 || result.MatchedByAppID != 1 ||
		result.MatchedByName != 1 || result.ImportedGames != 1 {
		t.Fatalf("unexpected sync counts: %+v", result)
	}
	if result.TotalPlaytimeSeconds != 9600 {
		t.Fatalf("TotalPlaytimeSeconds = %d, want 9600", result.TotalPlaytimeSeconds)
	}

	var storedByAppID, storedByName, imported models.Game
	if err := db.First(&storedByAppID, existingByAppID.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&storedByName, existingByName.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedByAppID.Name != "Renamed Locally" || storedByAppID.TotalSeconds != 900 ||
		storedByAppID.SteamPlaytimeSeconds != 7200 || storedByAppID.SteamAppID != 100 {
		t.Fatalf("AppID match modified unexpected fields: %+v", storedByAppID)
	}
	if storedByName.SteamPlaytimeSeconds != 1800 || storedByName.SteamAppID != 200 ||
		storedByName.TotalSeconds != 300 {
		t.Fatalf("name match failed: %+v", storedByName)
	}
	if err := db.First(&imported, "steam_app_id = ?", 300).Error; err != nil {
		t.Fatal(err)
	}
	if imported.Name != "Steam Only Game" || imported.SteamPlaytimeSeconds != 600 {
		t.Fatalf("Steam-only game import failed: %+v", imported)
	}

	status, err := service.GetSyncStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.LastResult == nil || status.LastResult.FetchedGames != 3 || status.LastError != "" {
		t.Fatalf("unexpected persisted sync status: %+v", status)
	}
}

func TestSteamSyncFailureIsVisibleAndDoesNotModifyGames(t *testing.T) {
	db := newTestDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "request rejected for key secret-api-key", http.StatusForbidden)
	}))
	defer server.Close()

	service := NewSteamService(db)
	service.endpoint = server.URL
	if err := service.SaveConfig(SteamConfig{APIKey: "secret-api-key", SteamID: "76561198000000000"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SyncLibrary(context.Background()); err == nil {
		t.Fatal("SyncLibrary() succeeded for a private profile response")
	}
	status, err := service.GetSyncStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.LastError == "" || status.LastAttemptAt == "" {
		t.Fatalf("sync failure was not persisted: %+v", status)
	}
	if strings.Contains(status.LastError, "secret-api-key") {
		t.Fatalf("Steam API Key leaked in persisted error: %q", status.LastError)
	}

	var gameCount int64
	if err := db.Model(&models.Game{}).Count(&gameCount).Error; err != nil {
		t.Fatal(err)
	}
	if gameCount != 0 {
		t.Fatalf("failed sync wrote %d games", gameCount)
	}
}

func TestSteamSaveConfigValidatesCredentials(t *testing.T) {
	service := NewSteamService(newTestDB(t))
	if err := service.SaveConfig(SteamConfig{APIKey: "key"}); err == nil {
		t.Fatal("SaveConfig() accepted an API Key without a SteamID")
	}
	if err := service.SaveConfig(SteamConfig{APIKey: string(make([]byte, 257)), SteamID: "76561198000000000"}); err == nil {
		t.Fatal("SaveConfig() accepted an API Key longer than 256 characters")
	}
	if err := service.SaveConfig(SteamConfig{APIKey: "key", SteamID: "not-a-steamid"}); err == nil {
		t.Fatal("SaveConfig() accepted an invalid SteamID")
	}
	if err := service.SaveConfig(SteamConfig{APIKey: "key", SteamID: "76561198000000000"}); err != nil {
		t.Fatalf("SaveConfig() rejected valid credentials: %v", err)
	}
}

func TestRedactSteamSecret(t *testing.T) {
	const secret = "secret+key"
	message := "request failed for key=secret%2Bkey or secret+key"
	redacted := redactSteamSecret(message, secret)
	if redacted != "request failed for key=[REDACTED] or [REDACTED]" {
		t.Fatalf("redactSteamSecret() = %q", redacted)
	}
}
