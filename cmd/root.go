package cmd

import (
	"crypto/tls"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/fsnotify/fsnotify"
	"github.com/google/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/config"
	_ "github.com/yonnic/goshop/docs"
	"github.com/yonnic/goshop/handler"
	"github.com/yonnic/goshop/models"
	"github.com/yonnic/goshop/storage"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
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
	// JSON
	//file := path.Join(DIR, "config.json")
	// OR TOML
	file := path.Join(dir, os.Getenv("CONFIG_FOLDER"), "config.toml")
	v = viper.GetViper()
	v.SetConfigFile(file)
	common.Config = config.NewConfig(file)
	if err := v.ReadInConfig(); err != nil {
		common.Config.Host = config.DEFAULT_HOST
		common.Config.Port = config.DEFAULT_PORT
		common.Config.Https.Enabled = true
		common.Config.Https.Host = config.DEFAULT_HOST
		common.Config.Https.Port = config.DEFAULT_HTTPS_PORT
		if os.Getenv("DOCKER") == "true" {
			common.Config.Database.Dialer = "mysql"
			common.Config.Database.Uri = "root:mysecret@tcp(mysql:3306)/goshop?parseTime=true"
		}else{
			common.Config.Database.Dialer = "sqlite"
			p := path.Join(dir, config.DEFAULT_DATABASE_URI)
			if pp := path.Dir(p); len(pp) > 0 {
				if _, err = os.Stat(pp); err != nil {
					if err = os.MkdirAll(pp, 0755); err != nil {
						logger.Warningf("%v", err)
					}
				}
			}
			common.Config.Database.Uri = p
		}
		/*common.Config.I18n.Enabled = true
		common.Config.I18n.Languages = []config.Language{
			{
				Enabled: true,
				Name: "Deutsche",
				Code: "de",
			},
		}*/
		if os.Getenv("DOCKER") == "true" {
			common.Config.Hugo.Bin = "/usr/bin/docker run --rm -v %DIR%/hugo:/src klakegg/hugo:0.80.0"
		}else{
			common.Config.Hugo.Bin = config.DEFAULT_HUGO
		}
		common.Config.Hugo.Theme = "default"
		common.Config.Hugo.Minify = true
		common.Config.Publisher.Enabled = true
		if os.Getenv("DOCKER") == "true" {
			common.Config.Publisher.Bin = "/usr/bin/docker run --rm -v %DIR%/hugo/public:/hugo/public -v %DIR%/worker/workers-site:/worker/workers-site -v %DIR%/worker/wrangler.toml:/worker/wrangler.toml goshop_wrangler"
		}else{
			common.Config.Publisher.Bin = "/bin/true"
		}
		common.Config.Products = "Products"
		common.Config.FlatUrl = true
		common.Config.Resize.Quality = 75
		common.Config.Resize.Thumbnail.Size = "64x0,128x0,256x0"
		common.Config.Resize.Image.Size= "128x0,324x0,512x0"
		common.Config.Currency = "usd"
		common.Config.Payment.Default = "stripe"
		common.Config.Payment.Country = "DE"
		common.Config.Payment.VAT = 19
		common.Config.Swagger.Enabled = false
		common.Config.Swagger.Url = fmt.Sprintf("http://localhost:%d/swagger.json", config.DEFAULT_PORT)
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
		folder := os.Getenv("SSL_FOLDER")
		if folder == ""{
			folder = "ssl"
		}
		crtPath := path.Join(dir, folder, "server.crt")
		if _, err := os.Stat(path.Dir(crtPath)); err != nil {
			if err = os.MkdirAll(path.Dir(crtPath), 0755); err != nil {
				logger.Warningf("%+v", err)
			}
		}
		keyPath := path.Join(dir, folder, "server.key")
		if _, err := os.Stat(path.Dir(keyPath)); err != nil {
			if err = os.MkdirAll(path.Dir(keyPath), 0755); err != nil {
				logger.Warningf("%+v", err)
			}
		}
		if err := config.GenerateSSL(crtPath, keyPath, strings.Join([]string{"localhost"}, ",")); err == nil {
			common.Config.Https.Crt = crtPath
			common.Config.Https.Key = keyPath
			if err = common.Config.Save(); err != nil {
				logger.Fatalf("%+v", err)
			}
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
		} else if common.Config.Database.Dialer == "postgres" {
			dialer = postgres.Open(common.Config.Database.Uri)
		} else {
			var uri = path.Join(dir, os.Getenv("DATABASE_FOLDER"), "database.sqlite")
			if common.Config.Database.Uri != "" {
				uri = common.Config.Database.Uri
			}
			if _, err := os.Stat(path.Dir(uri)); err != nil {
				if err = os.MkdirAll(path.Dir(uri), 0755); err != nil {
					logger.Warningf("%+v", err)
				}
			}
			dialer = sqlite.Open(uri)
		}
		var err error
		common.Database, err = gorm.Open(dialer, &gorm.Config{
			DisableForeignKeyConstraintWhenMigrating: true,
		})
		if err != nil {
			logger.Errorf("%v", err)
			os.Exit(1)
		}
		if _, err := common.Database.DB(); err != nil {
			logger.Fatalf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.Category{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.Product{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.Parameter{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.File{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.Image{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.Variation{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.Property{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.Option{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.Value{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.Rate{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.Price{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.Coupon{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.Discount{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.Order{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.Item{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.Transaction{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.Tag{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.Tariff{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.Transport{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.Zone{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.EmailTemplate{}); err != nil {
			logger.Warningf("%+v", err)
		}
		//
		if err := common.Database.AutoMigrate(&models.CacheCategory{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.CacheProduct{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.CacheFile{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.CacheImage{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.CacheVariation{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.CacheValue{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.CacheTag{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.CacheTransport{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.CacheVendor{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.CacheComment{}); err != nil {
			logger.Warningf("%+v", err)
		}
		//
		if err := common.Database.AutoMigrate(&models.BillingProfile{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.ShippingProfile{}); err != nil {
			logger.Warningf("%+v", err)
		}
		//
		if err := common.Database.AutoMigrate(&models.Vendor{}); err != nil {
			logger.Warningf("%+v", err)
		}
		//
		if err := common.Database.AutoMigrate(&models.Time{}); err != nil {
			logger.Warningf("%+v", err)
		}
		//
		if err := common.Database.AutoMigrate(&models.Widget{}); err != nil {
			logger.Warningf("%+v", err)
		}
		//
		if err := common.Database.AutoMigrate(&models.Wish{}); err != nil {
			logger.Warningf("%+v", err)
		}
		//
		if err := common.Database.AutoMigrate(&models.Menu{}); err != nil {
			logger.Warningf("%+v", err)
		}
		//
		if err := common.Database.AutoMigrate(&models.Comment{}); err != nil {
			logger.Warningf("%+v", err)
		}
		//
		if err := common.Database.Exec(`CREATE TABLE IF NOT EXISTS categories_products_sort (
		CategoryId BIGINT UNSIGNED NOT NULL,
		ProductId BIGINT UNSIGNED NOT NULL,
		Value BIGINT UNSIGNED NOT NULL,
			PRIMARY KEY (CategoryId, ProductId),
			CONSTRAINT Constr_CategoryId_ProductId_fk
		FOREIGN KEY (CategoryId) REFERENCES categories (ID)
		ON DELETE CASCADE ON UPDATE CASCADE,
			CONSTRAINT Constr_ProductId_CategoryId_fk
		FOREIGN KEY (ProductId) REFERENCES products (ID)
		ON DELETE CASCADE ON UPDATE CASCADE
		)`).Error; err != nil {
			logger.Errorf("%+v", err)
		}
		//
		if err := common.Database.Exec(`CREATE TABLE IF NOT EXISTS products_relations (
		ProductIdL BIGINT UNSIGNED NOT NULL,
		ProductIdR BIGINT UNSIGNED NOT NULL,
			PRIMARY KEY (ProductIdL, ProductIdR),
			CONSTRAINT Constr_ProductIdL_ProductIdR_fk
		FOREIGN KEY (ProductIdL) REFERENCES products (ID)
		ON DELETE CASCADE ON UPDATE CASCADE,
			CONSTRAINT Constr_ProductIdR_ProductIdL_fk
		FOREIGN KEY (ProductIdR) REFERENCES products (ID)
		ON DELETE CASCADE ON UPDATE CASCADE
		)`).Error; err != nil {
			logger.Errorf("%+v", err)
		}
		//
		if err := common.Database.Exec(`update categories set sort = id where sort is null or sort = 0`).Error; err != nil {
			logger.Errorf("%+v", err)
		}
		if err := common.Database.Exec(`update options set sort = id where sort is null or sort = 0`).Error; err != nil {
			logger.Errorf("%+v", err)
		}
		if err := common.Database.Exec("update `values` set sort = id where sort is null or sort = 0").Error; err != nil {
			logger.Errorf("%+v", err)
		}
		// Manual database migration
		/*reDimension := regexp.MustCompile(`^([0-9\.,]+)\s*x\s*([0-9\.,]+)\s*x\s*([0-9\.,]+)\s*`)
		if products, err := models.GetProducts(common.Database); err == nil {
			for _, product := range products {
				if product.Dimensions != "" {
					if res := reDimension.FindAllStringSubmatch(product.Dimensions, 1); len(res) > 0 && len(res[0]) > 1 {
						if v, err := strconv.ParseFloat(strings.Replace(res[0][1], ",", ".", 1), 10); err == nil {
							product.Width = v
						}
						if v, err := strconv.ParseFloat(strings.Replace(res[0][2], ",", ".", 1), 10); err == nil {
							product.Height = v
						}
						if v, err := strconv.ParseFloat(strings.Replace(res[0][3], ",", ".", 1), 10); err == nil {
							product.Depth = v
						}
						product.Dimensions = ""
					}
					if err = models.UpdateProduct(common.Database, product); err != nil {
						logger.Warningf("%+v", err)
					}
				}
			}
		}
		if variations, err := models.GetVariations(common.Database); err == nil {
			for _, variation := range variations {
				if variation.Dimensions != "" {
					if res := reDimension.FindAllStringSubmatch(variation.Dimensions, 1); len(res) > 0 && len(res[0]) > 1 {
						if v, err := strconv.ParseFloat(strings.Replace(res[0][1], ",", ".", 1), 10); err == nil {
							variation.Width = v
						}
						if v, err := strconv.ParseFloat(strings.Replace(res[0][2], ",", ".", 1), 10); err == nil {
							variation.Height = v
						}
						if v, err := strconv.ParseFloat(strings.Replace(res[0][3], ",", ".", 1), 10); err == nil {
							variation.Depth = v
						}
						variation.Dimensions = ""
					}
					if err = models.UpdateVariation(common.Database, variation); err != nil {
						logger.Warningf("%+v", err)
					}
				}
			}
		}*/
		// Project structure
		if admin := path.Join(dir, "admin"); len(admin) > 0 {
			if _, err := os.Stat(admin); err != nil {
				if err = os.MkdirAll(admin, 0755); err != nil {
					logger.Errorf("%v", err)
				}
			}
			if index := path.Join(admin, "index.html"); len(index) > 0 {
				if _, err := os.Stat(index); err != nil {
					if err = ioutil.WriteFile(index, []byte(`Admin UI should be here`), 0644); err != nil {
						logger.Errorf("%v", err)
					}
				}
			}
		}
		if hugo := path.Join(dir, "hugo"); len(hugo) > 0 {
			if _, err := os.Stat(hugo); err != nil {
				if err = os.MkdirAll(hugo, 0755); err != nil {
					logger.Errorf("%v", err)
				}
			}
		}
		if config := path.Join(dir, "hugo", "config.toml"); len(config) > 0 {
			if _, err := os.Stat(config); err != nil {
				var conf handler.HugoSettingsView
				//
				conf.Paginate = common.DEFAULT_PAGINATE
				conf.Title = common.DEFAULT_TITLE
				conf.Theme = common.DEFAULT_THEME
				conf.Related = struct {
					IncludeNewer bool `toml:"includeNewer"`
					Threshold    int  `toml:"threshold"`
					ToLower      bool `toml:"toLower"`
					Indices      []struct {
						Name   string `toml:"name"`
						Weight int    `toml:"weight"`
					} `toml:"indices"`
				}(struct {
					IncludeNewer bool
					Threshold    int
					ToLower      bool
					Indices      []struct {
						Name   string `toml:"name"`
						Weight int    `toml:"weight"`
					}
				}{IncludeNewer: true, Threshold: 80, ToLower: false, Indices: []struct {
					Name   string `toml:"name"`
					Weight int    `toml:"weight"`
				}{{Name: "Related", Weight: 100}}})
				conf.Params.Currency = strings.ToLower(common.Config.Currency)
				conf.Params.Symbol = strings.ToLower(common.Config.Symbol)
				conf.Params.Products = strings.ToLower(common.Config.Products)
				conf.Params.FlatUrl = common.Config.FlatUrl
				conf.Params.MollieProfileId = common.Config.Payment.Mollie.ProfileID
				conf.Params.StripePublishedKey = common.Config.Payment.Stripe.PublishedKey
				//
				f, err := os.Create(path.Join(dir, "hugo", "config.toml"))
				if err != nil {
					log.Fatal(err)
				}
				defer f.Close()
				if err = toml.NewEncoder(f).Encode(conf); err != nil {
					logger.Fatalf("%+v", err)
					os.Exit(1)
				}
			}
		}
		if themes := path.Join(dir, "hugo", "themes"); len(themes) > 0 {
			if _, err := os.Stat(themes); err != nil {
				if err = os.MkdirAll(themes, 0755); err != nil {
					logger.Errorf("%v", err)
				}
			}
		}
		if static := path.Join(dir, "hugo", "static"); len(static) > 0 {
			if _, err := os.Stat(static); err != nil {
				if err = os.MkdirAll(static, 0755); err != nil {
					logger.Errorf("%v", err)
				}
			}
		}
		if public := path.Join(dir, "hugo", "public"); len(public) > 0 {
			if _, err := os.Stat(public); err != nil {
				if err = os.MkdirAll(public, 0755); err != nil {
					logger.Errorf("%v", err)
				}
			}
			if index := path.Join(public, "index.html"); len(index) > 0 {
				if _, err := os.Stat(index); err != nil {
					if err = ioutil.WriteFile(index, []byte(`Public content is not generated yet`), 0755); err != nil {
						logger.Errorf("%v", err)
					}
				}
			}
		}
		// Payment
		if common.Config.Payment.Enabled {
			if common.Config.Payment.Stripe.Enabled {
				common.STRIPE = common.NewStripe(common.Config.Payment.Stripe.SecretKey)
			}
			if common.Config.Payment.Mollie.Enabled {
				common.MOLLIE = common.NewMollie(common.Config.Payment.Mollie.Key)
			}
		}
		// Notification
		if common.Config.Notification.Enabled {
			common.NOTIFICATION = common.NewNotification()
			if common.Config.Notification.Email.Enabled {
				if common.Config.Notification.Email.Key != "" {
					common.NOTIFICATION.SendGrid = common.NewSendGrid(common.Config.Notification.Email.Key)
				}
				if templates, err := models.GetEmailTemplates(common.Database); err == nil && len(templates) == 0 {
					// Admin
					if _, err = models.CreateEmailTemplate(common.Database, &models.EmailTemplate{
						Enabled: false,
						Type:    common.NOTIFICATION_TYPE_ADMIN_ORDER_PAID,
						Topic:   "New order #{{.Order.ID}} paid",
						Message: `<!DOCTYPE HTML PUBLIC "-//W3C//DTD XHTML 1.0 Transitional //EN"
  "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:v="urn:schemas-microsoft-com:vml"
      xmlns:o="urn:schemas-microsoft-com:office:office">
<head>
  <!--[if gte mso 9]>
  <xml>
    <o:OfficeDocumentSettings>
      <o:AllowPNG/>
      <o:PixelsPerInch>96</o:PixelsPerInch>
    </o:OfficeDocumentSettings>
  </xml>
  <![endif]-->
  <meta http-equiv="Content-Type" content="text/html; charset=UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta name="x-apple-disable-message-reformatting">
  <!--[if !mso]><!-->
  <meta http-equiv="X-UA-Compatible" content="IE=edge"><!--<![endif]-->
  <title>Order #{{.Order.ID}}</title>

  <style type="text/css">
    a {
      color: #6d6d6d;
      text-decoration: none;
    }

    @media (max-width: 480px) {
      #u_content_image_1 .v-src-width {
        width: auto !important;
      }

      #u_content_image_1 .v-src-max-width {
        max-width: 50% !important;
      }

      #u_content_text_1 .v-text-align {
        text-align: center !important;
      }

      #u_content_text_14 .v-text-align {
        text-align: center !important;
      }

      #u_content_text_15 .v-text-align {
        text-align: center !important;
      }
    }

    @media only screen and (min-width: 620px) {
      .u-row {
        width: 600px !important;
      }

      .u-row .u-col {
        vertical-align: top;
      }

      .u-row .u-col-33p33 {
        width: 199.98px !important;
      }

      .u-row .u-col-50 {
        width: 300px !important;
      }

      .u-row .u-col-66p67 {
        width: 400.02px !important;
      }

      .u-row .u-col-100 {
        width: 600px !important;
      }

    }

    @media (max-width: 620px) {
      .u-row-container {
        max-width: 100% !important;
        padding-left: 0px !important;
        padding-right: 0px !important;
      }

      .u-row .u-col {
        min-width: 320px !important;
        max-width: 100% !important;
        display: block !important;
      }

      .u-row {
        width: calc(100% - 40px) !important;
      }

      .u-col {
        width: 100% !important;
      }

      .u-col > div {
        margin: 0 auto;
      }

      .no-stack .u-col {
        min-width: 0 !important;
        display: table-cell !important;
      }

      .no-stack .u-col-50 {
        width: 50% !important;
      }

    }

    body {
      margin: 0;
      padding: 0;
    }

    table,
    tr,
    td {
      vertical-align: top;
      border-collapse: collapse;
    }

    p {
      margin: 0;
    }

    .ie-container table,
    .mso-container table {
      table-layout: fixed;
    }

    * {
      line-height: inherit;
    }

    a[x-apple-data-detectors='true'] {
      color: inherit !important;
      text-decoration: none !important;
    }

    @media (max-width: 480px) {
      .hide-mobile {
        display: none !important;
        max-height: 0px;
        overflow: hidden;
      }

    }
  </style>


  <!--[if !mso]><!-->
  <link href="https://fonts.googleapis.com/css?family=Open+Sans:400,700&display=swap" rel="stylesheet" type="text/css">
  <!--<![endif]-->

</head>

{{$url := .Url}}
{{$symbol := .Symbol}}

<body class="clean-body" style="margin: 0;padding: 0;-webkit-text-size-adjust: 100%;background-color: #eeeeee">
<!--[if IE]>
<div class="ie-container"><![endif]-->
<!--[if mso]>
<div class="mso-container"><![endif]-->
<table
  style="border-collapse: collapse;table-layout: fixed;border-spacing: 0;mso-table-lspace: 0pt;mso-table-rspace: 0pt;vertical-align: top;min-width: 320px;Margin: 0 auto;background-color: #eeeeee;width:100%"
  cellpadding="0" cellspacing="0">
  <tbody>
  <tr style="vertical-align: top">
    <td style="word-break: break-word;border-collapse: collapse !important;vertical-align: top">
      <!--[if (mso)|(IE)]>
      <table width="100%" cellpadding="0" cellspacing="0" border="0">
        <tr>
          <td align="center" style="background-color: #eeeeee;"><![endif]-->


      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;">
          <div
            style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;border-bottom: 1px solid lightgray;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style=""><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="200"
                style="width: 200px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-33p33"
                 style="max-width: 320px;min-width: 200px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table id="u_content_image_1" style="font-family:'Open Sans',sans-serif;" role="presentation"
                         cellpadding="0" cellspacing="0" width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:31px 10px 25px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <table width="100%" cellpadding="0" cellspacing="0" border="0">
                          <tr>
                            <td class="v-text-align" style="padding-right: 0px;padding-left: 0px;" align="center">

                              <img align="center" border="0" src="https://shop.servhost.org/img/logo.png" alt="Image"
                                   title="Image"
                                   style="outline: none;text-decoration: none;-ms-interpolation-mode: bicubic;clear: both;display: inline-block !important;border: none;height: auto;float: none;width: 84%;max-width: 151.2px;"
                                   width="151.2" class="v-src-width v-src-max-width"/>

                            </td>
                          </tr>
                        </table>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]>
            <td align="center" width="400"
                style="width: 400px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-66p67"
                 style="max-width: 320px;min-width: 400px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table class="hide-mobile" style="font-family:'Open Sans',sans-serif;" role="presentation"
                         cellpadding="0" cellspacing="0" width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <table height="0px" align="center" border="0" cellpadding="0" cellspacing="0" width="100%"
                               style="border-collapse: collapse;table-layout: fixed;border-spacing: 0;mso-table-lspace: 0pt;mso-table-rspace: 0pt;vertical-align: top;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                          <tbody>
                          <tr style="vertical-align: top">
                            <td
                              style="word-break: break-word;border-collapse: collapse !important;vertical-align: top;font-size: 0px;line-height: 0px;mso-line-height-rule: exactly;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                              <span>&#160;</span>
                            </td>
                          </tr>
                          </tbody>
                        </table>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <table id="u_content_text_1" style="font-family:'Open Sans',sans-serif;" role="presentation"
                         cellpadding="0" cellspacing="0" width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:13px 26px 16px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #ffffff; line-height: 140%; text-align: right; word-wrap: break-word;">
                          <!--p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">SHOP</span></p-->
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>


      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #ffffff;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #ffffff;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="600"
                style="width: 600px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-100"
                 style="max-width: 320px;min-width: 600px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:40px 10px 10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <table width="100%" cellpadding="0" cellspacing="0" border="0">
                          <tr>
                            <td class="v-text-align" style="padding-right: 0px;padding-left: 0px;" align="center">

                              <!--img align="center" border="0" src="images/image-1.png" alt="Image" title="Image" style="outline: none;text-decoration: none;-ms-interpolation-mode: bicubic;clear: both;display: inline-block !important;border: none;height: auto;float: none;width: 17%;max-width: 98.6px;" width="98.6" class="v-src-width v-src-max-width"/-->

                            </td>
                          </tr>
                        </table>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #47484b; line-height: 140%; text-align: center; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 140%;"><strong><span
                            style="font-size: 30px; line-height: 42px;">New order <a href="{{$url}}/admin/sales/orders/{{$item.Path}}">#{{.Order.ID}}</a> paid!</span></strong></p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:2px 40px 25px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #7a7676; line-height: 170%; text-align: left; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 170%; text-align: center;"><span
                            style="font-size: 16px; line-height: 27.2px;"></span>
                          </p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:0px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <table height="0px" align="center" border="0" cellpadding="0" cellspacing="0" width="90%"
                               style="border-collapse: collapse;table-layout: fixed;border-spacing: 0;mso-table-lspace: 0pt;mso-table-rspace: 0pt;vertical-align: top;border-top: 1px solid #BBBBBB;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                          <tbody>
                          <tr style="vertical-align: top">
                            <td
                              style="word-break: break-word;border-collapse: collapse !important;vertical-align: top;font-size: 0px;line-height: 0px;mso-line-height-rule: exactly;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                              <span>&#160;</span>
                            </td>
                          </tr>
                          </tbody>
                        </table>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>


      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #ffffff;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #ffffff;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="300"
                style="width: 300px;padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-50"
                 style="max-width: 320px;min-width: 300px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table id="u_content_text_14" style="font-family:'Open Sans',sans-serif;" role="presentation"
                         cellpadding="0" cellspacing="0" width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #615e5e; line-height: 140%; text-align: left; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 140%;"><strong><span
                            style="font-size: 14px; line-height: 19.6px;">ITEMS</span></strong></p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]>
            <td align="center" width="300"
                style="width: 300px;padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-50"
                 style="max-width: 320px;min-width: 300px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table id="u_content_text_15" style="font-family:'Open Sans',sans-serif;" role="presentation"
                         cellpadding="0" cellspacing="0" width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #615e5e; line-height: 140%; text-align: right; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 140%;"><strong><span
                            style="font-size: 14px; line-height: 19.6px;">#{{.Order.ID}}</span></strong></p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>


      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #ffffff;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #ffffff;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="600"
                style="width: 600px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-100"
                 style="max-width: 320px;min-width: 600px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:0px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <table height="0px" align="center" border="0" cellpadding="0" cellspacing="0" width="90%"
                               style="border-collapse: collapse;table-layout: fixed;border-spacing: 0;mso-table-lspace: 0pt;mso-table-rspace: 0pt;vertical-align: top;border-top: 1px solid #BBBBBB;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                          <tbody>
                          <tr style="vertical-align: top">
                            <td
                              style="word-break: break-word;border-collapse: collapse !important;vertical-align: top;font-size: 0px;line-height: 0px;mso-line-height-rule: exactly;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                              <span>&#160;</span>
                            </td>
                          </tr>
                          </tbody>
                        </table>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>

      {{range $index, $item := .Order.Items}}

      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row no-stack"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #ffffff;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #ffffff;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="300"
                style="width: 300px;padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-50"
                 style="max-width: 320px;min-width: 300px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #615e5e; line-height: 140%; text-align: left; word-wrap: break-word;">
                          {{if $item.Thumbnail}}
                          {{$arr := split $item.Thumbnail ","}}
                          <p style="font-size: 14px; line-height: 140%;">
                          <img align="center" border="0" src="{{absolute $url (index $arr 0)}}" alt="Image"
                               title="Image"
                               style="outline: none;text-decoration: none;-ms-interpolation-mode: bicubic;clear: both;display: inline-block !important;border: none;height: auto;float: none;width: 84%;max-width: 151.2px;"
                               width="151.2" class="v-src-width v-src-max-width"/>
                          </p>
                          {{ end }}
                          <p style="font-size: 14px; line-height: 140%;">
                            <a href="{{$url}}{{$item.Path}}?uuid={{toUuid $item.Uuid}}">
                              <span style="font-size: 14px; line-height: 19.6px;">
                              {{.Title}}
                              </span>
                            </a>
                            <ul style="padding: 0 20px;">{{range $item.Properties}}<li><span>{{.Title}}:&nbsp;</span><span>{{.Value}}</span></li>{{end}}</ul>
                          </p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]>
            <td align="center" width="300"
                style="width: 300px;padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-50"
                 style="max-width: 320px;min-width: 300px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #615e5e; line-height: 140%; text-align: right; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 140%;">
                            <span style="font-size: 14px; line-height: 19.6px;">
                              {{$symbol}}{{printf "%.2f" $item.Rate}} x {{$item.Quantity}} = {{$symbol}}{{printf "%.2f" $item.Total}}
                            </span>
                          </p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>

      {{end}}

      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #ffffff;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #ffffff;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="600"
                style="width: 600px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-100"
                 style="max-width: 320px;min-width: 600px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:0px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <table height="0px" align="center" border="0" cellpadding="0" cellspacing="0" width="90%"
                               style="border-collapse: collapse;table-layout: fixed;border-spacing: 0;mso-table-lspace: 0pt;mso-table-rspace: 0pt;vertical-align: top;border-top: 1px solid #BBBBBB;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                          <tbody>
                          <tr style="vertical-align: top">
                            <td
                              style="word-break: break-word;border-collapse: collapse !important;vertical-align: top;font-size: 0px;line-height: 0px;mso-line-height-rule: exactly;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                              <span>&#160;</span>
                            </td>
                          </tr>
                          </tbody>
                        </table>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>

      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row no-stack"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #ffffff;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #ffffff;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="300"
                style="width: 300px;padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-50"
                 style="max-width: 320px;min-width: 300px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #615e5e; line-height: 140%; text-align: left; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 140%;"><strong><span
                            style="font-size: 14px; line-height: 19.6px;">Shipping</span></strong></p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]>
            <td align="center" width="300"
                style="width: 300px;padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-50"
                 style="max-width: 320px;min-width: 300px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #615e5e; line-height: 140%; text-align: right; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 140%;"><strong><span
                            style="font-size: 14px; line-height: 19.6px;">{{$symbol}}{{printf "%.2f" .Order.Delivery}}</span></strong></p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>


      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #ffffff;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #ffffff;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="600"
                style="width: 600px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-100"
                 style="max-width: 320px;min-width: 600px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:0px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <table height="0px" align="center" border="0" cellpadding="0" cellspacing="0" width="90%"
                               style="border-collapse: collapse;table-layout: fixed;border-spacing: 0;mso-table-lspace: 0pt;mso-table-rspace: 0pt;vertical-align: top;border-top: 1px solid #BBBBBB;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                          <tbody>
                          <tr style="vertical-align: top">
                            <td
                              style="word-break: break-word;border-collapse: collapse !important;vertical-align: top;font-size: 0px;line-height: 0px;mso-line-height-rule: exactly;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                              <span>&#160;</span>
                            </td>
                          </tr>
                          </tbody>
                        </table>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>


      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row no-stack"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #ffffff;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #ffffff;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="300"
                style="width: 300px;padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-50"
                 style="max-width: 320px;min-width: 300px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #615e5e; line-height: 140%; text-align: left; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 140%;"><strong><span
                            style="font-size: 14px; line-height: 19.6px;">Total</span></strong></p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]>
            <td align="center" width="300"
                style="width: 300px;padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-50"
                 style="max-width: 320px;min-width: 300px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #615e5e; line-height: 140%; text-align: right; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 140%;"><strong><span
                            style="font-size: 14px; line-height: 19.6px;">{{$symbol}}{{printf "%.2f" .Order.Total}}</span></strong></p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>


      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #ffffff;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #ffffff;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="600"
                style="width: 600px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-100"
                 style="max-width: 320px;min-width: 600px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:0px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <table height="0px" align="center" border="0" cellpadding="0" cellspacing="0" width="90%"
                               style="border-collapse: collapse;table-layout: fixed;border-spacing: 0;mso-table-lspace: 0pt;mso-table-rspace: 0pt;vertical-align: top;border-top: 1px solid #BBBBBB;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                          <tbody>
                          <tr style="vertical-align: top">
                            <td
                              style="word-break: break-word;border-collapse: collapse !important;vertical-align: top;font-size: 0px;line-height: 0px;mso-line-height-rule: exactly;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                              <span>&#160;</span>
                            </td>
                          </tr>
                          </tbody>
                        </table>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>


      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #ffffff;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #ffffff;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="600"
                style="width: 600px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-100"
                 style="max-width: 320px;min-width: 600px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:14px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <table height="0px" align="center" border="0" cellpadding="0" cellspacing="0" width="90%"
                               style="border-collapse: collapse;table-layout: fixed;border-spacing: 0;mso-table-lspace: 0pt;mso-table-rspace: 0pt;vertical-align: top;border-top: 1px solid #ffffff;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                          <tbody>
                          <tr style="vertical-align: top">
                            <td
                              style="word-break: break-word;border-collapse: collapse !important;vertical-align: top;font-size: 0px;line-height: 0px;mso-line-height-rule: exactly;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                              <span>&#160;</span>
                            </td>
                          </tr>
                          </tbody>
                        </table>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>


      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #ffffff;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #ffffff;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="300"
                style="width: 300px;padding: 0px 30px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-50"
                 style="max-width: 320px;min-width: 300px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px 30px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #5c5757; line-height: 140%; text-align: left; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 140%;"><strong><span
                            style="font-size: 14px; line-height: 19.6px;">Billing:</span></strong></p>
                          <p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">{{.Order.Billing.Profile.Name}}&nbsp;{{.Order.Billing.Profile.Lastname}}</span></p>
                          <p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">{{.Order.Billing.Profile.Address}}&nbsp;{{.Order.Billing.Profile.City}}&nbsp;{{.Order.Billing.Profile.Country}}</span></p>
                          <p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">ZIP: {{.Order.Billing.Profile.Zip}}</span></p>
                          <p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">Payment: {{.Order.Billing.Title}}</span></p>
                          <p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">Method: {{.Order.Billing.Method}}</span></p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]>
            <td align="center" width="300"
                style="width: 300px;padding: 0px 30px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-50"
                 style="max-width: 320px;min-width: 300px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px 30px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #5c5757; line-height: 140%; text-align: left; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 140%;"><strong><span
                            style="font-size: 14px; line-height: 19.6px;">Shipping:</span></strong></p>
                          <p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">{{.Order.Shipping.Profile.Name}}&nbsp;{{.Order.Shipping.Profile.Lastname}}</span></p>
                          <p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">{{.Order.Shipping.Profile.Address}}&nbsp;{{.Order.Shipping.Profile.City}}&nbsp;{{.Order.Shipping.Profile.Country}}</span></p>
                          <p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">ZIP: {{.Order.Shipping.Profile.Zip}}</span></p>
                          <p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">Transport: {{.Order.Shipping.Title}}</span></p>
                          {{if .Order.Shipping.Services}}
                          <p style="font-size: 14px; line-height: 140%;">
                            <span style="font-size: 14px; line-height: 19.6px;">Services:
                            {{range .Order.Shipping.Services}}{{.Title}}{{end}}
                            </span>
                          </p>
                          {{end}}
                          <p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">Volume: {{.Order.Volume}}</span></p>
                          <p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">Weight: {{.Order.Weight}}</span></p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>


      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #689a8c;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #689a8c;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="600"
                style="width: 600px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-100"
                 style="max-width: 320px;min-width: 600px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:16px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #ecf7ff; line-height: 140%; text-align: center; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 140%;">&copy; Shop. All Rights Reserved</p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>


      <!--[if (mso)|(IE)]></td></tr></table><![endif]-->
    </td>
  </tr>
  </tbody>
</table>
<!--[if mso]></div><![endif]-->
<!--[if IE]></div><![endif]-->
</body>

</html>
`,
					}); err != nil {
						logger.Warningf("%v", err)
					}
					// User
					if _, err = models.CreateEmailTemplate(common.Database, &models.EmailTemplate{
						Enabled: false,
						Type:    common.NOTIFICATION_TYPE_RESET_PASSWORD,
						Topic:   "Reset Password",
						Message: `<!DOCTYPE HTML PUBLIC "-//W3C//DTD XHTML 1.0 Transitional //EN"
  "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:v="urn:schemas-microsoft-com:vml"
      xmlns:o="urn:schemas-microsoft-com:office:office">
<head>
  <!--[if gte mso 9]>
  <xml>
    <o:OfficeDocumentSettings>
      <o:AllowPNG/>
      <o:PixelsPerInch>96</o:PixelsPerInch>
    </o:OfficeDocumentSettings>
  </xml>
  <![endif]-->
  <meta http-equiv="Content-Type" content="text/html; charset=UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta name="x-apple-disable-message-reformatting">
  <!--[if !mso]><!-->
  <meta http-equiv="X-UA-Compatible" content="IE=edge"><!--<![endif]-->
  <title>Order #{{.Order.ID}}</title>

  <style type="text/css">
    a {
      color: #6d6d6d;
      text-decoration: none;
    }

    @media (max-width: 480px) {
      #u_content_image_1 .v-src-width {
        width: auto !important;
      }

      #u_content_image_1 .v-src-max-width {
        max-width: 50% !important;
      }

      #u_content_text_1 .v-text-align {
        text-align: center !important;
      }

      #u_content_text_14 .v-text-align {
        text-align: center !important;
      }

      #u_content_text_15 .v-text-align {
        text-align: center !important;
      }
    }

    @media only screen and (min-width: 620px) {
      .u-row {
        width: 600px !important;
      }

      .u-row .u-col {
        vertical-align: top;
      }

      .u-row .u-col-33p33 {
        width: 199.98px !important;
      }

      .u-row .u-col-50 {
        width: 300px !important;
      }

      .u-row .u-col-66p67 {
        width: 400.02px !important;
      }

      .u-row .u-col-100 {
        width: 600px !important;
      }

    }

    @media (max-width: 620px) {
      .u-row-container {
        max-width: 100% !important;
        padding-left: 0px !important;
        padding-right: 0px !important;
      }

      .u-row .u-col {
        min-width: 320px !important;
        max-width: 100% !important;
        display: block !important;
      }

      .u-row {
        width: calc(100% - 40px) !important;
      }

      .u-col {
        width: 100% !important;
      }

      .u-col > div {
        margin: 0 auto;
      }

      .no-stack .u-col {
        min-width: 0 !important;
        display: table-cell !important;
      }

      .no-stack .u-col-50 {
        width: 50% !important;
      }

    }

    body {
      margin: 0;
      padding: 0;
    }

    table,
    tr,
    td {
      vertical-align: top;
      border-collapse: collapse;
    }

    p {
      margin: 0;
    }

    .ie-container table,
    .mso-container table {
      table-layout: fixed;
    }

    * {
      line-height: inherit;
    }

    a[x-apple-data-detectors='true'] {
      color: inherit !important;
      text-decoration: none !important;
    }

    @media (max-width: 480px) {
      .hide-mobile {
        display: none !important;
        max-height: 0px;
        overflow: hidden;
      }

    }
  </style>


  <!--[if !mso]><!-->
  <link href="https://fonts.googleapis.com/css?family=Open+Sans:400,700&display=swap" rel="stylesheet" type="text/css">
  <!--<![endif]-->

</head>

{{$url := .Url}}
{{$symbol := .Symbol}}

<body class="clean-body" style="margin: 0;padding: 0;-webkit-text-size-adjust: 100%;background-color: #eeeeee">
<!--[if IE]>
<div class="ie-container"><![endif]-->
<!--[if mso]>
<div class="mso-container"><![endif]-->
<table
  style="border-collapse: collapse;table-layout: fixed;border-spacing: 0;mso-table-lspace: 0pt;mso-table-rspace: 0pt;vertical-align: top;min-width: 320px;Margin: 0 auto;background-color: #eeeeee;width:100%"
  cellpadding="0" cellspacing="0">
  <tbody>
  <tr style="vertical-align: top">
    <td style="word-break: break-word;border-collapse: collapse !important;vertical-align: top">
      <!--[if (mso)|(IE)]>
      <table width="100%" cellpadding="0" cellspacing="0" border="0">
        <tr>
          <td align="center" style="background-color: #eeeeee;"><![endif]-->


      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;">
          <div
            style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;border-bottom: 1px solid lightgray;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style=""><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="200"
                style="width: 200px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-33p33"
                 style="max-width: 320px;min-width: 200px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table id="u_content_image_1" style="font-family:'Open Sans',sans-serif;" role="presentation"
                         cellpadding="0" cellspacing="0" width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:31px 10px 25px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <table width="100%" cellpadding="0" cellspacing="0" border="0">
                          <tr>
                            <td class="v-text-align" style="padding-right: 0px;padding-left: 0px;" align="center">

                              <img align="center" border="0" src="https://shop.servhost.org/img/logo.png" alt="Image"
                                   title="Image"
                                   style="outline: none;text-decoration: none;-ms-interpolation-mode: bicubic;clear: both;display: inline-block !important;border: none;height: auto;float: none;width: 84%;max-width: 151.2px;"
                                   width="151.2" class="v-src-width v-src-max-width"/>

                            </td>
                          </tr>
                        </table>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]>
            <td align="center" width="400"
                style="width: 400px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-66p67"
                 style="max-width: 320px;min-width: 400px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table class="hide-mobile" style="font-family:'Open Sans',sans-serif;" role="presentation"
                         cellpadding="0" cellspacing="0" width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <table height="0px" align="center" border="0" cellpadding="0" cellspacing="0" width="100%"
                               style="border-collapse: collapse;table-layout: fixed;border-spacing: 0;mso-table-lspace: 0pt;mso-table-rspace: 0pt;vertical-align: top;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                          <tbody>
                          <tr style="vertical-align: top">
                            <td
                              style="word-break: break-word;border-collapse: collapse !important;vertical-align: top;font-size: 0px;line-height: 0px;mso-line-height-rule: exactly;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                              <span>&#160;</span>
                            </td>
                          </tr>
                          </tbody>
                        </table>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <table id="u_content_text_1" style="font-family:'Open Sans',sans-serif;" role="presentation"
                         cellpadding="0" cellspacing="0" width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:13px 26px 16px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #ffffff; line-height: 140%; text-align: right; word-wrap: break-word;">
                          <!--p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">SHOP</span></p-->
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>


      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #ffffff;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #ffffff;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="600"
                style="width: 600px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-100"
                 style="max-width: 320px;min-width: 600px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:40px 10px 10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <table width="100%" cellpadding="0" cellspacing="0" border="0">
                          <tr>
                            <td class="v-text-align" style="padding-right: 0px;padding-left: 0px;" align="center">

                              <!--img align="center" border="0" src="images/image-1.png" alt="Image" title="Image" style="outline: none;text-decoration: none;-ms-interpolation-mode: bicubic;clear: both;display: inline-block !important;border: none;height: auto;float: none;width: 17%;max-width: 98.6px;" width="98.6" class="v-src-width v-src-max-width"/-->

                            </td>
                          </tr>
                        </table>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #47484b; line-height: 140%; text-align: center; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 140%;"><strong><span
                            style="font-size: 30px; line-height: 42px;">Thanks for your order!</span></strong></p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:2px 40px 25px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #7a7676; line-height: 170%; text-align: left; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 170%; text-align: center;"><span
                            style="font-size: 16px; line-height: 27.2px;"></span>
                          </p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:0px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <table height="0px" align="center" border="0" cellpadding="0" cellspacing="0" width="90%"
                               style="border-collapse: collapse;table-layout: fixed;border-spacing: 0;mso-table-lspace: 0pt;mso-table-rspace: 0pt;vertical-align: top;border-top: 1px solid #BBBBBB;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                          <tbody>
                          <tr style="vertical-align: top">
                            <td
                              style="word-break: break-word;border-collapse: collapse !important;vertical-align: top;font-size: 0px;line-height: 0px;mso-line-height-rule: exactly;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                              <span>&#160;</span>
                            </td>
                          </tr>
                          </tbody>
                        </table>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>


      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #ffffff;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #ffffff;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="300"
                style="width: 300px;padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-50"
                 style="max-width: 320px;min-width: 300px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table id="u_content_text_14" style="font-family:'Open Sans',sans-serif;" role="presentation"
                         cellpadding="0" cellspacing="0" width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #615e5e; line-height: 140%; text-align: left; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 140%;"><strong><span
                            style="font-size: 14px; line-height: 19.6px;">ITEMS</span></strong></p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]>
            <td align="center" width="300"
                style="width: 300px;padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-50"
                 style="max-width: 320px;min-width: 300px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table id="u_content_text_15" style="font-family:'Open Sans',sans-serif;" role="presentation"
                         cellpadding="0" cellspacing="0" width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #615e5e; line-height: 140%; text-align: right; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 140%;"><strong><span
                            style="font-size: 14px; line-height: 19.6px;">#{{.Order.ID}}</span></strong></p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>


      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #ffffff;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #ffffff;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="600"
                style="width: 600px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-100"
                 style="max-width: 320px;min-width: 600px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:0px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <table height="0px" align="center" border="0" cellpadding="0" cellspacing="0" width="90%"
                               style="border-collapse: collapse;table-layout: fixed;border-spacing: 0;mso-table-lspace: 0pt;mso-table-rspace: 0pt;vertical-align: top;border-top: 1px solid #BBBBBB;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                          <tbody>
                          <tr style="vertical-align: top">
                            <td
                              style="word-break: break-word;border-collapse: collapse !important;vertical-align: top;font-size: 0px;line-height: 0px;mso-line-height-rule: exactly;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                              <span>&#160;</span>
                            </td>
                          </tr>
                          </tbody>
                        </table>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>

      {{range $index, $item := .Order.Items}}

      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row no-stack"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #ffffff;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #ffffff;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="300"
                style="width: 300px;padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-50"
                 style="max-width: 320px;min-width: 300px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #615e5e; line-height: 140%; text-align: left; word-wrap: break-word;">
                          {{if $item.Thumbnail}}
                          {{$arr := split $item.Thumbnail ","}}
                          <p style="font-size: 14px; line-height: 140%;">
                          <img align="center" border="0" src="{{absolute $url (index $arr 0)}}" alt="Image"
                               title="Image"
                               style="outline: none;text-decoration: none;-ms-interpolation-mode: bicubic;clear: both;display: inline-block !important;border: none;height: auto;float: none;width: 84%;max-width: 151.2px;"
                               width="151.2" class="v-src-width v-src-max-width"/>
                          </p>
                          {{ end }}
                          <p style="font-size: 14px; line-height: 140%;">
                            <a href="{{$url}}{{$item.Path}}?uuid={{toUuid $item.Uuid}}">
                              <span style="font-size: 14px; line-height: 19.6px;">
                              {{.Title}}
                              </span>
                            </a>
                            <ul style="padding: 0 20px;">{{range $item.Properties}}<li><span>{{.Title}}:&nbsp;</span><span>{{.Value}}</span></li>{{end}}</ul>
                          </p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]>
            <td align="center" width="300"
                style="width: 300px;padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-50"
                 style="max-width: 320px;min-width: 300px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #615e5e; line-height: 140%; text-align: right; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 140%;">
                            <span style="font-size: 14px; line-height: 19.6px;">
                              {{$symbol}}{{printf "%.2f" $item.Rate}} x {{$item.Quantity}} = {{$symbol}}{{printf "%.2f" $item.Total}}
                            </span>
                          </p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>

      {{end}}

      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #ffffff;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #ffffff;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="600"
                style="width: 600px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-100"
                 style="max-width: 320px;min-width: 600px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:0px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <table height="0px" align="center" border="0" cellpadding="0" cellspacing="0" width="90%"
                               style="border-collapse: collapse;table-layout: fixed;border-spacing: 0;mso-table-lspace: 0pt;mso-table-rspace: 0pt;vertical-align: top;border-top: 1px solid #BBBBBB;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                          <tbody>
                          <tr style="vertical-align: top">
                            <td
                              style="word-break: break-word;border-collapse: collapse !important;vertical-align: top;font-size: 0px;line-height: 0px;mso-line-height-rule: exactly;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                              <span>&#160;</span>
                            </td>
                          </tr>
                          </tbody>
                        </table>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>

      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row no-stack"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #ffffff;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #ffffff;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="300"
                style="width: 300px;padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-50"
                 style="max-width: 320px;min-width: 300px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #615e5e; line-height: 140%; text-align: left; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 140%;"><strong><span
                            style="font-size: 14px; line-height: 19.6px;">Shipping</span></strong></p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]>
            <td align="center" width="300"
                style="width: 300px;padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-50"
                 style="max-width: 320px;min-width: 300px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #615e5e; line-height: 140%; text-align: right; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 140%;"><strong><span
                            style="font-size: 14px; line-height: 19.6px;">{{$symbol}}{{printf "%.2f" .Order.Delivery}}</span></strong></p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>


      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #ffffff;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #ffffff;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="600"
                style="width: 600px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-100"
                 style="max-width: 320px;min-width: 600px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:0px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <table height="0px" align="center" border="0" cellpadding="0" cellspacing="0" width="90%"
                               style="border-collapse: collapse;table-layout: fixed;border-spacing: 0;mso-table-lspace: 0pt;mso-table-rspace: 0pt;vertical-align: top;border-top: 1px solid #BBBBBB;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                          <tbody>
                          <tr style="vertical-align: top">
                            <td
                              style="word-break: break-word;border-collapse: collapse !important;vertical-align: top;font-size: 0px;line-height: 0px;mso-line-height-rule: exactly;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                              <span>&#160;</span>
                            </td>
                          </tr>
                          </tbody>
                        </table>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>


      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row no-stack"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #ffffff;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #ffffff;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="300"
                style="width: 300px;padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-50"
                 style="max-width: 320px;min-width: 300px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #615e5e; line-height: 140%; text-align: left; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 140%;"><strong><span
                            style="font-size: 14px; line-height: 19.6px;">Total</span></strong></p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]>
            <td align="center" width="300"
                style="width: 300px;padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-50"
                 style="max-width: 320px;min-width: 300px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #615e5e; line-height: 140%; text-align: right; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 140%;"><strong><span
                            style="font-size: 14px; line-height: 19.6px;">{{$symbol}}{{printf "%.2f" .Order.Total}}</span></strong></p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>


      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #ffffff;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #ffffff;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="600"
                style="width: 600px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-100"
                 style="max-width: 320px;min-width: 600px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:0px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <table height="0px" align="center" border="0" cellpadding="0" cellspacing="0" width="90%"
                               style="border-collapse: collapse;table-layout: fixed;border-spacing: 0;mso-table-lspace: 0pt;mso-table-rspace: 0pt;vertical-align: top;border-top: 1px solid #BBBBBB;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                          <tbody>
                          <tr style="vertical-align: top">
                            <td
                              style="word-break: break-word;border-collapse: collapse !important;vertical-align: top;font-size: 0px;line-height: 0px;mso-line-height-rule: exactly;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                              <span>&#160;</span>
                            </td>
                          </tr>
                          </tbody>
                        </table>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>


      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #ffffff;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #ffffff;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="600"
                style="width: 600px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-100"
                 style="max-width: 320px;min-width: 600px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:14px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <table height="0px" align="center" border="0" cellpadding="0" cellspacing="0" width="90%"
                               style="border-collapse: collapse;table-layout: fixed;border-spacing: 0;mso-table-lspace: 0pt;mso-table-rspace: 0pt;vertical-align: top;border-top: 1px solid #ffffff;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                          <tbody>
                          <tr style="vertical-align: top">
                            <td
                              style="word-break: break-word;border-collapse: collapse !important;vertical-align: top;font-size: 0px;line-height: 0px;mso-line-height-rule: exactly;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                              <span>&#160;</span>
                            </td>
                          </tr>
                          </tbody>
                        </table>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>


      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #ffffff;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #ffffff;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="300"
                style="width: 300px;padding: 0px 30px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-50"
                 style="max-width: 320px;min-width: 300px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px 30px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #5c5757; line-height: 140%; text-align: left; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 140%;"><strong><span
                            style="font-size: 14px; line-height: 19.6px;">Billing:</span></strong></p>
                          <p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">{{.Order.Billing.Profile.Name}}&nbsp;{{.Order.Billing.Profile.Lastname}}</span></p>
                          <p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">{{.Order.Billing.Profile.Address}}&nbsp;{{.Order.Billing.Profile.City}}&nbsp;{{.Order.Billing.Profile.Country}}</span></p>
                          <p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">ZIP: {{.Order.Billing.Profile.Zip}}</span></p>
                          <p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">Payment: {{.Order.Billing.Title}}</span></p>
                          <p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">Method: {{.Order.Billing.Method}}</span></p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]>
            <td align="center" width="300"
                style="width: 300px;padding: 0px 30px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-50"
                 style="max-width: 320px;min-width: 300px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px 30px 30px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #5c5757; line-height: 140%; text-align: left; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 140%;"><strong><span
                            style="font-size: 14px; line-height: 19.6px;">Shipping:</span></strong></p>
                          <p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">{{.Order.Shipping.Profile.Name}}&nbsp;{{.Order.Shipping.Profile.Lastname}}</span></p>
                          <p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">{{.Order.Shipping.Profile.Address}}&nbsp;{{.Order.Shipping.Profile.City}}&nbsp;{{.Order.Shipping.Profile.Country}}</span></p>
                          <p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">ZIP: {{.Order.Shipping.Profile.Zip}}</span></p>
                          <p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">Transport: {{.Order.Shipping.Title}}</span></p>
                          {{if .Order.Shipping.Services}}
                          <p style="font-size: 14px; line-height: 140%;">
                            <span style="font-size: 14px; line-height: 19.6px;">Services:
                            {{range .Order.Shipping.Services}}{{.Title}}{{end}}
                            </span>
                          </p>
                          {{end}}
                          <p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">Volume: {{.Order.Volume}}</span></p>
                          <p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">Weight: {{.Order.Weight}}</span></p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>


      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #689a8c;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #689a8c;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="600"
                style="width: 600px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-100"
                 style="max-width: 320px;min-width: 600px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:16px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #ecf7ff; line-height: 140%; text-align: center; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 140%;">&copy; Shop. All Rights Reserved</p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>


      <!--[if (mso)|(IE)]></td></tr></table><![endif]-->
    </td>
  </tr>
  </tbody>
</table>
<!--[if mso]></div><![endif]-->
<!--[if IE]></div><![endif]-->
</body>

</html>`,
					}); err != nil {
						logger.Warningf("%v", err)
					}
					// Reset password
					if _, err = models.CreateEmailTemplate(common.Database, &models.EmailTemplate{
						Enabled: false,
						Type:    common.NOTIFICATION_TYPE_RESET_PASSWORD,
						Topic:   "Reset password",
						Message: `<!DOCTYPE HTML PUBLIC "-//W3C//DTD XHTML 1.0 Transitional //EN"
        "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:v="urn:schemas-microsoft-com:vml"
      xmlns:o="urn:schemas-microsoft-com:office:office">
<head>
    <!--[if gte mso 9]>
    <xml>
        <o:OfficeDocumentSettings>
            <o:AllowPNG/>
            <o:PixelsPerInch>96</o:PixelsPerInch>
        </o:OfficeDocumentSettings>
    </xml>
    <![endif]-->
    <meta http-equiv="Content-Type" content="text/html; charset=UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta name="x-apple-disable-message-reformatting">
    <!--[if !mso]><!-->
    <meta http-equiv="X-UA-Compatible" content="IE=edge"><!--<![endif]-->
    <title>Reset Password</title>

    <style type="text/css">
        a {
            color: #6d6d6d;
            text-decoration: none;
        }

        @media (max-width: 480px) {
            #u_content_image_1 .v-src-width {
                width: auto !important;
            }

            #u_content_image_1 .v-src-max-width {
                max-width: 50% !important;
            }

            #u_content_text_1 .v-text-align {
                text-align: center !important;
            }

            #u_content_text_14 .v-text-align {
                text-align: center !important;
            }

            #u_content_text_15 .v-text-align {
                text-align: center !important;
            }
        }

        @media only screen and (min-width: 620px) {
            .u-row {
                width: 600px !important;
            }

            .u-row .u-col {
                vertical-align: top;
            }

            .u-row .u-col-33p33 {
                width: 199.98px !important;
            }

            .u-row .u-col-50 {
                width: 300px !important;
            }

            .u-row .u-col-66p67 {
                width: 400.02px !important;
            }

            .u-row .u-col-100 {
                width: 600px !important;
            }

        }

        @media (max-width: 620px) {
            .u-row-container {
                max-width: 100% !important;
                padding-left: 0px !important;
                padding-right: 0px !important;
            }

            .u-row .u-col {
                min-width: 320px !important;
                max-width: 100% !important;
                display: block !important;
            }

            .u-row {
                width: calc(100% - 40px) !important;
            }

            .u-col {
                width: 100% !important;
            }

            .u-col > div {
                margin: 0 auto;
            }

            .no-stack .u-col {
                min-width: 0 !important;
                display: table-cell !important;
            }

            .no-stack .u-col-50 {
                width: 50% !important;
            }

        }

        body {
            margin: 0;
            padding: 0;
        }

        table,
        tr,
        td {
            vertical-align: top;
            border-collapse: collapse;
        }

        p {
            margin: 0;
        }

        .ie-container table,
        .mso-container table {
            table-layout: fixed;
        }

        * {
            line-height: inherit;
        }

        a[x-apple-data-detectors='true'] {
            color: inherit !important;
            text-decoration: none !important;
        }

        @media (max-width: 480px) {
            .hide-mobile {
                display: none !important;
                max-height: 0px;
                overflow: hidden;
            }

        }
    </style>


    <!--[if !mso]><!-->
    <link href="https://fonts.googleapis.com/css?family=Open+Sans:400,700&display=swap" rel="stylesheet" type="text/css">
    <!--<![endif]-->

</head>

{{$url := .Url}}
{{$symbol := .Symbol}}

<body class="clean-body" style="margin: 0;padding: 0;-webkit-text-size-adjust: 100%;background-color: #eeeeee">
<!--[if IE]>
<div class="ie-container"><![endif]-->
<!--[if mso]>
<div class="mso-container"><![endif]-->
<table
        style="border-collapse: collapse;table-layout: fixed;border-spacing: 0;mso-table-lspace: 0pt;mso-table-rspace: 0pt;vertical-align: top;min-width: 320px;Margin: 0 auto;background-color: #eeeeee;width:100%"
        cellpadding="0" cellspacing="0">
    <tbody>
    <tr style="vertical-align: top">
        <td style="word-break: break-word;border-collapse: collapse !important;vertical-align: top">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
                <tr>
                    <td align="center" style="background-color: #eeeeee;"><![endif]-->


            <div class="u-row-container" style="padding: 0px;background-color: transparent">
                <div class="u-row"
                     style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;">
                    <div
                            style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;border-bottom: 1px solid lightgray;">
                        <!--[if (mso)|(IE)]>
                        <table width="100%" cellpadding="0" cellspacing="0" border="0">
                            <tr>
                                <td style="padding: 0px;background-color: transparent;" align="center">
                                    <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                                        <tr style=""><![endif]-->

                        <!--[if (mso)|(IE)]>
                        <td align="center" width="200"
                            style="width: 200px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                            valign="top"><![endif]-->
                        <div class="u-col u-col-33p33"
                             style="max-width: 320px;min-width: 200px;display: table-cell;vertical-align: top;">
                            <div style="width: 100% !important;">
                                <!--[if (!mso)&(!IE)]><!-->
                                <div
                                        style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                                    <!--<![endif]-->


                                    <table id="u_content_image_1" style="font-family:'Open Sans',sans-serif;" role="presentation"
                                           cellpadding="0" cellspacing="0" width="100%" border="0">
                                        <tbody>
                                        <tr>
                                            <td
                                                    style="overflow-wrap:break-word;word-break:break-word;padding:31px 10px 25px;font-family:'Open Sans',sans-serif;"
                                                    align="left">

                                                <table width="100%" cellpadding="0" cellspacing="0" border="0">
                                                    <tr>
                                                        <td class="v-text-align" style="padding-right: 0px;padding-left: 0px;" align="center">

                                                            <img align="center" border="0" src="https://shop.servhost.org/img/logo.png" alt="Image"
                                                                 title="Image"
                                                                 style="outline: none;text-decoration: none;-ms-interpolation-mode: bicubic;clear: both;display: inline-block !important;border: none;height: auto;float: none;width: 84%;max-width: 151.2px;"
                                                                 width="151.2" class="v-src-width v-src-max-width"/>

                                                        </td>
                                                    </tr>
                                                </table>

                                            </td>
                                        </tr>
                                        </tbody>
                                    </table>


                                    <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
                            </div>
                        </div>
                        <!--[if (mso)|(IE)]></td><![endif]-->
                        <!--[if (mso)|(IE)]>
                        <td align="center" width="400"
                            style="width: 400px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                            valign="top"><![endif]-->
                        <div class="u-col u-col-66p67"
                             style="max-width: 320px;min-width: 400px;display: table-cell;vertical-align: top;">
                            <div style="width: 100% !important;">
                                <!--[if (!mso)&(!IE)]><!-->
                                <div
                                        style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                                    <!--<![endif]-->


                                    <table class="hide-mobile" style="font-family:'Open Sans',sans-serif;" role="presentation"
                                           cellpadding="0" cellspacing="0" width="100%" border="0">
                                        <tbody>
                                        <tr>
                                            <td
                                                    style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                                                    align="left">

                                                <table height="0px" align="center" border="0" cellpadding="0" cellspacing="0" width="100%"
                                                       style="border-collapse: collapse;table-layout: fixed;border-spacing: 0;mso-table-lspace: 0pt;mso-table-rspace: 0pt;vertical-align: top;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                                                    <tbody>
                                                    <tr style="vertical-align: top">
                                                        <td
                                                                style="word-break: break-word;border-collapse: collapse !important;vertical-align: top;font-size: 0px;line-height: 0px;mso-line-height-rule: exactly;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                                                            <span>&#160;</span>
                                                        </td>
                                                    </tr>
                                                    </tbody>
                                                </table>

                                            </td>
                                        </tr>
                                        </tbody>
                                    </table>


                                    <table id="u_content_text_1" style="font-family:'Open Sans',sans-serif;" role="presentation"
                                           cellpadding="0" cellspacing="0" width="100%" border="0">
                                        <tbody>
                                        <tr>
                                            <td
                                                    style="overflow-wrap:break-word;word-break:break-word;padding:13px 26px 16px;font-family:'Open Sans',sans-serif;"
                                                    align="left">

                                                <div class="v-text-align"
                                                     style="color: #ffffff; line-height: 140%; text-align: right; word-wrap: break-word;">
                                                    <!--p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">SHOP</span></p-->
                                                </div>

                                            </td>
                                        </tr>
                                        </tbody>
                                    </table>


                                    <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
                            </div>
                        </div>
                        <!--[if (mso)|(IE)]></td><![endif]-->
                        <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
                    </div>
                </div>
            </div>


            <div class="u-row-container" style="padding: 0px;background-color: transparent">
                <div class="u-row"
                     style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #ffffff;">
                    <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
                        <!--[if (mso)|(IE)]>
                        <table width="100%" cellpadding="0" cellspacing="0" border="0">
                            <tr>
                                <td style="padding: 0px;background-color: transparent;" align="center">
                                    <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                                        <tr style="background-color: #ffffff;"><![endif]-->

                        <!--[if (mso)|(IE)]>
                        <td align="center" width="600"
                            style="width: 600px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                            valign="top"><![endif]-->
                        <div class="u-col u-col-100"
                             style="max-width: 320px;min-width: 600px;display: table-cell;vertical-align: top;">
                            <div style="width: 100% !important;">
                                <!--[if (!mso)&(!IE)]><!-->
                                <div
                                        style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                                    <!--<![endif]-->


                                    <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                                           width="100%" border="0">
                                        <tbody>
                                        <tr>
                                            <td
                                                    style="overflow-wrap:break-word;word-break:break-word;padding:40px 10px 10px;font-family:'Open Sans',sans-serif;"
                                                    align="left">

                                                <table width="100%" cellpadding="0" cellspacing="0" border="0">
                                                    <tr>
                                                        <td class="v-text-align" style="padding-right: 0px;padding-left: 0px;" align="center">

                                                            <!--img align="center" border="0" src="images/image-1.png" alt="Image" title="Image" style="outline: none;text-decoration: none;-ms-interpolation-mode: bicubic;clear: both;display: inline-block !important;border: none;height: auto;float: none;width: 17%;max-width: 98.6px;" width="98.6" class="v-src-width v-src-max-width"/-->

                                                        </td>
                                                    </tr>
                                                </table>

                                            </td>
                                        </tr>
                                        </tbody>
                                    </table>


                                    <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                                           width="100%" border="0">
                                        <tbody>
                                        <tr>
                                            <td
                                                    style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                                                    align="left">

                                                <div class="v-text-align"
                                                     style="color: #47484b; line-height: 140%; text-align: center; word-wrap: break-word;">
                                                    <p style="font-size: 14px; line-height: 140%;"><strong><span
                                                            style="font-size: 30px; line-height: 42px;">Reset password</span></strong></p>
                                                    <p>Follow <a href="{{$url}}/reset_password?code={{.Code}}" style="text-decoration: underline">this link</a> to reset password or ignore this message</p>
                                                </div>

                                            </td>
                                        </tr>
                                        </tbody>
                                    </table>


                                    <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                                           width="100%" border="0">
                                        <tbody>
                                        <tr>
                                            <td
                                                    style="overflow-wrap:break-word;word-break:break-word;padding:2px 40px 25px;font-family:'Open Sans',sans-serif;"
                                                    align="left">

                                                <div class="v-text-align"
                                                     style="color: #7a7676; line-height: 170%; text-align: left; word-wrap: break-word;">
                                                    <p style="font-size: 14px; line-height: 170%; text-align: center;"><span
                                                            style="font-size: 16px; line-height: 27.2px;"></span>
                                                    </p>
                                                </div>

                                            </td>
                                        </tr>
                                        </tbody>
                                    </table>


                                    <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                                           width="100%" border="0">
                                        <tbody>
                                        <tr>
                                            <td
                                                    style="overflow-wrap:break-word;word-break:break-word;padding:0px;font-family:'Open Sans',sans-serif;"
                                                    align="left">

                                                <table height="0px" align="center" border="0" cellpadding="0" cellspacing="0" width="90%"
                                                       style="border-collapse: collapse;table-layout: fixed;border-spacing: 0;mso-table-lspace: 0pt;mso-table-rspace: 0pt;vertical-align: top;border-top: 1px solid #BBBBBB;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                                                    <tbody>
                                                    <tr style="vertical-align: top">
                                                        <td
                                                                style="word-break: break-word;border-collapse: collapse !important;vertical-align: top;font-size: 0px;line-height: 0px;mso-line-height-rule: exactly;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                                                            <span>&#160;</span>
                                                        </td>
                                                    </tr>
                                                    </tbody>
                                                </table>

                                            </td>
                                        </tr>
                                        </tbody>
                                    </table>


                                    <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
                            </div>
                        </div>
                        <!--[if (mso)|(IE)]></td><![endif]-->
                        <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
                    </div>
                </div>
            </div>


            <div class="u-row-container" style="padding: 0px;background-color: transparent">
                <div class="u-row"
                     style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #689a8c;">
                    <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
                        <!--[if (mso)|(IE)]>
                        <table width="100%" cellpadding="0" cellspacing="0" border="0">
                            <tr>
                                <td style="padding: 0px;background-color: transparent;" align="center">
                                    <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                                        <tr style="background-color: #689a8c;"><![endif]-->

                        <!--[if (mso)|(IE)]>
                        <td align="center" width="600"
                            style="width: 600px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                            valign="top"><![endif]-->
                        <div class="u-col u-col-100"
                             style="max-width: 320px;min-width: 600px;display: table-cell;vertical-align: top;">
                            <div style="width: 100% !important;">
                                <!--[if (!mso)&(!IE)]><!-->
                                <div
                                        style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                                    <!--<![endif]-->


                                    <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                                           width="100%" border="0">
                                        <tbody>
                                        <tr>
                                            <td
                                                    style="overflow-wrap:break-word;word-break:break-word;padding:16px;font-family:'Open Sans',sans-serif;"
                                                    align="left">

                                                <div class="v-text-align"
                                                     style="color: #ecf7ff; line-height: 140%; text-align: center; word-wrap: break-word;">
                                                    <p style="font-size: 14px; line-height: 140%;">&copy; Shop. All Rights Reserved</p>
                                                </div>

                                            </td>
                                        </tr>
                                        </tbody>
                                    </table>


                                    <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
                            </div>
                        </div>
                        <!--[if (mso)|(IE)]></td><![endif]-->
                        <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
                    </div>
                </div>
            </div>


            <!--[if (mso)|(IE)]></td></tr></table><![endif]-->
        </td>
    </tr>
    </tbody>
</table>
<!--[if mso]></div><![endif]-->
<!--[if IE]></div><![endif]-->
</body>

</html>`,
					}); err != nil {
						logger.Warningf("%v", err)
					}
					// You account
					if _, err = models.CreateEmailTemplate(common.Database, &models.EmailTemplate{
						Enabled: false,
						Type:    common.NOTIFICATION_TYPE_CREATE_ACCOUNT,
						Topic:   "Your Account",
						Message: `<!DOCTYPE HTML PUBLIC "-//W3C//DTD XHTML 1.0 Transitional //EN"
  "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:v="urn:schemas-microsoft-com:vml"
      xmlns:o="urn:schemas-microsoft-com:office:office">
<head>
  <!--[if gte mso 9]>
  <xml>
    <o:OfficeDocumentSettings>
      <o:AllowPNG/>
      <o:PixelsPerInch>96</o:PixelsPerInch>
    </o:OfficeDocumentSettings>
  </xml>
  <![endif]-->
  <meta http-equiv="Content-Type" content="text/html; charset=UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta name="x-apple-disable-message-reformatting">
  <!--[if !mso]><!-->
  <meta http-equiv="X-UA-Compatible" content="IE=edge"><!--<![endif]-->
  <title>Your account</title>

  <style type="text/css">
    a {
      color: #6d6d6d;
      text-decoration: none;
    }

    @media (max-width: 480px) {
      #u_content_image_1 .v-src-width {
        width: auto !important;
      }

      #u_content_image_1 .v-src-max-width {
        max-width: 50% !important;
      }

      #u_content_text_1 .v-text-align {
        text-align: center !important;
      }

      #u_content_text_14 .v-text-align {
        text-align: center !important;
      }

      #u_content_text_15 .v-text-align {
        text-align: center !important;
      }
    }

    @media only screen and (min-width: 620px) {
      .u-row {
        width: 600px !important;
      }

      .u-row .u-col {
        vertical-align: top;
      }

      .u-row .u-col-33p33 {
        width: 199.98px !important;
      }

      .u-row .u-col-50 {
        width: 300px !important;
      }

      .u-row .u-col-66p67 {
        width: 400.02px !important;
      }

      .u-row .u-col-100 {
        width: 600px !important;
      }

    }

    @media (max-width: 620px) {
      .u-row-container {
        max-width: 100% !important;
        padding-left: 0px !important;
        padding-right: 0px !important;
      }

      .u-row .u-col {
        min-width: 320px !important;
        max-width: 100% !important;
        display: block !important;
      }

      .u-row {
        width: calc(100% - 40px) !important;
      }

      .u-col {
        width: 100% !important;
      }

      .u-col > div {
        margin: 0 auto;
      }

      .no-stack .u-col {
        min-width: 0 !important;
        display: table-cell !important;
      }

      .no-stack .u-col-50 {
        width: 50% !important;
      }

    }

    body {
      margin: 0;
      padding: 0;
    }

    table,
    tr,
    td {
      vertical-align: top;
      border-collapse: collapse;
    }

    p {
      margin: 0;
    }

    .ie-container table,
    .mso-container table {
      table-layout: fixed;
    }

    * {
      line-height: inherit;
    }

    a[x-apple-data-detectors='true'] {
      color: inherit !important;
      text-decoration: none !important;
    }

    @media (max-width: 480px) {
      .hide-mobile {
        display: none !important;
        max-height: 0px;
        overflow: hidden;
      }

    }
  </style>


  <!--[if !mso]><!-->
  <link href="https://fonts.googleapis.com/css?family=Open+Sans:400,700&display=swap" rel="stylesheet" type="text/css">
  <!--<![endif]-->

</head>

{{$url := .Url}}
{{$symbol := .Symbol}}

<body class="clean-body" style="margin: 0;padding: 0;-webkit-text-size-adjust: 100%;background-color: #eeeeee">
<!--[if IE]>
<div class="ie-container"><![endif]-->
<!--[if mso]>
<div class="mso-container"><![endif]-->
<table
  style="border-collapse: collapse;table-layout: fixed;border-spacing: 0;mso-table-lspace: 0pt;mso-table-rspace: 0pt;vertical-align: top;min-width: 320px;Margin: 0 auto;background-color: #eeeeee;width:100%"
  cellpadding="0" cellspacing="0">
  <tbody>
  <tr style="vertical-align: top">
    <td style="word-break: break-word;border-collapse: collapse !important;vertical-align: top">
      <!--[if (mso)|(IE)]>
      <table width="100%" cellpadding="0" cellspacing="0" border="0">
        <tr>
          <td align="center" style="background-color: #eeeeee;"><![endif]-->


      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;">
          <div
            style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;border-bottom: 1px solid lightgray;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style=""><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="200"
                style="width: 200px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-33p33"
                 style="max-width: 320px;min-width: 200px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table id="u_content_image_1" style="font-family:'Open Sans',sans-serif;" role="presentation"
                         cellpadding="0" cellspacing="0" width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:31px 10px 25px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <table width="100%" cellpadding="0" cellspacing="0" border="0">
                          <tr>
                            <td class="v-text-align" style="padding-right: 0px;padding-left: 0px;" align="center">

                              <img align="center" border="0" src="https://shop.servhost.org/img/logo.png" alt="Image"
                                   title="Image"
                                   style="outline: none;text-decoration: none;-ms-interpolation-mode: bicubic;clear: both;display: inline-block !important;border: none;height: auto;float: none;width: 84%;max-width: 151.2px;"
                                   width="151.2" class="v-src-width v-src-max-width"/>

                            </td>
                          </tr>
                        </table>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]>
            <td align="center" width="400"
                style="width: 400px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-66p67"
                 style="max-width: 320px;min-width: 400px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table class="hide-mobile" style="font-family:'Open Sans',sans-serif;" role="presentation"
                         cellpadding="0" cellspacing="0" width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <table height="0px" align="center" border="0" cellpadding="0" cellspacing="0" width="100%"
                               style="border-collapse: collapse;table-layout: fixed;border-spacing: 0;mso-table-lspace: 0pt;mso-table-rspace: 0pt;vertical-align: top;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                          <tbody>
                          <tr style="vertical-align: top">
                            <td
                              style="word-break: break-word;border-collapse: collapse !important;vertical-align: top;font-size: 0px;line-height: 0px;mso-line-height-rule: exactly;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                              <span>&#160;</span>
                            </td>
                          </tr>
                          </tbody>
                        </table>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <table id="u_content_text_1" style="font-family:'Open Sans',sans-serif;" role="presentation"
                         cellpadding="0" cellspacing="0" width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:13px 26px 16px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #ffffff; line-height: 140%; text-align: right; word-wrap: break-word;">
                          <!--p style="font-size: 14px; line-height: 140%;"><span style="font-size: 14px; line-height: 19.6px;">SHOP</span></p-->
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>


      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #ffffff;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #ffffff;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="600"
                style="width: 600px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-100"
                 style="max-width: 320px;min-width: 600px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:40px 10px 10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <table width="100%" cellpadding="0" cellspacing="0" border="0">
                          <tr>
                            <td class="v-text-align" style="padding-right: 0px;padding-left: 0px;" align="center">

                              <!--img align="center" border="0" src="images/image-1.png" alt="Image" title="Image" style="outline: none;text-decoration: none;-ms-interpolation-mode: bicubic;clear: both;display: inline-block !important;border: none;height: auto;float: none;width: 17%;max-width: 98.6px;" width="98.6" class="v-src-width v-src-max-width"/-->

                            </td>
                          </tr>
                        </table>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:10px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #47484b; line-height: 140%; text-align: center; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 140%;"><strong><span
                            style="font-size: 30px; line-height: 42px;">Your account</span></strong></p>
                          <p>You account was just created!</p>
                          <p>Email: <span style="font-weight: bold;">{{.Email}}</span></p>
                          <p>Password: <span style="font-weight: bold;">{{.Password}}</span></p>
                          <p style="padding: 10px"><a href="{{$url}}" style="background-color: #689a8c;color: white;padding: 5px 10px;margin: 5px;">Login</a></p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:2px 40px 25px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #7a7676; line-height: 170%; text-align: left; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 170%; text-align: center;"><span
                            style="font-size: 16px; line-height: 27.2px;"></span>
                          </p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:0px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <table height="0px" align="center" border="0" cellpadding="0" cellspacing="0" width="90%"
                               style="border-collapse: collapse;table-layout: fixed;border-spacing: 0;mso-table-lspace: 0pt;mso-table-rspace: 0pt;vertical-align: top;border-top: 1px solid #BBBBBB;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                          <tbody>
                          <tr style="vertical-align: top">
                            <td
                              style="word-break: break-word;border-collapse: collapse !important;vertical-align: top;font-size: 0px;line-height: 0px;mso-line-height-rule: exactly;-ms-text-size-adjust: 100%;-webkit-text-size-adjust: 100%">
                              <span>&#160;</span>
                            </td>
                          </tr>
                          </tbody>
                        </table>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>


      <div class="u-row-container" style="padding: 0px;background-color: transparent">
        <div class="u-row"
             style="Margin: 0 auto;min-width: 320px;max-width: 600px;overflow-wrap: break-word;word-wrap: break-word;word-break: break-word;background-color: #689a8c;">
          <div style="border-collapse: collapse;display: table;width: 100%;background-color: transparent;">
            <!--[if (mso)|(IE)]>
            <table width="100%" cellpadding="0" cellspacing="0" border="0">
              <tr>
                <td style="padding: 0px;background-color: transparent;" align="center">
                  <table cellpadding="0" cellspacing="0" border="0" style="width:600px;">
                    <tr style="background-color: #689a8c;"><![endif]-->

            <!--[if (mso)|(IE)]>
            <td align="center" width="600"
                style="width: 600px;padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;"
                valign="top"><![endif]-->
            <div class="u-col u-col-100"
                 style="max-width: 320px;min-width: 600px;display: table-cell;vertical-align: top;">
              <div style="width: 100% !important;">
                <!--[if (!mso)&(!IE)]><!-->
                <div
                  style="padding: 0px;border-top: 0px solid transparent;border-left: 0px solid transparent;border-right: 0px solid transparent;border-bottom: 0px solid transparent;">
                  <!--<![endif]-->


                  <table style="font-family:'Open Sans',sans-serif;" role="presentation" cellpadding="0" cellspacing="0"
                         width="100%" border="0">
                    <tbody>
                    <tr>
                      <td
                        style="overflow-wrap:break-word;word-break:break-word;padding:16px;font-family:'Open Sans',sans-serif;"
                        align="left">

                        <div class="v-text-align"
                             style="color: #ecf7ff; line-height: 140%; text-align: center; word-wrap: break-word;">
                          <p style="font-size: 14px; line-height: 140%;">&copy; Shop. All Rights Reserved</p>
                        </div>

                      </td>
                    </tr>
                    </tbody>
                  </table>


                  <!--[if (!mso)&(!IE)]><!--></div><!--<![endif]-->
              </div>
            </div>
            <!--[if (mso)|(IE)]></td><![endif]-->
            <!--[if (mso)|(IE)]></tr></table></td></tr></table><![endif]-->
          </div>
        </div>
      </div>


      <!--[if (mso)|(IE)]></td></tr></table><![endif]-->
    </td>
  </tr>
  </tbody>
</table>
<!--[if mso]></div><![endif]-->
<!--[if IE]></div><![endif]-->
</body>

</html>
`,
					}); err != nil {
						logger.Warningf("%v", err)
					}
				}
			}
		}
		//
		common.STORAGE, err = storage.NewLocalStorage(path.Join(dir, "hugo"), common.Config.Resize.Quality)
		if err != nil {
			logger.Warningf("%v", err)
		}
		if common.Config.Storage.Enabled {
			if common.Config.Storage.S3.Enabled {
				if common.STORAGE, err = storage.NewAWSS3Storage(common.Config.Storage.S3.AccessKeyID,common.Config.Storage.S3.SecretAccessKey, common.Config.Storage.S3.Region, common.Config.Storage.S3.Bucket, common.Config.Storage.S3.Prefix, path.Join(dir, "temp", "s3"), common.Config.Resize.Quality, common.Config.Storage.S3.Rewrite); err != nil {
					logger.Warningf("%+v", err)
				}
			}
		}
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
					crt, _ = ioutil.ReadFile(common.Config.Https.Crt)
				}
				var key []byte
				if _, err := os.Stat(common.Config.Https.Key); err == nil {
					key, _ = ioutil.ReadFile(common.Config.Https.Key)
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
