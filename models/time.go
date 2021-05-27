package models

import "gorm.io/gorm"

type Time struct {
	gorm.Model
	Enabled bool
	Name string `gorm:"size:255;index:idx_time_name,unique"`
	Title string
	Value int
}

func GetTimes(connector *gorm.DB) ([]*Time, error) {
	db := connector
	var times []*Time
	db.Debug().Find(&times)
	if err := db.Error; err != nil {
		return nil, err
	}
	return times, nil
}

func GetTimesByVendorId(connector *gorm.DB, vendorId uint) ([]*Time, error) {
	db := connector
	var times []*Time
	db.Model(&Time{}).Joins("inner join vendors_times on vendors_times.time_id = times.id").Where("vendors_times.vendor_id = ?", vendorId).Find(&times)
	if err :=  db.Error; err != nil {
		return nil, err
	}
	return times, nil
}

func CreateTime(connector *gorm.DB, time *Time) (uint, error) {
	db := connector
	db.Debug().Create(&time)
	if err := db.Error; err != nil {
		return 0, err
	}
	return time.ID, nil
}

func GetTime(connector *gorm.DB, id int) (*Time, error) {
	db := connector
	var time Time
	if err := db.Debug().Where("id = ?", id).First(&time).Error; err != nil {
		return nil, err
	}
	return &time, nil
}

func GetTimeByName(connector *gorm.DB, name string) (*Time, error) {
	db := connector
	var time Time
	if err := db.Where("name = ?", name).First(&time).Error; err != nil {
		return nil, err
	}
	return &time, nil
}

func UpdateTime(connector *gorm.DB, time *Time) error {
	db := connector
	db.Debug().Save(&time)
	return db.Error
}

func DeleteTime(connector *gorm.DB, time *Time) error {
	db := connector
	db.Unscoped().Debug().Delete(&time)
	return db.Error
}