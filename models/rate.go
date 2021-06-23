package models

import "gorm.io/gorm"

type Rate struct {
	gorm.Model // ID is here
	Property *Property `gorm:"foreignKey:PropertyId"`
	PropertyId uint
	Value *Value `gorm:"foreignKey:ValueId"`
	ValueId uint
	//
	Enabled bool
	Price float64
	Availability string
	Sending string
	Sku string
	Stock uint
}

func GetRatesByProperty(connector *gorm.DB, propertyId uint) ([]*Rate, error) {
	db := connector
	var rates []*Rate
	if err := db.Debug().Preload("Property").Preload("Property.Option").Preload("Value").Where("property_id = ?", propertyId).Find(&rates).Error; err != nil {
		return nil, err
	}
	return rates, nil
}

func GetRatesByPropertyAndValue(connector *gorm.DB, propertyId, valueId uint) ([]*Rate, error) {
	db := connector
	var rates []*Rate
	if err := db.Where("property_id = ? and value_id = ?", propertyId, valueId).Find(&rates).Error; err != nil {
		return nil, err
	}
	return rates, nil
}

func GetRate(connector *gorm.DB, id int) (*Rate, error) {
	db := connector
	var rate Rate
	if err := db.Preload("Property").Preload("Property.Option").Preload("Value").Where("id = ?", id).First(&rate).Error; err != nil {
		return nil, err
	}
	return &rate, nil
}

func CreateRate(connector *gorm.DB, rate *Rate) (uint, error) {
	db := connector
	db.Debug().Create(&rate)
	if err := db.Error; err != nil {
		return 0, err
	}
	return rate.ID, nil
}

func UpdateRate(connector *gorm.DB, rate *Rate) error {
	db := connector
	db.Debug().Save(&rate)
	return db.Error
}

func DeleteRate(connector *gorm.DB, rate *Rate) error {
	db := connector
	db.Debug().Unscoped().Delete(&rate)
	return db.Error
}