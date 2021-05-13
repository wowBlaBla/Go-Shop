package models

import (
	"gorm.io/gorm"
)

type Option struct {
	gorm.Model
	Name string
	Title string
	Description string
	Value    *Value `gorm:"foreignKey:value_id;"`
	ValueId  uint
	Values []*Value `gorm:"foreignKey:OptionId"`
	Standard bool
	Sort int
}

func GetOptionsFull(connector *gorm.DB) ([]*Option, error) {
	db := connector
	var options []*Option
	db.Debug().Preload("Values").Order("Sort asc, ID asc").Find(&options)
	if err := db.Error; err != nil {
		return nil, err
	}
	return options, nil
}

func GetOptions(connector *gorm.DB) ([]*Option, error) {
	db := connector
	var options []*Option
	db.Debug().Order("Sort asc, ID asc").Find(&options)
	if err := db.Error; err != nil {
		return nil, err
	}
	return options, nil
}

func GetOptionsByName(connector *gorm.DB, name string) ([]*Option, error) {
	db := connector
	var options []*Option
	if err := db.Debug().Where("name = ?", name).Find(&options).Error; err != nil {
		return nil, err
	}
	return options, nil
}

func GetOptionsByStandard(connector *gorm.DB, standard bool) ([]*Option, error) {
	db := connector
	var options []*Option
	if err := db.Debug().Preload("Value").Where("standard = ?", standard).Find(&options).Error; err != nil {
		return nil, err
	}
	return options, nil
}

func CreateOption(connector *gorm.DB, option *Option) (uint, error) {
	db := connector
	db.Debug().Create(&option)
	if err := db.Error; err != nil {
		return 0, err
	}
	return option.ID, nil
}

func GetOption(connector *gorm.DB, id int) (*Option, error) {
	db := connector
	var option Option
	if err := db.Debug().Preload("Value").Preload("Values").Where("id = ?", id).First(&option).Error; err != nil {
		return nil, err
	}
	return &option, nil
}

func UpdateOption(connector *gorm.DB, option *Option) error {
	db := connector
	db.Debug().Save(&option)
	return db.Error
}


func DeleteOption(connector *gorm.DB, option *Option) error {
	db := connector
	db.Unscoped().Debug().Delete(&option)
	return db.Error
}