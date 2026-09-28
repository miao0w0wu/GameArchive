package services

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"GameArchive/internal/models"
)

const maxArchiveNoteLength = 1024

// ArchiveService 管理本地存档引用与备份副本。
type ArchiveService struct {
	db *gorm.DB
}

// AddArchive 将现有文件或目录登记为游戏存档；不会移动或复制源文件。
func (s *ArchiveService) AddArchive(gameID uint, name, sourcePath string, isDir bool, backupDir, note string) (*models.SaveArchive, error) {
	if gameID == 0 {
		return nil, errors.New("缺少游戏 ID")
	}
	if err := s.ensureGameExists(gameID); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("存档名称不能为空")
	}
	if len([]rune(name)) > 255 {
		return nil, errors.New("存档名称不能超过 255 个字符")
	}
	note = strings.TrimSpace(note)
	if len([]rune(note)) > maxArchiveNoteLength {
		return nil, fmt.Errorf("备注不能超过 %d 个字符", maxArchiveNoteLength)
	}

	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return nil, errors.New("存档路径不能为空")
	}
	sourcePath, err := filepath.Abs(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("解析存档路径失败: %w", err)
	}
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("读取存档路径失败: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("不支持将符号链接登记为存档")
	}
	if isDir != info.IsDir() || (!info.IsDir() && !info.Mode().IsRegular()) {
		return nil, errors.New("存档路径类型与选择类型不匹配，且仅支持普通文件或文件夹")
	}
	if isFilesystemRoot(sourcePath) {
		return nil, errors.New("不能将磁盘根目录登记为存档")
	}
	if backupDir != "" {
		backupDir = strings.TrimSpace(backupDir)
		backupDir, err = filepath.Abs(backupDir)
		if err != nil {
			return nil, fmt.Errorf("解析默认备份目录失败: %w", err)
		}
		if err := ensureExistingDirectory(backupDir); err != nil {
			return nil, err
		}
	}

	archive := models.SaveArchive{
		GameID:     gameID,
		Name:       name,
		SourcePath: sourcePath,
		BackupDir:  backupDir,
		Note:       note,
	}
	if err := s.db.Create(&archive).Error; err != nil {
		return nil, fmt.Errorf("登记游戏存档失败: %w", err)
	}
	return &archive, nil
}

// ListArchives 返回一个游戏的全部存档，按创建时间倒序。
func (s *ArchiveService) ListArchives(gameID uint) ([]models.SaveArchive, error) {
	if err := s.ensureGameExists(gameID); err != nil {
		return nil, err
	}
	archives := make([]models.SaveArchive, 0)
	if err := s.db.Where("game_id = ?", gameID).Order("created_at DESC, id DESC").Find(&archives).Error; err != nil {
		return nil, fmt.Errorf("查询游戏存档失败: %w", err)
	}
	return archives, nil
}

// ListBackups 返回一个存档的备份历史，按备份时间倒序。
func (s *ArchiveService) ListBackups(archiveID uint) ([]models.SaveBackup, error) {
	if _, err := s.getArchive(archiveID); err != nil {
		return nil, err
	}
	backups := make([]models.SaveBackup, 0)
	if err := s.db.Where("archive_id = ?", archiveID).Order("backed_up_at DESC, id DESC").Find(&backups).Error; err != nil {
		return nil, fmt.Errorf("查询备份历史失败: %w", err)
	}
	return backups, nil
}

// BackupArchive 将源文件 / 文件夹复制到指定目录，并在复制成功后记录备份历史。
func (s *ArchiveService) BackupArchive(archiveID uint, targetDir, note string) (*models.SaveBackup, error) {
	archive, err := s.getArchive(archiveID)
	if err != nil {
		return nil, err
	}
	note = strings.TrimSpace(note)
	if len([]rune(note)) > maxArchiveNoteLength {
		return nil, fmt.Errorf("备注不能超过 %d 个字符", maxArchiveNoteLength)
	}
	targetDir = strings.TrimSpace(targetDir)
	if targetDir == "" {
		return nil, errors.New("备份目录不能为空")
	}
	targetDir, err = filepath.Abs(targetDir)
	if err != nil {
		return nil, fmt.Errorf("解析备份目录失败: %w", err)
	}
	if err := ensureExistingDirectory(targetDir); err != nil {
		return nil, err
	}

	sourceInfo, err := os.Lstat(archive.SourcePath)
	if err != nil {
		return nil, fmt.Errorf("原始存档不可访问: %w", err)
	}
	if sourceInfo.Mode()&os.ModeSymlink != 0 || (!sourceInfo.IsDir() && !sourceInfo.Mode().IsRegular()) {
		return nil, errors.New("原始存档不是支持的普通文件或文件夹")
	}
	if sourceInfo.IsDir() {
		sourceRealPath, err := filepath.EvalSymlinks(archive.SourcePath)
		if err != nil {
			return nil, fmt.Errorf("解析原始存档真实路径失败: %w", err)
		}
		targetRealPath, err := filepath.EvalSymlinks(targetDir)
		if err != nil {
			return nil, fmt.Errorf("解析备份目录真实路径失败: %w", err)
		}
		if pathContains(sourceRealPath, targetRealPath) {
			return nil, errors.New("备份目录不能位于原始存档文件夹内")
		}
	}

	var game models.Game
	if err := s.db.First(&game, archive.GameID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("查询所属游戏失败: %w", err)
	}

	backedUpAt := time.Now()
	backupName := fmt.Sprintf("%s_%s_%s", safeArchiveName(game.Name), safeArchiveName(archive.Name), backedUpAt.Format("20060102-150405"))
	if !sourceInfo.IsDir() {
		backupName += filepath.Ext(archive.SourcePath)
	}
	stagingDir, err := os.MkdirTemp(targetDir, ".gamearchive-backup-")
	if err != nil {
		return nil, fmt.Errorf("创建备份暂存目录失败: %w", err)
	}
	defer os.RemoveAll(stagingDir)

	stagedCopyPath := filepath.Join(stagingDir, backupName)
	if err := copyPath(archive.SourcePath, stagedCopyPath); err != nil {
		return nil, fmt.Errorf("复制存档失败: %w", err)
	}

	backupPath, err := uniqueBackupPath(targetDir, backupName)
	if err != nil {
		return nil, err
	}
	if err := os.Rename(stagedCopyPath, backupPath); err != nil {
		return nil, fmt.Errorf("保存备份文件失败: %w", err)
	}

	backup := models.SaveBackup{
		ArchiveID:  archive.ID,
		BackupPath: backupPath,
		BackedUpAt: backedUpAt,
		Note:       note,
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&backup).Error; err != nil {
			return fmt.Errorf("写入备份历史失败: %w", err)
		}
		update := tx.Model(&models.SaveArchive{}).Where("id = ?", archive.ID).Updates(map[string]any{
			"backup_dir":     targetDir,
			"last_backup_at": backedUpAt,
		})
		if update.Error != nil {
			return fmt.Errorf("更新存档备份信息失败: %w", update.Error)
		}
		if update.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
	if err != nil {
		if cleanupErr := os.RemoveAll(backupPath); cleanupErr != nil {
			return nil, fmt.Errorf("%w；清理未登记的备份文件失败: %v", err, cleanupErr)
		}
		return nil, err
	}
	return &backup, nil
}

// DeleteArchive 删除存档记录；只有 deleteOriginal 为 true 时才删除原始文件。
// 备份文件始终保留在磁盘，避免删除存档记录时意外清除历史备份。
func (s *ArchiveService) DeleteArchive(archiveID uint, deleteOriginal bool) error {
	archive, err := s.getArchive(archiveID)
	if err != nil {
		return err
	}
	if deleteOriginal {
		if err := removeManagedPath(archive.SourcePath); err != nil {
			return fmt.Errorf("删除原始存档失败: %w", err)
		}
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("archive_id = ?", archiveID).Delete(&models.SaveBackup{}).Error; err != nil {
			return fmt.Errorf("删除备份历史记录失败: %w", err)
		}
		result := tx.Delete(&models.SaveArchive{}, archiveID)
		if result.Error != nil {
			return fmt.Errorf("删除存档记录失败: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// DeleteBackup 删除备份历史；只有 deleteFile 为 true 时才删除对应备份文件。
func (s *ArchiveService) DeleteBackup(backupID uint, deleteFile bool) error {
	var backup models.SaveBackup
	err := s.db.First(&backup, backupID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("查询备份记录失败: %w", err)
	}
	if deleteFile {
		if err := removeManagedPath(backup.BackupPath); err != nil {
			return fmt.Errorf("删除备份文件失败: %w", err)
		}
	}
	result := s.db.Delete(&models.SaveBackup{}, backupID)
	if result.Error != nil {
		return fmt.Errorf("删除备份记录失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *ArchiveService) ensureGameExists(gameID uint) error {
	if gameID == 0 {
		return errors.New("缺少游戏 ID")
	}
	var count int64
	if err := s.db.Model(&models.Game{}).Where("id = ?", gameID).Count(&count).Error; err != nil {
		return fmt.Errorf("查询游戏失败: %w", err)
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *ArchiveService) getArchive(archiveID uint) (*models.SaveArchive, error) {
	var archive models.SaveArchive
	err := s.db.First(&archive, archiveID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询存档失败: %w", err)
	}
	return &archive, nil
}

func ensureExistingDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("读取备份目录失败: %w", err)
	}
	if !info.IsDir() {
		return errors.New("备份目标路径不是文件夹")
	}
	return nil
}

func isFilesystemRoot(path string) bool {
	clean := filepath.Clean(path)
	return filepath.Dir(clean) == clean
}

func pathContains(parent, child string) bool {
	rel, err := filepath.Rel(filepath.Clean(parent), filepath.Clean(child))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func safeArchiveName(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r < 32 || strings.ContainsRune(`<>:"/\|?*`, r):
			builder.WriteRune('_')
		default:
			builder.WriteRune(r)
		}
	}
	value = strings.Trim(builder.String(), " .")
	if value == "" {
		return "save"
	}
	if len([]rune(value)) > 80 {
		value = string([]rune(value)[:80])
	}
	return value
}

func uniqueBackupPath(targetDir, name string) (string, error) {
	extension := filepath.Ext(name)
	stem := strings.TrimSuffix(name, extension)
	for suffix := 0; suffix < 1000; suffix++ {
		candidateName := name
		if suffix > 0 {
			candidateName = fmt.Sprintf("%s_%d%s", stem, suffix, extension)
		}
		candidate := filepath.Join(targetDir, candidateName)
		_, err := os.Lstat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		}
		if err != nil {
			return "", fmt.Errorf("检查备份路径失败: %w", err)
		}
	}
	return "", errors.New("无法生成唯一备份路径")
}

func copyPath(source, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("存档中包含不支持的符号链接: %s", source)
	}
	if info.IsDir() {
		return copyDirectory(source, target, info)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("存档中包含不支持的特殊文件: %s", source)
	}
	return copyRegularFile(source, target, info)
}

func copyDirectory(source, target string, rootInfo os.FileInfo) error {
	if err := os.Mkdir(target, rootInfo.Mode().Perm()); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == source {
			return nil
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(target, relative)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("存档中包含不支持的符号链接: %s", path)
		}
		if entry.IsDir() {
			return os.Mkdir(targetPath, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("存档中包含不支持的特殊文件: %s", path)
		}
		return copyRegularFile(path, targetPath, info)
	})
}

func copyRegularFile(source, target string, info os.FileInfo) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Chtimes(target, info.ModTime(), info.ModTime())
}

func removeManagedPath(path string) error {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." || isFilesystemRoot(path) {
		return errors.New("拒绝删除空路径或磁盘根目录")
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return os.Remove(path)
	}
	return os.RemoveAll(path)
}
