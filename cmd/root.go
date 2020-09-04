package cmd

import (
	"context"
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"syscall"
	"time"

	"github.com/cloudflare/tableflip"
	"github.com/fsnotify/fsnotify"
	"github.com/google/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/config"
	"github.com/yonnic/goshop/handler"
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

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "goshop",
	Short: "GoShop application",
	Long:  `GoShop application API server and tools`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Infof("%v v%v %v", common.APPLICATION, common.VERSION, common.COMPILED)
		if v := os.Getenv("SALT"); v != "" {
			common.SALT = v
		}
		var application = filepath.Base(os.Args[0])
		var extension = filepath.Ext(application)
		var pid = application[0:len(application)-len(extension)] + ".pid"

		upg, err := tableflip.New(tableflip.Options{
			PIDFile: path.Join("/tmp", pid),
		})
		if err != nil {
			panic(err)
		}
		defer upg.Stop()

		// Do an upgrade on SIGHUP
		go func() {
			sig := make(chan os.Signal, 1)
			signal.Notify(sig, syscall.SIGHUP)
			for range sig {
				err := upg.Upgrade()
				if err != nil {
					log.Println("ERR Upgrade failed:", err)
				}
			}
		}()

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
		common.Database, err = gorm.Open(dialer, &gorm.Config{})
		if err != nil {
			logger.Errorf("%v", err)
			os.Exit(1)
		}
		common.Database.DB()

		// Router
		router := handler.GetRouter()

		// Http
		host := common.Config.Host
		if host == "*" {
			host = ""
		}
		port := common.Config.Port
		bind1 := fmt.Sprintf("%v:%d", host, port)
		listen1, _ := upg.Listen("tcp", bind1)
		defer listen1.Close()

		server1 := http.Server{
			Handler: router,
		}

		go func() {
			logger.Infof("Listening HTTP Server on %v", bind1)
			err := server1.Serve(listen1)
			if err != http.ErrServerClosed {
				log.Println("[ERR] [APP] HTTP server:", err)
			}
		}()

		// Https
		if https := common.Config.Https; https.Enabled {
			host := https.Host
			if host == "*" {
				host = ""
			}
			port := https.Port
			bind := fmt.Sprintf("%v:%d", host, port)
			if listen, err := upg.Listen("tcp", bind); err == nil {
				defer listen.Close()

				server := http.Server{
					Handler: router,
				}

				var bad bool
				if _, err := os.Stat(common.Config.Https.Crt); err != nil {
					common.Config.Https.Crt = path.Join(dir, os.Getenv("SSL_FOLDER"), "cert.crt")
					bad = true
				}

				if _, err := os.Stat(common.Config.Https.Key); err != nil {
					common.Config.Https.Key = path.Join(dir, os.Getenv("SSL_FOLDER"), "cert.key")
					bad = true
				}

				var updated bool
				if bad {
					if err := config.GenerateSSL(common.Config.Https.Crt, common.Config.Https.Key, "localhost"); err == nil {
						bad = false
						updated = true
					} else {
						bad = true
						logger.Errorf(" %v", err)
					}
				}

				if updated {
					if err = common.Config.Save(); err != nil {
						logger.Errorf(" %v", err)
					}
				}

				go func() {
					logger.Infof("Listening HTTPS Server on %v", bind)
					err := server.ServeTLS(listen, common.Config.Https.Crt, common.Config.Https.Key)
					if err != http.ErrServerClosed {
						log.Println("[ERR] [APP] HTTP server:", err)
					}
				}()
			} else {
				log.Printf("ERR: %v", err)
			}
		}
		// /Https

		// TODO: Yet another initialization code

		if err := upg.Ready(); err != nil {
			panic(err)
		}
		<-upg.Exit()

		// Make sure to set a deadline on exiting the process
		// after upg.Exit() is closed. No new upgrades can be
		// performed if the parent doesn't exit.
		time.AfterFunc(15*time.Second, func() {
			log.Println("ERR: Graceful shutdown timed out")
			os.Exit(1)
		})

		// Wait for connections to drain.
		server1.Shutdown(context.Background())
	},
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
