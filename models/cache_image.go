package models

import "gorm.io/gorm"

type CacheImage struct {
	gorm.Model
	Path string // ex. /products/furniture/living-room/armchair/sissi-lounge-chair-with-wickerwork
	Origin string // image-1-1605269978.jpg
	Height int // 0
	Width int // 256
}

func (CacheImage) TableName() string {
	return "cache_images"
}

func CreateCacheImage(connector *gorm.DB, image *CacheImage) (uint, error) {
	db := connector
	db.Debug().Create(&image)
	if err := db.Error; err != nil {
		return 0, err
	}
	return image.ID, nil
}