package models

import "gorm.io/gorm"

type CacheProduct struct {
	gorm.Model
	ProductID   uint
	Path        string
	Name        string
	Title       string
	Description string
	Thumbnail   string
	Images string
	CategoryID  uint
	BasePrice   float64 // min value
}

func (CacheProduct) TableName() string {
	return "cache_products"
}

func CreateCacheProduct(connector *gorm.DB, product *CacheProduct) (uint, error) {
	db := connector
	db.Debug().Create(&product)
	if err := db.Error; err != nil {
		return 0, err
	}
	return product.ID, nil
}