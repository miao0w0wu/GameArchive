package services

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"GameArchive/internal/models"
)

func TestCoverSettingsAndLibraryDisplaySettings(t *testing.T) {
	db := newTestDB(t)
	service := NewCoverService(db, t.TempDir())

	display, err := service.GetLibraryDisplaySettings()
	if err != nil {
		t.Fatal(err)
	}
	if display.ViewMode != "list" || display.Columns != 3 || display.CardSize != "medium" {
		t.Fatalf("unexpected display defaults: %+v", display)
	}
	display = LibraryDisplaySettings{ViewMode: "card", Columns: 5, CardSize: "large"}
	if err := service.SaveLibraryDisplaySettings(display); err != nil {
		t.Fatal(err)
	}
	gotDisplay, err := service.GetLibraryDisplaySettings()
	if err != nil {
		t.Fatal(err)
	}
	if gotDisplay != display {
		t.Fatalf("display settings = %+v, want %+v", gotDisplay, display)
	}
	if err := service.SaveLibraryDisplaySettings(LibraryDisplaySettings{ViewMode: "grid", Columns: 1, CardSize: "huge"}); err == nil {
		t.Fatal("SaveLibraryDisplaySettings() accepted invalid values")
	}

	config, err := service.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !config.OnlineEnabled || config.CacheDir != service.cacheDir ||
		config.IconCacheDir != service.iconsDir {
		t.Fatalf("unexpected cover settings defaults: %+v", config)
	}
	config.SteamGridDBKey = "sgdb-key"
	config.RAWGKey = "rawg-key"
	config.OnlineEnabled = false
	if err := service.SaveSettings(config); err != nil {
		t.Fatal(err)
	}
	gotConfig, err := service.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if gotConfig.OnlineEnabled || gotConfig.SteamGridDBKey != config.SteamGridDBKey || gotConfig.RAWGKey != config.RAWGKey {
		t.Fatalf("cover settings not persisted: %+v", gotConfig)
	}
}

func TestSetImageCopiesToCacheAndPersistsManualSource(t *testing.T) {
	db := newTestDB(t)
	cacheDir := t.TempDir()
	service := NewCoverService(db, cacheDir)
	game := models.Game{Name: "Cover Test"}
	if err := db.Create(&game).Error; err != nil {
		t.Fatal(err)
	}

	source := filepath.Join(t.TempDir(), "source.png")
	file, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	picture := image.NewRGBA(image.Rect(0, 0, 4, 4))
	picture.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(file, picture); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	path, err := service.SetImage(game.ID, "cover", source)
	if err != nil {
		t.Fatalf("SetImage() error = %v", err)
	}
	if path == source {
		t.Fatal("SetImage() did not copy image into the cache")
	}
	var stored models.Game
	if err := db.First(&stored, game.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.CoverPath != path || stored.CoverSource != "manual" || stored.CoverUpdatedAt == nil {
		t.Fatalf("manual cover metadata not stored: %+v", stored)
	}
	dataURL, err := service.GetImageData(game.ID, "cover")
	if err != nil {
		t.Fatal(err)
	}
	if len(dataURL) < len("data:image/png;base64,") || dataURL[:len("data:image/png;base64,")] != "data:image/png;base64," {
		t.Fatalf("GetImageData() returned invalid data URL: %q", dataURL)
	}
	if err := service.ClearImage(game.ID, "cover"); err != nil {
		t.Fatal(err)
	}
	stored = models.Game{}
	if err := db.First(&stored, game.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.CoverPath != "" || stored.CoverSource != "none" || stored.CoverUpdatedAt != nil {
		t.Fatalf("cover metadata was not cleared: %+v", stored)
	}
}

func TestAutoFetchUsesLocalImageAndPreservesExistingImage(t *testing.T) {
	db := newTestDB(t)
	root := t.TempDir()
	service := NewCoverService(db, filepath.Join(root, "cache"))
	gameDir := filepath.Join(root, "installed-game")
	if err := os.MkdirAll(gameDir, 0o755); err != nil {
		t.Fatal(err)
	}

	imagePath := filepath.Join(gameDir, "cover.jpg")
	picture := image.NewRGBA(image.Rect(0, 0, 3, 3))
	file, err := os.Create(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, picture); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	game := models.Game{Name: "Local Cover", InstallDir: gameDir}
	if err := db.Create(&game).Error; err != nil {
		t.Fatal(err)
	}

	result, err := service.AutoFetch(game.ID, "cover", false)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Found || result.Source != "local" {
		t.Fatalf("local image was not used: %+v", result)
	}
	skipped, err := service.AutoFetch(game.ID, "cover", false)
	if err != nil {
		t.Fatal(err)
	}
	if skipped.Found || skipped.Reason != "已有图片，跳过自动覆盖" {
		t.Fatalf("existing image was not protected from automatic overwrite: %+v", skipped)
	}
}

func TestGameServiceMarksEditedCoverAsManual(t *testing.T) {
	db := newTestDB(t)
	service := &GameService{db: db}
	game, err := service.Add(models.Game{Name: "Manual cover"})
	if err != nil {
		t.Fatal(err)
	}
	game.CoverPath = filepath.Join(t.TempDir(), "cover.jpg")
	updated, err := service.Update(*game)
	if err != nil {
		t.Fatal(err)
	}
	if updated.CoverSource != "manual" || updated.CoverUpdatedAt == nil {
		t.Fatalf("updated cover was not marked manual: %+v", updated)
	}
	updated.CoverPath = ""
	cleared, err := service.Update(*updated)
	if err != nil {
		t.Fatal(err)
	}
	if cleared.CoverSource != "none" || cleared.CoverUpdatedAt != nil {
		t.Fatalf("cleared cover metadata is incorrect: %+v", cleared)
	}
}
