package models

import "gorm.io/gorm"

type Price struct {
	gorm.Model
	Property *Property `gorm:"foreignKey:PropertyId"`
	PropertyId uint
	Value *Value `gorm:"foreignKey:ValueId"`
	ValueId uint
	//
	Enabled bool
	Price float64
	Sku string
}

func GetPricesByProperty(connector *gorm.DB, propertyId uint) ([]*Price, error) {
	db := connector
	var prices []*Price
	if err := db.Debug().Where("property_id = ?", propertyId).Find(&prices).Error; err != nil {
		return nil, err
	}
	return prices, nil
}

func GetPricesByPropertyAndValue(connector *gorm.DB, propertyId, valueId uint) ([]*Price, error) {
	db := connector
	var prices []*Price
	if err := db.Where("property_id = ? and value_id = ?", propertyId, valueId).Find(&prices).Error; err != nil {
		return nil, err
	}
	return prices, nil
}

func GetPrice(connector *gorm.DB, id int) (*Price, error) {
	db := connector
	var price Price
	if err := db.Preload("Property").Preload("Property.Option").Preload("Value").Where("id = ?", id).First(&price).Error; err != nil {
		return nil, err
	}
	return &price, nil
}

func CreatePrice(connector *gorm.DB, price *Price) (uint, error) {
	db := connector
	db.Debug().Create(&price)
	if err := db.Error; err != nil {
		return 0, err
	}
	return price.ID, nil
}

func UpdatePrice(connector *gorm.DB, price *Price) error {
	db := connector
	db.Debug().Save(&price)
	return db.Error
}

func DeletePrice(connector *gorm.DB, price *Price) error {
	db := connector
	db.Debug().Unscoped().Delete(&price)
	return db.Error
}