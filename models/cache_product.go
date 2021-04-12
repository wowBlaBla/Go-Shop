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
	Variations string
	CategoryID  uint
	Price   float64 // min value
	Width float64
	Height float64
	Depth float64
	Weight float64
}

func (CacheProduct) TableName() string {
	return "cache_products"
}

func CreateCacheProduct(connector *gorm.DB, product *CacheProduct) (uint, error) {
	db := connector
	db.Create(&product)
	if err := db.Error; err != nil {
		return 0, err
	}
	return product.ID, nil
}

func GetCacheProductByProductId(connector *gorm.DB, productId uint) (*CacheProduct, error){
	db := connector
	var cacheProduct CacheProduct
	if err := db.Debug().Where("product_id = ?", productId).First(&cacheProduct).Error; err != nil {
		return nil, err
	}
	return &cacheProduct, db.Error
}