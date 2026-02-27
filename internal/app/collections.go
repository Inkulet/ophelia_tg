package app

import (
	"fmt"
	"strings"
)

func (wm *WomanManager) CreateCollection(c *Collection) error {
	if c == nil {
		return fmt.Errorf("empty collection")
	}
	c.Name = strings.TrimSpace(c.Name)
	c.Description = strings.TrimSpace(c.Description)
	c.Field = strings.TrimSpace(c.Field)
	c.Tags = normalizeTags(c.Tags)
	if c.Name == "" {
		return fmt.Errorf("empty name")
	}
	return wm.DB.Create(c).Error
}

func (wm *WomanManager) ListCollections(publishedOnly bool) []Collection {
	var cols []Collection
	q := wm.DB.Model(&Collection{})
	if publishedOnly {
		q = q.Where("is_published = ?", true)
	}
	q.Order("id asc").Find(&cols)
	return cols
}

func (wm *WomanManager) GetCollection(id uint) (*Collection, error) {
	var c Collection
	if err := wm.DB.First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (wm *WomanManager) DeleteCollection(id uint) error {
	return wm.DB.Delete(&Collection{}, id).Error
}

func (wm *WomanManager) UpdateCollection(c *Collection) error {
	if c == nil {
		return fmt.Errorf("empty collection")
	}
	c.Name = strings.TrimSpace(c.Name)
	c.Description = strings.TrimSpace(c.Description)
	c.Field = strings.TrimSpace(c.Field)
	c.Tags = normalizeTags(c.Tags)
	return wm.DB.Save(c).Error
}

func collectionToFilters(c *Collection) SearchFilters {
	if c == nil {
		return SearchFilters{Limit: 5, PublishedOnly: true}
	}
	return SearchFilters{
		Field:         c.Field,
		Tags:          c.Tags,
		YearFrom:      c.YearFrom,
		YearTo:        c.YearTo,
		Limit:         5,
		PublishedOnly: true,
	}
}
