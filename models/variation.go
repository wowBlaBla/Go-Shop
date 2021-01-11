package models

import (
	"gorm.io/gorm"
)

type Variation struct {
	gorm.Model
	ID        uint `gorm:"primarykey"`
	Name string
	Title string
	Description string
	Thumbnail string
	Properties []*Property `gorm:"foreignKey:VariationId"`
	BasePrice float64          `sql:"type:decimal(8,2);"`
	Dimensions string // width x height x depth in cm
	Weight float64 `sql:"type:decimal(8,2);"`
	Availability string
	Sending string
	Sku string
	//
	ProductId uint
}

func GetVariationsByProductAndName(connector *gorm.DB, productId uint, name string) ([]*Variation, error) {
	db := connector
	var variations []*Variation
	if err := db.Debug().Where("product_id = ? and name = ?", productId, name).Find(&variations).Error; err != nil {
		return nil, err
	}
	return variations, nil
}

func GetVariation(connector *gorm.DB, id int) (*Variation, error) {
	db := connector
	var variation Variation
	db.Preload("Properties").Preload("Properties.Option").Preload("Properties.Prices").Preload("Properties.Prices.Value").Find(&variation, id)
	if err := db.Error; err != nil {
		return nil, err
	}
	return &variation, nil
}

func CreateVariation(connector *gorm.DB, variation *Variation) (uint, error) {
	db := connector
	db.Debug().Create(&variation)
	if err := db.Error; err != nil {
		return 0, err
	}
	return variation.ID, nil
}

func UpdateVariation(connector *gorm.DB, variation *Variation) error {
	db := connector
	db.Debug().Save(&variation)
	return db.Error
}

func DeleteVariation(connector *gorm.DB, variation *Variation) error {
	db := connector
	db.Debug().Unscoped().Delete(&variation)
	return db.Error
}