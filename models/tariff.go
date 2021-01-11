package models

import "gorm.io/gorm"

type Tariff struct {
	gorm.Model
	Transport   *Transport `gorm:"foreignKey:TransportId"`
	TransportId uint
	Zone        *Zone `gorm:"foreignKey:ZoneId"`
	ZoneId      uint
	Order    string
	Item     string
	Kg       float64 `sql:"type:decimal(8,2);"`
	M3       float64 `sql:"type:decimal(8,3);"`
}

func GetTariffsByZoneId(connector *gorm.DB, zoneId int) ([]*Tariff, error) {
	db := connector
	var tariffs []*Tariff
	db.Debug().Model(&Tariff{}).Where("zone_id = ?", zoneId).Find(&tariffs)
	if err :=  db.Error; err != nil {
		return nil, err
	}
	return tariffs, nil
}


func CreateTariff(connector *gorm.DB, tariff *Tariff) (uint, error) {
	db := connector
	db.Debug().Create(&tariff)
	if err := db.Error; err != nil {
		return 0, err
	}
	return tariff.ID, nil
}

func GetTariff(connector *gorm.DB, id int) (*Tariff, error) {
	db := connector
	var tariff Tariff
	if err := db.Debug().Where("id = ?", id).First(&tariff).Error; err != nil {
		return nil, err
	}
	return &tariff, nil
}

func GetTariffByTransportIdAndZoneId(connector *gorm.DB, transportId, zoneId uint) (*Tariff, error) {
	db := connector
	var tariff Tariff
	if err := db.Where("transport_id = ? and zone_id = ?", transportId, zoneId).First(&tariff).Error; err != nil {
		return nil, err
	}
	return &tariff, nil
}

func UpdateTariff(connector *gorm.DB, tariff *Tariff) error {
	db := connector
	db.Debug().Save(&tariff)
	return db.Error
}


func DeleteTariff(connector *gorm.DB, tariff *Tariff) error {
	db := connector
	db.Debug().Unscoped().Delete(&tariff)
	return db.Error
}