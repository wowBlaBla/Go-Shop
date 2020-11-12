package models

import "gorm.io/gorm"

type Tag struct {
	gorm.Model
	//
	Enabled bool
	Hidden bool
	Name string
	Title string
	Description string
	//
	Products  []*Product  `gorm:"many2many:products_tags;"`
}

func GetTags(connector *gorm.DB) ([]*Tag, error) {
	db := connector
	var tags []*Tag
	if err := db.Debug().Find(&tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}

func CreateTag(connector *gorm.DB, tag *Tag) (uint, error) {
	db := connector
	db.Debug().Create(&tag)
	if err := db.Error; err != nil {
		return 0, err
	}
	return tag.ID, nil
}

func GetTag(connector *gorm.DB, id int) (*Tag, error) {
	db := connector
	var tag Tag
	if err := db.Debug().Where("id = ?", id).First(&tag).Error; err != nil {
		return nil, err
	}
	return &tag, nil
}

func UpdateTag(connector *gorm.DB, tag *Tag) error {
	db := connector
	db.Debug().Save(&tag)
	return db.Error
}


func DeleteTag(connector *gorm.DB, tag *Tag) error {
	db := connector
	db.Debug().Delete(&tag)
	return db.Error
}