package models

import "gorm.io/gorm"

type EmailTemplate struct {
	gorm.Model
	Enabled bool
	Type string
	Topic string
	Message string
}

func (EmailTemplate) TableName() string {
	return "email_templates"
}

func GetEmailTemplates(connector *gorm.DB) ([]*EmailTemplate, error) {
	db := connector
	var template []*EmailTemplate
	if err := db.Find(&template).Error; err != nil {
		return nil, err
	}
	return template, nil
}

func GetEmailTemplateByType(connector *gorm.DB, type_ string) (*EmailTemplate, error) {
	db := connector
	var template EmailTemplate
	if err := db.Debug().Where("type = ?", type_).First(&template).Error; err != nil {
		return nil, err
	}
	return &template, nil
}

/*
func GetValueByOptionIdAndTitle(connector *gorm.DB, id int, title string) (*Value, error) {
	db := connector
	var value Value
	if err := db.Where("option_id = ? and title = ?", id).First(&title).Error; err != nil {
		return nil, err
	}
	return &value, nil
}
*/

func CreateEmailTemplate(connector *gorm.DB, template *EmailTemplate) (uint, error) {
	db := connector
	db.Create(&template)
	if err := db.Error; err != nil {
		return 0, err
	}
	return template.ID, nil
}

func GetEmailTemplate(connector *gorm.DB, id int) (*EmailTemplate, error) {
	db := connector
	var template EmailTemplate
	if err := db.Debug().Where("id = ?", id).First(&template).Error; err != nil {
		return nil, err
	}
	return &template, nil
}

func UpdateEmailTemplate(connector *gorm.DB, template *EmailTemplate) error {
	db := connector
	db.Save(&template)
	return db.Error
}


func DeleteEmailTemplate(connector *gorm.DB, template *EmailTemplate) error {
	db := connector
	db.Debug().Unscoped().Delete(&template)
	return db.Error
}