package models

import (
	"gorm.io/gorm"
)

type Profile struct {
	gorm.Model
	Name string
	Lastname string
	Company string
	Phone string
	Address string
	Zip string
	City string
	Region string
	Country string
	Payment string
	Billing bool //?
	ITN string
	//
	UserId uint
}

type ProfilePayment struct {
	Stripe ProfilePaymentStripe
}

type ProfilePaymentStripe struct {
	CustomerId string
}

func GetProfilesByUser(connector *gorm.DB, userId uint) ([]*Profile, error) {
	db := connector
	var profiles []*Profile
	if err := db.Debug().Where("user_id = ?", userId).Find(&profiles).Error; err != nil {
		return nil, err
	}
	return profiles, nil
}

func GetProfile(connector *gorm.DB, id uint) (*Profile, error) {
	db := connector
	var profile Profile
	if err := db.Where("id = ?", id).First(&profile).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

func CreateProfile(connector *gorm.DB, profile *Profile) (uint, error) {
	db := connector
	db.Debug().Create(&profile)
	if err := db.Error; err != nil {
		return 0, err
	}
	return profile.ID, nil
}

func UpdateProfile(connector *gorm.DB, profile *Profile) error {
	db := connector
	db.Debug().Save(&profile)
	return db.Error
}

func DeleteProfile(connector *gorm.DB, profile *Profile) error {
	db := connector
	db.Debug().Unscoped().Delete(&profile)
	return db.Error
}