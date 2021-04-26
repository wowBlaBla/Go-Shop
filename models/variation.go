package models

import (
	"encoding/json"
	"gorm.io/gorm"
	"sort"
	"time"
)

type Variation struct {
	gorm.Model
	ID        uint `gorm:"primarykey"`
	Name string
	Title string
	Description string
	Notes string
	Thumbnail string
	Properties []*Property `gorm:"foreignKey:VariationId"`
	BasePrice float64      `sql:"type:decimal(8,2);"`
	SalePrice float64      `sql:"type:decimal(8,2);"`
	Start time.Time
	End time.Time
	Prices []*Price `gorm:"foreignKey:VariationId"`
	//
	Pattern string
	Dimensions string
	Width float64 `sql:"type:decimal(8,2);"`
	Height float64 `sql:"type:decimal(8,2);"`
	Depth float64 `sql:"type:decimal(8,2);"`
	Weight float64 `sql:"type:decimal(8,2);"`
	Availability string
	//Sending string
	Sku string
	Images      []*Image   `gorm:"many2many:variations_images;"`
	Files       []*File    `gorm:"many2many:variations_files;"`
	Customization string
	//
	TimeId uint
	Time       *Time `gorm:"foreignKey:time_id;"`
	//
	ProductId uint
}

func GetVariations(connector *gorm.DB) ([]*Variation, error) {
	db := connector
	var variations []*Variation
	db.Debug().Find(&variations)
	if err := db.Error; err != nil {
		return nil, err
	}
	return variations, nil
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
	if err := db.Preload("Properties").Preload("Properties.Option").Preload("Properties.Rates").Preload("Properties.Rates.Value").Preload("Prices").Preload("Prices.Rates").Preload("Prices.Rates.Property").Preload("Prices.Rates.Value").Preload("Images").Preload("Files").First(&variation, id).Error; err != nil {
		return nil, err
	}
	// Customization
	var customization struct {
		Images struct {
			Order []uint
		}
	}
	if err := json.Unmarshal([]byte(variation.Customization), &customization); err == nil {
		images := variation.Images
		sort.SliceStable(images, func(i, j int) bool {
			var x, y = -1, -1
			for k, id := range customization.Images.Order {
				if id == images[i].ID {
					x = k
				}
				if id == images[j].ID {
					y = k
				}
			}
			if x == -1 || y == -1 {
				return images[i].ID < images[j].ID
			}else{
				return x < y
			}
		})
		variation.Images = images
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

func AddFileToVariation(connector *gorm.DB, variation *Variation, file *File) error {
	db := connector
	return db.Model(&variation).Association("Files").Append(file)
}

func AddImageToVariation(connector *gorm.DB, variation *Variation, image *Image) error {
	db := connector
	return db.Model(&variation).Association("Images").Append(image)
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