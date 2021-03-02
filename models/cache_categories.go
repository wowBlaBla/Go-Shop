package models

import "gorm.io/gorm"

type CacheCategory struct {
	gorm.Model
	CategoryID   uint
	Path        string
	Name        string
	Title       string
	Thumbnail   string
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

func GetCacheCategoryByProductId(connector *gorm.DB, categoryId uint) (*CacheCategory, error){
	db := connector
	var cacheCategory CacheCategory
	db.Where("category_id = ?", categoryId).First(&cacheCategory)
	return &cacheCategory, db.Error
}