package models

import "gorm.io/gorm"

const (
	MENU_TYPE_HEADER = "header"
	MENU_TYPE_FOOTER = "footer"
)

type Menu struct {
	gorm.Model
	Enabled bool
	Type string
	Name string
	Title string
	Description string
	Location string
}

func GetMenus(connector *gorm.DB) ([]*Menu, error) {
	db := connector
	var menus []*Menu
	if err := db.Debug().Find(&menus).Error; err != nil {
		return nil, err
	}
	return menus, nil
}

func CreateMenu(connector *gorm.DB, menu *Menu) (uint, error) {
	db := connector
	db.Debug().Create(&menu)
	if err := db.Error; err != nil {
		return 0, err
	}
	return menu.ID, nil
}

func GetMenu(connector *gorm.DB, id int) (*Menu, error) {
	db := connector
	var menu Menu
	db.Debug().Find(&menu, id)
	if err := db.Error; err != nil {
		return nil, err
	}
	return &menu, nil
}

func UpdateMenu(connector *gorm.DB, menu *Menu) error {
	db := connector
	db.Debug().Save(&menu)
	return db.Error
}

func DeleteMenu(connector *gorm.DB, menu *Menu) error {
	db := connector
	db.Debug().Unscoped().Delete(&menu)
	return db.Error
}