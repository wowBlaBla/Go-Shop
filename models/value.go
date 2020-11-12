package models

import "gorm.io/gorm"

type Value struct {
	gorm.Model
	OptionId uint
	//
	Title string
	Thumbnail string
	Value string
}

func GetValues(connector *gorm.DB) ([]*Value, error) {
	db := connector
	var value []*Value
	if err := db.Debug().Find(&value).Error; err != nil {
		return nil, err
	}
	return value, nil
}

func GetValuesByOptionId(connector *gorm.DB, id int) ([]*Value, error) {
	db := connector
	var value []*Value
	if err := db.Where("option_id = ?", id).Find(&value).Error; err != nil {
		return nil, err
	}
	return value, nil
}

func GetValue(connector *gorm.DB, id int) (*Value, error) {
	db := connector
	var value Value
	if err := db.Where("id = ?", id).First(&value).Error; err != nil {
		return nil, err
	}
	return &value, nil
}

func GetValueByOptionIdAndTitle(connector *gorm.DB, id int, title string) (*Value, error) {
	db := connector
	var value Value
	if err := db.Where("option_id = ? and title = ?", id).First(&title).Error; err != nil {
		return nil, err
	}
	return &value, nil
}

func GetValueByOptionIdAndValue(connector *gorm.DB, id int, val string) (*Value, error) {
	db := connector
	var value Value
	if err := db.Where("option_id = ? and value = ?", id).First(&val).Error; err != nil {
		return nil, err
	}
	return &value, nil
}

func CreateValue(connector *gorm.DB, value *Value) (uint, error) {
	db := connector
	db.Debug().Create(&value)
	if err := db.Error; err != nil {
		return 0, err
	}
	return value.ID, nil
}

func UpdateValue(connector *gorm.DB, value *Value) error {
	db := connector
	db.Debug().Save(&value)
	return db.Error
}


func DeleteValue(connector *gorm.DB, value *Value) error {
	db := connector
	db.Debug().Unscoped().Delete(&value)
	return db.Error
}