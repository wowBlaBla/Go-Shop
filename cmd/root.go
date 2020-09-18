package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/cloudflare/tableflip"
	"github.com/fsnotify/fsnotify"
	"github.com/google/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/config"
	"github.com/yonnic/goshop/handler"
	"github.com/yonnic/goshop/models"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"syscall"
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
		/*router := handler.GetRouter()

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
		*/
		common.Database.AutoMigrate(&models.Category{})
		common.Database.AutoMigrate(&models.Product{})
		//common.Database.AutoMigrate(&models.ProductProperty{})
		common.Database.AutoMigrate(&models.Offer{})
		common.Database.AutoMigrate(&models.Property{})
		common.Database.AutoMigrate(&models.Option{})
		common.Database.AutoMigrate(&models.Value{})
		common.Database.AutoMigrate(&models.Price{})
		// DEMO
		mode := "create" // create, select
		if mode == "create" {
			// Create category Living Areas
			category1 := &models.Category{
				Name:  "living-areas",
				Title: "Living Areas",
			}
			if _, err := models.CreateCategory(common.Database, category1); err != nil {
				logger.Errorf("%v", err)
			}
			// Create category Living Areas >> Dining Room
			subcategory1 := &models.Category{
				Name:   "dining-room",
				Title:  "Dining Room",
				Parent: category1,
			}
			if _, err := models.CreateCategory(common.Database, subcategory1); err == nil {
				logger.Errorf("%v", err)
			}
			// Create category Living Areas >> Dining Room >> Dining Tables
			subsubcategory1 := &models.Category{
				Name:   "dining-tables",
				Title:  "Dining Tables",
				Parent: subcategory1,
			}
			if _, err := models.CreateCategory(common.Database, subsubcategory1); err == nil {
				logger.Errorf("%v", err)
			}
			// Create product
			// example: https://www.moebelhausduesseldorf.de/wohnraum/esszimmer/tafel/wei%c3%9fer-esstisch-aus-massivem-kiefernholz-958479
			product1 := &models.Product{
				Name:        "white-dining-table",
				Title:       "White dining table",
				Description: "Cool table",
			}
			if _, err := models.CreateProduct(common.Database, product1); err != nil {
				logger.Errorf("%v", err)
			}
			if err := models.AddProductToCategory(common.Database, subsubcategory1, product1); err != nil {
				logger.Errorf("%v", err)
			}
			//
			ral9010 := &models.Value{
				Title: "RAL9010 - Weiß lackiert - unsere beliebteste Farbe",
				Thumbnail: "https://www.moebelhausduesseldorf.de/galleries/ral9010-weiss-lackiert-unsere-beliebteste-farbe-209104-hh-thumb.jpg",
				Value: "RAL9010",
			}
			ral9001 := &models.Value{
				Title: "RAL9001 - Leicht Cremeweiß lackiert",
				Thumbnail: "https://www.moebelhausduesseldorf.de/galleries/ral9001-leicht-cremeweiss-lackiert-209216-hh-thumb.jpg",
				Value: "RAL9001",
			}
			m803 := &models.Value{
				Title: "M803 - Pearl Grey Pinseleffekt",
				Thumbnail: "https://www.moebelhausduesseldorf.de/galleries/pearl-grey-pinseleffekt-m803-459219-hh-thumb.jpg",
				Value: "M803",
			}
			// Option: color
			color := &models.Option{
				Name: "color",
				Title: "Color",
				Values: []*models.Value{
					ral9010,
					ral9001,
					m803,
				},
			}
			if _, err := models.CreateOption(common.Database, color); err != nil {
				logger.Errorf("%v", err)
			}
			logger.Infof("Color: #%v %+v", color.ID, color.Title)
			if bts, err := json.MarshalIndent(color, "",  "   "); err == nil {
				logger.Infof("JSON: %+v", string(bts))
			}
			// Offer 1
			offer1 := &models.Offer{
				Name:       "body-color-ral9010-and-plate-color-ral9010",
				Title:      "Body Color Milk White and Plate of Same Color",
				Properties: []*models.Property{
					{
						Name: "body-color",
						Title: "Body Color",
						Option: color,
						Prices: []*models.Price{
							{
								Enabled: true,
								Value: ral9010,
								Price: 1.23,
							},
							{
								Enabled: true,
								Value: ral9001,
								Price: 2.34,
							},
						},
					},
					{
						Name: "plate-color",
						Title: "Plate Color",
						Option: color,
						Prices: []*models.Price{
							{
								Enabled: true,
								Value: ral9010,
								Price: 3.21,
							},
							{
								Enabled: true,
								Value: ral9001,
								Price: 4.32,
							},
						},
					},
				},
				BasePrice:      1000.0,
				ProductId:  product1.ID,
			}
			if _, err := models.CreateOffer(common.Database, offer1); err != nil {
				logger.Errorf("%v", err)
			}
			// Offer 2
			offer2 := &models.Offer{
				Name:       "plate-color-ral9010-and-plate-color-ral9010",
				Title:      "Plate Color Milk White and Plate of Same Color",
				Properties: []*models.Property{
					{
						Name: "body-color",
						Title: "Body Color",
						Option: color,
						Prices: []*models.Price{
							{
								Enabled: true,
								Value: ral9010,
								Price: 3.45,
							},
							{
								Enabled: true,
								Value: m803,
								Price: 4.56,
							},
						},
					}, {
						Name: "plate-color",
						Title: "Plate Color",
						Option: color,
						Prices: []*models.Price{
							{
								Enabled: true,
								Value: ral9010,
								Price: 13.45,
							},
							{
								Enabled: true,
								Value: m803,
								Price: 14.56,
							},
						},
					},
				},
				BasePrice:      1100.0,
				ProductId:  product1.ID,
			}
			if _, err := models.CreateOffer(common.Database, offer2); err != nil {
				logger.Errorf("%v", err)
			}
		}
		// /DEMO
		// TODO: Yet another initialization code

		/*if err := upg.Ready(); err != nil {
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
		server1.Shutdown(context.Background())*/

		app := handler.GetFiber()
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
