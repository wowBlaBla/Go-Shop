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
	BasePrice   float64 // min value
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
	db.Where("product_id = ?", productId).First(&cacheProduct)
	return &cacheProduct, db.Error
}