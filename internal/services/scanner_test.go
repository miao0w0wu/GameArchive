package services

import (
	"os"
	"path/filepath"
	"testing"

	"GameArchive/internal/models"
)

func TestScanGamesInDirectoryFiltersAndRespectsDepth(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"Game.exe":                              "game",
		"unins000.exe":                          "installer",
		filepath.Join("child", "A.exe"):         "child",
		filepath.Join("child", "deep", "B.exe"): "deep",
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	scanned, err := (&ScannerService{}).ScanGamesInDirectory(root, 1)
	if err != nil {
		t.Fatalf("ScanGamesInDirectory() error = %v", err)
	}
	if len(scanned) != 2 {
		t.Fatalf("got %d scan results, want 2: %#v", len(scanned), scanned)
	}
	if scanned[0].Name != "A" || scanned[1].Name != "Game" {
		t.Fatalf("unexpected scan order or names: %#v", scanned)
	}
	if scanned[1].SizeBytes != int64(len("game")) {
		t.Fatalf("game size = %d, want %d", scanned[1].SizeBytes, len("game"))
	}
}

func TestScanGamesInDirectoryRejectsInvalidDepthAndFile(t *testing.T) {
	root := t.TempDir()
	if _, err := (&ScannerService{}).ScanGamesInDirectory(root, maxScanDepth+1); err == nil {
		t.Fatal("expected excessive depth to fail")
	}
	file := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (&ScannerService{}).ScanGamesInDirectory(file, 1); err == nil {
		t.Fatal("expected file path to fail")
	}
}

func TestImportScannedGamesSkipsDuplicateExecutablePaths(t *testing.T) {
	db := newTrackerTestDB(t)
	path := filepath.Join(t.TempDir(), "Sample.exe")
	game := models.ScannedGame{
		Name:        "Sample",
		ExePath:     path,
		ProcessName: "Sample.exe",
	}
	result, err := (&GameService{db: db}).ImportScannedGames([]models.ScannedGame{game, game})
	if err != nil {
		t.Fatalf("ImportScannedGames() error = %v", err)
	}
	if result.Imported != 1 || result.Skipped != 1 {
		t.Fatalf("import result = %#v, want one imported and one skipped", result)
	}
	var count int64
	if err := db.Model(&models.Game{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("game count = %d, want 1", count)
	}
}
