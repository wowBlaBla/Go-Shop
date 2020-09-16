package models

import "gorm.io/gorm"

type Value struct {
	gorm.Model
	Option Option `gorm:"foreignKey:OptionId"`
	OptionId uint
	//
	Title string
	Thumbnail string
	Value string
}

func CreateValue(connector *gorm.DB, value *Value) (uint, error) {
	db := connector
	db.Debug().Create(&value)
	if err := db.Error; err != nil {
		return 0, err
	}
	return value.ID, nil
}