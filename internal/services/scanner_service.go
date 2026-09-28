package services

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"gorm.io/gorm"

	"GameArchive/internal/models"
)

// ScannerService 负责目录扫描结果的批量导入。
type ScannerService struct{}

// ImportResult 汇报批量导入成功与跳过的数量。
type ImportResult struct {
	Imported int `json:"imported"`
	Skipped  int `json:"skipped"`
}

// ImportScannedGames 将扫描预览结果导入游戏库；已存在的可执行文件路径会跳过。
func (s *GameService) ImportScannedGames(scanned []models.ScannedGame) (ImportResult, error) {
	result := ImportResult{}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, item := range scanned {
			name := strings.TrimSpace(item.Name)
			exePath := strings.TrimSpace(item.ExePath)
			if name == "" || exePath == "" || !strings.EqualFold(filepath.Ext(exePath), ".exe") {
				return errors.New("扫描结果缺少有效的游戏名称或 .exe 路径")
			}
			absolutePath, err := filepath.Abs(exePath)
			if err != nil {
				return fmt.Errorf("解析游戏路径失败: %w", err)
			}

			var existing int64
			if err := tx.Model(&models.Game{}).
				Where("lower(exe_path) = lower(?)", absolutePath).
				Count(&existing).Error; err != nil {
				return fmt.Errorf("检查重复游戏失败: %w", err)
			}
			if existing > 0 {
				result.Skipped++
				continue
			}

			var categoryID *uint
			if item.CategoryID != nil && *item.CategoryID > 0 {
				var count int64
				if err := tx.Model(&models.Category{}).Where("id = ?", *item.CategoryID).Count(&count).Error; err != nil {
					return fmt.Errorf("校验游戏分类失败: %w", err)
				}
				if count == 0 {
					return errors.New("扫描结果中的游戏分类不存在")
				}
				categoryID = item.CategoryID
			}
			game := models.Game{
				Name:        name,
				ExePath:     absolutePath,
				ProcessName: strings.TrimSpace(item.ProcessName),
				InstallDir:  strings.TrimSpace(item.InstallDir),
				CategoryID:  categoryID,
			}
			if game.ProcessName == "" {
				game.ProcessName = filepath.Base(absolutePath)
			}
			if game.InstallDir == "" {
				game.InstallDir = filepath.Dir(absolutePath)
			}
			if err := normalizeAndValidate(&game, true); err != nil {
				return fmt.Errorf("校验游戏 %q 失败: %w", name, err)
			}
			if err := tx.Omit("Category", "Tags").Create(&game).Error; err != nil {
				return fmt.Errorf("导入游戏 %q 失败: %w", name, err)
			}
			result.Imported++
		}
		return nil
	})
	if err != nil {
		return ImportResult{}, fmt.Errorf("批量导入扫描结果失败: %w", err)
	}
	return result, nil
}
