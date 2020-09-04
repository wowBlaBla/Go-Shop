package models

import (
	"crypto/sha256"
	"fmt"
	"github.com/yonnic/goshop/common"
	"gorm.io/gorm"
	"time"
)

const (
	ROLE_ROOT = "root"
	ROLE_ADMIN = "admin"
	ROLE_MANAGER = "manager"
	ROLE_USER = "user"
)

type User struct {
	Id        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	Enable    bool
	Login string
	Password  string
	Email     string
	//
	UpdatedAt time.Time
}

func GetUsers(connector *gorm.DB) (users []*User, err error){
	db := connector
	db.Find(&users)
	return users, db.Error
}

func GetUser(connector *gorm.DB, id int) (*User, error){
	db := connector
	var user User
	db.Where("id = ?", id).First(&user)
	if user.Email == "" {
		return nil, fmt.Errorf("user not found")
	}
	return &user, db.Error
}

func GetUserByLoginAndPassword(connector *gorm.DB, login string, password string) (*User, error){
	db := connector
	var user User
	db.Where("login = ? and password = ?", login, password).First(&user)
	if user.Email == "" {
		return nil, fmt.Errorf("user not found")
	}
	return &user, db.Error
}

func GetUserByEmail(connector *gorm.DB, email string) (*User, error){
	db := connector
	var user User
	db.Where("email = ?", email).First(&user)
	if user.Email == "" {
		return nil, fmt.Errorf("user not found")
	}
	return &user, db.Error
}

func CreateUser(connector *gorm.DB, user *User) (uint, error) {
	db := connector
	db.Debug().Create(&user)
	if err := db.Error; err != nil {
		return 0, err
	}
	return user.Id, nil
}

func UpdateUser(connector *gorm.DB, id int, patch map[string]string) error {
	db := connector
	var user *User
	db.Debug().First(&user, id)
	if db.Error != nil || user.Id == 0 {
		return fmt.Errorf("user not found")
	}
	//
	if v, found := patch["Password"]; found{
		user.Password = v
	}
	//
	db.Debug().Save(&user)
	return db.Error
}

func DeleteUser(connector *gorm.DB, id int) error {
	db := connector
	db.Delete(&User{}, id)
	return db.Error
}

func MakeUserPassword(password string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(common.SALT + "@" + password)))
}
