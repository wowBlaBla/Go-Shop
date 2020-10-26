package models

import "gorm.io/gorm"

type Image struct {
	gorm.Model
	Name string
	Path string
	Url string
	Width int
	Height int
	Size int64
}

func CreateImage(connector *gorm.DB, image *Image) (uint, error) {
	db := connector
	db.Debug().Create(&image)
	if err := db.Error; err != nil {
		return 0, err
	}
	return image.ID, nil
}

func GetImage(connector *gorm.DB, id int) (*Image, error) {
	db := connector
	var image Image
	if err := db.Debug().Where("id = ?", id).First(&image).Error; err != nil {
		return nil, err
	}
	return &image, nil
}

func UpdateImage(connector *gorm.DB, image *Image) error {
	db := connector
	db.Debug().Save(&image)
	return db.Error
}

func DeleteImage(connector *gorm.DB, image *Image) error {
	db := connector
	db.Debug().Unscoped().Delete(&image)
	return db.Error
}