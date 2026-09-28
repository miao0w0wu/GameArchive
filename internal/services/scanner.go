package services

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"GameArchive/internal/models"
)

const (
	defaultScanDepth = 3
	maxScanDepth     = 20
)

var ignoredExecutableKeywords = []string{
	"unins", "uninstall", "redist", "crash", "launcher", "setup", "update",
}

// ScanGamesInDirectory 递归扫描目录中的 Windows 可执行游戏，并返回可供预览的结果。
func (s *ScannerService) ScanGamesInDirectory(dir string, depth int) ([]models.ScannedGame, error) {
	root, err := filepath.Abs(strings.TrimSpace(dir))
	if err != nil {
		return nil, fmt.Errorf("解析扫描目录失败: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("读取扫描目录失败: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("扫描路径不是目录")
	}
	if depth <= 0 {
		depth = defaultScanDepth
	}
	if depth > maxScanDepth {
		return nil, fmt.Errorf("扫描深度不能超过 %d 层", maxScanDepth)
	}

	results := make([]models.ScannedGame, 0)
	seen := make(map[string]struct{})
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("扫描路径 %q 失败: %w", path, walkErr)
		}
		if entry.IsDir() {
			if path != root {
				relative, relErr := filepath.Rel(root, path)
				if relErr != nil {
					return fmt.Errorf("计算扫描深度失败: %w", relErr)
				}
				if strings.Count(relative, string(filepath.Separator))+1 > depth {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || !strings.EqualFold(filepath.Ext(entry.Name()), ".exe") {
			return nil
		}

		lowerName := strings.ToLower(entry.Name())
		for _, keyword := range ignoredExecutableKeywords {
			if strings.Contains(lowerName, keyword) {
				return nil
			}
		}

		absolutePath, absErr := filepath.Abs(path)
		if absErr != nil {
			return fmt.Errorf("解析可执行文件路径失败: %w", absErr)
		}
		key := strings.ToLower(filepath.Clean(absolutePath))
		if _, exists := seen[key]; exists {
			return nil
		}
		seen[key] = struct{}{}

		fileInfo, statErr := entry.Info()
		if statErr != nil {
			return fmt.Errorf("读取可执行文件信息失败: %w", statErr)
		}
		installDir := filepath.Dir(absolutePath)
		name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		results = append(results, models.ScannedGame{
			Name:              name,
			ExePath:           absolutePath,
			ProcessName:       entry.Name(),
			InstallDir:        installDir,
			SizeBytes:         fileInfo.Size(),
			SuggestedCategory: suggestCategory(name),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("扫描游戏目录失败: %w", err)
	}

	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
	})
	return results, nil
}

func suggestCategory(name string) string {
	lower := strings.ToLower(name)
	for _, match := range []struct {
		category string
		keywords []string
	}{
		{"射击", []string{"shooter", "fps", "battlefield", "counter-strike"}},
		{"动作", []string{"action", "devilmaycry", "eldenring", "souls"}},
		{"角色扮演", []string{"rpg", "witcher", "persona", "finalfantasy"}},
		{"策略", []string{"strategy", "civilization", "totalwar"}},
		{"竞速体育", []string{"racing", "forza", "fifa", "nba"}},
		{"模拟经营", []string{"simulator", "simulation", "sims"}},
		{"冒险", []string{"adventure", "tombraider", "uncharted"}},
	} {
		for _, keyword := range match.keywords {
			if strings.Contains(lower, keyword) {
				return match.category
			}
		}
	}
	return ""
}
