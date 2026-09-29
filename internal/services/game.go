package services

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"

	"GameArchive/internal/models"
)

// GameService 负责游戏的增删改查。
type GameService struct{ db *gorm.DB }

// GameDetail 游戏详情：基础信息 + 便于前端展示的派生字段。
type GameDetail struct {
	models.Game
	// TagNames 标签名称列表（按游戏标签顺序）。
	TagNames []string `json:"tagNames"`
	// TotalHours 累计时长（小时，保留两位小数）。
	TotalHours float64 `json:"totalHours"`
}

// List 返回全部游戏（含分类与标签），按最近更新倒序。
func (s *GameService) List() ([]models.Game, error) {
	games := make([]models.Game, 0)
	err := s.db.Preload("Category").Preload("Tags").
		Order("updated_at DESC, id DESC").
		Find(&games).Error
	if err != nil {
		return nil, fmt.Errorf("查询游戏列表失败: %w", err)
	}
	return games, nil
}

// Get 按 ID 查询游戏（含分类与标签）。
func (s *GameService) Get(id uint) (*models.Game, error) {
	var game models.Game
	err := s.db.Preload("Category").Preload("Tags").First(&game, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询游戏失败: %w", err)
	}
	return &game, nil
}

// Detail 返回游戏详情。
func (s *GameService) Detail(id uint) (*GameDetail, error) {
	game, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(game.Tags))
	for _, t := range game.Tags {
		names = append(names, t.Name)
	}
	return &GameDetail{
		Game:       *game,
		TagNames:   names,
		TotalHours: roundHours(game.TotalSeconds),
	}, nil
}

// Add 新增游戏。
//
// 入参 game 中的 Tags 只需要带 id 字段即可关联已存在的标签；
// total_seconds / last_played_at 由阶段 2 的进程监控维护，这里强制初始化为 0。
func (s *GameService) Add(game models.Game) (*models.Game, error) {
	if err := normalizeAndValidate(&game, true); err != nil {
		return nil, err
	}

	if err := s.ensureCategoryExists(game.CategoryID); err != nil {
		return nil, err
	}

	tagIDs := collectTagIDs(game.Tags)
	if game.CoverPath != "" {
		now := time.Now()
		game.CoverSource = "manual"
		game.CoverUpdatedAt = &now
	}
	game.ID = 0
	game.Tags = nil

	if err := s.db.Omit("Category", "Tags").Create(&game).Error; err != nil {
		return nil, fmt.Errorf("新增游戏失败: %w", err)
	}
	if err := s.replaceTags(game.ID, tagIDs); err != nil {
		return nil, err
	}
	return s.Get(game.ID)
}

// Update 更新游戏基础信息与标签关联。
//
// 只更新用户可编辑的字段，累计时长与最后游玩时间不受前端影响。
func (s *GameService) Update(game models.Game) (*models.Game, error) {
	if game.ID == 0 {
		return nil, errors.New("缺少游戏 ID")
	}
	if err := normalizeAndValidate(&game, false); err != nil {
		return nil, err
	}
	current, err := s.Get(game.ID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureCategoryExists(game.CategoryID); err != nil {
		return nil, err
	}

	// 使用 map 更新，保证清空分类（category_id = NULL）等零值也能落库。
	fields := map[string]any{
		"name":         game.Name,
		"exe_path":     game.ExePath,
		"process_name": game.ProcessName,
		"install_dir":  game.InstallDir,
		"cover_path":   game.CoverPath,
		"category_id":  game.CategoryID,
	}
	if game.CoverPath != current.CoverPath {
		if game.CoverPath == "" {
			fields["cover_source"] = "none"
			fields["cover_updated_at"] = nil
		} else {
			fields["cover_source"] = "manual"
			fields["cover_updated_at"] = time.Now()
		}
	}
	if err := s.db.Model(&models.Game{}).Where("id = ?", game.ID).Updates(fields).Error; err != nil {
		return nil, fmt.Errorf("更新游戏失败: %w", err)
	}
	if game.CoverPath == "" && game.CoverPath != current.CoverPath {
		if err := s.db.Exec("UPDATE games SET cover_updated_at = NULL WHERE id = ?", game.ID).Error; err != nil {
			return nil, fmt.Errorf("清除游戏封面更新时间失败: %w", err)
		}
	}
	if err := s.replaceTags(game.ID, collectTagIDs(game.Tags)); err != nil {
		return nil, err
	}
	return s.Get(game.ID)
}

// Delete 删除游戏，并清理其关联数据（标签关联、游玩记录、存档记录）。
// 磁盘上的存档文件不会被删除。
func (s *GameService) Delete(id uint) error {
	if id == 0 {
		return errors.New("缺少游戏 ID")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		cleanups := []string{
			"DELETE FROM game_tags WHERE game_id = ?",
			"DELETE FROM game_genres WHERE game_id = ?",
			"DELETE FROM play_sessions WHERE game_id = ?",
			"DELETE FROM save_backups WHERE archive_id IN (SELECT id FROM save_archives WHERE game_id = ?)",
			"DELETE FROM save_archives WHERE game_id = ?",
		}
		for _, sql := range cleanups {
			if err := tx.Exec(sql, id).Error; err != nil {
				return fmt.Errorf("清理游戏关联数据失败: %w", err)
			}
		}

		res := tx.Delete(&models.Game{}, id)
		if res.Error != nil {
			return fmt.Errorf("删除游戏失败: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// replaceTags 用给定标签 ID 覆盖游戏的标签关联，只保留真实存在的标签。
func (s *GameService) replaceTags(gameID uint, tagIDs []uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM game_tags WHERE game_id = ?", gameID).Error; err != nil {
			return fmt.Errorf("清理游戏标签失败: %w", err)
		}
		if len(tagIDs) == 0 {
			return nil
		}

		valid := make([]uint, 0, len(tagIDs))
		if err := tx.Model(&models.Tag{}).Where("id IN ?", tagIDs).Pluck("id", &valid).Error; err != nil {
			return fmt.Errorf("校验标签失败: %w", err)
		}
		for _, tagID := range valid {
			if err := tx.Exec(
				"INSERT OR IGNORE INTO game_tags (game_id, tag_id) VALUES (?, ?)",
				gameID, tagID,
			).Error; err != nil {
				return fmt.Errorf("写入游戏标签关联失败: %w", err)
			}
		}
		return nil
	})
}

// ensureCategoryExists 校验分类是否存在，避免外键约束报出难以理解的错误。
func (s *GameService) ensureCategoryExists(categoryID *uint) error {
	if categoryID == nil {
		return nil
	}
	var count int64
	if err := s.db.Model(&models.Category{}).Where("id = ?", *categoryID).Count(&count).Error; err != nil {
		return fmt.Errorf("校验分类失败: %w", err)
	}
	if count == 0 {
		return errors.New("所选分类不存在，请刷新后重试")
	}
	return nil
}

// collectTagIDs 提取并去重标签 ID（忽略 0）。
func collectTagIDs(tags []models.Tag) []uint {
	ids := make([]uint, 0, len(tags))
	seen := make(map[uint]struct{}, len(tags))
	for _, t := range tags {
		// 忽略未持久化的标签（阶段 1 只支持关联已有标签，不支持内联新建）
		if t.ID == 0 {
			continue
		}
		if _, ok := seen[t.ID]; ok {
			continue
		}
		seen[t.ID] = struct{}{}
		ids = append(ids, t.ID)
	}
	return ids
}

// normalizeAndValidate 规范化输入并校验必填项。
func normalizeAndValidate(game *models.Game, isCreate bool) error {
	game.Name = strings.TrimSpace(game.Name)
	game.ExePath = strings.TrimSpace(game.ExePath)
	game.ProcessName = strings.TrimSpace(game.ProcessName)
	game.InstallDir = strings.TrimSpace(game.InstallDir)
	game.CoverPath = strings.TrimSpace(game.CoverPath)

	if game.Name == "" {
		return errors.New("游戏名称不能为空")
	}
	if len([]rune(game.Name)) > 100 {
		return errors.New("游戏名称不能超过 100 个字符")
	}
	if game.CategoryID != nil && *game.CategoryID == 0 {
		game.CategoryID = nil
	}
	if isCreate {
		game.TotalSeconds = 0
		game.LastPlayedAt = nil
	}
	return nil
}

// roundHours 把秒转换为小时，保留两位小数。
func roundHours(seconds int64) float64 {
	return math.Round(float64(seconds)/36.0) / 100
}
