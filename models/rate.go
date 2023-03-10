package models

import (
	"github.com/yonnic/goshop/common"
	"gorm.io/gorm"
)

type Rate struct {
	gorm.Model // ID is here
	Property *Property `gorm:"foreignKey:PropertyId"`
	PropertyId uint `gorm:"index:property_id"`
	Value *Value `gorm:"foreignKey:ValueId"`
	ValueId uint `gorm:"index:value_id"`
	//
	Enabled bool
	Price float64
	Prices []*Price `gorm:"many2many:prices_rates;"`
	Availability string
	Sending string
	Sku string
	Stock uint
}

/*func (r *Rate) AfterDelete(tx *gorm.DB) error {
	return tx.Debug().Exec("delete from prices_rates where rate_id = ?", r.ID).Error
}*/

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
	if err := db.Preload("Property").Preload("Property.Option").Preload("Prices").Preload("Value").Where("id = ?", id).First(&rate).Error; err != nil {
		return nil, err
	}
	return &rate, nil
}

func CreateRate(connector *gorm.DB, rate *Rate) (uint, error) {
	db := connector
	db.Create(&rate)
	if err := db.Error; err != nil {
		return 0, err
	}
	return rate.ID, nil
}

func UpdateRate(connector *gorm.DB, rate *Rate) error {
	db := connector
	db.Save(&rate)
	return db.Error
}

func DeleteRate(connector *gorm.DB, rate *Rate) error {
	db := connector
	db.Model(&rate).Association("Prices").Clear()
	if prices, err := GetPricesOfRate(common.Database, rate); err == nil {
		for _, price := range prices {
			if err = DeletePrice(common.Database, price); err != nil {
				return err
			}
		}
	}
	if rate.Value != nil && rate.Value.OptionId == 0 {
		if err := DeleteValue(common.Database, rate.Value); err != nil {
			return err
		}
	}
	db.Unscoped().Delete(&rate)
	return db.Error
}