package models

import "gorm.io/gorm"

type CachePrice struct {
	gorm.Model
	PriceID   uint
	Thumbnail   string
}

func (CachePrice) TableName() string {
	return "cache_prices"
}

func CreateCachePrice(connector *gorm.DB, price *CachePrice) (uint, error) {
	db := connector
	db.Create(&price)
	if err := db.Error; err != nil {
		return 0, err
	}
	return price.ID, nil
}

func GetCachePriceByPriceId(connector *gorm.DB, priceId uint) (*CachePrice, error){
	db := connector
	var CachePrice CachePrice
	db.Where("price_id = ?", priceId).First(&CachePrice)
	return &CachePrice, db.Error
}

func HasCachePriceByPriceId(connector *gorm.DB, priceId uint) bool {
	db := connector
	var count int64
	db.Model(CachePrice{}).Where("price_id = ?", priceId).Count(&count)
	return count > 0
}

func DeleteCachePriceByPriceId(connector *gorm.DB, priceId uint) error {
	db := connector
	db.Unscoped().Where("price_id = ?", priceId).Delete(&CachePrice{})
	if err := db.Error; err != nil {
		return err
	}
	return nil
}