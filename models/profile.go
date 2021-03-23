package models

import (
	"gorm.io/gorm"
)



/**/

type BillingProfile struct {
	gorm.Model
	Email string
	Name     string
	Lastname string
	Company  string
	Phone    string
	Address  string
	Zip      string
	City     string
	Region   string
	Country  string
	Payment  string
	//
	UserId uint
}

func GetBillingProfilesByUser(connector *gorm.DB, userId uint) ([]*BillingProfile, error) {
	db := connector
	var profiles []*BillingProfile
	if err := db.Debug().Where("user_id = ?", userId).Find(&profiles).Error; err != nil {
		return nil, err
	}
	return profiles, nil
}

func GetBillingProfile(connector *gorm.DB, id uint) (*BillingProfile, error) {
	db := connector
	var profile BillingProfile
	if err := db.Where("id = ?", id).First(&profile).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

func CreateBillingProfile(connector *gorm.DB, profile *BillingProfile) (uint, error) {
	db := connector
	db.Debug().Create(&profile)
	if err := db.Error; err != nil {
		return 0, err
	}
	return profile.ID, nil
}

func UpdateBillingProfile(connector *gorm.DB, profile *BillingProfile) error {
	db := connector
	db.Debug().Save(&profile)
	return db.Error
}

func DeleteBillingProfile(connector *gorm.DB, profile *BillingProfile) error {
	db := connector
	db.Debug().Unscoped().Delete(&profile)
	return db.Error
}

/**/

type ShippingProfile struct {
	gorm.Model
	Email string
	Name     string
	Lastname string
	Company  string
	Phone    string
	Address  string
	Zip      string
	City     string
	Region   string
	Country  string
	//
	UserId uint
}

func GetShippingProfilesByUser(connector *gorm.DB, userId uint) ([]*ShippingProfile, error) {
	db := connector
	var profiles []*ShippingProfile
	if err := db.Debug().Where("user_id = ?", userId).Find(&profiles).Error; err != nil {
		return nil, err
	}
	return profiles, nil
}

func GetShippingProfile(connector *gorm.DB, id uint) (*ShippingProfile, error) {
	db := connector
	var profile ShippingProfile
	if err := db.Where("id = ?", id).First(&profile).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

func CreateShippingProfile(connector *gorm.DB, profile *ShippingProfile) (uint, error) {
	db := connector
	db.Debug().Create(&profile)
	if err := db.Error; err != nil {
		return 0, err
	}
	return profile.ID, nil
}

func UpdateShippingProfile(connector *gorm.DB, profile *ShippingProfile) error {
	db := connector
	db.Debug().Save(&profile)
	return db.Error
}

func DeleteShippingProfile(connector *gorm.DB, profile *ShippingProfile) error {
	db := connector
	db.Debug().Unscoped().Delete(&profile)
	return db.Error
}