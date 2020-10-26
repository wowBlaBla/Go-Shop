package models

import "gorm.io/gorm"

type Item struct {
	gorm.Model
	//
	Uuid        string
	Title       string
	Description string
	Path string
	Thumbnail  string
	Thumbnails  string
	Price       float64          `sql:"type:decimal(8,2);"`
	Quantity    int
	Total       float64          `sql:"type:decimal(8,2);"`
	OrderId     uint
}

func DeleteItem(connector *gorm.DB, item *Item) error {
	db := connector
	db.Debug().Unscoped().Delete(&item)
	return db.Error
}