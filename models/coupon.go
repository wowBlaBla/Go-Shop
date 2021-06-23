package models

import (
	"gorm.io/gorm"
	"time"
)

type Coupon struct {
	gorm.Model
	Enabled bool
	Title string
	Description string
	Code string
	Type string // order, item, shipping,
	Start time.Time
	End time.Time
	Amount string // fixed value or %
	Minimum float64 // minimum total to by applied
	Count int // total count
	Limit int // limit per user
	//
	ApplyTo string
	Categories  []*Category  `gorm:"many2many:categories_coupons;"`
	Products  []*Product  `gorm:"many2many:products_coupons;"`
	Discounts []*Discount `gorm:"foreignKey:OrderId"`
}

func GetCouponsFull(connector *gorm.DB) ([]*Coupon, error) {
	db := connector
	var coupons []*Coupon
	db.Debug().Preload("Discounts").Find(&coupons).Order("ID asc")
	if err := db.Error; err != nil {
		return nil, err
	}
	return coupons, nil
}

func GetCoupons(connector *gorm.DB) ([]*Coupon, error) {
	db := connector
	var coupons []*Coupon
	db.Debug().Find(&coupons)
	if err := db.Error; err != nil {
		return nil, err
	}
	return coupons, nil
}

func GetCouponsByTitle(connector *gorm.DB, title string) ([]*Coupon, error) {
	db := connector
	var coupons []*Coupon
	if err := db.Debug().Where("title = ?", title).Find(&coupons).Error; err != nil {
		return nil, err
	}
	return coupons, nil
}

func GetCouponsByCode(connector *gorm.DB, code string) ([]*Coupon, error) {
	db := connector
	var coupons []*Coupon
	if err := db.Debug().Where("code = ?", code).Find(&coupons).Error; err != nil {
		return nil, err
	}
	return coupons, nil
}

func CreateCoupon(connector *gorm.DB, coupon *Coupon) (uint, error) {
	db := connector
	db.Debug().Create(&coupon)
	if err := db.Error; err != nil {
		return 0, err
	}
	return coupon.ID, nil
}

func GetCoupon(connector *gorm.DB, id int) (*Coupon, error) {
	db := connector
	var coupon Coupon
	if err := db.Debug().Preload("Categories").Preload("Products").Preload("Discounts").Where("id = ?", id).First(&coupon).Error; err != nil {
		return nil, err
	}
	return &coupon, nil
}

func GetCouponByCode(connector *gorm.DB, code string) (*Coupon, error){
	db := connector
	var coupon Coupon
	db.Preload("Categories").Preload("Products").Where("code = ?", code).First(&coupon)
	return &coupon, db.Error
}

func AddCategoryToCoupon(connector *gorm.DB, coupon *Coupon, category *Category) error {
	db := connector
	return db.Model(&coupon).Association("Categories").Append(category)
}

func DeleteAllCategoriesFromCoupon(connector *gorm.DB, coupon *Coupon) error {
	db := connector
	return db.Debug().Unscoped().Model(&coupon).Association("Categories").Clear()
}

func AddProductToCoupon(connector *gorm.DB, coupon *Coupon, product *Product) error {
	db := connector
	return db.Model(&coupon).Association("Products").Append(product)
}

func DeleteAllProductsFromCoupon(connector *gorm.DB, coupon *Coupon) error {
	db := connector
	return db.Debug().Unscoped().Model(&coupon).Association("Products").Clear()
}


func UpdateCoupon(connector *gorm.DB, coupon *Coupon) error {
	db := connector
	db.Debug().Save(&coupon)
	return db.Error
}


func DeleteCoupon(connector *gorm.DB, coupon *Coupon) error {
	db := connector
	db.Unscoped().Debug().Delete(&coupon)
	return db.Error
}