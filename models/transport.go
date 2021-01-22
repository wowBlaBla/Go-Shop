package models

import "gorm.io/gorm"

type Transport struct {
	gorm.Model
	Enabled bool
	Name  string
	Title string
	Thumbnail string
	Weight float64 `sql:"type:decimal(8,2);"`
	Volume float64 `sql:"type:decimal(8,3);"`
	Order string
	Item string
	Kg float64 `sql:"type:decimal(8,2);"`
	M3 float64 `sql:"type:decimal(8,3);"`
}

func GetTransports(connector *gorm.DB) ([]*Transport, error) {
	db := connector
	var transports []*Transport
	if err := db.Debug().Find(&transports).Error; err != nil {
		return nil, err
	}
	return transports, nil
}

func GetTransportsByName(connector *gorm.DB, name string) ([]*Transport, error) {
	db := connector
	var transports []*Transport
	if err := db.Debug().Where("name = ?", name).Find(&transports).Error; err != nil {
		return nil, err
	}
	return transports, nil
}

func CreateTransport(connector *gorm.DB, transport *Transport) (uint, error) {
	db := connector
	db.Debug().Create(&transport)
	if err := db.Error; err != nil {
		return 0, err
	}
	return transport.ID, nil
}

func GetTransport(connector *gorm.DB, id int) (*Transport, error) {
	db := connector
	var transport Transport
	if err := db.Debug().Where("id = ?", id).First(&transport).Error; err != nil {
		return nil, err
	}
	return &transport, nil
}

func UpdateTransport(connector *gorm.DB, transport *Transport) error {
	db := connector
	db.Debug().Save(&transport)
	return db.Error
}


func DeleteTransport(connector *gorm.DB, transport *Transport) error {
	db := connector
	db.Debug().Unscoped().Delete(&transport)
	return db.Error
}