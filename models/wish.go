package models

import "gorm.io/gorm"

type Wish struct {
	gorm.Model
	Uuid 		string
	CategoryId uint
	Path        string
	Name        string
	Title       string
	Description string
	Thumbnail   string
	Price       float64
	//
	User            *User `gorm:"foreignKey:UserId"`
	UserId          uint
}

func CreateWish(connector *gorm.DB, wish *Wish) (uint, error) {
	db := connector
	db.Debug().Create(&wish)
	if err := db.Error; err != nil {
		return 0, err
	}
	return wish.ID, nil
}

func GetWishesByUserId(connector *gorm.DB, userId uint) ([]*Wish, error){
	db := connector
	var wishes []*Wish
	db.Debug().Where("user_id = ?", userId).Order("id desc").Find(&wishes)
	if err := db.Error; err != nil {
		return nil, err
	}
	return wishes, nil
}

func GetWish(connector *gorm.DB, id int) (*Wish, error) {
	db := connector
	var wish Wish
	if err := db.Debug().First(&wish, id).Error; err != nil {
		return nil, err
	}
	return &wish, nil
}

func GetWishFull(connector *gorm.DB, id int) (*Wish, error) {
	db := connector
	var wish Wish
	db.Debug().Find(&wish, id)
	if err := db.Error; err != nil {
		return nil, err
	}
	return &wish, nil
}

func UpdateWish(connector *gorm.DB, wish *Wish) error {
	db := connector
	db.Debug().Save(&wish)
	return db.Error
}

func DeleteWish(connector *gorm.DB, wish *Wish) error {
	db := connector
	db.Debug().Unscoped().Delete(&wish)
	return db.Error
}