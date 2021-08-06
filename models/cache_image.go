package models

import (
	"gorm.io/gorm"
)

type CacheImage struct {
	gorm.Model
	ImageId uint
	Name string
	Thumbnail string
	//Path string // ex. /products/furniture/living-room/armchair/sissi-lounge-chair-with-wickerwork
	//Origin string // image-1-1605269978.jpg
	//Height int // 0
	//Width int // 256
}

func (CacheImage) TableName() string {
	return "cache_images"
}

func HasCacheImageByImageId(connector *gorm.DB, imageId uint) bool {
	db := connector
	var count int64
	db.Model(&CacheImage{}).Where("image_id = ?", imageId).Count(&count)
	return count > 0
}

func GetCacheImageByImageId(connector *gorm.DB, imageId uint) (*CacheImage, error){
	db := connector
	var cacheImage CacheImage
	db.Where("image_id = ?", imageId).First(&cacheImage)
	return &cacheImage, db.Error
}

func CreateCacheImage(connector *gorm.DB, image *CacheImage) (uint, error) {
	db := connector
	db.Create(&image)
	if err := db.Error; err != nil {
		return 0, err
	}
	return image.ID, nil
}

func DeleteCacheImageByImageId(connector *gorm.DB, imageId uint) error {
	db := connector
	db.Unscoped().Where("image_id = ?", imageId).Delete(&CacheImage{})
	if err := db.Error; err != nil {
		return err
	}
	return nil
}