package models

import "gorm.io/gorm"

type CacheCategory struct {
	gorm.Model
	CategoryID   uint
	Path        string
	Name        string
	Title       string
	Thumbnail   string
	Link string
}

func (CacheCategory) TableName() string {
	return "cache_categories"
}

func CreateCacheCategory(connector *gorm.DB, category *CacheCategory) (uint, error) {
	db := connector
	db.Create(&category)
	if err := db.Error; err != nil {
		return 0, err
	}
	return category.ID, nil
}

func GetCacheCategoryByCategoryId(connector *gorm.DB, categoryId uint) (*CacheCategory, error){
	db := connector
	var cacheCategory CacheCategory
	db.Where("category_id = ?", categoryId).First(&cacheCategory)
	return &cacheCategory, db.Error
}

func GetCacheCategoryByLink(connector *gorm.DB, link string) (*CacheCategory, error){
	db := connector
	var cacheCategory CacheCategory
	if err := db.Debug().Where("link = ?", link).First(&cacheCategory).Error; err != nil {
		return nil, err
	}
	return &cacheCategory, db.Error
}