package models

import "gorm.io/gorm"

type CacheValue struct {
	gorm.Model
	ValueID   uint
	Title       string
	Thumbnail   string
	Value        string
}

func (CacheValue) TableName() string {
	return "cache_values"
}

func CreateCacheValue(connector *gorm.DB, value *CacheValue) (uint, error) {
	db := connector
	db.Create(&value)
	if err := db.Error; err != nil {
		return 0, err
	}
	return value.ID, nil
}

func GetCacheValueByValueId(connector *gorm.DB, valueId uint) (*CacheValue, error){
	db := connector
	var cacheValue CacheValue
	db.Where("value_id = ?", valueId).First(&cacheValue)
	return &cacheValue, db.Error
}

func HasCacheValueByValueId(connector *gorm.DB, valueId uint) bool {
	db := connector
	var count int64
	db.Model(CacheValue{}).Where("value_id = ?", valueId).Count(&count)
	return count > 0
}

func DeleteCacheValueByValueId(connector *gorm.DB, valueId uint) error {
	db := connector
	db.Unscoped().Where("value_id = ?", valueId).Delete(&CacheValue{})
	if err := db.Error; err != nil {
		return err
	}
	return nil
}