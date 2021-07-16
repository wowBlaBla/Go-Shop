package models

import (
	"gorm.io/gorm"
)

type Property struct {
	gorm.Model
	Type string // select / radio
	Size string // small / medium / large
	Mode string // image (default) / color
	Name        string
	Title       string
	ProductId uint
	VariationId uint
	Option      *Option `gorm:"foreignKey:OptionId"`
	OptionId    uint
	Sku string
	Filtering   bool
	Stock uint
	//
	Rates []*Rate `gorm:"foreignKey:PropertyId"`
}

func (p *Property) AfterDelete(tx *gorm.DB) error {
	return tx.Model(&Rate{}).Where("property_id = ?", p.ID).Unscoped().Delete(&Rate{}).Error
}

func GetProperties(connector *gorm.DB) ([]*Property, error) {
	db := connector
	var properties []*Property
	if err := db.Debug().Find(&properties).Error; err != nil {
		return nil, err
	}
	return properties, nil
}

func GetPropertiesByProductId(connector *gorm.DB, productId int) ([]*Property, error) {
	db := connector
	var properties []*Property
	if err := db.Debug().Preload("Option").Preload("Rates").Preload("Rates.Value").Where("product_id = ?", productId).Find(&properties).Error; err != nil {
		return nil, err
	}
	return properties, nil
}

func GetProperty(connector *gorm.DB, id int) (*Property, error) {
	db := connector
	var property Property
	if err := db.Debug().Preload("Option").First(&property, id).Error; err != nil {
		return nil, err
	}
	return &property, nil
}

func GetPropertyFull(connector *gorm.DB, id int) (*Property, error) {
	db := connector
	var property Property
	if err := db.Debug().Preload("Option").Preload("Rates").Preload("Rates.Value").First(&property, id).Error; err != nil {
		return nil, err
	}
	return &property, nil
}

func GetPropertiesByProductAndName(connector *gorm.DB, productId int, name string) ([]*Property, error) {
	db := connector
	var properties []*Property
	if err := db.Where("product_id = ? and name = ?", productId, name).Find(&properties).Error; err != nil {
		return nil, err
	}
	return properties, nil
}

func GetPropertiesByVariationAndName(connector *gorm.DB, variationId int, name string) ([]*Property, error) {
	db := connector
	var properties []*Property
	if err := db.Where("variation_id = ? and name = ?", variationId, name).Find(&properties).Error; err != nil {
		return nil, err
	}
	return properties, nil
}

func CreateProperty(connector *gorm.DB, property *Property) (uint, error) {
	db := connector
	db.Debug().Create(&property)
	if err := db.Error; err != nil {
		return 0, err
	}
	return property.ID, nil
}

func UpdateProperty(connector *gorm.DB, property *Property) error {
	db := connector
	db.Debug().Save(&property)
	return db.Error
}

func DeleteProperty(connector *gorm.DB, property *Property) error {
	db := connector
	db.Debug().Unscoped().Delete(&property)
	return db.Error
}