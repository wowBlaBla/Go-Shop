package models

import "gorm.io/gorm"

type Image struct {
	gorm.Model
	Path string
	Url string
	Width int
	Height int
	Size int
}

func CreateImage(connector *gorm.DB, image *Image) (uint, error) {
	db := connector
	db.Debug().Create(&image)
	if err := db.Error; err != nil {
		return 0, err
	}
	return image.ID, nil
}