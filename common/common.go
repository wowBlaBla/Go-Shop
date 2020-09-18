package common

import (
	"github.com/yonnic/goshop/config"
	"gorm.io/gorm"
	"time"
)

const (
	DEFAULT_PASSWORD = "goshoppass"
)

var (
	APPLICATION = "GoShop"
	VERSION = "1.0.0"
	COMPILED = "20200918152853"
	//
	Started          time.Time
	Config           *config.Config
	Database *gorm.DB
	//
	SALT = "goshop"
)
