package models

import (
	"gorm.io/gorm"
)

type Migration struct {
	gorm.Model
	Timestamp string
	Name string
	Description string
	Output string
	Run func() (string, error) `gorm:"-"`
}

func GetMigrations(connector *gorm.DB) ([]*Migration, error) {
	db := connector
	var migrations []*Migration
	if err := db.Order("Timestamp asc").Find(&migrations).Error; err != nil {
		return nil, err
	}
	return migrations, nil
}

func CreateMigration(connector *gorm.DB, migration *Migration) (uint, error) {
	db := connector
	db.Debug().Create(&migration)
	if err := db.Error; err != nil {
		return 0, err
	}
	return migration.ID, nil
}

func GetMigration(connector *gorm.DB, id int) (*Migration, error) {
	db := connector
	var migration Migration
	if err := db.Debug().Where("id = ?", id).First(&migration).Error; err != nil {
		return nil, err
	}
	return &migration, nil
}

func UpdateMigration(connector *gorm.DB, migration *Migration) error {
	db := connector
	db.Debug().Save(&migration)
	return db.Error
}

func DeleteMigration(connector *gorm.DB, migration *Migration) error {
	db := connector
	db.Unscoped().Debug().Delete(&migration)
	return db.Error
}