package cmd

import (
	"context"
	"fmt"
	"github.com/yonnic/goshop/models"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
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
		common.Database.AutoMigrate(&models.Category{})
		common.Database.AutoMigrate(&models.Product{})
		common.Database.AutoMigrate(&models.Offer{})
		common.Database.AutoMigrate(&models.Option{})
		common.Database.AutoMigrate(&models.Property{})
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
				Name:  "dining-room",
				Title: "Dining Room",
				Parent: category1,
			}
			if _, err := models.CreateCategory(common.Database, subcategory1); err == nil {
				logger.Errorf("%v", err)
			}
			/*if err := models.AddSubcategoryToCategory(common.Database, category1, subcategory1); err != nil {
				logger.Errorf("%v", err)
			}*/
			// Create category Living Areas >> Dining Room >> Dining Tables
			subsubcategory1 := &models.Category{
				Name:  "dining-tables",
				Title: "Dining Tables",
				Parent: subcategory1,
			}
			if _, err := models.CreateCategory(common.Database, subsubcategory1); err == nil {
				logger.Errorf("%v", err)
			}
			/*if err := models.AddSubcategoryToCategory(common.Database, subcategory1, subsubcategory1); err != nil {
				logger.Errorf("%v", err)
			}*/
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
			// Create option Body Color
			option1 := &models.Option{
				Name:  "body-color",
				Title: "Body Color",
			}
			if _, err := models.CreateOption(common.Database, option1); err != nil {
				logger.Errorf("%v", err)
			}
			if err := models.AddOptionToCategory(common.Database, subsubcategory1, option1); err != nil {
				logger.Errorf("%v", err)
			}
			// Create option Plate Color
			option2 := &models.Option{
				Name:  "plate-color",
				Title: "Plate Color",
			}
			if _, err := models.CreateOption(common.Database, option2); err != nil {
				logger.Errorf("%v", err)
			}
			if err := models.AddOptionToCategory(common.Database, subsubcategory1, option2); err != nil {
				logger.Errorf("%v", err)
			}
			// Create offer #1
			offer1 := &models.Offer{
				Name:       "Body Color RAL9010 and Plate Color RAL9010",
				Title:      "Body Color Milk White and Plate of Same Color",
				Properties: []*models.Property{&models.Property{
					Option: *option1,
					Value:  "RAL9010",
				}, &models.Property{
					Option: *option2,
					Value:  "RAL9010",
				}},
				Price:      1000.0,
				ProductId:  product1.ID,
			}
			if _, err := models.CreateOffer(common.Database, offer1); err != nil {
				logger.Errorf("%v", err)
			}
			// Add Offer #1 to product
			if err := models.AddOfferToProduct(common.Database, product1, offer1); err != nil {
				logger.Errorf("%v", err)
			}
			// Create offer #2
			offer2 := &models.Offer{
				Name:       "Body Color RAL9010 and Plate Color A801",
				Title:      "Body Color White Milk and Plate Color Soft Gray",
				Properties: []*models.Property{&models.Property{
					Option: *option1,
					Value:  "RAL9010",
				}, &models.Property{
					Option: *option2,
					Value:  "A801",
				}},
				Price:      1010.0,
				ProductId:  product1.ID,
			}
			if _, err := models.CreateOffer(common.Database, offer2); err != nil {
				logger.Errorf("%v", err)
			}
			// Add Offer #2 to product
			if err := models.AddOfferToProduct(common.Database, product1, offer2); err != nil {
				logger.Errorf("%v", err)
			}
		}
		// ***
		// Get Products
		if products, err := models.GetProducts(common.Database); err == nil {
			logger.Infof("Products: %v", len(products))
			for _, product := range products {
				logger.Infof("\t#%v %v %v", product.ID, product.Name, product.Title)
				if categories, err := models.GetCategoriesOfProduct(common.Database, product); err == nil {
					logger.Infof("\tCategories: %v", len(categories))
					for _, category := range categories {
						var crumbs = []*models.Category{category}
						for ;category != nil && category.ParentId != 0; {
							if category = models.GetParentFromCategory(common.Database, category); category != nil {
								crumbs = append([]*models.Category{category}, crumbs...)
							}
						}
						var p []string
						for _, category := range crumbs {
							p = append(p, category.Title)
						}
						logger.Infof("\t\tPath: %v", strings.Join(p, " => "))
					}
				}
				if offers, err := models.GetOffersFromProduct(common.Database, product); err == nil {
					logger.Infof("\tOffers: %v", len(offers))
					for _, offer := range offers {
						logger.Infof("\t\t#%v %v $%.2f", offer.ID, offer.Title, offer.Price)
						if properties, err := models.GetPropertiesFromOffer(common.Database, offer); err == nil {
							logger.Infof("\t\tProperties: %v", len(properties))
							for _, property := range properties {
								logger.Infof("\t\t\t#%v %v = %v", property.Option.ID, property.Option.Name, property.Value)
							}
						}
					}
				}
			}
		}
		/*cat1 := &models.Category{
			Name: "cat1",
			Title: "Category 1",
		}
		models.CreateCategory(common.Database, cat1)
		cat2 := &models.Category{
			Name: "cat2",
			Title: "Category 2",
		}
		models.CreateCategory(common.Database, cat2)
		prod1 := &models.Product{
			Name: "prod1",
			Title: "Product 1",
		}
		models.CreateProduct(common.Database, prod1)
		if err := models.AddProductToCategory(common.Database, cat1, prod1); err != nil {
			logger.Errorf("%v", err)
		}
		prod2 := &models.Product{
			Name: "prod2",
			Title: "Product 2",
		}
		models.CreateProduct(common.Database, prod2)
		if err := models.AddProductToCategory(common.Database, cat1, prod2); err != nil {
			logger.Errorf("%v", err)
		}
		if err := models.DeleteProductFromCategory(common.Database, cat1, prod1); err != nil {
			logger.Errorf("%v", err)
		}
		prod3 := &models.Product{
			Name: "prod3",
			Title: "Product 3",
		}
		if _, err := models.CreateProduct(common.Database, prod3); err != nil {
			logger.Errorf("%v", err)
		}
		if err := models.AddProductToCategory(common.Database, cat1, prod3); err != nil {
			logger.Errorf("%v", err)
		}
		products, err := models.GetProductsFromCategory(common.Database, cat1)
		if err != nil {
			logger.Errorf("%v", err)
		}
		logger.Infof("Products: %v", len(products))
		for i, product := range products {
			logger.Infof("%d: %+v", i, product)
		}
		subcat1 := &models.Category{
			Name: "subcat1",
			Title: "Sub Category 1",
		}
		models.CreateCategory(common.Database, subcat1)
		if err := models.AddSubcategoryToCategory(common.Database, cat1, subcat1); err != nil {
			logger.Errorf("%v", err)
		}
		subcategories, err := models.GetSubcategoriesFromCategory(common.Database, cat1)
		if err != nil {
			logger.Errorf("%v", err)
		}
		logger.Infof("Subcategories: %v", len(subcategories))
		for i, subcategory := range subcategories {
			logger.Infof("%d: %+v", i, subcategory)
		}*/
		// /DEMO
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
