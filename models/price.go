package models

import "gorm.io/gorm"

type Price struct {
	gorm.Model
	Property *Property `gorm:"foreignKey:PropertyId"`
	PropertyId uint
	Value *Value `gorm:"foreignKey:ValueId"`
	ValueId uint
	//
	Enabled bool
	Price float64
}