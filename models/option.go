package models

import "gorm.io/gorm"

type Option struct {
	gorm.Model
	Name string
	Title string
	Description string
	Values []*Value `gorm:"foreignKey:OptionId"`
}

func CreateOption(connector *gorm.DB, option *Option) (uint, error) {
	db := connector
	db.Debug().Create(&option)
	if err := db.Error; err != nil {
		return 0, err
	}
	return option.ID, nil
}