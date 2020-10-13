package cmd

import (
	"crypto/tls"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/google/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/config"
	_ "github.com/yonnic/goshop/docs"
	"github.com/yonnic/goshop/handler"
	"github.com/yonnic/goshop/models"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var (
	dir, _  = filepath.Abs(filepath.Dir(os.Args[0]))
	cfgFile string
	v       *viper.Viper
)

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is config.json)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func initConfig() {
	var file string
	// JSON
	//file = path.Join(DIR, "config.json")
	// OR TOML
	file = path.Join(dir, os.Getenv("CONFIG_FOLDER"), "config.toml")
	v = viper.GetViper()
	v.SetConfigFile(file)
	common.Config = config.NewConfig(file)
	if err := v.ReadInConfig(); err != nil {
		common.Config.Host = config.DEFAULT_HOST
		common.Config.Port = config.DEFAULT_PORT
		common.Config.Https.Enabled = true
		common.Config.Https.Host = config.DEFAULT_HOST
		common.Config.Https.Port = config.DEFAULT_HTTPS_PORT
		common.Config.Database.Dialer = "sqlite"
		common.Config.Database.Uri = path.Join(dir, "database.sqlite")
		if err = common.Config.Save(); err != nil {
			logger.Errorf(" %v", err.Error())
		}
	}
	if err := v.Unmarshal(common.Config); err != nil {
		logger.Errorf(" %v", err.Error())
	}
	var ok = true
	if common.Config.Https.Crt == "" || common.Config.Https.Key == "" {
		ok = false
	}
	if ok {
		var crt []byte
		if _, err := os.Stat(common.Config.Https.Crt); err == nil {
			if crt, err = ioutil.ReadFile(common.Config.Https.Crt); err != nil {
				ok = false
			}
		}else{
			ok = false
		}
		var key []byte
		if _, err := os.Stat(common.Config.Https.Key); err == nil {
			if key, err = ioutil.ReadFile(common.Config.Https.Key); err != nil {
				ok = false
			}
		}else{
			ok = false
		}
		if ok {
			if _, err := tls.X509KeyPair(crt, key); err != nil {
				ok = false
			}
		}
	}
	if !ok {
		crtPath := path.Join(dir, "server.crt")
		keyPath := path.Join(dir, "server.key")
		if err := config.GenerateSSL(crtPath, keyPath, strings.Join([]string{"localhost"}, ",")); err == nil {
			common.Config.Https.Crt = crtPath
			common.Config.Https.Key = keyPath
			common.Config.Save()
		}else{
			log.Printf("[ERR] [APP] %v", err)
		}
	}
	v.WatchConfig()
	var loading bool
	v.OnConfigChange(func(e fsnotify.Event) {
		if !loading {
			loading = true
			go func() {
				log.Printf("Config file changed: %v", e)
				if err := v.Unmarshal(common.Config); err != nil {
					logger.Errorf(" %v", err.Error())
				}
				loading = false
			}()
		}
	})
}

var RootCmd = &cobra.Command{
	Use:   "goshop",
	Short: "GoShop application",
	Long:  `GoShop application API server and tools`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Infof("%v v%v %v", common.APPLICATION, common.VERSION, common.COMPILED)
		if v := os.Getenv("SALT"); v != "" {
			common.SALT = v
		}
		// Database
		var dialer gorm.Dialector
		if common.Config.Database.Dialer == "mysql" {
			dialer = mysql.Open(common.Config.Database.Uri)
		}else {
			var uri = path.Join(dir, os.Getenv("DATABASE_FOLDER"), "database.sqlite")
			if common.Config.Database.Uri != "" {
				uri = common.Config.Database.Uri
			}
			dialer = sqlite.Open(uri)
		}
		var err error
		common.Database, err = gorm.Open(dialer, &gorm.Config{})
		if err != nil {
			logger.Errorf("%v", err)
			os.Exit(1)
		}
		common.Database.DB()
		common.Database.AutoMigrate(&models.Category{})
		common.Database.AutoMigrate(&models.Product{})
		common.Database.AutoMigrate(&models.Image{})
		common.Database.AutoMigrate(&models.Offer{})
		common.Database.AutoMigrate(&models.Property{})
		common.Database.AutoMigrate(&models.Option{})
		common.Database.AutoMigrate(&models.Value{})
		common.Database.AutoMigrate(&models.Price{})
		//
		common.Database.AutoMigrate(&models.Order{})
		common.Database.AutoMigrate(&models.Item{})
		//
		app := handler.GetFiber()
		// Https
		go func(){
			if common.Config.Https.Enabled {
				host := common.Config.Https.Host
				if host == "*" {
					host = ""
				}
				port := common.Config.Https.Port
				var crt []byte
				if _, err := os.Stat(common.Config.Https.Crt); err == nil {
					crt, err = ioutil.ReadFile(common.Config.Https.Crt)
				}
				var key []byte
				if _, err := os.Stat(common.Config.Https.Key); err == nil {
					key, err = ioutil.ReadFile(common.Config.Https.Key)
				}
				if cert, err := tls.X509KeyPair(crt, key); err == nil {
					tlsConfig := &tls.Config{
						Certificates: []tls.Certificate{cert},
					}
					listen, _ := net.Listen("tcp", fmt.Sprintf("%v:%d", host, port))
					listener := tls.NewListener(listen, tlsConfig)
					logger.Fatalf("Listening https: %+v", app.Listener(listener))
				}
			}
		}()
		// Http
		host := common.Config.Host
		if host == "*" {
			host = ""
		}
		port := common.Config.Port
		logger.Fatalf("Listening http: %+v", app.Listen(fmt.Sprintf("%v:%d", host, port)))
	},
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
