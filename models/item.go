package models

import "gorm.io/gorm"

type Item struct {
	gorm.Model
	//
	Uuid        string
	CategoryId  uint
	ProductId uint
	VariationId uint
	Title       string
	Description string `json:",omitempty"`
	Path string
	Thumbnail   string
	BasePrice   float64          `sql:"type:decimal(8,2);"`
	SalePrice   float64          `sql:"type:decimal(8,2);"`
	Price       float64          `sql:"type:decimal(8,2);"`
	VAT float64 `sql:"type:decimal(8,2);"`
	Discount    float64          `sql:"type:decimal(8,2);"`
	Quantity    int
	Delivery    float64          `sql:"type:decimal(8,2);"`
	Total       float64          `sql:"type:decimal(8,2);"`
	OrderId     uint
	Volume float64 `sql:"type:decimal(8,3);"`
	Weight float64 `sql:"type:decimal(8,3);"`
	//
	CommentId uint
	Comment       *Comment `gorm:"foreignKey:comment_id;"`
}

func GetItemsCountByProductId(connector *gorm.DB, productId uint) (int64, error) {
	db := connector
	var count int64
	if err := db.Model(&Item{}).Where("product_id = ?", productId).Count(&count).Error; err != nil {
		return count, err
	}
	return count, nil
}

func GetItem(connector *gorm.DB, id int) (*Item, error) {
	db := connector
	var Item Item
	if err := db.Debug().Where("id = ?", id).First(&Item).Error; err != nil {
		return nil, err
	}
	return &Item, nil
}

func GetItemByComment(connector *gorm.DB, commentId int) (*Item, error) {
	db := connector
	var Item Item
	if err := db.Debug().Where("comment_id = ?", commentId).First(&Item).Error; err != nil {
		return nil, err
	}
	return &Item, nil
}

func UpdateItem(connector *gorm.DB, item *Item) error {
	db := connector
	db.Debug().Save(&item)
	return db.Error
}

func DeleteItem(connector *gorm.DB, item *Item) error {
	db := connector
	db.Debug().Unscoped().Delete(&item)
	return db.Error
}