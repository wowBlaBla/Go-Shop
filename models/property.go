package models

import (
	"gorm.io/gorm"
)

type Property struct {
	gorm.Model
	Name string
	Title string
	OfferId uint
	Option *Option `gorm:"foreignKey:OptionId"`
	OptionId uint
	//
	Prices []*Price `gorm:"foreignKey:PropertyId"`
}

func (p *Property) AfterDelete(tx *gorm.DB) error {
	return tx.Model(&Price{}).Where("property_id = ?", p.ID).Unscoped().Delete(&Price{}).Error
}

func GetProperty(connector *gorm.DB, id int) (*Property, error) {
	db := connector
	var property Property
	if err := db.Debug().Preload("Option").First(&property, id).Error; err != nil {
		return nil, err
	}
	return &property, nil
}

func GetPropertiesByOfferAndName(connector *gorm.DB, offerId int, name string) ([]*Property, error) {
	db := connector
	var properties []*Property
	if err := db.Where("offer_id = ? and name = ?", offerId, name).Find(&properties).Error; err != nil {
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