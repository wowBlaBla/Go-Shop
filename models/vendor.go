package models

import "gorm.io/gorm"

type Vendor struct {
	gorm.Model
	Enabled bool
	Name  string
	Title string
	Thumbnail string
	Description string
	Content string
	//
	Times []*Time `gorm:"many2many:vendors_times;"`
}

func GetVendors(connector *gorm.DB) ([]*Vendor, error) {
	db := connector
	var vendors []*Vendor
	if err := db.Debug().Find(&vendors).Error; err != nil {
		return nil, err
	}
	return vendors, nil
}

func GetVendorsByName(connector *gorm.DB, name string) ([]*Vendor, error) {
	db := connector
	var vendors []*Vendor
	if err := db.Debug().Where("name = ?", name).Find(&vendors).Error; err != nil {
		return nil, err
	}
	return vendors, nil
}

func CreateVendor(connector *gorm.DB, vendor *Vendor) (uint, error) {
	db := connector
	db.Debug().Create(&vendor)
	if err := db.Error; err != nil {
		return 0, err
	}
	return vendor.ID, nil
}

func GetVendor(connector *gorm.DB, id int) (*Vendor, error) {
	db := connector
	var vendor Vendor
	if err := db.Debug().Preload("Times").Where("id = ?", id).First(&vendor).Error; err != nil {
		return nil, err
	}
	return &vendor, nil
}

func AddTimeToVendor(connector *gorm.DB, time *Time, vendor *Vendor) error {
	db := connector
	return db.Model(&vendor).Association("Times").Append(time)
}

func DeleteAllTimesFromVendor(connector *gorm.DB, vendor *Vendor) error {
	db := connector
	return db.Debug().Unscoped().Model(&vendor).Association("Times").Clear()
}

func UpdateVendor(connector *gorm.DB, vendor *Vendor) error {
	db := connector
	db.Debug().Save(&vendor)
	return db.Error
}


func DeleteVendor(connector *gorm.DB, vendor *Vendor) error {
	db := connector
	db.Debug().Unscoped().Delete(&vendor)
	return db.Error
}
