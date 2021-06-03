package models

import "gorm.io/gorm"

type Comment struct {
	gorm.Model
	Enabled bool
	Uuid string
	Title string
	Body string
	Max int
	Images string
	Product            *Product `gorm:"foreignKey:ProductId"`
	ProductId          uint
	//
	User            *User `gorm:"foreignKey:UserId"`
	UserId          uint
}

func GetCommentsByProduct(connector *gorm.DB, id uint) ([]*Comment, error) {
	db := connector
	var comments []*Comment
	if err := db.Where("product_id = ?", id).Order("id desc").Find(&comments).Error; err != nil {
		return nil, err
	}
	return comments, nil
}

func GetCommentsByProductWithOffsetLimit(connector *gorm.DB, id uint, offset int, limit int) ([]*Comment, error) {
	db := connector
	var comments []*Comment
	if err := db.Debug().Where("product_id = ?", id).Offset(offset).Limit(limit).Order("id desc").Find(&comments).Error; err != nil {
		return nil, err
	}
	return comments, nil
}

func CreateComment(connector *gorm.DB, comment *Comment) (uint, error) {
	db := connector
	db.Debug().Create(&comment)
	if err := db.Error; err != nil {
		return 0, err
	}
	return comment.ID, nil
}

func GetComment(connector *gorm.DB, id int) (*Comment, error) {
	db := connector
	var comment Comment
	if err := db.Debug()/*.Preload("Products").Preload("Users")*/.Where("id = ?", id).First(&comment).Error; err != nil {
		return nil, err
	}
	return &comment, nil
}

func UpdateComment(connector *gorm.DB, comment *Comment) error {
	db := connector
	db.Debug().Save(&comment)
	return db.Error
}

func DeleteComment(connector *gorm.DB, comment *Comment) error {
	db := connector
	db.Unscoped().Debug().Delete(&comment)
	return db.Error
}