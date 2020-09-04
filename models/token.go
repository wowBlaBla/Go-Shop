package models

import (
	"fmt"
	"gorm.io/gorm"
)

const (
	TOKEN_STATUS_REVOKED = "revoked"
)

type Token struct {
	Id uint `gorm:"primaryKey"`
	Token string
	Status string
	ExpiresAt int64
}

/*func GetTokenByToken(connector *gorm.DB, token string) (tokens []*Token, err error){
	db := connector
	db.Where("token = ?", token).Find(&tokens)
	return tokens, db.Error
}*/

func GetTokenByTokenAndStatus(connector *gorm.DB, token string, status string) (*Token, error){
	db := connector
	var t Token
	db.Where("token = ? and status = ?", token, status).First(&t)
	if t.Token == "" {
		return nil, fmt.Errorf("token not found")
	}
	return &t, db.Error
}

func CreateToken(connector *gorm.DB, token *Token) (uint, error) {
	db := connector
	db.Debug().Create(&token)
	if err := db.Error; err != nil {
		return 0, err
	}
	return token.Id, nil
}

func DeleteTokenByExpiration(connector *gorm.DB) error {
	db := connector
	db.Where("ExpiresAt < now()").Delete(Token{})
	return db.Error
}