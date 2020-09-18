package models

import "gorm.io/gorm"

type Property struct {
	gorm.Model
	OfferId uint
	Option Option `gorm:"foreignKey:OptionId"`
	OptionId uint
	/*Value Value `gorm:"foreignKey:ValueId"`
	ValueId uint
	Price float64
	*/
	Values []*Value `gorm:"foreignKey:PropertyId"`
}

func CreateProperty(connector *gorm.DB, property *Property) (uint, error) {
	db := connector
	db.Debug().Create(&property)
	if err := db.Error; err != nil {
		return 0, err
	}
	return property.ID, nil
}