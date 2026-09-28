package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/gorm"

	"GameArchive/internal/models"
)

func TestAddArchiveAndBackupFile(t *testing.T) {
	db := newTestDB(t)
	service := &ArchiveService{db: db}
	game := createArchiveTestGame(t, db)
	root := t.TempDir()
	source := filepath.Join(root, "slot.dat")
	if err := os.WriteFile(source, []byte("save data"), 0o600); err != nil {
		t.Fatal(err)
	}
	targetDir := filepath.Join(root, "backups")
	if err := os.Mkdir(targetDir, 0o700); err != nil {
		t.Fatal(err)
	}

	archive, err := service.AddArchive(game.ID, "Main save", source, false, targetDir, "manual source")
	if err != nil {
		t.Fatalf("AddArchive() error = %v", err)
	}
	if archive.SourcePath != source || archive.Note != "manual source" || archive.BackupDir != targetDir {
		t.Fatalf("archive = %#v", archive)
	}

	backup, err := service.BackupArchive(archive.ID, targetDir, "before update")
	if err != nil {
		t.Fatalf("BackupArchive() error = %v", err)
	}
	if backup.ArchiveID != archive.ID || backup.Note != "before update" || backup.BackedUpAt.IsZero() {
		t.Fatalf("backup = %#v", backup)
	}
	if filepath.Ext(backup.BackupPath) != filepath.Ext(source) {
		t.Fatalf("backup path should preserve the source extension: %q", backup.BackupPath)
	}
	data, err := os.ReadFile(backup.BackupPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "save data" {
		t.Fatalf("backup contents = %q", data)
	}
	original, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("backup unexpectedly removed source: %v", err)
	}
	if string(original) != "save data" {
		t.Fatalf("original contents = %q", original)
	}
	secondBackup, err := service.BackupArchive(archive.ID, targetDir, "second copy")
	if err != nil {
		t.Fatalf("second BackupArchive() error = %v", err)
	}
	if secondBackup.BackupPath == backup.BackupPath {
		t.Fatalf("repeated backups should use unique paths: %q", backup.BackupPath)
	}

	archives, err := service.ListArchives(game.ID)
	if err != nil {
		t.Fatalf("ListArchives() error = %v", err)
	}
	if len(archives) != 1 || archives[0].LastBackupAt == nil || archives[0].BackupDir != targetDir {
		t.Fatalf("listed archives = %#v", archives)
	}
	backups, err := service.ListBackups(archive.ID)
	if err != nil {
		t.Fatalf("ListBackups() error = %v", err)
	}
	if len(backups) != 2 || backups[0].ID != secondBackup.ID || backups[1].ID != backup.ID {
		t.Fatalf("listed backups = %#v", backups)
	}
}

func TestBackupArchiveCopiesDirectoryAndRejectsNestedTarget(t *testing.T) {
	db := newTestDB(t)
	service := &ArchiveService{db: db}
	game := createArchiveTestGame(t, db)
	root := t.TempDir()
	source := filepath.Join(root, "save-folder")
	if err := os.Mkdir(source, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(source, "slot"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "slot", "save.dat"), []byte("folder save"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive, err := service.AddArchive(game.ID, "Folder", source, true, "", "")
	if err != nil {
		t.Fatalf("AddArchive() error = %v", err)
	}

	if _, err := service.BackupArchive(archive.ID, filepath.Join(source, "inside"), ""); err == nil {
		t.Fatal("expected a nested target directory to be rejected")
	}
	targetDir := filepath.Join(root, "backup-root")
	if err := os.Mkdir(targetDir, 0o700); err != nil {
		t.Fatal(err)
	}
	backup, err := service.BackupArchive(archive.ID, targetDir, "")
	if err != nil {
		t.Fatalf("BackupArchive() error = %v", err)
	}
	copyContents, err := os.ReadFile(filepath.Join(backup.BackupPath, "slot", "save.dat"))
	if err != nil {
		t.Fatal(err)
	}
	if string(copyContents) != "folder save" {
		t.Fatalf("copied contents = %q", copyContents)
	}
}

func TestDeleteArchivePreservesFilesUnlessConfirmed(t *testing.T) {
	db := newTestDB(t)
	service := &ArchiveService{db: db}
	game := createArchiveTestGame(t, db)
	root := t.TempDir()
	source := filepath.Join(root, "save.dat")
	if err := os.WriteFile(source, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive, err := service.AddArchive(game.ID, "Save", source, false, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteArchive(archive.ID, false); err != nil {
		t.Fatalf("DeleteArchive(false) error = %v", err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("deleting a record removed the original: %v", err)
	}
	if _, err := service.getArchive(archive.ID); err == nil {
		t.Fatal("expected the archive record to be deleted")
	}

	archive, err = service.AddArchive(game.ID, "Save", source, false, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteArchive(archive.ID, true); err != nil {
		t.Fatalf("DeleteArchive(true) error = %v", err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("confirmed deletion should remove the original; stat error = %v", err)
	}
}

func TestDeleteBackupPreservesBackupUnlessConfirmed(t *testing.T) {
	db := newTestDB(t)
	service := &ArchiveService{db: db}
	game := createArchiveTestGame(t, db)
	root := t.TempDir()
	source := filepath.Join(root, "save.dat")
	if err := os.WriteFile(source, []byte("save"), 0o600); err != nil {
		t.Fatal(err)
	}
	targetDir := filepath.Join(root, "backups")
	if err := os.Mkdir(targetDir, 0o700); err != nil {
		t.Fatal(err)
	}
	archive, err := service.AddArchive(game.ID, "Save", source, false, "", "")
	if err != nil {
		t.Fatal(err)
	}
	backup, err := service.BackupArchive(archive.ID, targetDir, "")
	if err != nil {
		t.Fatal(err)
	}

	if err := service.DeleteBackup(backup.ID, false); err != nil {
		t.Fatalf("DeleteBackup(false) error = %v", err)
	}
	if _, err := os.Stat(backup.BackupPath); err != nil {
		t.Fatalf("deleting a backup record removed its files: %v", err)
	}

	backup, err = service.BackupArchive(archive.ID, targetDir, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteBackup(backup.ID, true); err != nil {
		t.Fatalf("DeleteBackup(true) error = %v", err)
	}
	if _, err := os.Stat(backup.BackupPath); !os.IsNotExist(err) {
		t.Fatalf("confirmed deletion should remove backup files; stat error = %v", err)
	}
}

func TestAddArchiveRejectsInvalidPathsAndTypes(t *testing.T) {
	db := newTestDB(t)
	service := &ArchiveService{db: db}
	game := createArchiveTestGame(t, db)
	root := t.TempDir()
	source := filepath.Join(root, "save.dat")
	if err := os.WriteFile(source, []byte("save"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		path string
		dir  bool
	}{
		{name: "empty source"},
		{name: "wrong type", path: source, dir: true},
		{name: "filesystem root", path: filepath.VolumeName(root) + string(filepath.Separator), dir: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := service.AddArchive(game.ID, "Save", testCase.path, testCase.dir, "", "")
			if err == nil {
				t.Fatal("expected invalid archive input to be rejected")
			}
		})
	}

	if _, err := service.AddArchive(9999, "Save", source, false, "", ""); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("AddArchive() missing game error = %v", err)
	}
	if _, err := service.AddArchive(game.ID, "Save", source, false, root, strings.Repeat("x", maxArchiveNoteLength+1)); err == nil {
		t.Fatal("expected long note to be rejected")
	}
}

func createArchiveTestGame(t *testing.T, db *gorm.DB) models.Game {
	t.Helper()
	game := models.Game{Name: "Test Game"}
	if err := db.Create(&game).Error; err != nil {
		t.Fatal(err)
	}
	return game
}
