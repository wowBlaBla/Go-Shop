package models

import "gorm.io/gorm"

type Parameter struct {
	gorm.Model
	Name        string
	Title       string
	Type string
	// Select from existing option
	Option *Option `gorm:"foreignKey:OptionId"`
	OptionId      uint
	Value *Value `gorm:"foreignKey:ValueId"`
	ValueId   uint
	// or type custom value
	CustomValue   string
	//
	Filtering bool
	//
	ProductId uint
}

func GetParametersByProductId(connector *gorm.DB, productId int) ([]*Parameter, error) {
	db := connector
	var parameters []*Parameter
	if err := db.Debug().Where("product_id = ?", productId).Find(&parameters).Error; err != nil {
		return nil, err
	}
	return parameters, nil
}

func GetParametersByProductAndName(connector *gorm.DB, productId int, name string) ([]*Parameter, error) {
	db := connector
	var parameters []*Parameter
	if err := db.Where("product_id = ? and name = ?", productId, name).Find(&parameters).Error; err != nil {
		return nil, err
	}
	return parameters, nil
}

func GetParameter(connector *gorm.DB, id int) (*Parameter, error) {
	db := connector
	var parameter Parameter
	if err := db.Debug().Preload("Option").Preload("Value").First(&parameter, id).Error; err != nil {
		return nil, err
	}
	return &parameter, nil
}

func CreateParameter(connector *gorm.DB, parameter *Parameter) (uint, error) {
	db := connector
	db.Debug().Create(&parameter)
	if err := db.Error; err != nil {
		return 0, err
	}
	return parameter.ID, nil
}

func UpdateParameter(connector *gorm.DB, parameter *Parameter) error {
	db := connector
	db.Debug().Save(&parameter)
	return db.Error
}

func DeleteParameter(connector *gorm.DB, parameter *Parameter) error {
	db := connector
	db.Debug().Unscoped().Delete(&parameter)
	return db.Error
}