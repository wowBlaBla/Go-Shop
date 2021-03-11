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
		common.Config.Hugo.Theme = "multikart"
		common.Config.Hugo.Minify = true
		if os.Getenv("DOCKER") == "true" {
			common.Config.Wrangler.Bin = "/usr/bin/docker run --rm -v %DIR%/hugo/public:/hugo/public -v %DIR%/worker/workers-site:/worker/workers-site -v %DIR%/worker/wrangler.toml:/worker/wrangler.toml goshop_wrangler"
		}
		common.Config.Products = "Products"
		common.Config.FlatUrl = true
		common.Config.Resize.Quality = 75
		common.Config.Resize.Thumbnail.Size = "64x0,128x0,256x0"
		common.Config.Resize.Image.Size= "128x0,324x0,512x0"
		common.Config.Currency = "usd"
		common.Config.Payment.Default = "stripe"
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
		if err := common.Database.AutoMigrate(&models.CacheImage{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.CacheVariation{}); err != nil {
			logger.Warningf("%+v", err)
		}
		if err := common.Database.AutoMigrate(&models.CacheValue{}); err != nil {
			logger.Warningf("%+v", err)
		}
		//
		if err := common.Database.AutoMigrate(&models.Profile{}); err != nil {
			logger.Warningf("%+v", err)
		}
		//
		if err := common.Database.AutoMigrate(&models.Widget{}); err != nil {
			logger.Warningf("%+v", err)
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
						Topic:   "New order id {{.Order.ID}} paid",
						Message: `<div>
<h1><a href="http://example.com/admin/sales/orders?id={{.Order.ID}}">Order #{{.Order.ID}}</a></h1>
<table>
<tbody>
<tr><th style="text-align: left">Created</th><td>{{.Order.CreatedAt.Format "2006-01-02 15:04:05"}}</td></tr>
<tr><th style="text-align: left">Volume</th><td>{{printf "%.3f" .Order.Volume}}</td>
<tr><th style="text-align: left">Weight</th><td>{{printf "%.1f" .Order.Weight}}</td>
<tr><th style="text-align: left">Status</th><td>{{.Order.Status}}</td>
<tr><th style="text-align: left">Sum</th><td>{{printf "%.2f" .Order.Sum}}</td>
<tr><th style="text-align: left">Delivery</th><td>{{printf "%.2f" .Order.Delivery}}</td>
<tr><th style="text-align: left">Total</th><td>{{printf "%.2f" .Order.Total}}</td></tr>
</tbody>
</table>
</div>
<div>
<h2>Items</h2>
<table>
<thead><tr style="background-color: lightgray;"><th style="min-width: 50px;">#</th><th>UUID</th><th>Products</th><th>Variation</th><th>Properties</th><th>Volume</th><th>Weight</th><th>Price</th><th>Quantity</th><th>Total</th></tr></thead>
<tbody>
{{range $index, $item := .Order.Items}}
<tr style="background-color: {{if even $index}}#fff{{else}}#f5f5f5{{end}}">
<td style="text-align: center;">{{add $index 1}}</td>
<td style="padding: 0 10px;">{{toUuid .Uuid}}</td>
<td style="padding: 0 10px;"><a href="http://example.com{{$item.Path}}?uuid={{toUuid $item.Uuid}}">{{.Title}}</a></td>
<td style="padding: 0 10px;">{{$item.Variation.Title}}</td>
<td><ul style="padding: 0 20px;">{{range $item.Properties}}<li><span>{{.Title}}:&nbsp;</span><span>{{.Value}}</span></li>{{end}}</ul></td>
<td style="padding: 0 10px;">{{printf "%.3f" $item.Volume}}</td>
<td style="padding: 0 10px;">{{printf "%.1f" $item.Weight}}</td>
<td style="padding: 0 10px;">{{printf "%.2f" $item.Price}}</td>
<td style="text-align: center;">{{$item.Quantity}}</td>
<td style="padding: 0 10px;">{{printf "%.2f" $item.Total}}</td>
</tr>
{{end}}
</tbody>
</table>
</div>
<div>
<h2>Delivery</h2>
<table>
<tbody>
<tr><th style="text-align: left">Name</th><td>{{.Order.Profile.Name}}</td></tr>
<tr><th style="text-align: left">Lastname</th><td>{{.Order.Profile.Lastname}}</td></tr>
<tr><th style="text-align: left">Address</th><td>{{.Order.Profile.Address}}</td></tr>
<tr><th style="text-align: left">Zip</th><td>{{.Order.Profile.Zip}}</td></tr>
<tr><th style="text-align: left">City</th><td>{{.Order.Profile.City}}</td></tr>
<tr><th style="text-align: left">Region</th><td>{{.Order.Profile.Region}}</td></tr>
<tr><th style="text-align: left">Country</th><td>{{.Order.Profile.Country}}</td></tr>
<tr><th style="text-align: left">Transport</th><td>{{.Order.Profile.Transport.Title}}</td></tr>
</tbody>
</table>
</div>`,
					}); err != nil {
						logger.Warningf("%v", err)
					}
					// User
					if _, err = models.CreateEmailTemplate(common.Database, &models.EmailTemplate{
						Enabled: false,
						Type:    common.NOTIFICATION_TYPE_USER_ORDER_PAID,
						Topic:   "You order #{{.Order.ID}}",
						Message: `<div>
<h1><a href="http://example.com/orders/">Order #{{.Order.ID}}</a></h1>
<table>
<tbody>
<tr><th style="text-align: left">Created</th><td>{{.Order.CreatedAt.Format "2006-01-02 15:04:05"}}</td></tr>
<tr><th style="text-align: left">Volume</th><td>{{printf "%.3f" .Order.Volume}}</td>
<tr><th style="text-align: left">Weight</th><td>{{printf "%.1f" .Order.Weight}}</td>
<tr><th style="text-align: left">Status</th><td>{{.Order.Status}}</td>
<tr><th style="text-align: left">Sum</th><td>{{printf "%.2f" .Order.Sum}}</td>
<tr><th style="text-align: left">Delivery</th><td>{{printf "%.2f" .Order.Delivery}}</td>
<tr><th style="text-align: left">Total</th><td>{{printf "%.2f" .Order.Total}}</td></tr>
</tbody>
</table>
</div>
<div>
<h2>Items</h2>
<table>
<thead><tr style="background-color: lightgray;"><th style="min-width: 50px;">#</th><th>UUID</th><th>Products</th><th>Variation</th><th>Properties</th><th>Volume</th><th>Weight</th><th>Price</th><th>Quantity</th><th>Total</th></tr></thead>
<tbody>
{{range $index, $item := .Order.Items}}
<tr style="background-color: {{if even $index}}#fff{{else}}#f5f5f5{{end}}">
<td style="text-align: center;">{{add $index 1}}</td>
<td style="padding: 0 10px;">{{toUuid .Uuid}}</td>
<td style="padding: 0 10px;"><a href="http://example.com{{$item.Path}}?uuid={{toUuid $item.Uuid}}">{{.Title}}</a></td>
<td style="padding: 0 10px;">{{$item.Variation.Title}}</td>
<td><ul style="padding: 0 20px;">{{range $item.Properties}}<li><span>{{.Title}}:&nbsp;</span><span>{{.Value}}</span></li>{{end}}</ul></td>
<td style="padding: 0 10px;">{{printf "%.3f" $item.Volume}}</td>
<td style="padding: 0 10px;">{{printf "%.1f" $item.Weight}}</td>
<td style="padding: 0 10px;">{{printf "%.2f" $item.Price}}</td>
<td style="text-align: center;">{{$item.Quantity}}</td>
<td style="padding: 0 10px;">{{printf "%.2f" $item.Total}}</td>
</tr>
{{end}}
</tbody>
</table>
</div>
<div>
<h2>Delivery</h2>
<table>
<tbody>
<tr><th style="text-align: left">Name</th><td>{{.Order.Profile.Name}}</td></tr>
<tr><th style="text-align: left">Lastname</th><td>{{.Order.Profile.Lastname}}</td></tr>
<tr><th style="text-align: left">Address</th><td>{{.Order.Profile.Address}}</td></tr>
<tr><th style="text-align: left">Zip</th><td>{{.Order.Profile.Zip}}</td></tr>
<tr><th style="text-align: left">City</th><td>{{.Order.Profile.City}}</td></tr>
<tr><th style="text-align: left">Region</th><td>{{.Order.Profile.Region}}</td></tr>
<tr><th style="text-align: left">Country</th><td>{{.Order.Profile.Country}}</td></tr>
<tr><th style="text-align: left">Transport</th><td>{{.Order.Profile.Transport.Title}}</td></tr>
</tbody>
</table>
</div>
<div>
<h1>Thank you for your purchase!</h1>
</div>`,
					}); err != nil {
						logger.Warningf("%v", err)
					}
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
