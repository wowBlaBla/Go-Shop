package models

import "gorm.io/gorm"

type Discount struct {
	gorm.Model
	//
	Coupon *Property `gorm:"foreignKey:CouponId"`
	CouponId uint
	OrderId uint
}

func GetDiscounts(connector *gorm.DB) ([]*Discount, error) {
	db := connector
	var value []*Discount
	if err := db.Debug().Find(&value).Error; err != nil {
		return nil, err
	}
	return value, nil
}

func GetDiscountsByCouponId(connector *gorm.DB, id int) ([]*Discount, error) {
	db := connector
	var value []*Discount
	if err := db.Debug().Where("coupon_id = ?", id).Order("Name asc").Find(&value).Error; err != nil {
		return nil, err
	}
	return value, nil
}

func GetDiscount(connector *gorm.DB, id int) (*Discount, error) {
	db := connector
	var value Discount
	if err := db.Where("id = ?", id).First(&value).Error; err != nil {
		return nil, err
	}
	return &value, nil
}

func GetDiscountByCouponIdAndTitle(connector *gorm.DB, id int, title string) (*Discount, error) {
	db := connector
	var value Discount
	if err := db.Where("coupon_id = ? and title = ?", id).First(&title).Error; err != nil {
		return nil, err
	}
	return &value, nil
}

func GetDiscountByCouponIdAndDiscount(connector *gorm.DB, id int, val string) (*Discount, error) {
	db := connector
	var value Discount
	if err := db.Where("coupon_id = ? and value = ?", id).First(&val).Error; err != nil {
		return nil, err
	}
	return &value, nil
}

func CreateDiscount(connector *gorm.DB, value *Discount) (uint, error) {
	db := connector
	db.Debug().Create(&value)
	if err := db.Error; err != nil {
		return 0, err
	}
	return value.ID, nil
}

func UpdateDiscount(connector *gorm.DB, value *Discount) error {
	db := connector
	db.Debug().Save(&value)
	return db.Error
}


func DeleteDiscount(connector *gorm.DB, value *Discount) error {
	db := connector
	db.Debug().Unscoped().Delete(&value)
	return db.Error
}