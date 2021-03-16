package models

import (
	"gorm.io/gorm"
)

const (
	ORDER_STATUS_NEW                  = "new"
	ORDER_STATUS_WAITING_FROM_PAYMENT = "waiting for payment"
	ORDER_STATUS_PAID                 = "paid"
	ORDER_STATUS_MANUFACTURING        = "manufacturing"
	ORDER_STATUS_SHIPPING             = "shipping"
	ORDER_STATUS_COMPLETE             = "complete"
	ORDER_STATUS_CANCELED             = "canceled"
)

type Order struct {
	gorm.Model
	//
	Description string // put here coupons too
	Items []*Item `gorm:"foreignKey:OrderId"`
	Quantity int
	Sum  float64 `sql:"type:decimal(8,2);"`
	Discount float64 `sql:"type:decimal(8,2);"`
	Delivery float64 `sql:"type:decimal(8,2);"`
	Discount2 float64 `sql:"type:decimal(8,2);"`
	Total float64 `sql:"type:decimal(8,2);"`
	VAT float64 `sql:"type:decimal(8,2);"`
	Status string
	Comment string
	Volume float64 `sql:"type:decimal(8,3);"`
	Weight float64 `sql:"type:decimal(8,3);"`
	//
	Discounts []*Discount `gorm:"foreignKey:OrderId"`
	User *User `gorm:"foreignKey:UserId"`
	UserId uint
	Profile *Profile `gorm:"foreignKey:ProfileId"`
	ProfileId uint
	Transport *Transport `gorm:"foreignKey:TransportId"`
	TransportId uint
	PaymentMethod string
}

func CreateOrder(connector *gorm.DB, order *Order) (uint, error) {
	db := connector
	db.Debug().Create(&order)
	if err := db.Error; err != nil {
		return 0, err
	}
	return order.ID, nil
}

func GetOrdersByUserId(connector *gorm.DB, userId uint) ([]*Order, error){
	db := connector
	var orders []*Order
	db.Debug().Preload("Items").Preload("User").Where("user_id = ?", userId).Order("id desc").Find(&orders)
	if err := db.Error; err != nil {
		return nil, err
	}
	return orders, nil
}

func GetOrder(connector *gorm.DB, id int) (*Order, error) {
	db := connector
	var order Order
	db.Debug().Preload("Items").Find(&order, id)
	if err := db.Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func GetOrderFull(connector *gorm.DB, id int) (*Order, error) {
	db := connector
	var order Order
	db.Debug().Preload("Items").Preload("Profile").Preload("Transport").Preload("User").Find(&order, id)
	if err := db.Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func UpdateOrder(connector *gorm.DB, order *Order) error {
	db := connector
	db.Debug().Save(&order)
	return db.Error
}

func DeleteOrder(connector *gorm.DB, order *Order) error {
	db := connector
	db.Debug().Unscoped().Delete(&order)
	return db.Error
}
