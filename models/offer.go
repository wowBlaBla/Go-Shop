package models

import (
	"gorm.io/gorm"
)

type Offer struct {
	gorm.Model
	ID        uint `gorm:"primarykey"`
	Name string
	Title string
	Description string
	Thumbnail string
	Properties []*Property `gorm:"foreignKey:OfferId"`
	BasePrice float64          `sql:"type:decimal(8,2);"`
	//
	ProductId uint
}

func GetOffersByProductAndName(connector *gorm.DB, productId uint, name string) ([]*Offer, error) {
	db := connector
	var offers []*Offer
	if err := db.Debug().Where("product_id = ? and name = ?", productId, name).Find(&offers).Error; err != nil {
		return nil, err
	}
	return offers, nil
}

func GetOffer(connector *gorm.DB, id int) (*Offer, error) {
	db := connector
	var offer Offer
	db.Preload("Properties").Preload("Properties.Option").Preload("Properties.Prices").Preload("Properties.Prices.Value").Find(&offer, id)
	if err := db.Error; err != nil {
		return nil, err
	}
	return &offer, nil
}

func CreateOffer(connector *gorm.DB, offer *Offer) (uint, error) {
	db := connector
	db.Debug().Create(&offer)
	if err := db.Error; err != nil {
		return 0, err
	}
	return offer.ID, nil
}

func GetPropertiesFromOffer(connector *gorm.DB, offer *Offer) ([]*Property, error) {
	db := connector
	var properties []*Property
	if err := db.Model(&offer).Preload("Option").Preload("Values").Association("Properties").Find(&properties); err != nil {
		return nil, err
	}
	return properties, nil
}

func UpdateOffer(connector *gorm.DB, offer *Offer) error {
	db := connector
	db.Debug().Save(&offer)
	return db.Error
}

func DeleteOffer(connector *gorm.DB, offer *Offer) error {
	db := connector
	db.Debug().Unscoped().Delete(&offer)
	return db.Error
}