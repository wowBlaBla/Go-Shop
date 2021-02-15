package models

import "gorm.io/gorm"

type Item struct {
	gorm.Model
	//
	Uuid        string
	CategoryId  uint
	Title       string
	Description string `json:",omitempty"`
	Path string
	Thumbnail  string
	Price       float64          `sql:"type:decimal(8,2);"`
	Discount    float64          `sql:"type:decimal(8,2);"`
	Discount2   float64          `sql:"type:decimal(8,2);"`
	Quantity    int
	Delivery    float64          `sql:"type:decimal(8,2);"`
	Total       float64          `sql:"type:decimal(8,2);"`
	OrderId     uint
	Volume float64 `sql:"type:decimal(8,3);"`
	Weight float64 `sql:"type:decimal(8,3);"`
}

func DeleteItem(connector *gorm.DB, item *Item) error {
	db := connector
	db.Debug().Unscoped().Delete(&item)
	return db.Error
}