package models

import "gorm.io/gorm"

type Form struct {
	gorm.Model
	Enabled bool
	Name string
	Title string
	Description string `json:",omitempty"`
	Type string `json:",omitempty"` // empty = default
	//
	Messages []*Message `gorm:"foreignKey:FormId"`
}

func GetForms(connector *gorm.DB) ([]*Form, error) {
	db := connector
	var forms []*Form
	db.Debug().Find(&forms)
	if err := db.Error; err != nil {
		return nil, err
	}
	return forms, nil
}

func CreateForm(connector *gorm.DB, form *Form) (uint, error) {
	db := connector
	db.Debug().Create(&form)
	if err := db.Error; err != nil {
		return 0, err
	}
	return form.ID, nil
}

func GetForm(connector *gorm.DB, id int) (*Form, error) {
	db := connector
	var form Form
	if err := db.Debug().Where("id = ?", id).First(&form).Error; err != nil {
		return nil, err
	}
	return &form, nil
}

func UpdateForm(connector *gorm.DB, form *Form) error {
	db := connector
	db.Debug().Save(&form)
	return db.Error
}

func DeleteForm(connector *gorm.DB, form *Form) error {
	db := connector
	db.Unscoped().Debug().Delete(&form)
	return db.Error
}