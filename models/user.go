package models

import (
	"crypto/sha256"
	"fmt"
	"github.com/yonnic/goshop/common"
	"gorm.io/gorm"
	"time"
)

const (
	ROLE_ROOT = iota
	ROLE_ADMIN
	ROLE_MANAGER
	ROLE_USER
)

type User struct {
	gorm.Model
	CreatedAt      time.Time
	Enabled        bool
	Login          string
	Password       string
	Email          string
	EmailConfirmed bool
	Name string
	Lastname string
	Role           int
	Notification   bool
	Code           string
	Attempt        time.Time
	//
	BillingProfiles       []*BillingProfile `gorm:"foreignKey:UserId"`
	ShippingProfiles       []*ShippingProfile `gorm:"foreignKey:UserId"`
	//
	AllowReceiveEmails bool `gorm:"foreignKey:UserId"`
	UpdatedAt time.Time
}

func GetUsers(connector *gorm.DB) (users []*User, err error){
	db := connector
	db.Find(&users)
	return users, db.Error
}

func GetUsersByRoleLessOrEqualsAndNotification(connector *gorm.DB, role int, notification bool) ([]*User, error) {
	db := connector
	var users []*User
	if err := db.Debug().Where("role <= ? and notification = ?", role, notification).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
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

func GetUserFull(connector *gorm.DB, id uint) (*User, error) {
	db := connector
	var user User
	if err := db.Preload("BillingProfiles").Preload("ShippingProfiles").First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func GetUserByLogin(connector *gorm.DB, login string) (*User, error){
	db := connector
	var user User
	db.Where("login = ?", login).First(&user)
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

func GetUserByEmailAndPassword(connector *gorm.DB, email string, password string) (*User, error){
	db := connector
	var user User
	db.Where("email = ? and password = ?", email, password).First(&user)
	if user.Email == "" {
		return nil, fmt.Errorf("user not found")
	}
	return &user, db.Error
}

func GetUserByEmailOrLogin(connector *gorm.DB, emailOrLogin string) (*User, error){
	db := connector
	var user User
	if err := db.Where("login = ? or email = ?", emailOrLogin, emailOrLogin).First(&user).Error; err != nil {
		return nil, fmt.Errorf("user not found")
	}
	return &user, db.Error
}

func GetUserByEmailOrLoginAndPassword(connector *gorm.DB, emailOrLogin string, password string) (*User, error){
	db := connector
	var user User
	db.Where("(login = ? or email = ?) and password = ?", emailOrLogin, emailOrLogin, password).First(&user)
	if user.Email == "" {
		return nil, fmt.Errorf("user not found")
	}
	return &user, db.Error
}

func GetUserByCode(connector *gorm.DB, code string) (*User, error){
	db := connector
	var user User
	db.Where("code = ?", code).First(&user)
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
	return user.ID, nil
}

/*func UpdateUser(connector *gorm.DB, id int, patch map[string]string) error {
	db := connector
	var user *User
	db.Preview().First(&user, id)
	if db.Error != nil || user.ID == 0 {
		return fmt.Errorf("user not found")
	}
	//
	if v, found := patch["Password"]; found{
		user.Password = v
	}
	//
	db.Preview().Save(&user)
	return db.Error
}*/

func UpdateUser(connector *gorm.DB, user *User) error {
	db := connector
	db.Debug().Unscoped().Save(&user)
	return db.Error
}

func DeleteUser(connector *gorm.DB, user *User) error {
	db := connector
	db.Debug().Unscoped().Delete(user)
	return db.Error
}

func MakeUserPassword(password string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(common.SALT + "@" + password)))
}
