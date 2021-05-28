package models

import "gorm.io/gorm"

type Message struct {
	gorm.Model
	Title string
	Body string
	//
	FormId uint
}

func GetMessages(connector *gorm.DB) ([]*Message, error) {
	db := connector
	var messages []*Message
	db.Debug().Find(&messages)
	if err := db.Error; err != nil {
		return nil, err
	}
	return messages, nil
}

func GetMessagesByFormId(connector *gorm.DB, id int) ([]*Message, error) {
	db := connector
	var messages []*Message
	if err := db.Debug().Where("form_id = ?", id).Order("id desc").Find(&messages).Error; err != nil {
		return nil, err
	}
	return messages, nil
}

func CreateMessage(connector *gorm.DB, message *Message) (uint, error) {
	db := connector
	db.Debug().Create(&message)
	if err := db.Error; err != nil {
		return 0, err
	}
	return message.ID, nil
}

func GetMessage(connector *gorm.DB, id int) (*Message, error) {
	db := connector
	var message Message
	if err := db.Debug().Where("id = ?", id).First(&message).Error; err != nil {
		return nil, err
	}
	return &message, nil
}

func UpdateMessage(connector *gorm.DB, message *Message) error {
	db := connector
	db.Debug().Save(&message)
	return db.Error
}

func DeleteMessage(connector *gorm.DB, message *Message) error {
	db := connector
	db.Unscoped().Debug().Delete(&message)
	return db.Error
}