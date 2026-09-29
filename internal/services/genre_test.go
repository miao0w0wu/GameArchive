package services

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"GameArchive/internal/models"
)

func TestFetchGameGenresParsesAndReplacesSteamAssociations(t *testing.T) {
	db := newTestDB(t)
	game := models.Game{Name: "Test Game", SteamAppID: 440}
	if err := db.Create(&game).Error; err != nil {
		t.Fatal(err)
	}
	manualGenre := models.Genre{SteamGenreID: "manual", Name: "手动类型"}
	if err := db.Create(&manualGenre).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.GameGenre{GameID: game.ID, GenreID: manualGenre.ID, Source: "manual"}).Error; err != nil {
		t.Fatal(err)
	}

	var requestCount atomic.Int32
	requestTimes := make(chan time.Time, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("appids") != "440" || r.URL.Query().Get("l") != "schinese" {
			t.Errorf("unexpected appdetails query: %s", r.URL.RawQuery)
		}
		requestTimes <- time.Now()
		if requestCount.Add(1) == 1 {
			_, _ = fmt.Fprint(w, `{"440":{"success":true,"data":{"genres":[{"id":"1","description":"Action"},{"id":"2","description":" action "},{"id":"3","description":"RPG"}]}}}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"440":{"success":true,"data":{"genres":[{"id":"3","description":"RPG"}]}}}`)
	}))
	defer server.Close()

	service := NewGenreService(db)
	service.endpoint = server.URL
	service.httpClient = server.Client()
	got, err := service.FetchGameGenres(game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "Action" || got[1].Name != "RPG" {
		t.Fatalf("Steam genre names were not parsed and deduplicated: %+v", got)
	}
	if _, err := service.FetchGameGenres(game.ID); err != nil {
		t.Fatal(err)
	}
	firstRequestAt, secondRequestAt := <-requestTimes, <-requestTimes
	if secondRequestAt.Sub(firstRequestAt) < genreRequestInterval-50*time.Millisecond {
		t.Fatalf("Steam requests were not rate limited: interval=%s", secondRequestAt.Sub(firstRequestAt))
	}

	var manualCount, steamCount int64
	db.Model(&models.GameGenre{}).Where("game_id = ? AND source = ?", game.ID, "manual").Count(&manualCount)
	db.Model(&models.GameGenre{}).Where("game_id = ? AND source = ?", game.ID, "steam").Count(&steamCount)
	if manualCount != 1 || steamCount != 1 {
		t.Fatalf("unexpected association counts: manual=%d steam=%d", manualCount, steamCount)
	}
	refreshed, err := service.GetGameGenres(game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(refreshed) != 2 {
		t.Fatalf("Steam refresh did not preserve the manual type and replace Steam types: %+v", refreshed)
	}
}

func TestFetchGameGenresSkipsGamesWithoutSteamAppID(t *testing.T) {
	db := newTestDB(t)
	game := models.Game{Name: "Non-Steam Game"}
	if err := db.Create(&game).Error; err != nil {
		t.Fatal(err)
	}
	service := NewGenreService(db)

	got, err := service.FetchGameGenres(game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no genres for a game without AppID, got %+v", got)
	}
}

func TestFetchGameGenresTreatsSteamHTTPFailureAsEmptyResult(t *testing.T) {
	db := newTestDB(t)
	game := models.Game{Name: "Test Game", SteamAppID: 123}
	if err := db.Create(&game).Error; err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	service := NewGenreService(db)
	service.endpoint = server.URL
	service.httpClient = server.Client()
	got, err := service.FetchGameGenres(game.ID)
	if err != nil {
		t.Fatalf("Steam API failure should not escape as a fatal error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected an empty result on Steam API failure, got %+v", got)
	}
}

func TestBatchFetchGameGenresContinuesAfterRequestFailure(t *testing.T) {
	db := newTestDB(t)
	games := []models.Game{
		{Name: "Unavailable", SteamAppID: 123},
		{Name: "No Steam AppID"},
	}
	if err := db.Create(&games).Error; err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	service := NewGenreService(db)
	service.endpoint = server.URL
	service.httpClient = server.Client()
	result, err := service.BatchFetchGameGenres([]uint{games[0].ID, games[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	if result.Requested != 2 || result.Failed != 1 || result.Skipped != 1 || result.Succeeded != 0 {
		t.Fatalf("unexpected batch result: %+v", result)
	}
}
