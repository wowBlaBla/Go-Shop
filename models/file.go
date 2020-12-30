package models

import "gorm.io/gorm"

type File struct {
	gorm.Model
	Type string
	Name string
	Path string
	Url string
	Size int64
}

func GetFiles(connector *gorm.DB) ([]*File, error) {
	db := connector
	var files []*File
	db.Debug().Find(&files)
	if err := db.Error; err != nil {
		return nil, err
	}
	return files, nil
}

func CreateFile(connector *gorm.DB, file *File) (uint, error) {
	db := connector
	db.Debug().Create(&file)
	if err := db.Error; err != nil {
		return 0, err
	}
	return file.ID, nil
}

func GetFile(connector *gorm.DB, id int) (*File, error) {
	db := connector
	var file File
	if err := db.Debug().Where("id = ?", id).First(&file).Error; err != nil {
		return nil, err
	}
	return &file, nil
}

func UpdateFile(connector *gorm.DB, file *File) error {
	db := connector
	db.Debug().Save(&file)
	return db.Error
}

func DeleteFile(connector *gorm.DB, file *File) error {
	db := connector
	db.Debug().Unscoped().Delete(&file)
	return db.Error
}