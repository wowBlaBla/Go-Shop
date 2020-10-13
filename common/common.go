package common

import (
	"github.com/yonnic/goshop/config"
	"gorm.io/gorm"
	"time"
)

const (
	DEFAULT_PASSWORD = "goshoppass"
	PRODUCTS_NAME    = "products"
)

var (
	APPLICATION = "GoShop"
	VERSION = "1.0.0"
	COMPILED = "20201012184209"
	//
	Started          time.Time
	Config           *config.Config
	Database *gorm.DB
	//
	SALT = "goshop"
)
