package models

import (
	"gorm.io/gorm"
)

type Price struct {
	gorm.Model // ID is here
	//
	Product *Product `gorm:"foreignKey:ProductId"`
	ProductId uint `gorm:"index:product_id"`
	Variation *Variation `gorm:"foreignKey:VariationId"`
	VariationId uint `gorm:"index:variation_id"`
	//
	//RateIds string
	Rates      []*Rate `gorm:"many2many:prices_rates;"`
	//
	Enabled bool
	Thumbnail string `gorm:"many2many:prices_rates;"`
	BasePrice float64
	SalePrice float64
	Availability string
	Sending string
	Sku string
	Stock uint
}

/*func (p *Price) AfterDelete(tx *gorm.DB) error {
	return tx.Debug().Exec("delete from prices_rates where price_id = ?", p.ID).Error
}*/

func GetPrices(connector *gorm.DB) ([]*Price, error) {
	db := connector
	var prices []*Price
	db.Debug().Find(&prices)
	if err := db.Error; err != nil {
		return nil, err
	}
	return prices, nil
}

func GetPricesByProductId(connector *gorm.DB, productId uint) ([]*Price, error) {
	db := connector
	var prices []*Price
	if err := db.Debug().Preload("Rates").Preload("Rates.Property").Preload("Rates.Value").Where("product_id = ?", productId).Find(&prices).Error; err != nil {
		return nil, err
	}
	return prices, nil
}

func GetPricesByVariationId(connector *gorm.DB, variationId uint) ([]*Price, error) {
	db := connector
	var prices []*Price
	if err := db.Debug().Preload("Rates").Preload("Rates.Property").Preload("Rates.Value").Where("variation_id = ?", variationId).Find(&prices).Error; err != nil {
		return nil, err
	}
	return prices, nil
}

func GetPricesOfRate(connector *gorm.DB, rate *Rate) ([]*Price, error) {
	db := connector
	var prices []*Price
	if err := db.Model(&rate).Association("Prices").Find(&prices); err != nil {
		return nil, err
	}
	return prices, nil
}

func GetPrice(connector *gorm.DB, id int) (*Price, error) {
	db := connector
	var price Price
	if err := db.Preload("Rates").Preload("Rates.Property").Preload("Rates.Value").Where("id = ?", id).First(&price).Error; err != nil {
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