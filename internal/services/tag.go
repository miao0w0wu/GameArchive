package services

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"GameArchive/internal/models"
)

// TagService 负责标签的增删改查。
type TagService struct{ db *gorm.DB }

// List 返回全部标签，按名称升序。
func (s *TagService) List() ([]models.Tag, error) {
	items := make([]models.Tag, 0)
	if err := s.db.Order("name ASC, id ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("查询标签失败: %w", err)
	}
	return items, nil
}

// Add 新增标签。
func (s *TagService) Add(tag models.Tag) (*models.Tag, error) {
	if err := normalizeTag(&tag); err != nil {
		return nil, err
	}
	if err := s.ensureNameUnique(tag.Name, 0); err != nil {
		return nil, err
	}

	tag.ID = 0
	if err := s.db.Create(&tag).Error; err != nil {
		return nil, fmt.Errorf("新增标签失败: %w", err)
	}
	return &tag, nil
}

// Update 更新标签。
func (s *TagService) Update(tag models.Tag) (*models.Tag, error) {
	if tag.ID == 0 {
		return nil, errors.New("缺少标签 ID")
	}
	if err := normalizeTag(&tag); err != nil {
		return nil, err
	}
	if _, err := s.get(tag.ID); err != nil {
		return nil, err
	}
	if err := s.ensureNameUnique(tag.Name, tag.ID); err != nil {
		return nil, err
	}

	fields := map[string]any{"name": tag.Name, "color": tag.Color}
	if err := s.db.Model(&models.Tag{}).Where("id = ?", tag.ID).Updates(fields).Error; err != nil {
		return nil, fmt.Errorf("更新标签失败: %w", err)
	}
	return s.get(tag.ID)
}

// Delete 删除标签，并清理游戏与标签的关联。
func (s *TagService) Delete(id uint) error {
	if id == 0 {
		return errors.New("缺少标签 ID")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.Tag{}).Where("id = ?", id).Count(&count).Error; err != nil {
			return fmt.Errorf("查询标签失败: %w", err)
		}
		if count == 0 {
			return ErrNotFound
		}

		if err := tx.Exec("DELETE FROM game_tags WHERE tag_id = ?", id).Error; err != nil {
			return fmt.Errorf("清理游戏标签关联失败: %w", err)
		}
		if err := tx.Delete(&models.Tag{}, id).Error; err != nil {
			return fmt.Errorf("删除标签失败: %w", err)
		}
		return nil
	})
}

// get 按 ID 查询标签。
func (s *TagService) get(id uint) (*models.Tag, error) {
	var tag models.Tag
	err := s.db.First(&tag, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询标签失败: %w", err)
	}
	return &tag, nil
}

// normalizeTag 规范化输入并校验必填项。
func normalizeTag(tag *models.Tag) error {
	tag.Name = strings.TrimSpace(tag.Name)
	tag.Color = strings.TrimSpace(tag.Color)

	if tag.Name == "" {
		return errors.New("标签名称不能为空")
	}
	if len([]rune(tag.Name)) > 50 {
		return errors.New("标签名称不能超过 50 个字符")
	}
	if tag.Color == "" {
		tag.Color = defaultColor
	}
	return nil
}

// ensureNameUnique 校验标签名称唯一（excludeID > 0 时忽略自身）。
func (s *TagService) ensureNameUnique(name string, excludeID uint) error {
	q := s.db.Model(&models.Tag{}).Where("name = ?", name)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}

	var count int64
	if err := q.Count(&count).Error; err != nil {
		return fmt.Errorf("校验标签名称失败: %w", err)
	}
	if count > 0 {
		return errors.New("已存在同名标签")
	}
	return nil
}
