package models

import "gorm.io/gorm"

type CacheTag struct {
	gorm.Model
	TagID   uint
	Title       string
	Name       string
	Thumbnail   string
}

func (CacheTag) TableName() string {
	return "cache_tags"
}

func CreateCacheTag(connector *gorm.DB, value *CacheTag) (uint, error) {
	db := connector
	db.Create(&value)
	if err := db.Error; err != nil {
		return 0, err
	}
	return value.ID, nil
}

func GetCacheTagByTagId(connector *gorm.DB, valueId uint) (*CacheTag, error){
	db := connector
	var cacheTag CacheTag
	db.Where("tag_id = ?", valueId).First(&cacheTag)
	return &cacheTag, db.Error
}

func HasCacheTagByTagId(connector *gorm.DB, valueId uint) bool {
	db := connector
	var count int64
	db.Model(CacheTag{}).Where("tag_id = ?", valueId).Count(&count)
	return count > 0
}

func DeleteCacheTagByTagId(connector *gorm.DB, tagId uint) error {
	db := connector
	db.Unscoped().Where("tag_id = ?", tagId).Delete(&CacheTag{})
	if err := db.Error; err != nil {
		return err
	}
	return nil
}