package models

import (
	"gorm.io/gorm"
)

type CacheVariation struct {
	gorm.Model
	VariationID   uint
	Name        string
	Title       string
	Description string
	Thumbnail   string
	BasePrice   float64 // min value
}

func (CacheVariation) TableName() string {
	return "cache_variation"
}

func CreateCacheVariation(connector *gorm.DB, variation *CacheVariation) (uint, error) {
	db := connector
	db.Create(&variation)
	if err := db.Error; err != nil {
		return 0, err
	}
	return variation.ID, nil
}

func GetCacheVariationByVariationId(connector *gorm.DB, variationId uint) (*CacheVariation, error){
	db := connector
	var cacheVariation CacheVariation
	db.Where("variation_id = ?", variationId).First(&cacheVariation)
	return &cacheVariation, db.Error
}