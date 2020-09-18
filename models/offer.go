package models

import (
	"gorm.io/gorm"
)

type Offer struct {
	gorm.Model
	Name string
	Title string
	Description string
	Thumbnail string
	//Properties []*Property `gorm:"many2many:offer_properties;"`
	Properties []*Property `gorm:"foreignKey:OfferId"`
	Price float64          `sql:"type:decimal(8,2);"`
	//
	ProductId uint
}

func GetOffer(connector *gorm.DB, id int) (*Offer, error) {
	db := connector
	var offer Offer
	db.Debug().Preload("Properties").Preload("Properties.Option").Preload("Properties.Values").Find(&offer, id)
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