package services

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"GameArchive/internal/models"
)

// defaultColor 未指定颜色时使用的默认色。
const defaultColor = "#6366f1"

// CategoryService 负责游戏分类的增删改查。
type CategoryService struct{ db *gorm.DB }

// List 返回全部分类，按排序值、ID 升序。
func (s *CategoryService) List() ([]models.Category, error) {
	items := make([]models.Category, 0)
	if err := s.db.Order("sort_order ASC, id ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("查询分类失败: %w", err)
	}
	return items, nil
}

// Add 新增分类。
func (s *CategoryService) Add(category models.Category) (*models.Category, error) {
	if err := s.normalize(&category); err != nil {
		return nil, err
	}
	if err := s.ensureNameUnique(category.Name, 0); err != nil {
		return nil, err
	}

	category.ID = 0
	if err := s.db.Create(&category).Error; err != nil {
		return nil, fmt.Errorf("新增分类失败: %w", err)
	}
	return &category, nil
}

// Update 更新分类。
func (s *CategoryService) Update(category models.Category) (*models.Category, error) {
	if category.ID == 0 {
		return nil, errors.New("缺少分类 ID")
	}
	if err := s.normalize(&category); err != nil {
		return nil, err
	}
	if _, err := s.get(category.ID); err != nil {
		return nil, err
	}
	if err := s.ensureNameUnique(category.Name, category.ID); err != nil {
		return nil, err
	}

	fields := map[string]any{
		"name":       category.Name,
		"color":      category.Color,
		"sort_order": category.SortOrder,
	}
	if err := s.db.Model(&models.Category{}).Where("id = ?", category.ID).Updates(fields).Error; err != nil {
		return nil, fmt.Errorf("更新分类失败: %w", err)
	}
	return s.get(category.ID)
}

// Delete 删除分类，并把原本属于该分类的游戏置为「未分类」。
func (s *CategoryService) Delete(id uint) error {
	if id == 0 {
		return errors.New("缺少分类 ID")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.Category{}).Where("id = ?", id).Count(&count).Error; err != nil {
			return fmt.Errorf("查询分类失败: %w", err)
		}
		if count == 0 {
			return ErrNotFound
		}

		// 先解除游戏与分类的关联，避免留下悬空引用
		if err := tx.Model(&models.Game{}).Where("category_id = ?", id).
			Update("category_id", nil).Error; err != nil {
			return fmt.Errorf("解除游戏分类关联失败: %w", err)
		}
		if err := tx.Delete(&models.Category{}, id).Error; err != nil {
			return fmt.Errorf("删除分类失败: %w", err)
		}
		return nil
	})
}

// get 按 ID 查询分类。
func (s *CategoryService) get(id uint) (*models.Category, error) {
	var category models.Category
	err := s.db.First(&category, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("查询分类失败: %w", err)
	}
	return &category, nil
}

// normalize 规范化输入并校验必填项。
func (s *CategoryService) normalize(category *models.Category) error {
	category.Name = strings.TrimSpace(category.Name)
	category.Color = strings.TrimSpace(category.Color)

	if category.Name == "" {
		return errors.New("分类名称不能为空")
	}
	if len([]rune(category.Name)) > 50 {
		return errors.New("分类名称不能超过 50 个字符")
	}
	if category.Color == "" {
		category.Color = defaultColor
	}
	if category.SortOrder < 0 {
		category.SortOrder = 0
	}
	return nil
}

// ensureNameUnique 校验分类名称唯一（excludeID > 0 时忽略自身）。
func (s *CategoryService) ensureNameUnique(name string, excludeID uint) error {
	q := s.db.Model(&models.Category{}).Where("name = ?", name)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}

	var count int64
	if err := q.Count(&count).Error; err != nil {
		return fmt.Errorf("校验分类名称失败: %w", err)
	}
	if count > 0 {
		return errors.New("已存在同名分类")
	}
	return nil
}
