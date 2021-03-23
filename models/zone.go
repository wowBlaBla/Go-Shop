package models

import "gorm.io/gorm"

type Zone struct {
	gorm.Model
	Enabled bool
	Title string
	Country string
	ZIP string `gorm:"column:zip"`
	Description string
}

func GetZones(connector *gorm.DB) ([]*Zone, error) {
	db := connector
	var zones []*Zone
	if err := db.Debug().Find(&zones).Error; err != nil {
		return nil, err
	}
	return zones, nil
}

func GetZonesByCountryAndZIP(connector *gorm.DB, country, zip string) ([]*Zone, error) {
	db := connector
	var zones []*Zone
	if err := db.Debug().Where("country = ? and zip = ?", country, zip).Find(&zones).Error; err != nil {
		return nil, err
	}
	return zones, nil
}

func CreateZone(connector *gorm.DB, zone *Zone) (uint, error) {
	db := connector
	db.Debug().Create(&zone)
	if err := db.Error; err != nil {
		return 0, err
	}
	return zone.ID, nil
}

func GetZone(connector *gorm.DB, id int) (*Zone, error) {
	db := connector
	var zone Zone
	if err := db.Debug().Where("id = ?", id).First(&zone).Error; err != nil {
		return nil, err
	}
	return &zone, nil
}

func GetZoneByCountry(connector *gorm.DB, country string) (*Zone, error) {
	db := connector
	var zone Zone
	if err := db.Where("country = ? and (zip = '' or zip is null)", country).First(&zone).Error; err != nil {
		return nil, err
	}
	return &zone, nil
}

func GetZoneByCountryAndZIP(connector *gorm.DB, country, zip string) (*Zone, error) {
	db := connector
	var zone Zone
	if zip == "" {
		if err := db.Where("country = ? and (zip = '' or zip is null)", country).First(&zone).Error; err != nil {
			return nil, err
		}
	}else{
		if err := db.Where("country = ? and zip = ?", country, zip).First(&zone).Error; err != nil {
			return nil, err
		}
	}
	return &zone, nil
}

func UpdateZone(connector *gorm.DB, zone *Zone) error {
	db := connector
	db.Debug().Save(&zone)
	return db.Error
}


func DeleteZone(connector *gorm.DB, zone *Zone) error {
	db := connector
	db.Debug().Unscoped().Delete(&zone)
	return db.Error
}