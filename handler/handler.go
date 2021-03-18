package handler

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	crypto_rand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/BurntSushi/toml"
	swagger "github.com/arsmn/fiber-swagger/v2"
	"github.com/dannyvankooten/vat"
	"github.com/dgrijalva/jwt-go"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/google/logger"
	"github.com/jinzhu/now"
	"github.com/nfnt/resize"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/stripe/stripe-go/v71"
	checkout_session "github.com/stripe/stripe-go/v71/checkout/session"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/common/mollie"
	"github.com/yonnic/goshop/config"
	"github.com/yonnic/goshop/models"
	"gorm.io/gorm"
	"html/template"
	_ "image/gif"
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	BODY_LIMIT = 20 * 1024 * 1024
	HAS_CHANGES = "temp/.has_changes"
)

var (
	reCSV = regexp.MustCompile(`,\s*`)
	reNotAbc = regexp.MustCompile("(?i)[^a-z0-9]+")
	reVolume = regexp.MustCompile(`^(\d+)\s*x\s*(\d+)\s*x\s*(\d+)\s*$`)
	rePercent = regexp.MustCompile(`^(\d+(:?\.\d{1,3})?)%$`)
)


func GetFiber() *fiber.App {
	app, authRequired, authOptional, csrf := CreateFiberAppWithAuthMultiple(AuthMultipleConfig{
			CookieDuration: time.Duration(365 * 24) * time.Hour,
			Log: true,
			//UseForm: true,
		},
		compress.New(compress.Config{
			Level: compress.LevelBestSpeed, // 1
		}), cors.New(cors.Config{
			AllowCredentials: true,
			AllowHeaders:  "Accept, Authorization, Cache-Control, Content-Type, Cookie, Ignoreloadingbar, Origin, Set-Cookie, X-Requested-With",
			AllowOrigins: "*",
		},
	))
	//
	changed := func (messages ...string) func (c *fiber.Ctx) error {
		return func (c *fiber.Ctx) error {
			p := path.Join(dir, HAS_CHANGES)
			if pp := path.Dir(p); len(pp) > 0 {
				if _, err := os.Stat(pp); err != nil {
					if err = os.MkdirAll(pp, 0755); err != nil {
						logger.Warningf("%v", err)
					}
				}
			}
			file, err := os.OpenFile(p,
				os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				logger.Warningf("%v", err)
			}
			defer file.Close()
			for _, message := range messages {
				if _, err := file.WriteString(message + "\n"); err != nil {
					logger.Warningf("%v", err)
				}
			}

			return c.Next()
		}
	}
	hasRole := func (roles ...int) func (c *fiber.Ctx) error {
		return func (c *fiber.Ctx) error {
			if v := c.Locals("user"); v != nil {
				if user, ok := v.(*models.User); ok {
					for _, n := range roles {
						if user.Role == n {
							return c.Next()
						}
					}
				}
			}
			c.Status(http.StatusForbidden)
			return c.JSON(HTTPError{"AccessViolation"})
		}
	}
	//
	api := app.Group("/api")
	v1 := api.Group("/v1")
	v1.Post("/login", postLoginHandler)
	v1.Post("/reset", postResetHandler)
	v1.Get("/logout", authRequired, getLogoutHandler)
	v1.Get("/preview", authOptional, getPreviewHandler)
	v1.Get("/info", authRequired, hasRole(models.ROLE_ROOT, models.ROLE_ADMIN, models.ROLE_MANAGER), getInfoHandler)
	v1.Get("/dashboard", authRequired, hasRole(models.ROLE_ROOT, models.ROLE_ADMIN, models.ROLE_MANAGER), getDashboardHandler)
	v1.Get("/resize", getResizeHandler)
	v1.Get("/settings/basic", authRequired, hasRole(models.ROLE_ROOT, models.ROLE_ADMIN, models.ROLE_MANAGER), getBasicSettingsHandler)
	v1.Put("/settings/basic", authRequired, hasRole(models.ROLE_ROOT, models.ROLE_ADMIN, models.ROLE_MANAGER), changed("basic settings updated"), putBasicSettingsHandler)
	v1.Get("/settings/hugo", authRequired, hasRole(models.ROLE_ROOT, models.ROLE_ADMIN, models.ROLE_MANAGER), getHugoSettingsHandler)
	v1.Put("/settings/hugo", authRequired, hasRole(models.ROLE_ROOT, models.ROLE_ADMIN, models.ROLE_MANAGER), changed("hugo settings updated"), putHugoSettingsHandler)
	v1.Get("/settings/wrangler", authRequired, hasRole(models.ROLE_ROOT, models.ROLE_ADMIN, models.ROLE_MANAGER), getWranglerSettingsHandler)
	v1.Put("/settings/wrangler", authRequired, hasRole(models.ROLE_ROOT, models.ROLE_ADMIN, models.ROLE_MANAGER), changed("wrangler settings updated"), putWranglerSettingsHandler)
	//
	v1.Get("/categories", authRequired, getCategoriesHandler)
	v1.Post("/categories", authRequired, changed("category created"), postCategoriesHandler)
	v1.Post("/categories/autocomplete", authRequired, postCategoriesAutocompleteHandler)
	v1.Post("/categories/list", authRequired, postCategoriesListHandler)
	v1.Get("/categories/:id", authRequired, getCategoryHandler)
	v1.Put("/categories/:id", authRequired, changed("category updated"), putCategoryHandler)
	v1.Delete("/categories/:id", authRequired, changed("category deleted"), delCategoryHandler)
	//
	v1.Get("/contents", authRequired, getContentsHandler)
	v1.Get("/contents/*", authRequired, getContentHandler)
	v1.Patch("/contents/*", authRequired, changed("content updated"), patchContentHandler)
	v1.Post("/contents/*", authRequired, changed("content created"), postContentHandler)
	v1.Put("/contents/*", authRequired, changed("content updated"), putContentHandler)
	//
	v1.Get("/products", authRequired, getProductsHandler)
	v1.Post("/products", authRequired, changed("product created"), postProductsHandler)
	v1.Post("/products/list", authRequired, postProductsListHandler)
	v1.Get("/products/:id", authRequired, getProductHandler)
	v1.Put("/products/:id", authRequired, changed("product updated"), putProductHandler)
	v1.Delete("/products/:id", authRequired, changed("product deleted"), delProductHandler)
	//
	v1.Post("/parameters", authRequired, changed("parameter created"), postParameterHandler)
	v1.Get("/parameters/:id", authRequired, getParameterHandler)
	v1.Put("/parameters/:id", authRequired, changed("parameter updated"), putParameterHandler)
	v1.Delete("/parameters/:id", authRequired, changed("parameter deleted"), deleteParameterHandler)
	//
	v1.Get("/variations", authRequired, getVariationsHandler)
	v1.Post("/variations", authRequired, changed("variation created"), postVariationHandler)
	v1.Post("/variations/list", authRequired, postVariationsListHandler)
	v1.Get("/variations/:id", authRequired, getVariationHandler)
	v1.Put("/variations/:id", authRequired, changed("variation updated"), putVariationHandler)
	v1.Delete("/variations/:id", authRequired, changed("variation deleted"), delVariationHandler)
	//
	v1.Post("/properties", authRequired, changed("property created"), postPropertyHandler)
	v1.Post("/properties/list", authRequired, postPropertiesListHandler)
	v1.Get("/properties/:id", authRequired, getPropertyHandler)
	v1.Put("/properties/:id", authRequired, changed("property updated"), putPropertyHandler)
	v1.Delete("/properties/:id", authRequired, changed("property deleted"), deletePropertyHandler)
	//
	v1.Get("/prices", authRequired, getPricesHandler)
	v1.Post("/prices/list", authRequired, postPricesListHandler)
	v1.Post("/prices", authRequired, changed("price created"), postPriceHandler)
	v1.Get("/prices/:id", authRequired, getPriceHandler)
	v1.Put("/prices/:id", authRequired, changed("price updated"), putPriceHandler)
	v1.Delete("/prices/:id", authRequired, changed("price deleted"), deletePriceHandler)
	//
	v1.Get("/tags", authRequired, getTagsHandler)
	v1.Post("/tags", authRequired, changed("tag created"), postTagHandler)
	v1.Post("/tags/list", authRequired, postTagsListHandler)
	v1.Get("/tags/:id", authRequired, getTagHandler)
	v1.Put("/tags/:id", authRequired, changed("tag updated"), putTagHandler)
	v1.Delete("/tags/:id", authRequired, changed("tag deleted"), delTagHandler)
	// Options
	v1.Get("/options", authRequired, getOptionsHandler)
	v1.Post("/options", authRequired, changed("option created"), postOptionHandler)
	v1.Post("/options/list", authRequired, postOptionsListHandler)
	v1.Get("/options/:id", authRequired, getOptionHandler)
	v1.Patch("/options/:id", authRequired, changed("option updated"), patchOptionHandler)
	v1.Put("/options/:id", authRequired, changed("option updated"), putOptionHandler)
	v1.Delete("/options/:id", authRequired, changed("option deleted"), delOptionHandler)
	// Values
	v1.Get("/values", authRequired, getValuesHandler)
	v1.Post("/values", authRequired, changed("value created"), postValueHandler)
	v1.Post("/values/list", authRequired, postValuesListHandler)
	v1.Get("/values/:id", authRequired, getValueHandler)
	v1.Put("/values/:id", authRequired, changed("value updated"), putValueHandler)
	v1.Delete("/values/:id", authRequired, changed("value deleted"), delValueHandler)
	// Files
	v1.Post("/files", authRequired, changed("file created"), postFileHandler)
	v1.Post("/files/list", authRequired, postFilesListHandler)
	v1.Get("/files/:id", authRequired, getFileHandler)
	v1.Put("/files/:id", authRequired, changed("file updated"), putFileHandler)
	v1.Delete("/files/:id", authRequired, changed("file deleted"), delFileHandler)
	// Images
	v1.Post("/images", authRequired, changed("image created"), postImageHandler)
	v1.Post("/images/list", authRequired, postImagesListHandler)
	v1.Get("/images/:id", authRequired, getImageHandler)
	v1.Put("/images/:id", authRequired, changed("image updated"), putImageHandler)
	v1.Delete("/images/:id", authRequired, changed("image deleted"), delImageHandler)
	// Coupons
	v1.Get("/coupons", authRequired, getCouponsHandler)
	v1.Post("/coupons", authRequired, changed("coupon created"), postCouponHandler)
	v1.Post("/coupons/list", authRequired, postCouponsListHandler)
	v1.Get("/coupons/:id", authRequired, getCouponHandler)
	v1.Put("/coupons/:id", authRequired, changed("coupon updated"), putCouponHandler)
	v1.Delete("/options/:id", authRequired, changed("option deleted"), delCouponHandler)
	// Discounts
	v1.Get("/values", authRequired, getValuesHandler)
	v1.Post("/values", authRequired, changed("value created"), postValueHandler)
	v1.Post("/values/list", authRequired, postValuesListHandler)
	v1.Get("/values/:id", authRequired, getValueHandler)
	v1.Put("/values/:id", authRequired, changed("value updated"), putValueHandler)
	v1.Delete("/values/:id", authRequired, changed("value deleted"), delValueHandler)
	// Orders
	v1.Post("/orders/list", authRequired, postOrdersListHandler)
	v1.Get("/orders/:id", authRequired, getOrderHandler)
	v1.Put("/orders/:id", authRequired, putOrderHandler)
	v1.Delete("/orders/:id", authRequired, delOrderHandler)
	// Transactions
	v1.Post("/transactions/list", authRequired, postTransactionsListHandler)
	v1.Get("/transactions/:id", authRequired, getTransactionHandler)
	v1.Put("/transactions/:id", authRequired, putTransactionHandler)
	v1.Delete("/transactions/:id", authRequired, delTransactionHandler)
	// Widgets
	v1.Post("/widgets", authRequired, changed("widget created"), postWidgetHandler)
	v1.Post("/widgets/list", authRequired, postWidgetsListHandler)
	v1.Get("/widgets/:id", authRequired, getWidgetHandler)
	v1.Put("/widgets/:id", authRequired, changed("widget updated"), putWidgetHandler)
	v1.Delete("/widgets/:id", authRequired, changed("widget deleted"), delWidgetHandler)
	//
	v1.Get("/me", authRequired, getMeHandler)
	//
	v1.Post("/calculate", postDeliveryHandler) // DEPRECATED
	v1.Post("/delivery", postDeliveryHandler)
	//v1.Post("/discount", postDiscountHandler)
	//
	v1.Get("/tariffs", authRequired, getTariffsHandler)
	//
	v1.Get("/transports", getTransportsHandler)
	v1.Post("/transports", authRequired, changed("transport created"), postTransportHandler)
	v1.Post("/transports/list", authRequired, postTransportsListHandler)
	v1.Get("/transports/:id", authRequired, getTransportHandler)
	v1.Put("/transports/:id", authRequired, changed("transport updated"), putTransportHandler)
	v1.Delete("/transports/:id", authRequired, changed("transport deleted"), delTransportHandler)
	//
	v1.Get("/zones", authRequired, getZonesHandler)
	v1.Post("/zones", authRequired, changed("zone created"), postZoneHandler)
	v1.Post("/zones/list", authRequired, postZonesListHandler)
	v1.Get("/zones/:id", authRequired, getZoneHandler)
	v1.Put("/zones/:id", authRequired, changed("zone updated"), putZoneHandler)
	v1.Delete("/zones/:id", authRequired, changed("zone deleted"), delZoneHandler)
	//
	v1.Post("/notification/email", authRequired, postEmailTemplateHandler)
	v1.Post("/notification/email/list", authRequired, changed("email template created"), postEmailTemplatesListHandler)
	v1.Get("/notification/email/:id", authRequired, getEmailTemplateHandler)
	v1.Put("/notification/email/:id", authRequired, changed("email template updated"), putEmailTemplateHandler)
	v1.Delete("/notification/email/:id", authRequired, changed("email template deleted"), delEmailTemplateHandler)
	//
	// Vendors
	v1.Get("/vendors", authRequired, getVendorsHandler)
	v1.Post("/vendors", authRequired, changed("vendor created"), postVendorHandler)
	v1.Post("/vendors/list", authRequired, postVendorsListHandler)
	v1.Get("/vendors/:id", authRequired, getVendorHandler)
	v1.Put("/vendors/:id", authRequired, changed("vendor updated"), putVendorHandler)
	v1.Delete("/vendors/:id", authRequired, changed("vendor deleted"), delVendorHandler)
	// Times
	v1.Get("/times", authRequired, getTimesHandler)
	v1.Post("/times", authRequired, changed("time created"), postTimeHandler)
	v1.Post("/times/list", authRequired, postTimesListHandler)
	v1.Get("/times/:id", authRequired, getTimeHandler)
	v1.Put("/times/:id", authRequired, changed("time updated"), putTimeHandler)
	v1.Delete("/times/:id", authRequired, changed("time deleted"), delTimeHandler)
	//
	v1.Get("/users", authRequired, getUsersHandler)
	v1.Post("/users/list", authRequired, postUsersListHandler)
	v1.Get("/users/:id", authRequired, getUserHandler)
	v1.Put("/users/:id", authRequired, putUserHandler)
	v1.Delete("/users/:id", authRequired, delUserHandler)
	//
	v1.Post("/prepare", authRequired, postPrepareHandler)
	v1.Post("/render", authRequired, postRenderHandler)
	v1.Post("/publish", authRequired, postPublishHandler)
	//
	v1.Get("/account", authRequired, getAccountHandler)
	v1.Post("/account", csrf, postAccountHandler)
	v1.Put("/account", authRequired, putAccountHandler)
	v1.Get("/account/profiles", authRequired, getAccountProfilesHandler)
	v1.Post("/account/profiles", authRequired, postAccountProfileHandler)
	//
	v1.Get("/account/orders", authRequired, getAccountOrdersHandler)
	v1.Post("/account/orders", authRequired, postAccountOrdersHandler)
	v1.Get("/account/orders/:id", authRequired, getAccountOrderHandler)
	v1.Put("/account/orders/:id", authRequired, putAccountOrderHandler)
	v1.Post("/account/orders/:id/checkout", authRequired, postAccountOrderCheckoutHandler)
	// Advance Payment
	v1.Post("/account/orders/:id/advance_payment/submit", authRequired, postAccountOrderAdvancePaymentSubmitHandler)
	// On Delivery
	v1.Post("/account/orders/:id/on_delivery/submit", authRequired, postAccountOrderOnDeliverySubmitHandler)
	// Stripe
	v1.Get("/account/orders/:id/stripe/customer", authRequired, getAccountOrderStripeCustomerHandler) // +
	v1.Get("/account/orders/:id/stripe/card", authRequired, getAccountOrderStripeCardHandler)         // +
	v1.Post("/account/orders/:id/stripe/card", authRequired, postAccountOrderStripeCardHandler)       // +
	v1.Post("/account/orders/:id/stripe/submit", authRequired, postAccountOrderStripeSubmitHandler)   // +
	v1.Get("/account/orders/:id/stripe/success", authRequired, getAccountOrderStripeSuccessHandler)   // +
	// Mollie
	v1.Post("/account/orders/:id/mollie/submit", authRequired, postAccountOrderMollieSubmitHandler)
	v1.Get("/account/orders/:id/mollie/success", authOptional, getAccountOrderMollieSuccessHandler)
	//
	v1.Get("/payment_methods", authRequired, getPaymentMethodsHandler)
	//
	v1.Post("/email", postEmailHandler)
	v1.Post("/profiles", postProfileHandler)
	v1.Get("/test", getTestHandler)
	v1.Post("/test", getTestHandler)
	v1.Post("/vat", postVATHandler)
	v1.Post("/checkout", postCheckoutHandler)
	//
	v1.Post("/filter", postFilterHandler)
	//
	v1.Get("/error/200", func(c *fiber.Ctx) error {
		c.Status(http.StatusOK)
		return c.SendString("OK")
	})
	v1.Get("/error/403", func(c *fiber.Ctx) error {
		c.Status(http.StatusForbidden)
		return c.SendString("403 Forbidden")
	})
	v1.Get("/error/404", func(c *fiber.Ctx) error {
		c.Status(http.StatusNotFound)
		return c.SendString("404 Not Found")
	})
	v1.Get("/error/500", func(c *fiber.Ctx) error {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Some error"})
	})
	//
	if common.Config.Swagger.Enabled {
		//app.Use("/swagger", swagger.Handler) // default
		app.Use("/swagger", swagger.New(swagger.Config{ // custom
			URL: common.Config.Swagger.Url,
			DeepLinking: false,
		}))
		app.Static("/swagger.json", path.Join(dir, "swagger.json"))
	}
	// Admin
	admin := path.Join(dir, "admin")
	app.Static("/admin", admin)
	app.Static("/admin/*", path.Join(admin, "index.html"))
	// Static
	storage := path.Join(dir, "storage")
	app.Static("/storage", storage)
	// Public
	public := path.Join(dir, "hugo", "public")
	app.Static("/", public)
	app.Static("*", public)
	//
	return app
}

// Reset password godoc
// @Summary reset password
// @Accept json
// @Produce json
// @Param form body LoginRequest true "body"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/reset [post]
// @Tags auth
// @Tags frontend
func postResetHandler(c *fiber.Ctx) error {
	var request LoginRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	var emailOrLogin string
	if request.Email != "" {
		emailOrLogin = request.Email
	}
	if request.Login != "" {
		emailOrLogin = request.Login
	}
	var user *models.User
	var err error
	if user, err = models.GetUserByEmail(common.Database, emailOrLogin); err == nil {
		if user.ResetAttempt.Add(time.Duration(1) * time.Minute).After(time.Now()) {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Please try again later"})
		}
		password := NewPassword(12)
		logger.Infof("CHANGE ME: User %v restart old password to %v", user.Email, password)
		user.Password = models.MakeUserPassword(password)
		user.ResetAttempt = time.Now()
		if err = models.UpdateUser(common.Database, user); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
		c.Status(http.StatusOK)
		return c.JSON(HTTPMessage{"OK"})
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// @security BasicAuth
// Logout godoc
// @Summary preview
// @Description run preview
// @Accept json
// @Produce json
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/preview [get]
// @Tags frontend
func getPreviewHandler (c *fiber.Ctx) error {
	//logger.Infof("getPreviewHandler")
	if v := c.Query("step", "1"); v == "1" {
		//logger.Infof("Step1")
		if v := c.Locals("auth"); v != nil {
			//logger.Infof("v: %+v", v)
			if vv, ok := v.(bool); ok && vv {
				if u, err := url.Parse(c.Request().URI().String()); err == nil {
					u.Scheme = "https"
					u.Host = common.Config.Preview
					query := u.Query()
					if referer := string(c.Request().Header.Referer()); referer != "" {
						if res := regexp.MustCompile(`/products/(\d+)`).FindAllStringSubmatch(referer, 1); len(res) > 0 && len(res[0]) > 1 {
							if v, err := strconv.Atoi(res[0][1]); err == nil {
								if c, err := models.GetCacheProductByProductId(common.Database, uint(v)); err == nil {
									query.Set("referer", path.Join(c.Path, c.Name))

								}
							}
						}else if res := regexp.MustCompile(`/categories/(\d+)`).FindAllStringSubmatch(referer, 1); len(res) > 0 && len(res[0]) > 1 {
							logger.Infof("res: %+v", res)
							if v, err := strconv.Atoi(res[0][1]); err == nil {
								if c, err := models.GetCacheCategoryByProductId(common.Database, uint(v)); err == nil {
									logger.Infof("c: %+v", c)
									query.Set("referer", path.Join(c.Path, c.Name))

								}
							}
						}
					}
					query.Set("step", "2")
					enc, err := encrypt([]byte(common.SECRET), []byte(fmt.Sprintf("%d", time.Now().Unix())))
					if err != nil {
						c.Status(http.StatusInternalServerError)
						return c.SendString(err.Error())
					}
					query.Set("token", base64.URLEncoding.EncodeToString(enc))
					u.RawQuery = query.Encode()
					logger.Infof("Redirect: %+v", u.String())
					return c.Redirect(u.String(), http.StatusFound)
				}else{
					c.Status(http.StatusInternalServerError)
					return c.SendString(err.Error())
				}
			}
		}else{
			err := fmt.Errorf("auth: %+v", v)
			c.Status(http.StatusInternalServerError)
			return c.SendString(err.Error())
		}
	}else if v == "2" {
		//logger.Infof("Step2")
		if v := c.Query("token", ""); v != "" {
			if token, err := base64.URLEncoding.DecodeString(v); err == nil {
				if bts, err := decrypt([]byte(common.SECRET), token); err == nil {
					if vvv, err := strconv.Atoi(string(bts)); err == nil {
						t := time.Unix(int64(vvv), 0)
						if time.Now().Sub(t).Seconds() <= 30 {
							cookie := &fiber.Cookie{
								Name:  "preview",
								Value: "true",
								Path:  "/",
								Expires: time.Now().AddDate(0, 0, 1),
								SameSite: authMultipleConfig.SameSite,
								HTTPOnly: true,
							}
							c.Cookie(cookie)
							if referer := c.Query("referer", ""); referer != "" {
								//logger.Infof("referer: %+v", referer)
								return c.Redirect(referer, http.StatusFound)
							}
							return c.Redirect("/", http.StatusFound)
						}else{
							err := fmt.Errorf("token expired")
							c.Status(http.StatusInternalServerError)
							return c.SendString(err.Error())
						}
					}else{
						c.Status(http.StatusInternalServerError)
						return c.SendString(err.Error())
					}
				}else{
					c.Status(http.StatusInternalServerError)
					return c.SendString(err.Error())
				}
			}else{
				c.Status(http.StatusInternalServerError)
				return c.SendString(err.Error())
			}
		}else{
			err := fmt.Errorf("empty token")
			c.Status(http.StatusInternalServerError)
			return c.SendString(err.Error())
		}
	}
	/*var auth bool
	if v := c.Locals("auth"); v != nil {
		if vv, ok := v.(bool); ok {
			auth = vv
		}
	}
	logger.Infof("auth: %+v", auth)
	if auth {

	}else{
		logger.Infof("case2")
		c.Status(http.StatusForbidden)
		return c.SendString("Unauthenticated")
	}*/
	c.Status(http.StatusInternalServerError)
	return c.SendString("Something went wrong")
}

type InfoView struct {
	Application string
	Started string
	Debug bool `json:",omitempty"`
	Pattern string `json:",omitempty"`
	Preview string `json:",omitempty"`
	Authorization string `json:",omitempty"`
	ExpirationAt string `json:",omitempty"`
	HasChanges struct {
		Messages []string `json:",omitempty"`
		Updated *time.Time `json:",omitempty"`
	} `json:",omitempty"`
	User UserView `json:",omitempty"`
}

// @security BasicAuth
// GetInfo godoc
// @Summary Get info
// @Description get string
// @Accept json
// @Produce json
// @Success 200 {object} InfoView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/info [get]
func getInfoHandler(c *fiber.Ctx) error {
	var view InfoView
	view.Application = fmt.Sprintf("%v v%v, build: %v", common.APPLICATION, common.VERSION, common.COMPILED)
	view.Started = common.Started.Format(time.RFC3339)
	view.Debug = common.Config.Debug
	view.Pattern = common.Config.Pattern
	view.Preview = common.Config.Preview
	if v := c.Locals("authorization"); v != nil {
		view.Authorization = v.(string)
	}
	if v := c.Locals("expiration"); v != nil {
		if expiration := v.(int64); expiration > 0 {
			view.ExpirationAt = time.Unix(expiration, 0).Format(time.RFC3339)
		}
	}
	if fi, err := os.Stat(path.Join(dir, HAS_CHANGES)); err == nil {
		file, err := os.Open(path.Join(dir, HAS_CHANGES))
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			view.HasChanges.Messages = append([]string{scanner.Text()}, view.HasChanges.Messages...)
		}
		if err := scanner.Err(); err != nil {
			logger.Warningf("%v", err)
		}
		m := fi.ModTime()
		view.HasChanges.Updated = &m
	}
	if v := c.Locals("user"); v != nil {
		user := v.(*models.User)
		if bts, err := json.Marshal(user); err == nil {
			if err = json.Unmarshal(bts, &view.User); err != nil {
				logger.Errorf("%v", err)
			}
		}
	}
	return c.JSON(view)
}

type DashboardView struct {
	Earnings   float64
	Pending    float64
	Orders     float64
	Items      float64
	Transfers  TransferView
	LastOrders []LastOrderView
}

type TransferView struct {
	Labels []string
	Series [][]float64
}

type LastOrderView struct {
	ID uint
	Created time.Time
	Total float64
	Status string
}

// GetDashboard godoc
// @Summary Get dashboard
// @Accept json
// @Produce json
// @Success 200 {object} DashboardView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/dashboard [get]
func getDashboardHandler(c *fiber.Ctx) error {
	var view DashboardView
	from := now.BeginningOfMonth()
	till := time.Now()
	common.Database.Model(&models.Transaction{}).Select("sum(Amount) as sum").Where("created_at > ? and created_at < ? and (status = ? or status = ?)", from.Format("2006-01-02 15:04:05"), till.Format("2006-01-02 15:04:05"), models.TRANSACTION_STATUS_NEW, models.TRANSACTION_STATUS_PENDING).Scan(&view.Pending)
	if err := common.Database.Error; err != nil {
		logger.Errorf("%v", err.Error())
	}
	common.Database.Model(&models.Transaction{}).Select("sum(Amount) as sum").Where("created_at > ? and created_at < ? and status = ?", from.Format("2006-01-02 15:04:05"), till.Format("2006-01-02 15:04:05"), models.TRANSACTION_STATUS_COMPLETE).Scan(&view.Earnings)
	if err := common.Database.Error; err != nil {
		logger.Errorf("%v", err.Error())
	}
	common.Database.Model(&models.Order{}).Select("count(ID) as c").Where("created_at > ? and created_at < ?", from.Format("2006-01-02 15:04:05"), till.Format("2006-01-02 15:04:05")).Scan(&view.Orders)
	if err := common.Database.Error; err != nil {
		logger.Errorf("%v", err.Error())
	}
	common.Database.Model(&models.Item{}).Select("sum(Quantity) as c").Where("created_at > ? and created_at < ?", from.Format("2006-01-02 15:04:05"), till.Format("2006-01-02 15:04:05")).Scan(&view.Items)
	if err := common.Database.Error; err != nil {
		logger.Errorf("%v", err.Error())
	}
	// Transfers
	var row []float64
	month := from.AddDate(-1,0,0)
	for ; month.Before(till); {
		view.Transfers.Labels = append(view.Transfers.Labels, month.Format("Jan`06"))
		var sum float64
		common.Database.Model(&models.Transaction{}).Select("sum(Amount) as sum").Where("created_at > ? and created_at < ? and status = ?", month.Format("2006-01-02 15:04:05"), month.AddDate(0, 1, 0).Format("2006-01-02 15:04:05"), models.TRANSACTION_STATUS_COMPLETE).Scan(&sum)
		if err := common.Database.Error; err != nil {
			logger.Errorf("%v", err.Error())
		}
		row = append(row, sum)
		month = month.AddDate(0,1,0)
	}
	view.Transfers.Series = append(view.Transfers.Series, row)
	// Last Orders
	func() {
		rows, err := common.Database.Debug().Model(&models.Order{}).Select("orders.ID, orders.Created_At as Created, orders.Total, orders.Status").Where("created_at > ? and created_at < ?", from.Format("2006-01-02 15:04:05"), till.Format("2006-01-02 15:04:05")).Order("ID desc").Limit(10).Rows()
		if err == nil {
			if err == nil {
				for rows.Next() {
					var item LastOrderView
					if err = common.Database.ScanRows(rows, &item); err == nil {
						view.LastOrders = append(view.LastOrders, item)
					} else {
						logger.Errorf("%v", err)
					}
				}
			} else {
				logger.Errorf("%v", err)
			}
			rows.Close()
		}
	}()

	return c.JSON(view)
}

func getResizeHandler (c *fiber.Ctx) error {
	width := 120
	if v := c.Query("width", ""); v != "" {
		if vv, err := strconv.Atoi(v); err == nil {
			width = vv
		}
	}
	height := 120
	if v := c.Query("height", ""); v != "" {
		if vv, err := strconv.Atoi(v); err == nil {
			height = vv
		}
	}
	if p := c.Query("path", ""); p != "" {
		src := path.Join(dir, strings.Replace(p, "../", "/", -1))
		if fi, err := os.Stat(src); err == nil {
			req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(c.Request().Header.Header())))
			if err != nil {
				return err
			}
			if v := req.Header.Get("If-Modified-Since"); v != "" {
				if vv, err := time.Parse(time.RFC1123, v); err == nil {
					if vv.Sub(fi.ModTime()) <= time.Second {
						return c.SendStatus(http.StatusNotModified)
					}
				}else{
					logger.Infof("err: %+v", err)
				}
			}
			if file, err := os.Open(src); err == nil {
				img, err := jpeg.Decode(file)
				if err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
				file.Close()
				m := resize.Resize(uint(width), uint(height), img, resize.Lanczos3)
				out := new(bytes.Buffer)
				if err = jpeg.Encode(out, m, &jpeg.Options{Quality: 80}); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
				c.Status(http.StatusOK)
				c.Response().Header.SetContentType("image/jpeg")
				c.Response().Header.SetLastModified(fi.ModTime())
				return c.SendStream(bytes.NewReader(out.Bytes()), len(out.Bytes()))
			} else {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Path not set"})
	}
}

type BasicSettingsView struct {
	Debug bool
	Currency string
	Symbol string
	Products string
	FlatUrl bool
	Pattern string
	Preview      string
	Payment      config.PaymentConfig
	Resize       config.ResizeConfig
	Notification config.NotificationConfig
}

// GetBasicSettings godoc
// @Summary Get basic settings
// @Accept json
// @Produce json
// @Success 200 {object} BasicSettingsView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/settings/basic [get]
// @Tags settings
func getBasicSettingsHandler(c *fiber.Ctx) error {
	var conf BasicSettingsView
	conf.Debug = common.Config.Debug
	conf.Currency = common.Config.Currency
	conf.Symbol = common.Config.Symbol
	conf.Products = common.Config.Products
	conf.FlatUrl = common.Config.FlatUrl
	conf.Pattern = common.Config.Pattern
	conf.Preview = common.Config.Preview
	conf.Payment = common.Config.Payment
	conf.Resize = common.Config.Resize
	conf.Notification = common.Config.Notification
	return c.JSON(conf)
}

// @security BasicAuth
// PutBasicSettings godoc
// @Summary Set basic settings
// @Accept json
// @Produce json
// @Param category body BasicSettingsView true "body"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/settings/basic [put]
// @Tags settings
func putBasicSettingsHandler(c *fiber.Ctx) error {
	var request BasicSettingsView
	if err := c.BodyParser(&request); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	// Hugo
	var hugo bool
	if request.Currency != common.Config.Currency {
		common.Config.Currency = request.Currency
		hugo = true
	}
	if request.Symbol != common.Config.Symbol {
		common.Config.Symbol = request.Symbol
		hugo = true
	}
	if request.Products != common.Config.Products {
		if common.Config.Products != "" {
			p := path.Join(dir, "hugo", "content", strings.ToLower(common.Config.Products))
			if fi, err := os.Stat(p); err == nil && fi.IsDir() {
				if err = os.RemoveAll(p); err != nil {
					logger.Errorf("%v", err.Error())
				}
			}
		}
		common.Config.Products = request.Products
		hugo = true
	}
	if request.FlatUrl != common.Config.FlatUrl {
		common.Config.FlatUrl = request.FlatUrl
		hugo = true
	}
	if request.Payment.Mollie.ProfileID != common.Config.Payment.Mollie.ProfileID {
		common.Config.Payment.Mollie.ProfileID = request.Payment.Mollie.ProfileID
		hugo = true
	}
	if request.Payment.Stripe.PublishedKey != common.Config.Payment.Stripe.PublishedKey {
		common.Config.Payment.Stripe.PublishedKey = request.Payment.Stripe.PublishedKey
		hugo = true
	}
	if hugo {
		var conf HugoSettingsView
		if _, err := toml.DecodeFile(path.Join(dir, "hugo", "config.toml"), &conf); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
		//
		conf.Params.Currency = strings.ToLower(common.Config.Currency)
		conf.Params.Symbol = strings.ToLower(common.Config.Symbol)
		conf.Params.Products = strings.ToLower(common.Config.Products)
		conf.Params.MollieProfileId = common.Config.Payment.Mollie.ProfileID
		conf.Params.StripePublishedKey = common.Config.Payment.Stripe.PublishedKey
		//
		f, err := os.Create(path.Join(dir, "hugo", "config.toml"))
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		if err = toml.NewEncoder(f).Encode(conf); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}
	//
	var message string
	common.Config.Debug = request.Debug
	common.Config.Pattern = request.Pattern
	common.Config.Preview = request.Preview
	// Payment
	common.Config.Payment = request.Payment
	var render = hugo
	// Resize ?
	if (request.Resize.Enabled && !common.Config.Resize.Enabled) ||
		(request.Resize.Enabled && common.Config.Resize.Quality != request.Resize.Quality) ||
		(request.Resize.Enabled && request.Resize.Thumbnail.Enabled && common.Config.Resize.Thumbnail.Size != request.Resize.Thumbnail.Size) ||
		(request.Resize.Enabled && request.Resize.Image.Enabled && common.Config.Resize.Image.Size != request.Resize.Image.Size) {
		render = true
	}
	if render {
		message = "Saved, background rendering started"
		logger.Info(message)
		go func() {
			// Prepare
			cmd := exec.Command(os.Args[0], "render", "-p", path.Join(dir, "hugo", "content"))
			buff := &bytes.Buffer{}
			cmd.Stderr = buff
			cmd.Stdout = buff
			if err := cmd.Run(); err != nil {
				stdout := buff.String()
				stderr := err.Error()
				logger.Errorf("%v\n%v", stdout, stderr)
			}
			// Render
			bin := strings.Split(common.Config.Hugo.Bin, " ")
			var arguments []string
			if len(bin) > 1 {
				for _, x := range bin[1:]{
					x = strings.Replace(x, "%DIR%", dir, -1)
					arguments = append(arguments, x)
				}
			}
			arguments = append(arguments, "--cleanDestinationDir")
			if common.Config.Hugo.Minify {
				arguments = append(arguments, "--minify")
			}
			if len(bin) == 1 {
				arguments = append(arguments, []string{"-s", path.Join(dir, "hugo")}...)
			}
			cmd = exec.Command(bin[0], arguments...)
			buff = &bytes.Buffer{}
			cmd.Stderr = buff
			cmd.Stdout = buff
			if err := cmd.Run(); err != nil {
				stdout := buff.String()
				stderr := err.Error()
				logger.Errorf("%v\n%v", stdout, stderr)
			}
			//
			time.Sleep(1 * time.Second)
			logger.Infof("Restart application because of settings change")
			os.Exit(0)
		}()
	}else{
		message = "OK"
		go func() {
			time.Sleep(1 * time.Second)
			logger.Infof("Restart application because of settings change")
			os.Exit(0)
		}()
	}
	common.Config.Resize = request.Resize
	// Notification
	common.Config.Notification = request.Notification
	//
	if err := common.Config.Save(); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	return c.JSON(HTTPMessage{message})
}

type HugoSettingsView struct {
	Title string `toml:"title"`
	Theme string `toml:"theme"`
	Paginate int `toml:"paginate"`
	Params struct {
		Description string `toml:"description"`
		Keywords string `toml:"keywords"`
		Logo string `toml:"logo"`
		Currency string `toml:"currency"`
		Symbol string `toml:"symbol"`
		Products string `toml:"products"`
		MollieProfileId string `toml:"mollieProfileId"`
		StripePublishedKey string `toml:"stripePublishedKey"`
	} `toml:"params"`
	Languages map[string]struct {
		LanguageName string `toml:"languageName"`
		Weight int `toml:"weight"`
	} `toml:"languages"`
	Related struct {
		IncludeNewer bool `toml:"includeNewer"`
		Threshold int `toml:"threshold"`
		ToLower bool `toml:"toLower"`
		Indices []struct{
			Name string `toml:"name"`
			Weight int `toml:"weight"`
		} `toml:"indices"`
	} `toml:"related"`
}

// GetHugoSettings godoc
// @Summary Get hugo settings
// @Accept json
// @Produce json
// @Success 200 {object} HugoSettingsView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/settings/hugo [get]
// @Tags settings
func getHugoSettingsHandler(c *fiber.Ctx) error {
	var conf HugoSettingsView
	if _, err := toml.DecodeFile(path.Join(dir, "hugo", "config.toml"), &conf); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	return c.JSON(conf)
}

type HugoSettingsRequest struct {
	Title string
	Theme string
	Paginate int
	Description string
	Keywords string
	Logo string
}

// @security BasicAuth
// PutHugoSettings godoc
// @Summary Set hugo settings
// @Accept json
// @Produce json
// @Param category body HugoSettingsRequest true "body"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/settings/hugo [put]
// @Tags settings
func putHugoSettingsHandler(c *fiber.Ctx) error {
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			//
			var conf HugoSettingsView
			if _, err := toml.DecodeFile(path.Join(dir, "hugo", "config.toml"), &conf); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if v, found := data.Value["Title"]; found && len(v) > 0 {
				conf.Title = v[0]
			}
			if v, found := data.Value["Description"]; found && len(v) > 0 {
				conf.Params.Description = v[0]
			}
			if v, found := data.Value["Keywords"]; found && len(v) > 0 {
				conf.Params.Keywords = v[0]
			}
			if v, found := data.Value["Currency"]; found && len(v) > 0 {
				conf.Params.Currency = v[0]
			}
			if v, found := data.Value["Products"]; found && len(v) > 0 {
				conf.Params.Products = v[0]
			}
			if v, found := data.Value["Theme"]; found && len(v) > 0 {
				conf.Theme = v[0]
			}
			if v, found := data.File["Logo"]; found && len(v) > 0 {
				p := path.Join(dir, "hugo", "themes", conf.Theme, "static", "img")
				if _, err := os.Stat(p); err != nil {
					if err = os.MkdirAll(p, 0755); err != nil {
						logger.Errorf("%v", err)
					}
				}
				filename := "logo.png"
				if p := path.Join(p, filename); len(p) > 0 {
					if in, err := v[0].Open(); err == nil {
						out, err := os.OpenFile(p, os.O_WRONLY | os.O_CREATE, 0644)
						if err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
						defer out.Close()
						if _, err := io.Copy(out, in); err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
					}
				}
			}
			if v, found := data.Value["Paginate"]; found && len(v) > 0 {
				if vv, err := strconv.Atoi(v[0]); err == nil {
					conf.Paginate = vv
				}
			}
			f, err := os.Create(path.Join(dir, "hugo", "config.toml"))
			if err != nil {
				log.Fatal(err)
			}
			defer f.Close()
			if err = toml.NewEncoder(f).Encode(conf); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			return c.JSON(HTTPMessage{"OK"})
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	c.Status(http.StatusInternalServerError)
	return c.JSON(HTTPError{"Something went wrong"})
}



type WranglerSettingsView struct {
	Name string `toml:"name"`
	Type string `toml:"type"`
	AccountId string `toml:"account_id"`
	WorkersDev bool `toml:"workers_dev"`
	Route string `toml:"route"`
	ZoneId string `toml:"zone_id"`
	Site struct {
		Bucket string `toml:"bucket"`
	} `toml:"site"`
}

type BasicWranglerView config.WranglerConfig

// GetWranglerSettings godoc
// @Summary Get wrangler settings
// @Accept json
// @Produce json
// @Success 200 {object} BasicWranglerView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/settings/wrangler [get]
// @Tags settings
func getWranglerSettingsHandler(c *fiber.Ctx) error {
	var conf struct {
		WranglerSettingsView
		config.WranglerConfig
	}
	if _, err := toml.DecodeFile(path.Join(dir, "worker", "wrangler.toml"), &conf); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	conf.Enabled = common.Config.Wrangler.Enabled
	conf.ApiToken = common.Config.Wrangler.ApiToken
	return c.JSON(conf)
}

// @security BasicAuth
// PutWranglerSettings godoc
// @Summary Set wrangler settings
// @Accept json
// @Produce json
// @Param category body WranglerSettingsView true "body"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/settings/wrangler [put]
// @Tags settings
func putWranglerSettingsHandler(c *fiber.Ctx) error {
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if v, found := data.Value["Enabled"]; found && len(v) > 0 {
				if value, err := strconv.ParseBool(v[0]); err == nil {
					common.Config.Wrangler.Enabled = value
				}
			}
			if v, found := data.Value["ApiToken"]; found && len(v) > 0 {
				common.Config.Wrangler.ApiToken = v[0]
			}
			if err = common.Config.Save(); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			//
			var conf WranglerSettingsView
			if _, err := toml.DecodeFile(path.Join(dir, "worker", "wrangler.toml"), &conf); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if v, found := data.Value["Name"]; found && len(v) > 0 {
				conf.Name = v[0]
			}
			if v, found := data.Value["AccountId"]; found && len(v) > 0 {
				conf.AccountId = v[0]
			}
			if v, found := data.Value["DeveloperMode"]; found && len(v) > 0 {
				if value, err := strconv.ParseBool(v[0]); err == nil {
					conf.WorkersDev = value
				}
			}
			if v, found := data.Value["Route"]; found && len(v) > 0 {
				conf.Route = v[0]
			}
			if v, found := data.Value["ZoneId"]; found && len(v) > 0 {
				conf.ZoneId = v[0]
			}
			f, err := os.Create(path.Join(dir, "worker", "wrangler.toml"))
			if err != nil {
				log.Fatal(err)
			}
			defer f.Close()
			if err = toml.NewEncoder(f).Encode(conf); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			return c.JSON(HTTPMessage{"OK"})
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	c.Status(http.StatusInternalServerError)
	return c.JSON(HTTPError{"Something went wrong"})
}

// @security BasicAuth
// GetCategories godoc
// @Summary Get categories
// @Description get string
// @Accept json
// @Produce json
// @Param id query int false "Root ID"
// @Param depth query int false "Depth, default 1"
// @Success 200 {object} CategoryView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/categories [get]
// @Tags category
func getCategoriesHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Query("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var depth = 1
	if v := c.Query("depth"); v != "" {
		depth, _ = strconv.Atoi(v)
	}
	var noProducts = false
	if v := c.Query("no-products"); v != "" {
		if vv, err := strconv.ParseBool(v); err == nil {
			noProducts = vv
		}
	}
	view, err := GetCategoriesView(common.Database, id, depth, noProducts)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	common.Database.Debug().Model(&models.Product{}).Select("count(ID)").Joins("left join categories_products on categories_products.product_id = products.id").Group("categories_products.category_id").Where("categories_products.category_id = ?", id).Count(&view.Products)
	//
	return c.JSON(view)
}

type NewCategory struct {
	Name string
	Title string
	Description string
	ParentId uint
}

// @security BasicAuth
// CreateCategory godoc
// @Summary Create categories
// @Accept json
// @Produce json
// @Param parent_id query int false "Parent id"
// @Param category body NewCategory true "body"
// @Success 200 {object} CategoriesView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/categories [post]
// @Tags category
func postCategoriesHandler(c *fiber.Ctx) error {
	var view CategoryView
	var request NewCategory
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	//
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			if err := c.BodyParser(&request); err != nil {
				return err
			}
		} else if strings.HasPrefix(contentType, fiber.MIMEApplicationForm) {
			data := make(map[string][]string)
			c.Request().PostArgs().VisitAll(func(key []byte, val []byte) {
				data[string(key)] = append(data[string(key)], string(val))
			})
		} else if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				return err
			}
			var pid int
			if v, found := data.Value["ParentId"]; found && len(v) > 0 {
				if vv, err :=  strconv.Atoi(v[0]); err == nil {
					pid = vv
				}
			}
			if pid > 0 {
				if _, err := models.GetCategory(common.Database, pid); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{"Parent category is not exists"})
				}
			}
			var name string
			if v, found := data.Value["Name"]; found && len(v) > 0 {
				name = strings.TrimSpace(v[0])
			}
			if name == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Invalid name"})
			}
			/*if category, err := models.GetCategoryByName(common.Database, name); err == nil {
				if parentCategory, err := models.GetCategory(common.Database, pid); err == nil {
					if category.ParentId == parentCategory.ID {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{"Name is already in use"})
					}
				}
			}*/
			var title string
			if v, found := data.Value["Title"]; found && len(v) > 0 {
				title = strings.TrimSpace(v[0])
			}
			if title == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Invalid title"})
			}
			var description string
			if v, found := data.Value["Description"]; found && len(v) > 0 {
				description = strings.TrimSpace(v[0])
			}
			var content string
			if v, found := data.Value["Content"]; found && len(v) > 0 {
				content = strings.TrimSpace(v[0])
			}
			category := &models.Category{Name: name, Title: title, Description: description, Content: content, ParentId: request.ParentId}
			if id, err := models.CreateCategory(common.Database, category); err == nil {
				if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
					p := path.Join(dir, "storage", "categories")
					if _, err := os.Stat(p); err != nil {
						if err = os.MkdirAll(p, 0755); err != nil {
							logger.Errorf("%v", err)
						}
					}
					filename := fmt.Sprintf("%d-%s%s", id, category.Name, path.Ext(v[0].Filename))
					if p := path.Join(p, filename); len(p) > 0 {
						if in, err := v[0].Open(); err == nil {
							out, err := os.OpenFile(p, os.O_WRONLY | os.O_CREATE, 0644)
							if err != nil {
								c.Status(http.StatusInternalServerError)
								return c.JSON(HTTPError{err.Error()})
							}
							defer out.Close()
							if _, err := io.Copy(out, in); err != nil {
								c.Status(http.StatusInternalServerError)
								return c.JSON(HTTPError{err.Error()})
							}
							// TODO: Update category with Thumbnail
							category.Thumbnail = "/" + path.Join("categories", filename)
							if err = models.UpdateCategory(common.Database, category); err != nil {
								c.Status(http.StatusInternalServerError)
								return c.JSON(HTTPError{err.Error()})
							}
						}
					}
				}
				if bts, err := json.Marshal(category); err == nil {
					if err = json.Unmarshal(bts, &view); err != nil {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
				}
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	return c.JSON(view)
}

type AutocompleteRequest struct {
	Search string
	Include string
}

type CategoriesAutocompleteResponse []*CategoriesAutocompleteItem

type CategoriesAutocompleteItem struct {
	ID uint
	Path string
	Name string
	Title string
}

// @security BasicAuth
// SearchCategoriesAutocomplete godoc
// @Summary Search categories autocomplete
// @Accept json
// @Produce json
// @Param category body AutocompleteRequest true "body"
// @Success 200 {object} CategoriesAutocompleteResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/categories/autocomplete [post]
// @Tags category
func postCategoriesAutocompleteHandler(c *fiber.Ctx) error {
	var response CategoriesAutocompleteResponse
	var request AutocompleteRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	request.Search = strings.TrimSpace(request.Search)
	var keys []string
	var values []interface{}
	if request.Search == "" {
		if request.Include == "" {
			keys = append(keys, "parent_id = ?")
			values = append(values, 0)
		} else {
			for _, id := range strings.Split(request.Include, ",") {
				keys = append(keys, "id = ?")
				values = append(values, id)
			}
		}
	} else {
		keys = append(keys, "title like ?")
		values = append(values, "%" + request.Search + "%")
	}
	//logger.Infof("request: %+v", request)
	//
	var categories []*models.Category
	if err := common.Database.Debug().Where(strings.Join(keys, " or "), values...).Limit(10).Find(&categories).Error; err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	for _, category := range categories {
		item := &CategoriesAutocompleteItem {
			ID: category.ID,
			Name: category.Name,
			Title: category.Title,
		}
		//
		if category.ParentId != 0 {
			breadcrumbs := &[]*models.Category{}
			var f3 func(connector *gorm.DB, id uint)
			f3 = func(connector *gorm.DB, id uint) {
				if id != 0 {
					if category, err := models.GetCategory(connector, int(id)); err == nil {
						*breadcrumbs = append([]*models.Category{category}, *breadcrumbs...)
						f3(connector, category.ParentId)
					}
				}
			}
			f3(common.Database, category.ParentId)
			var titles []string
			for _, breadcrumb := range *breadcrumbs {
				titles = append(titles, breadcrumb.Title)
			}
			//
			item.Path = "/ " + strings.Join(titles, " / ") + " /"
		}
		response = append(response, item)
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

type ListRequest struct{
	Filter map[string]string
	Sort map[string]string
	Start int
	Length int
}

type CategoriesListResponse struct {
	Data []CategoriesListItem
	Filtered int64
	Total int64
}

type CategoriesListItem struct {
	ID uint
	Name string
	Title string
	Thumbnail string
	Description string
	Products int
}

// @security BasicAuth
// SearchCategories godoc
// @Summary Search categories
// @Accept json
// @Produce json
// @Param parent_id query int false "Parent id"
// @Param request body ListRequest true "body"
// @Success 200 {object} CategoriesListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/categories/list [post]
// @Tags category
func postCategoriesListHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Query("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var response CategoriesListResponse
	var request ListRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	if len(request.Sort) == 0 {
		request.Sort["Title"] = "asc"
	}
	if request.Length == 0 {
		request.Length = 10
	}
	//logger.Infof("request: %+v", request)
	// Filter
	var keys1 []string
	var values1 []interface{}
	var keys2 []string
	var values2 []interface{}
	if len(request.Filter) > 0 {
		for key, value := range request.Filter {
			if key != "" && len(strings.TrimSpace(value)) > 0 {

				switch key {
				case "Products":
					v := strings.TrimSpace(value)
					if strings.Index(v, ">=") == 0 {
						if vv, err := strconv.Atoi(v[2:]); err == nil {
							keys2 = append(keys2, fmt.Sprintf("%v >= ?", key))
							values2 = append(values2, vv)
						}
					} else if strings.Index(v, "<=") == 0 {
						if vv, err := strconv.Atoi(v[2:]); err == nil {
							keys2 = append(keys2, fmt.Sprintf("%v <= ?", key))
							values2 = append(values2, vv)
						}
					} else if strings.Index(v, "!=") == 0 || strings.Index(v, "<>") == 0 {
						if vv, err := strconv.Atoi(v[2:]); err == nil {
							keys2 = append(keys2, fmt.Sprintf("%v <> ?", key))
							values2 = append(values2, vv)
						}
					} else if strings.Index(v, ">") == 0 {
						if vv, err := strconv.Atoi(v[1:]); err == nil {
							keys2 = append(keys2, fmt.Sprintf("%v > ?", key))
							values2 = append(values2, vv)
						}
					} else if strings.Index(v, "<") == 0 {
						if vv, err := strconv.Atoi(v[1:]); err == nil {
							keys2 = append(keys2, fmt.Sprintf("%v < ?", key))
							values2 = append(values2, vv)
						}
					} else {
						if vv, err := strconv.Atoi(v); err == nil {
							keys2 = append(keys2, fmt.Sprintf("%v = ?", key))
							values2 = append(values2, vv)
						}
					}
				default:
					keys1 = append(keys1, fmt.Sprintf("categories.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	var filtering = true
	if len(keys1) == 0 {
		filtering = false
		keys1 = append(keys1, "parent_id = ?")
		values1 = append(values1, id)
	}
	logger.Infof("keys1: %+v, values1: %+v", strings.Join(keys1, " and "), values1)
	//
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				orders = append(orders, fmt.Sprintf("%v %v", key, value))
			}
		}
		order = strings.Join(orders, ", ")
	}
	//logger.Infof("order: %+v", order)
	//
	rows, err := common.Database.Debug().Model(&models.Category{}).Select("categories.ID, categories.Name, categories.Title, categories.Thumbnail, categories.Description, count(categories_products.product_id) as Products").Joins("left join categories_products on categories_products.category_id = categories.id").Group("categories.id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item CategoriesListItem
				if err = common.Database.ScanRows(rows, &item); err == nil {
					
					response.Data = append(response.Data, item)
				} else {
					logger.Errorf("%v", err)
				}
			}
		}else{
			logger.Errorf("%v", err)
		}
		rows.Close()
	}
	//
	rows, err = common.Database.Debug().Model(&models.Category{}).Select("categories.ID, categories.Name, categories.Title, categories.Thumbnail, categories.Description, count(categories_products.product_id) as Products").Joins("left join categories_products on categories_products.category_id = categories.id").Group("categories.id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}

	if filtering {
		common.Database.Debug().Model(&models.Category{}).Where(strings.Join(keys1, " and "), values1...).Count(&response.Filtered)
		common.Database.Debug().Model(&models.Category{}).Count(&response.Total)
	}else{
		common.Database.Debug().Model(&models.Category{}).Where("parent_id = ?", id).Count(&response.Filtered)
		response.Total = response.Filtered
	}
	//
	c.Status(http.StatusOK)
	return c.JSON(response)
}



type CategoryFullView struct {
	ID uint
	Name string
	Title string
	Description string
	Thumbnail string
	Content string
	ParentId uint
}

// @security BasicAuth
// GetCategory godoc
// @Summary Get category
// @Accept json
// @Produce json
// @Param id path int true "Category ID"
// @Success 200 {object} CategoryFullView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/categories/{id} [get]
// @Tags category
func getCategoryHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if category, err := models.GetCategory(common.Database, id); err == nil {
		var view CategoryFullView
		if bts, err := json.MarshalIndent(category, "", "   "); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				return c.JSON(view)
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// @security BasicAuth
// UpdateCategory godoc
// @Summary Update category
// @Accept json
// @Produce json
// @Param category body NewCategory true "body"
// @Success 200 {object} CategoryView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/categories/{id} [put]
// @Tags category
func putCategoryHandler(c *fiber.Ctx) error {
	var view CategoryView
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var category *models.Category
	var err error
	if category, err = models.GetCategory(common.Database, id); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	var request NewCategory
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	//
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				return err
			}
			var pid int
			if v, found := data.Value["ParentId"]; found && len(v) > 0 {
				if vv, err :=  strconv.Atoi(v[0]); err == nil {
					pid = vv
				}
			}
			if pid > 0 {
				if _, err := models.GetCategory(common.Database, pid); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{"Parent category is not exists"})
				}
			}
			var name string
			if v, found := data.Value["Name"]; found && len(v) > 0 {
				name = strings.TrimSpace(v[0])
			}
			if name == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Invalid name"})
			}
			//
			/*if parentCategory, err := models.GetCategory(common.Database, pid); err == nil {
				for _, category := range models.GetChildrenOfCategoryById(common.Database, parentCategory.ID) {
					if int(category.ID) != id && category.Name == request.Name {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{"Name is already in use"})
					}
				}
			}*/
			//
			var title string
			if v, found := data.Value["Title"]; found && len(v) > 0 {
				title = strings.TrimSpace(v[0])
			}
			if title == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Invalid title"})
			}
			var description string
			if v, found := data.Value["Description"]; found && len(v) > 0 {
				description = strings.TrimSpace(v[0])
			}
			var content string
			if v, found := data.Value["Content"]; found && len(v) > 0 {
				content = strings.TrimSpace(v[0])
			}
			category.Name = name
			category.Title = title
			category.Description = description
			category.Content = content
			if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
				p := path.Join(dir, "storage", "categories")
				if _, err := os.Stat(p); err != nil {
					if err = os.MkdirAll(p, 0755); err != nil {
						logger.Errorf("%v", err)
					}
				}
				filename := fmt.Sprintf("%d-%s%s", id, category.Name, path.Ext(v[0].Filename))
				if p := path.Join(p, filename); len(p) > 0 {
					if in, err := v[0].Open(); err == nil {
						out, err := os.OpenFile(p, os.O_WRONLY | os.O_CREATE, 0644)
						if err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
						defer out.Close()
						if _, err := io.Copy(out, in); err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
						// TODO: Update category with Thumbnail
						category.Thumbnail = "/" + path.Join("categories", filename)
					}
				}
			}
			if err = models.UpdateCategory(common.Database, category); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if bts, err := json.Marshal(category); err == nil {
				if err = json.Unmarshal(bts, &view); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	return c.JSON(view)
}

// @security BasicAuth
// DelCategory godoc
// @Summary Delete category
// @Accept json
// @Produce json
// @Param id path int true "Category ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/category/{id} [delete]
// @Tags category
func delCategoryHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if category, err := models.GetCategory(common.Database, id); err == nil {
		//
		categories := models.GetChildrenCategories(common.Database, category)
		for _, category := range categories {
			if category.Thumbnail != "" {
				p := path.Join(dir, "hugo", category.Thumbnail)
				if _, err := os.Stat(p); err == nil {
					if err = os.Remove(p); err != nil {
						logger.Errorf("%v", err.Error())
					}
				}
			}
			if err = models.DeleteCategory(common.Database, category); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}
		//
		if category.Thumbnail != "" {
			p := path.Join(dir, "hugo", category.Thumbnail)
			if _, err := os.Stat(p); err == nil {
				if err = os.Remove(p); err != nil {
					logger.Errorf("%v", err.Error())
				}
			}
		}
		if err = models.DeleteCategory(common.Database, category); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	c.Status(http.StatusOK)
	return c.JSON(HTTPMessage{"OK"})
}

type NodeView struct {
	*Meta `json:",omitempty"`
	Path string
	Name string
	File bool `json:",omitempty"`
	Modified *time.Time `json:",omitempty"`
	Size int64 `json:",omitempty"`
	Children []*NodeView `json:",omitempty"`
}

func (n *NodeView) getChild(name string) (*NodeView, error) {
	for _, ch := range n.Children {
		if ch.Name == name {
			return ch, nil
		}
	}
	return nil, errors.New("not found")
}

// @security BasicAuth
// GetContents godoc
// @Summary Get contents
// @Accept json
// @Produce json
// @Param home query int false "home"
// @Param depth query int false "depth"
// @Success 200 {object} NodeView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/contents [get]
// @Tags content
func getContentsHandler(c *fiber.Ctx) error {
	var root = &NodeView{Path: "/", Name: "/"}
	var home string
	base := path.Join(dir, "hugo", "content")
	if v := c.Query("home"); v != "" {
		home = strings.Replace(v, "../", "", -1)
		root.Path = home
	}
	base = path.Join(base, home)
	depth := -1
	if v := c.Query("depth"); v != "" {
		if vv, err := strconv.Atoi(v); err == nil && vv >= 0 {
			depth = vv
		}
	}
	if _, err := os.Stat(base); os.IsNotExist(err) {
		if err = os.MkdirAll(base, 0755); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}
	if err := filepath.Walk(base,
		func(filename string, info os.FileInfo, err error) error {
			//logger.Infof("filename: %+v, info: %+v, err: %+v", filename, info, err)
			if filename == path.Join(base, "products") {
				return filepath.SkipDir
			}
			if err != nil {
				return err
			}
			if x := strings.Replace(filename, base, "", 1); len(x) > 0 {
				if len(x) > 0 && x[0] == filepath.Separator {
					x = x[1:]
				}
				if strings.Count(x, string(filepath.Separator)) > depth && depth >= 0 {
					return filepath.SkipDir
				}
				p := root
				chunks := strings.Split(x, string(filepath.Separator))
				for i := 0; i < len(chunks); i++ {
					if n, err := p.getChild(chunks[i]); err == nil {
						p = n
					}else{
						n := &NodeView{Path: path.Join(home, x), Name: chunks[i]}
						if !info.IsDir() {
							n.File = true
							n.Size = info.Size()
							t := info.ModTime()
							n.Modified = &t
							//
							if path.Ext(info.Name()) == ".html" {
								//
								var page Page
								if bts, err := ioutil.ReadFile(filename); err == nil {
									if err = page.UnmarshalJSON(bts); err != nil {
										logger.Warningf("%+v", err)
									}
									n.Meta = page.Meta
								}
								//
								p.Children = append(p.Children, n)
							}
						}else{
							p.Children = append(p.Children, n)
						}
						sort.Slice(p.Children, func(i, j int) bool {
							if p.Children[i].File == p.Children[j].File {
								return p.Children[i].Name < p.Children[j].Name
							}else{
								return !p.Children[i].File
							}
						})
						p = n
					}
				}
			}
			return nil
		}); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	c.Status(http.StatusOK)
	return c.JSON(root)
}

func NewPage() Page {
	page := Page{}
	page.Meta = &Meta{}
	return page
}

type Page struct {
	*Meta
	//
	Content string
}

type Meta struct {
	Type string
	Title string
	Description string
	Draft bool
}

func (p *Page) MarshalJSON() ([]byte, error) {
	if bts, err := json.MarshalIndent(&struct {
		Type string
		Title string
		Description string
		Draft bool `json:",omitempty"`
	}{
		Type: p.Type,
		Title: p.Title,
		Description: p.Description,
		Draft: p.Draft,
	}, "", "   "); err == nil {
		bts = append(bts, "\n\n"...)
		bts = append(bts, p.Content...)
		return bts, nil
	}else{
		return []byte{}, err
	}
}

func (p *Page) UnmarshalJSON(data []byte) error {
	if n := bytes.Index(data, []byte("\n\n")); n > -1 {
		type Alias Page
		v := &struct {
			*Alias
		}{
			Alias: (*Alias)(p),
		}
		if err := json.Unmarshal(data[:n], &v); err != nil {
			return err
		}
		v.Content = strings.TrimSpace(string(data[n:]))
	}
	return nil
}

type FileView struct {
	*Meta
	Url string
	Name string
	Content string
	Modified time.Time
}

// @security BasicAuth
// GetContent godoc
// @Summary Get content
// @Accept json
// @Produce json
// @Success 200 {object} FileView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/contents/{any} [get]
// @Tags content
func getContentHandler(c *fiber.Ctx) error {
	thefilepath := strings.Replace(string(c.Request().URI().Path()), "/api/v1/contents", "", 1)
	p := path.Join(dir, "hugo", "content", thefilepath)
	if fi, err := os.Stat(p); err == nil {
		var page Page
		if bts, err := ioutil.ReadFile(p); err == nil {
			if err = page.UnmarshalJSON(bts); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			view := FileView{}
			ext := filepath.Ext(thefilepath)
			view.Url = thefilepath[0:len(thefilepath)-len(ext)]
			view.Name = filepath.Base(thefilepath)
			view.Meta = page.Meta
			view.Content = page.Content
			view.Modified = fi.ModTime()
			return c.JSON(view)
		}else{
			logger.Errorf("%v", err)
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

type ContentAction struct {
	Action string // create
	Type string // folder, file
	Name string // new name
	Title string
}

// @security BasicAuth
// PatchContent godoc
// @Summary Patch content
// @Accept json
// @Produce json
// @Param action body ContentAction true "body"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/contents/{any} [post]
// @Tags content
func patchContentHandler(c *fiber.Ctx) error {
	thefilepath := strings.Replace(string(c.Request().URI().Path()), "/api/v1/contents", "", 1)
	p := path.Join(dir, "hugo", "content", thefilepath)
	var request ContentAction
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	switch request.Action {
	case "create":
		switch request.Type {
		case "folder":
			if err := os.MkdirAll(p, 0755); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			page := NewPage()
			page.Title = request.Title
			if bts, err := page.MarshalJSON(); err == nil {
				if err = ioutil.WriteFile(path.Join(p, "index.html"), bts, 0644); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}
			c.Status(http.StatusOK)
			return c.JSON(HTTPMessage{"OK"})
		case "file":
			if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			page := NewPage()
			page.Title = request.Title
			if bts, err := page.MarshalJSON(); err == nil {
				if err = ioutil.WriteFile(p, bts, 0644); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}
			c.Status(http.StatusOK)
			return c.JSON(HTTPMessage{"OK"})
		default:
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unknown type"})
		}
	case "draft": {
		if _, err := os.Stat(p); err == nil {
			if bts, err := ioutil.ReadFile(p); err == nil {
				var page Page
				if err = page.UnmarshalJSON(bts); err == nil {
					page.Draft = !page.Draft
				}
				if bts, err = page.MarshalJSON(); err == nil {
					if err = ioutil.WriteFile(p, bts, 0644); err == nil {
						c.Status(http.StatusOK)
						return c.JSON(HTTPMessage{"OK"})
					}else{
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
				}else{
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}
	case "duplicated":
		if _, err := os.Stat(p); err == nil {
			if bts, err := ioutil.ReadFile(p); err == nil {
				var page Page
				if err = page.UnmarshalJSON(bts); err == nil {
					page.Draft = !page.Draft
				}
				if bts, err = page.MarshalJSON(); err == nil {
					if err = ioutil.WriteFile(p, bts, 0644); err == nil {
						c.Status(http.StatusOK)
						return c.JSON(HTTPMessage{"OK"})
					}else{
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
				}else{
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	case "rename":
		if strings.Contains(request.Name, "/") {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Invalid name"})
		}
		if err := os.Rename(p, path.Join(path.Dir(p), request.Name)); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
		c.Status(http.StatusOK)
		return c.JSON(HTTPMessage{"OK"})
	case "remove":
		if err := os.RemoveAll(p); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
		c.Status(http.StatusOK)
		return c.JSON(HTTPMessage{"OK"})
	default:
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Unknown action"})
	}
}

// @security BasicAuth
// PostContent godoc
// @Summary Post content
// @Accept json
// @Produce json
// @Param category body NewProduct true "body"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/contents/{any} [post]
// @Tags content
func postContentHandler(c *fiber.Ctx) error {
	thefilepath := strings.Replace(string(c.Request().URI().Path()), "/api/v1/contents", "", 1)
	p := path.Join(dir, "hugo", "content", thefilepath)
	var request FileView
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	page := NewPage()
	logger.Infof("Page: %+v", page)
	page.Type = request.Type
	page.Title = request.Title
	page.Description = request.Description
	page.Draft = request.Draft
	page.Content = request.Content
	if bts, err := page.MarshalJSON(); err == nil {
		if err = ioutil.WriteFile(path.Join(p, request.Name), bts, 0644); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	c.Status(http.StatusOK)
	return c.JSON(HTTPMessage{"OK"})
}

// @security BasicAuth
// UpdateContent godoc
// @Summary Put content
// @Accept json
// @Produce json
// @Success 200 {object} FileView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/contents/{any} [put]
// @Tags content
func putContentHandler(c *fiber.Ctx) error {
	thefilepath := strings.Replace(string(c.Request().URI().Path()), "/api/v1/contents", "", 1)
	p := path.Join(dir, "hugo", "content", thefilepath)
	var request FileView
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	//
	var page Page
	if bts, err := ioutil.ReadFile(p); err == nil {
		if err = page.UnmarshalJSON(bts); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		page = NewPage()
	}
	page.Title = request.Title
	page.Description = request.Description
	page.Draft = request.Draft
	page.Content = request.Content
	if bts, err := page.MarshalJSON(); err == nil {
		if _, err := os.Stat(path.Dir(p)); err != nil {
			if err = os.MkdirAll(path.Dir(p), 0755); err != nil {
				logger.Errorf("%v", err)
			}
		}
		// Rename
		if path.Base(p) != request.Name {
			if _, err := os.Stat(path.Join(path.Dir(p), request.Name)); err == nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{fmt.Sprintf("File %v already exists", request.Name)})
			}
			if err = os.Remove(p); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			p = path.Join(path.Dir(p), request.Name)
		}
		//
		if err = ioutil.WriteFile(p, bts, 0644); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	c.Status(http.StatusOK)
	return c.JSON(HTTPMessage{"OK"})
}

type OrdersListResponse struct {
	Data []OrdersListItem
	Filtered int64
	Total int64
}

type OrdersListItem struct {
	ID          uint
	Created time.Time
	Description string
	Total       float64
	Status      string
	UserId uint
	UserEmail   string
}

// @security BasicAuth
// SearchOrders godoc
// @Summary Search orders
// @Accept json
// @Produce json
// @Param order body ListRequest true "body"
// @Success 200 {object} OrdersListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/orders/list [post]
// @Tags order
func postOrdersListHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Query("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var userId int
	if v := c.Query("user_id"); v != "" {
		userId, _ = strconv.Atoi(v)
	}
	var response OrdersListResponse
	var request ListRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	if len(request.Sort) == 0 {
		request.Sort["ID"] = "desc"
	}
	if request.Length == 0 {
		request.Length = 10
	}
	// Filter
	var keys1 []string
	var values1 []interface{}
	if len(request.Filter) > 0 {
		for key, value := range request.Filter {
			if key != "" && len(strings.TrimSpace(value)) > 0 {
				switch key {
				case "UserEmail":
					keys1 = append(keys1, "users.Email like ?")
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				default:
					keys1 = append(keys1, fmt.Sprintf("orders.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	if id > 0 {
		keys1 = append(keys1, "orders.ID = ?")
		values1 = append(values1, id)
	}
	if userId > 0 {
		keys1 = append(keys1, "orders.User_Id = ?")
		values1 = append(values1, userId)
	}
	//logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
	//
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				case "UserEmail":
					orders = append(orders, fmt.Sprintf("users.Email %v", value))
				default:
					orders = append(orders, fmt.Sprintf("orders.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//logger.Infof("order: %+v", order)
	//
	func() {
		rows, err := common.Database.Debug().Model(&models.Order{}).Select("orders.ID, orders.Created_At as Created, orders.Description, orders.Total, orders.Status, orders.User_Id as UserId, users.Email as UserEmail").Joins("left join users on users.id = orders.user_id").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
		if err == nil {
			if err == nil {
				for rows.Next() {
					var item OrdersListItem
					if err = common.Database.ScanRows(rows, &item); err == nil {
						
						response.Data = append(response.Data, item)
					} else {
						logger.Errorf("%v", err)
					}
				}
			} else {
				logger.Errorf("%v", err)
			}
			rows.Close()
		}
	}()
	func() {
		rows, err := common.Database.Debug().Model(&models.Order{}).Select("orders.ID, orders.Created_At as Created, orders.Description, orders.Total, orders.Status, orders.User_Id as UserId, users.Email as UserEmail").Joins("left join users on users.id = orders.user_id").Where(strings.Join(keys1, " and "), values1...).Rows()
		if err == nil {
			for rows.Next() {
				response.Filtered++
			}
			rows.Close()
		}
	}()
	//
	if len(keys1) > 0 {
		common.Database.Debug().Model(&models.Order{}).Select("orders.ID, orders.Created_At as Created, orders.Description, orders.Total, orders.Status, orders.User_Id, users.Email as UserEmail, count(items.Id) as items").Joins("left join users on users.id = orders.user_id").Where(strings.Join(keys1, " and "), values1...).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	//
	c.Status(http.StatusOK)
	return c.JSON(response)
}

// GetOrder godoc
// @Summary Get order
// @Accept json
// @Produce json
// @Param id path int true "Order ID"
// @Success 200 {object} OrderView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/orders/{id} [get]
// @Tags order
func getOrderHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if order, err := models.GetOrder(common.Database, id); err == nil {
		var view OrderView
		if bts, err := json.MarshalIndent(order, "", "   "); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				return c.JSON(view)
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

type ExistingOrder struct {
	Status string
	Comment string
}

// @security BasicAuth
// UpdateOrder godoc
// @Summary update order
// @Accept json
// @Produce json
// @Param order body ExistingOrder true "body"
// @Param id path int true "Order ID"
// @Success 200 {object} OrderShortView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/orders/{id} [put]
// @Tags order
func putOrderHandler(c *fiber.Ctx) error {
	var request ExistingOrder
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "ID is not defined"})
	}
	var order *models.Order
	var err error
	if order, err = models.GetOrder(common.Database, int(id)); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	if len(request.Comment) > 1024 {
		request.Comment = request.Comment[0:1023]
	}
	order.Status = request.Status
	order.Comment = request.Comment
	if err := models.UpdateOrder(common.Database, order); err == nil {
		return c.JSON(HTTPMessage{"OK"})
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// @security BasicAuth
// DelOrder godoc
// @Summary Delete order
// @Accept json
// @Produce json
// @Param id path int true "Order ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/orders/{id} [delete]
// @Tags order
func delOrderHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if order, err := models.GetOrder(common.Database, id); err == nil {
		for _, item := range order.Items {
			if err = models.DeleteItem(common.Database, item); err != nil {
				logger.Errorf("%v", err)
			}
		}
		if err = models.DeleteOrder(common.Database, order); err == nil {
			return c.JSON(HTTPMessage{"OK"})
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

type TransactionsListResponse struct {
	Data []TransactionsListItem
	Filtered int64
	Total int64
}

type TransactionsListItem struct {
	ID uint
	CreatedAt time.Time
	Amount float64
	Status string
	OrderId uint
	UserId uint
	UserEmail string
	UpdatedAt time.Time
}

// @security BasicAuth
// SearchTransactions godoc
// @Summary Search transactions
// @Accept json
// @Produce json
// @Param request body ListRequest true "body"
// @Success 200 {object} TransactionsListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/transactions/list [post]
// @Tags transaction
func postTransactionsListHandler(c *fiber.Ctx) error {
	var response TransactionsListResponse
	var request ListRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	if len(request.Sort) == 0 {
		request.Sort["ID"] = "desc"
	}
	if request.Length == 0 {
		request.Length = 10
	}
	// Filter
	var keys1 []string
	var values1 []interface{}
	if len(request.Filter) > 0 {
		for key, value := range request.Filter {
			if key != "" && len(strings.TrimSpace(value)) > 0 {
				switch key {
				case "Amount":
					v := strings.TrimSpace(value)
					if strings.Index(v, ">=") == 0 {
						if vv, err := strconv.Atoi(v[2:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("transactions.%v >= ?", key))
							values1 = append(values1, vv)
						}
					} else if strings.Index(v, "<=") == 0 {
						if vv, err := strconv.Atoi(v[2:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("transactions.%v <= ?", key))
							values1 = append(values1, vv)
						}
					} else if strings.Index(v, "!=") == 0 || strings.Index(v, "<>") == 0 {
						if vv, err := strconv.Atoi(v[2:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("transactions.%v <> ?", key))
							values1 = append(values1, vv)
						}
					} else if strings.Index(v, ">") == 0 {
						if vv, err := strconv.Atoi(v[1:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("transactions.%v > ?", key))
							values1 = append(values1, vv)
						}
					} else if strings.Index(v, "<") == 0 {
						if vv, err := strconv.Atoi(v[1:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("transactions.%v < ?", key))
							values1 = append(values1, vv)
						}
					} else {
						if vv, err := strconv.Atoi(v); err == nil {
							keys1 = append(keys1, fmt.Sprintf("transactions.%v = ?", key))
							values1 = append(values1, vv)
						}
					}
				default:
					keys1 = append(keys1, fmt.Sprintf("transactions.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	//logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
	//
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				case "Values":
					orders = append(orders, fmt.Sprintf("%v %v", key, value))
				default:
					orders = append(orders, fmt.Sprintf("transactions.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//logger.Infof("order: %+v", order)
	//
	func() {
		rows, err := common.Database.Debug().Model(&models.Transaction{}).Select("transactions.ID, transactions.Created_At as CreatedAt, transactions.Amount, transactions.Status, transactions.Order_Id as OrderId, users.Id as UserId, users.Email as UserEmail, transactions.Updated_At as UpdatedAt").Joins("left join orders on orders.id = transactions.order_id").Joins("left join users on users.id = orders.user_id").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
		if err == nil {
			if err == nil {
				for rows.Next() {
					var item TransactionsListItem
					if err = common.Database.ScanRows(rows, &item); err == nil {
						response.Data = append(response.Data, item)
					} else {
						logger.Errorf("%v", err)
					}
				}
			} else {
				logger.Errorf("%v", err)
			}
			rows.Close()
		}
	}()
	func() {
		rows, err := common.Database.Debug().Model(&models.Transaction{}).Select("transactions.ID, transactions.Amount, transactions.Status, transactions.Order_Id as OrderId, users.Id as UserId, users.Email as UserEmail, transactions.Updated_At as UpdatedAt").Joins("left join orders on orders.id = transactions.order_id").Joins("left join users on users.id = orders.user_id").Where(strings.Join(keys1, " and "), values1...).Order(order).Rows()
		if err == nil {
			for rows.Next() {
				response.Filtered++
			}
			rows.Close()
		}
	}()
	if len(keys1) > 0 {
		common.Database.Debug().Model(&models.Transaction{}).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

type TransactionView struct {
	ID uint
	CreatedAt time.Time
	Amount float64
	Status string
	OrderId uint
	UserId uint
	UpdatedAt time.Time
}

// GetTransaction godoc
// @Summary Get transaction
// @Accept json
// @Produce json
// @Param id path int true "Transaction ID"
// @Success 200 {object} TransactionView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/transactions/{id} [get]
// @Tags transaction
func getTransactionHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if order, err := models.GetTransaction(common.Database, id); err == nil {
		var view TransactionView
		if bts, err := json.MarshalIndent(order, "", "   "); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				return c.JSON(view)
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

type ExistingTransaction struct {
	Status string
}

// @security BasicAuth
// UpdateTransaction godoc
// @Summary update transaction
// @Accept json
// @Produce json
// @Param transaction body ExistingTransaction true "body"
// @Param id path int true "Transaction ID"
// @Success 200 {object} TransactionView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/transactions/{id} [put]
// @Tags transaction
func putTransactionHandler(c *fiber.Ctx) error {
	var request ExistingTransaction
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "ID is not defined"})
	}
	var transaction *models.Transaction
	var err error
	if transaction, err = models.GetTransaction(common.Database, int(id)); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	transaction.Status = request.Status
	if err := models.UpdateTransaction(common.Database, transaction); err == nil {
		return c.JSON(HTTPMessage{"OK"})
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// @security BasicAuth
// DelTransaction godoc
// @Summary Delete transaction
// @Accept json
// @Produce json
// @Param id path int true "Transaction ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/transactions/{id} [delete]
// @Tags transaction
func delTransactionHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if transaction, err := models.GetTransaction(common.Database, id); err == nil {
		if err = models.DeleteTransaction(common.Database, transaction); err == nil {
			return c.JSON(HTTPMessage{"OK"})
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// Widgets
type NewWidget struct {
	Enabled bool
	Name string
	Title string
	Description string
	Content string
	Location string
	ApplyTo string
}

// @security BasicAuth
// CreateWidget godoc
// @Summary Create widget
// @Accept json
// @Produce json
// @Param option body NewWidget true "body"
// @Success 200 {object} WidgetView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/widgets [post]
// @Tags widget
func postWidgetHandler(c *fiber.Ctx) error {
	var view WidgetView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewWidget
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			request.Name = strings.TrimSpace(request.Name)
			if request.Name == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": "Name is not defined"})
			}
			request.Title = strings.TrimSpace(request.Title)
			if request.Title == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": "Title is not defined"})
			}
			if len(request.Description) > 256 {
				request.Description = request.Description[0:255]
			}
			widget := &models.Widget {
				Enabled: request.Enabled,
				Name: request.Name,
				Title: request.Title,
				Description: request.Description,
				Content: request.Content,
				Location: request.Location,
				ApplyTo: request.ApplyTo,
			}
			if _, err := models.CreateWidget(common.Database, widget); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if bts, err := json.Marshal(widget); err == nil {
				if err = json.Unmarshal(bts, &view); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}
			return c.JSON(view)
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	return c.JSON(view)
}

type WidgetsListResponse struct {
	Data []WidgetListItem
	Filtered int64
	Total int64
}

type WidgetListItem struct {
	ID uint
	Enabled bool
	Name string
	Title string
	Description string
	Location string
	ApplyTo string
}

// @security BasicAuth
// SearchWidgets godoc
// @Summary Search widgets
// @Accept json
// @Produce json
// @Param request body ListRequest true "body"
// @Success 200 {object} WidgetsListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/widgets/list [post]
// @Tags widget
func postWidgetsListHandler(c *fiber.Ctx) error {
	var response WidgetsListResponse
	var request ListRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	if len(request.Sort) == 0 {
		request.Sort["ID"] = "desc"
	}
	if request.Length == 0 {
		request.Length = 10
	}
	// Filter
	var keys1 []string
	var values1 []interface{}
	if len(request.Filter) > 0 {
		for key, value := range request.Filter {
			if key != "" && len(strings.TrimSpace(value)) > 0 {
				switch key {
				default:
					keys1 = append(keys1, fmt.Sprintf("widgets.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				default:
					orders = append(orders, fmt.Sprintf("widgets.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//
	rows, err := common.Database.Debug().Model(&models.Widget{}).Select("widgets.ID, widgets.Enabled, widgets.Name, widgets.Title, widgets.Description, widgets.Location, widgets.Apply_To as ApplyTo").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item WidgetListItem
				if err = common.Database.ScanRows(rows, &item); err == nil {
					response.Data = append(response.Data, item)
				} else {
					logger.Errorf("%v", err)
				}
			}
		}else{
			logger.Errorf("%v", err)
		}
		rows.Close()
	}
	rows, err = common.Database.Debug().Model(&models.Widget{}).Select("widgets.ID, widgets.Enabled, widgets.Name, widgets.Title, widgets.Description, widgets.Location, widgets.Apply_To as ApplyTo").Where(strings.Join(keys1, " and "), values1...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	if len(keys1) > 0 {
		common.Database.Debug().Model(&models.Coupon{}).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

type WidgetView struct {
	ID uint
	Enabled bool
	Name string `json:",omitempty"`
	Title string `json:",omitempty"`
	Description string `json:",omitempty"`
	Content string `json:",omitempty"`
	Location string `json:",omitempty"`
	ApplyTo string `json:",omitempty"`
	Categories []CategoryView `json:",omitempty"`
	Products []ProductShortView `json:",omitempty"`
}

// @security BasicAuth
// GetWidget godoc
// @Summary Get widget
// @Accept json
// @Produce json
// @Param id path int true "Widget ID"
// @Success 200 {object} WidgetView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/widgets/{id} [get]
// @Tags widget
func getWidgetHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if widget, err := models.GetWidget(common.Database, id); err == nil {
		var view WidgetView
		if bts, err := json.MarshalIndent(widget, "", "   "); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				return c.JSON(view)
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

type WidgetRequest struct {
	WidgetView
	Categories string
	Products string
}

// @security BasicAuth
// UpdateWidget godoc
// @Summary update widget
// @Accept json
// @Produce json
// @Param widget body WidgetRequest true "body"
// @Param id path int true "Coupon ID"
// @Success 200 {object} WidgetView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/widgets/{id} [put]
// @Tags widget
func putWidgetHandler(c *fiber.Ctx) error {
	var request WidgetRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "ID is not defined"})
	}
	var widget *models.Widget
	var err error
	if widget, err = models.GetWidget(common.Database, int(id)); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	widget.Enabled = request.Enabled
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Name is not defined"})
	}
	request.Title = strings.TrimSpace(request.Title)
	if request.Title == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Title is not defined"})
	}
	if len(request.Description) > 256 {
		request.Description = request.Description[0:255]
	}
	widget.Title = request.Title
	widget.Description = request.Description
	widget.Content = request.Content
	widget.Location = request.Location
	widget.ApplyTo = request.ApplyTo
	for _, v := range strings.Split(request.Categories, ",") {
		if id, err := strconv.Atoi(v); err == nil {
			if category, err := models.GetCategory(common.Database, id); err == nil {
				if err = models.AddCategoryToWidget(common.Database, widget, category); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}
		}
	}
	for _, v := range strings.Split(request.Products, ",") {
		if id, err := strconv.Atoi(v); err == nil {
			if product, err := models.GetProduct(common.Database, id); err == nil {
				if err = models.AddProductToWidget(common.Database, widget, product); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}
		}
	}
	if err := models.UpdateWidget(common.Database, widget); err == nil {
		var view WidgetView
		if bts, err := json.MarshalIndent(widget, "", "   "); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				return c.JSON(view)
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// @security BasicAuth
// DelWidget godoc
// @Summary Delete widget
// @Accept json
// @Produce json
// @Param id path int true "Widget ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/widgets/{id} [delete]
// @Tags widget
func delWidgetHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if widget, err := models.GetWidget(common.Database, id); err == nil {
		if err = models.DeleteWidget(common.Database, widget); err == nil {
			return c.JSON(HTTPMessage{MESSAGE: "OK"})
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

type MeView struct {
	ID uint
	Enabled bool
	Login string
	Email string
	EmailConfirmed bool
	//
	Name string
	Lastname string
	Company string
	Phone string
	Address string
	Zip string
	City string
	Region string
	Country string
	ITN string `json:",omitempty"`
	//
	Role int `json:",omitempty"`
	Profiles []ProfileView `json:",omitempty"`
	IsAdmin bool `json:",omitempty"`
}

type ProfileView struct {
	ID uint
	Name string
	Lastname string
	Company string
	Address string
	Zip string
	City string
	Region string
	Country string
	Phone string `json:",omitempty"`
	ITN string `json:",omitempty"`
	Billing bool `json:",omitempty"`
	TransportId uint
}

// @security BasicAuth
// GetMe godoc
// @Summary Get me
// @Accept json
// @Produce json
// @Success 200 {object} MeView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/me [get]
func getMeHandler(c *fiber.Ctx) error {
	if v := c.Locals("user"); v != nil {
		if user, ok := v.(*models.User); ok {
			var err error
			if user, err = models.GetUserFull(common.Database, user.ID); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			var view MeView
			if bts, err := json.Marshal(user); err == nil {
				if err = json.Unmarshal(bts, &view); err == nil {
					view.IsAdmin = user.Role < models.ROLE_USER
					return c.JSON(view)
				}else{
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}
	}
	return c.JSON(HTTPError{"Something went wrong"})
}

// Delivery
type DeliveryRequest struct {
	Items []NewItem
	Country string
	Zip string
}

// @security BasicAuth
// PostDelivery godoc
// @Description my description
// @Summary (DEPRECATED) Calculate shipping cost
// @Accept json
// @Produce json
// @Param request body DeliveryRequest true "body"
// @Success 200 {object} DeliveriesCosts
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/delivery [post]
// @Tags frontend
func postDeliveryHandler(c *fiber.Ctx) error {
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request DeliveryRequest
			if err := c.BodyParser(&request); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			// 1 Get all transports
			if transports, err := models.GetTransports(common.Database); err == nil {
				var costs []*DeliveryCost
				for _, transport := range transports {
					if transport.Enabled {
						// 2 Get Zone by Country and Zip
						var zoneId uint
						zone, err := models.GetZoneByCountryAndZIP(common.Database, request.Country, request.Zip)
						if err == nil {
							zoneId = zone.ID
						}
						// 3 Get Tariff by Transport and Zone
						tariff, _ := models.GetTariffByTransportIdAndZoneId(common.Database, transport.ID, zoneId)
						if cost, err := Delivery(transport, tariff, request.Items); err == nil {
							costs = append(costs, cost)
						}
					}
				}
				//
				c.Status(http.StatusOK)
				return c.JSON(costs)
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			//
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Content-Type not set"})
	}
}


// Discount
type DiscountRequest struct {
	Items []NewItem
	Coupons []string
	Country string
	Zip string
	TransportId int
}

type Discounts2View []*DiscountCost

// @security BasicAuth
// PostDiscount godoc
// @Summary Calculate discount cost
// @Accept json
// @Produce json
// @Param request body DiscountRequest true "body"
// @Success 200 {object} Discounts2View
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/discount [post]
// @Tags frontend
/*func postDiscountHandler(c *fiber.Ctx) error {
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request DiscountRequest
			if err := c.BodyParser(&request); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			// 1 Get delivery cost
			// 2 Coupons
			var coupons []*models.Coupon
			for _, code := range request.Coupons {
				if coupon, err := models.GetCouponByCode(common.Database, code); err == nil {
					coupons = append(coupons, coupon)
				}else{
					logger.Warningf("%+v", err)
				}
			}
			Discount(coupons, request.Items)
			//
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Content-Type not set"})
	}
}*/

// Tariffs
type TariffsView []TariffView

type TariffView struct{
	ID          uint
	TransportId uint
	ZoneId      uint
	Order    string
	Item    string
	Kg       float64
	M3    float64
}

// @security BasicAuth
// GetTariffs godoc
// @Summary Get tariffs
// @Accept json
// @Produce json
// @Success 200 {object} TariffsView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/tariffs [get]
func getTariffsHandler(c *fiber.Ctx) error {
	var zoneId int
	if v := c.Query("zoneId"); v != "" {
		if vv, err := strconv.Atoi(v); err == nil {
			zoneId = vv
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		return c.JSON(HTTPError{"zoneId required"})
	}
	if tariffs, err := models.GetTariffsByZoneId(common.Database, zoneId); err == nil {
		var view []TariffView
		if bts, err := json.Marshal(tariffs); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				return c.JSON(view)
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}


// Transport
type TransportsView []TransportView

type TransportView struct{
	ID uint
	Enabled bool
	Name string
	Title string
	Thumbnail string
	Weight float64
	Volume float64
	Order string
	Item string
	Kg float64
	M3 float64
	Free float64 `json:",omitempty"`
	Services string `json:",omitempty"`
}

// @security BasicAuth
// GetTransports godoc
// @Summary Get transports
// @Accept json
// @Produce json
// @Success 200 {object} TransportsView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/transports [get]
// @Tags transport
func getTransportsHandler(c *fiber.Ctx) error {
	if transports, err := models.GetTransports(common.Database); err == nil {
		sort.Slice(transports, func(i, j int) bool {
			return transports[i].Weight < transports[j].Weight
		})
		var view TransportsView
		if bts, err := json.MarshalIndent(transports, "", "   "); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				return c.JSON(view)
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

type NewTransport struct {
	Enabled bool
	Name string
	Title string
	Weight float64
	Volume float64
	Order string
	Item string
	Kg float64
	M3 float64
}

// @security BasicAuth
// CreateTransport godoc
// @Summary Create transport
// @Accept json
// @Produce json
// @Param transport body NewTransport true "body"
// @Success 200 {object} TransportView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/transports [post]
// @Tags transport
func postTransportHandler(c *fiber.Ctx) error {
	var view TransportView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			var enabled bool
			if v, found := data.Value["Enabled"]; found && len(v) > 0 {
				enabled, err = strconv.ParseBool(v[0])
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var name string
			if v, found := data.Value["Name"]; found && len(v) > 0 {
				name = strings.TrimSpace(v[0])
			}
			var title string
			if v, found := data.Value["Title"]; found && len(v) > 0 {
				title = strings.TrimSpace(v[0])
			}
			var weight float64
			if v, found := data.Value["Weight"]; found && len(v) > 0 {
				weight, err = strconv.ParseFloat(v[0],10)
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var volume float64
			if v, found := data.Value["Volume"]; found && len(v) > 0 {
				volume, err = strconv.ParseFloat(v[0],10)
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var order string
			if v, found := data.Value["Order"]; found && len(v) > 0 {
				order = strings.TrimSpace(v[0])
			}
			var item string
			if v, found := data.Value["Item"]; found && len(v) > 0 {
				item = strings.TrimSpace(v[0])
			}
			var kg float64
			if v, found := data.Value["Kg"]; found && len(v) > 0 {
				kg, err = strconv.ParseFloat(v[0],10)
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var m3 float64
			if v, found := data.Value["M3"]; found && len(v) > 0 {
				m3, err = strconv.ParseFloat(v[0],10)
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var free float64
			if v, found := data.Value["Free"]; found && len(v) > 0 {
				free, err = strconv.ParseFloat(v[0],10)
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var services string
			if v, found := data.Value["Services"]; found && len(v) > 0 {
				services = strings.TrimSpace(v[0])
			}
			transport := &models.Transport {
				Enabled: enabled,
				Name:    name,
				Title:   title,
				Weight:  weight,
				Volume:  volume,
				Order:   order,
				Item:    item,
				Kg:      kg,
				M3:      m3,
				Free: free,
				Services: services,
			}
			if id, err := models.CreateTransport(common.Database, transport); err == nil {
				if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
					p := path.Join(dir, "storage", "values")
					if _, err := os.Stat(p); err != nil {
						if err = os.MkdirAll(p, 0755); err != nil {
							logger.Errorf("%v", err)
						}
					}
					filename := fmt.Sprintf("%d-%s-thumbnail%s", id, regexp.MustCompile(`(?i)[^-a-z0-9]+`).ReplaceAllString(transport.Name, "-"), path.Ext(v[0].Filename))
					if p := path.Join(p, filename); len(p) > 0 {
						if in, err := v[0].Open(); err == nil {
							out, err := os.OpenFile(p, os.O_WRONLY | os.O_CREATE, 0644)
							if err != nil {
								c.Status(http.StatusInternalServerError)
								return c.JSON(HTTPError{err.Error()})
							}
							defer out.Close()
							if _, err := io.Copy(out, in); err != nil {
								c.Status(http.StatusInternalServerError)
								return c.JSON(HTTPError{err.Error()})
							}
							transport.Thumbnail = "/" + path.Join("transports", filename)
							if err = models.UpdateTransport(common.Database, transport); err != nil {
								c.Status(http.StatusInternalServerError)
								return c.JSON(HTTPError{err.Error()})
							}
						}
					}
				}
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if bts, err := json.Marshal(transport); err == nil {
				if err = json.Unmarshal(bts, &view); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}
			return c.JSON(view)
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	return c.JSON(view)
}

type TransportsListResponse struct {
	Data []TransportsListItem
	Filtered int64
	Total int64
}

type TransportsListItem struct {
	ID uint
	Enabled bool
	Name string
	Title string
	Weight float64
	Volume float64
	Order string
	Item string
	Kg float64
	M3 float64
}

// @security BasicAuth
// SearchTransports godoc
// @Summary Search transports
// @Accept json
// @Produce json
// @Param request body ListRequest true "body"
// @Success 200 {object} TransportsListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/transports/list [post]
// @Tags transport
func postTransportsListHandler(c *fiber.Ctx) error {
	var response TransportsListResponse
	var request ListRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	if len(request.Sort) == 0 {
		request.Sort["ID"] = "asc"
	}
	if request.Length == 0 {
		request.Length = 10
	}
	// Filter
	var keys1 []string
	var values1 []interface{}
	if len(request.Filter) > 0 {
		for key, value := range request.Filter {
			if key != "" && len(strings.TrimSpace(value)) > 0 {
				switch key {
				case "Order":
					keys1 = append(keys1, fmt.Sprintf("transports.`%v` like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				default:
					keys1 = append(keys1, fmt.Sprintf("transports.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	//logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
	//
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				case "Order":
					orders = append(orders, fmt.Sprintf("transports.`%v` %v", key, value))
				default:
					orders = append(orders, fmt.Sprintf("transports.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//logger.Infof("order: %+v", order)
	//
	rows, err := common.Database.Debug().Model(&models.Transport{}).Select("transports.ID, transports.Enabled, transports.Name, transports.Title, transports.Weight, transports.Volume, transports.`Order`, transports.Item, transports.Kg, transports.M3").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item TransportsListItem
				if err = common.Database.ScanRows(rows, &item); err == nil {
					response.Data = append(response.Data, item)
				} else {
					logger.Errorf("%v", err)
				}
			}
		}else{
			logger.Errorf("%v", err)
		}
		rows.Close()
	}
	rows, err = common.Database.Debug().Model(&models.Transport{}).Select("transports.ID, transports.Enabled, transports.Name, transports.Title, transports.Weight, transports.Volume, transports.`Order`, transports.Item, transports.Kg, transports.M3").Where(strings.Join(keys1, " and "), values1...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	if len(keys1) > 0 {
		common.Database.Debug().Model(&models.Transport{}).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

// @security BasicAuth
// GetTransport godoc
// @Summary Get transport
// @Accept json
// @Produce json
// @Param id path int true "Transport ID"
// @Success 200 {object} TransportView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/transports/{id} [get]
// @Tags transport
func getTransportHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if transport, err := models.GetTransport(common.Database, id); err == nil {
		var view TransportView
		if bts, err := json.MarshalIndent(transport, "", "   "); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				return c.JSON(view)
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// @security BasicAuth
// UpdateTransport godoc
// @Summary update transport
// @Accept json
// @Produce json
// @Param transport body TransportView true "body"
// @Param id path int true "Transport ID"
// @Success 200 {object} TagView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/transports/{id} [put]
// @Tags transport
func putTransportHandler(c *fiber.Ctx) error {
	var transport *models.Transport
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
		var err error
		if transport, err = models.GetTransport(common.Database, id); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "ID is not defined"})
	}
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			var enabled bool
			if v, found := data.Value["Enabled"]; found && len(v) > 0 {
				enabled, err = strconv.ParseBool(v[0])
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var title string
			if v, found := data.Value["Title"]; found && len(v) > 0 {
				title = strings.TrimSpace(v[0])
			}
			var weight float64
			if v, found := data.Value["Weight"]; found && len(v) > 0 {
				weight, err = strconv.ParseFloat(v[0],10)
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var volume float64
			if v, found := data.Value["Volume"]; found && len(v) > 0 {
				volume, err = strconv.ParseFloat(v[0],10)
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var order string
			if v, found := data.Value["Order"]; found && len(v) > 0 {
				order = strings.TrimSpace(v[0])
			}
			var item string
			if v, found := data.Value["Item"]; found && len(v) > 0 {
				item = strings.TrimSpace(v[0])
			}
			var kg float64
			if v, found := data.Value["Kg"]; found && len(v) > 0 {
				kg, err = strconv.ParseFloat(v[0],10)
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var m3 float64
			if v, found := data.Value["M3"]; found && len(v) > 0 {
				m3, err = strconv.ParseFloat(v[0],10)
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var free float64
			if v, found := data.Value["Free"]; found && len(v) > 0 {
				free, err = strconv.ParseFloat(v[0],10)
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var services string
			if v, found := data.Value["Services"]; found && len(v) > 0 {
				services = strings.TrimSpace(v[0])
			}
			transport.Enabled = enabled
			transport.Title = title
			transport.Weight = weight
			transport.Volume = volume
			transport.Order = order
			transport.Item = item
			transport.Kg = kg
			transport.M3 = m3
			transport.Free = free
			transport.Services = services
			if v, found := data.Value["Thumbnail"]; found && len(v) > 0 && v[0] == "" {
				// To delete existing
				if transport.Thumbnail != "" {
					if err = os.Remove(path.Join(dir, transport.Thumbnail)); err != nil {
						logger.Errorf("%v", err)
					}
					transport.Thumbnail = ""
				}
			}else if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
				p := path.Join(dir, "storage", "transports")
				if _, err := os.Stat(p); err != nil {
					if err = os.MkdirAll(p, 0755); err != nil {
						logger.Errorf("%v", err)
					}
				}
				filename := fmt.Sprintf("%d-%s-thumbnail%s", id, regexp.MustCompile(`(?i)[^-a-z0-9]+`).ReplaceAllString(transport.Name, "-"), path.Ext(v[0].Filename))
				if p := path.Join(p, filename); len(p) > 0 {
					if in, err := v[0].Open(); err == nil {
						out, err := os.OpenFile(p, os.O_WRONLY | os.O_CREATE, 0644)
						if err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
						defer out.Close()
						if _, err := io.Copy(out, in); err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
						transport.Thumbnail = "/" + path.Join("transports", filename)
						if err = models.UpdateTransport(common.Database, transport); err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
					}
				}
			}
			//
			if err := models.UpdateTransport(common.Database, transport); err == nil {
				var view TransportView
				if bts, err := json.Marshal(transport); err == nil {
					if err = json.Unmarshal(bts, &view); err == nil {
						return c.JSON(view)
					}else{
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
				}else{
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	c.Status(http.StatusInternalServerError)
	return c.JSON(HTTPError{"Something went wrong"})
}

// @security BasicAuth
// DelTransport godoc
// @Summary Delete transport
// @Accept json
// @Produce json
// @Param id path int true "Transport ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/transports/{id} [delete]
// @Tags transport
func delTransportHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if transport, err := models.GetTransport(common.Database, id); err == nil {
		if err = models.DeleteTransport(common.Database, transport); err == nil {
			return c.JSON(HTTPMessage{MESSAGE: "OK"})
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// Zone
type ZonesView []ZoneView

type ZoneView struct{
	ID      uint
	Enabled bool
	Title string
	Country string
	ZIP     string
}

// @security BasicAuth
// GetZones godoc
// @Summary Get zones
// @Accept json
// @Produce json
// @Success 200 {object} ZonesView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/zones [get]
// @Tags zone
func getZonesHandler(c *fiber.Ctx) error {
	if tags, err := models.GetZones(common.Database); err == nil {
		var view ZonesView
		if bts, err := json.MarshalIndent(tags, "", "   "); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				return c.JSON(view)
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

type NewZone struct {
	Enabled bool
	Title string
	Country string
	ZIP     string
}

// @security BasicAuth
// CreateZone godoc
// @Summary Create zone
// @Accept json
// @Produce json
// @Param zone body NewZone true "body"
// @Success 200 {object} ZoneView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/zones [post]
// @Tags zone
func postZoneHandler(c *fiber.Ctx) error {
	var view ZoneView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request struct {
				NewZone
				Tariffs []struct {
					TransportId uint
					Order string
					Item string
					Kg float64
					M3 float64
				}
			}
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			//logger.Infof("request: %+v", request)
			request.Country = strings.TrimSpace(request.Country)
			if request.Country == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": "Country is not defined"})
			}
			request.ZIP = strings.TrimSpace(request.ZIP)
			if request.Title == "" {
				request.Title = request.Country + "-" + request.ZIP
			}
			if zones, err := models.GetZonesByCountryAndZIP(common.Database, request.Country, request.ZIP); err == nil && len(zones) > 0 {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Zone exists"})
			}
			zone := &models.Zone {
				Enabled: request.Enabled,
				Title: request.Title,
				Country: request.Country,
				ZIP: request.ZIP,
			}
			var zoneId uint
			var err error
			if zoneId, err = models.CreateZone(common.Database, zone); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			var tariffs []string
			// Fill tariffs
			for _, tariff := range request.Tariffs {
				if transport, err := models.GetTransport(common.Database, int(tariff.TransportId)); err == nil {
					if _, err := models.CreateTariff(common.Database, &models.Tariff{
						TransportId: tariff.TransportId,
						ZoneId:      zoneId,
						Order: tariff.Order,
						Item: tariff.Item,
						Kg:       tariff.Kg,
						M3:    tariff.M3,
					}); err == nil {
						tariffs = append(tariffs, fmt.Sprintf("%v: order=%v, item=%v, kg=%.2f, m3=%.3f", transport.Title, tariff.Order, tariff.Item, tariff.Kg, tariff.M3))
					} else {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
				}else{
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}
			//
			if len(tariffs) > 0 {
				zone.Description = strings.Join(tariffs, "; ")
				if err = models.UpdateZone(common.Database, zone); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}
			if bts, err := json.Marshal(zone); err == nil {
				if err = json.Unmarshal(bts, &view); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}
			return c.JSON(view)
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	return c.JSON(view)
}

type ZonesListResponse struct {
	Data []ZonesListItem
	Filtered int64
	Total int64
}

type ZonesListItem struct {
	ID      uint
	Enabled bool
	Title string
	Country string
	ZIP     string `gorm:"column:zip" json:"ZIP"`
	Description string
}

// @security BasicAuth
// SearchZones godoc
// @Summary Search zones
// @Accept json
// @Produce json
// @Param request body ListRequest true "body"
// @Success 200 {object} ZonesListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/zones/list [post]
// @Tags zone
func postZonesListHandler(c *fiber.Ctx) error {
	var response ZonesListResponse
	var request ListRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	if len(request.Sort) == 0 {
		request.Sort["ID"] = "desc"
	}
	if request.Length == 0 {
		request.Length = 10
	}
	// Filter
	var keys1 []string
	var values1 []interface{}
	if len(request.Filter) > 0 {
		for key, value := range request.Filter {
			if key != "" && len(strings.TrimSpace(value)) > 0 {
				switch key {
				default:
					keys1 = append(keys1, fmt.Sprintf("zones.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	//logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
	//
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				case "Values":
					orders = append(orders, fmt.Sprintf("%v %v", key, value))
				default:
					orders = append(orders, fmt.Sprintf("zones.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//logger.Infof("order: %+v", order)
	//
	rows, err := common.Database.Debug().Model(&models.Zone{}).Select("zones.ID, zones.Enabled, zones.Title, zones.Country, zones.ZIP, zones.Description").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item ZonesListItem
				if err = common.Database.ScanRows(rows, &item); err == nil {
					response.Data = append(response.Data, item)
				} else {
					logger.Errorf("%v", err)
				}
			}
		}else{
			logger.Errorf("%v", err)
		}
		rows.Close()
	}
	rows, err = common.Database.Debug().Model(&models.Zone{}).Select("zones.ID, zone.Enabled, zones.Title, zones.Country, zones.ZIP, zones.Description").Where(strings.Join(keys1, " and "), values1...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	if len(keys1) > 0 {
		common.Database.Debug().Model(&models.Zone{}).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

// @security BasicAuth
// GetZone godoc
// @Summary Get zone
// @Accept json
// @Produce json
// @Param id path int true "Zone ID"
// @Success 200 {object} ZoneView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/zones/{id} [get]
// @Tags zone
func getZoneHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if zone, err := models.GetZone(common.Database, id); err == nil {
		var view ZoneView
		if bts, err := json.MarshalIndent(zone, "", "   "); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				return c.JSON(view)
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// @security BasicAuth
// UpdateZone godoc
// @Summary update zone
// @Accept json
// @Produce json
// @Param zone body ZoneView true "body"
// @Param id path int true "Zone ID"
// @Success 200 {object} ZoneView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/zones/{id} [put]
// @Tags zone
func putZoneHandler(c *fiber.Ctx) error {
	var request struct {
		ZoneView
		Tariffs []struct {
			TransportId uint
			Order    string
			Item    string
			Kg       float64
			M3    float64
		}
	}
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "ID is not defined"})
	}
	var zone *models.Zone
	var err error
	if zone, err = models.GetZone(common.Database, int(id)); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	request.Country = strings.TrimSpace(request.Country)
	if request.Country == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Country is not defined"})
	}
	request.ZIP = strings.TrimSpace(request.ZIP)
	if request.Title == "" {
		request.Title = request.Country + "-" + request.ZIP
	}
	zone.Enabled = request.Enabled
	zone.Title = request.Title
	zone.Country = request.Country
	zone.ZIP = request.ZIP
	var tariffs []string
	// Fill tariffs
	common.Database.Debug().Unscoped().Where("zone_id = ?", zone.ID).Delete(models.Tariff{})
	for _, tariff := range request.Tariffs {
		if transport, err := models.GetTransport(common.Database, int(tariff.TransportId)); err == nil {
			if _, err := models.CreateTariff(common.Database, &models.Tariff{
				TransportId: tariff.TransportId,
				ZoneId:      zone.ID,
				Order: tariff.Order,
				Item: tariff.Item,
				Kg: tariff.Kg,
				M3: tariff.M3,
			}); err == nil {
				tariffs = append(tariffs, fmt.Sprintf("%v: order=%v, item=%v, kg=%.2f, m3=%.3f", transport.Title, tariff.Order, tariff.Item, tariff.Kg, tariff.M3))
			} else {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}
	if len(tariffs) > 0 {
		zone.Description = strings.Join(tariffs, "; ")
	}
	if err := models.UpdateZone(common.Database, zone); err == nil {
		return c.JSON(ZoneView{ID: zone.ID, Enabled: zone.Enabled, Country: zone.Country, ZIP: zone.ZIP})
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// @security BasicAuth
// DelZone godoc
// @Summary Delete zone
// @Accept json
// @Produce json
// @Param id path int true "Zone ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/zones/{id} [delete]
// @Tags zone
func delZoneHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if zone, err := models.GetZone(common.Database, id); err == nil {
		if err = models.DeleteZone(common.Database, zone); err == nil {
			return c.JSON(HTTPMessage{MESSAGE: "OK"})
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// Notification Email
type EmailTemplatesView []EmailTemplateView

type EmailTemplateView struct{
	ID      uint
	Enabled bool
	Type string
	Topic string
	Message string
}

type NewEmailTemplate struct {
	Enabled bool
	Type string
	Topic string
	Message string
}

// @security BasicAuth
// CreateEmailTemplate godoc
// @Summary Create email template
// @Accept json
// @Produce json
// @Param zone body NewEmailTemplate true "body"
// @Success 200 {object} EmailTemplateView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/notification/email [post]
// @Tags notification
func postEmailTemplateHandler(c *fiber.Ctx) error {
	var view EmailTemplateView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewEmailTemplate
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			//logger.Infof("request: %+v", request)
			request.Type = strings.TrimSpace(request.Type)
			if request.Type == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": "Type is not defined"})
			}
			request.Topic = strings.TrimSpace(request.Topic)
			if request.Topic == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": "Topic is not defined"})
			}
			request.Message = strings.TrimSpace(request.Message)
			if request.Message == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": "Message is not defined"})
			}
			if template, err := models.GetEmailTemplateByType(common.Database, request.Type); err == nil && template != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Template exists"})
			}
			template := &models.EmailTemplate {
				Enabled: request.Enabled,
				Type: request.Type,
				Topic: request.Topic,
				Message: request.Message,
			}
			var err error
			if _, err = models.CreateEmailTemplate(common.Database, template); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if bts, err := json.Marshal(template); err == nil {
				if err = json.Unmarshal(bts, &view); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}
			return c.JSON(view)
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	return c.JSON(view)
}

type EmailsListResponse struct {
	Data []EmailsListItem
	Filtered int64
	Total int64
}

type EmailsListItem struct {
	ID uint
	Enabled bool
	Type string
	Topic string
}

// @security BasicAuth
// SearchEmails godoc
// @Summary Search email templates
// @Accept json
// @Produce json
// @Param request body ListRequest true "body"
// @Success 200 {object} EmailsListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/notification/email/list [post]
// @Tags notification
func postEmailTemplatesListHandler(c *fiber.Ctx) error {
	var response EmailsListResponse
	var request ListRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	if len(request.Sort) == 0 {
		request.Sort["ID"] = "desc"
	}
	if request.Length == 0 {
		request.Length = 10
	}
	// Filter
	var keys1 []string
	var values1 []interface{}
	if len(request.Filter) > 0 {
		for key, value := range request.Filter {
			if key != "" && len(strings.TrimSpace(value)) > 0 {
				switch key {
				default:
					keys1 = append(keys1, fmt.Sprintf("email_templates.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	//logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
	//
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				default:
					orders = append(orders, fmt.Sprintf("email_templates.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//logger.Infof("order: %+v", order)
	//
	rows, err := common.Database.Debug().Model(&models.EmailTemplate{}).Select("email_templates.ID, email_templates.Enabled, email_templates.Type, email_templates.Topic, email_templates.Message").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item EmailsListItem
				if err = common.Database.ScanRows(rows, &item); err == nil {
					response.Data = append(response.Data, item)
				} else {
					logger.Errorf("%v", err)
				}
			}
		}else{
			logger.Errorf("%v", err)
		}
		rows.Close()
	}
	rows, err = common.Database.Debug().Model(&models.EmailTemplate{}).Select("email_templates.ID, email_templates.Enabled, email_templates.Type, email_templates.Topic, email_templates.Message").Where(strings.Join(keys1, " and "), values1...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	if len(keys1) > 0 {
		common.Database.Debug().Model(&models.EmailTemplate{}).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

// @security BasicAuth
// GetEmailTemplate godoc
// @Summary Get email template
// @Accept json
// @Produce json
// @Param id path int true "EmailTemplate ID"
// @Success 200 {object} EmailTemplateView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/notification/email/{id} [get]
// @Tags notification
func getEmailTemplateHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if template, err := models.GetEmailTemplate(common.Database, id); err == nil {
		var view EmailTemplateView
		if bts, err := json.MarshalIndent(template, "", "   "); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				return c.JSON(view)
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// @security BasicAuth
// UpdateEmailTemplate godoc
// @Summary update email template
// @Accept json
// @Produce json
// @Param transport body EmailTemplateView true "body"
// @Param id path int true "Email Template ID"
// @Success 200 {object} EmailTemplateView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/notification/email/{id} [put]
// @Tags notification
func putEmailTemplateHandler(c *fiber.Ctx) error {
	var request struct {
		EmailTemplateView
		Name string
		Email string
	}
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "ID is not defined"})
	}
	var emailTemplate *models.EmailTemplate
	var err error
	if emailTemplate, err = models.GetEmailTemplate(common.Database, int(id)); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	emailTemplate.Enabled = request.Enabled
	emailTemplate.Type = strings.TrimSpace(request.Type)
	if emailTemplate.Type == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Type is not defined"})
	}
	emailTemplate.Topic = strings.TrimSpace(request.Topic)
	if emailTemplate.Topic == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Topic is not defined"})
	}
	emailTemplate.Message = strings.TrimSpace(request.Message)
	if emailTemplate.Message == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Message is not defined"})
	}
	if common.NOTIFICATION != nil && common.NOTIFICATION.SendGrid != nil && request.Email != "" {
		//
		/*vars := make(map[string]interface{})
		vars["OrderId"] = 123
		vars["OrderTotal"] = 1234.56
		vars["UserEmail"] = "test@mail.com"*/
		vars := &common.NotificationTemplateVariables{ }
		if order, err := models.GetOrderFull(common.Database, 30); err == nil {
			var orderView struct {
				models.Order
				Items []struct {
					ItemShortView
					Description string
				}
			}
			if bts, err := json.Marshal(order); err == nil {
				if err = json.Unmarshal(bts, &orderView); err == nil {
					for i := 0; i < len(orderView.Items); i++ {
						var itemView ItemShortView
						if err = json.Unmarshal([]byte(orderView.Items[i].Description), &itemView); err == nil {
							orderView.Items[i].Variation = itemView.Variation
							orderView.Items[i].Properties = itemView.Properties
						}
					}
					vars.Order = orderView
				} else {
					logger.Infof("%+v", err)
				}
			}
		}
		//
		if err = common.NOTIFICATION.SendEmail(mail.NewEmail(common.Config.Notification.Email.Name, common.Config.Notification.Email.Email), mail.NewEmail(request.Name, request.Email), emailTemplate.Topic, emailTemplate.Message, vars); err != nil {
			logger.Warningf("%+v", err)
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}
	if err := models.UpdateEmailTemplate(common.Database, emailTemplate); err == nil {
		return c.JSON(EmailTemplateView{ID: emailTemplate.ID, Enabled: emailTemplate.Enabled, Type: emailTemplate.Type, Topic: emailTemplate.Topic, Message: emailTemplate.Message})
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// @security BasicAuth
// DelEmailTemplate godoc
// @Summary Delete email template
// @Accept json
// @Produce json
// @Param id path int true "Email template ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/notification/email/{id} [delete]
// @Tags notification
func delEmailTemplateHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if template, err := models.GetEmailTemplate(common.Database, id); err == nil {
		if err = models.DeleteEmailTemplate(common.Database, template); err == nil {
			return c.JSON(HTTPMessage{MESSAGE: "OK"})
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// Vendor
type VendorsView []VendorView

// @security BasicAuth
// GetVendors godoc
// @Summary Get vendors
// @Accept json
// @Produce json
// @Success 200 {object} VendorsView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/vendors [get]
// @Tags vendor
func getVendorsHandler(c *fiber.Ctx) error {
	if vendors, err := models.GetVendors(common.Database); err == nil {
		var view VendorsView
		if bts, err := json.MarshalIndent(vendors, "", "   "); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				return c.JSON(view)
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

type VendorView struct{
	ID uint
	Enabled bool
	Name string
	Title string
	Thumbnail string
	Description string
	Content string
	Times []TimeView `json:",omitempty"`
}

type NewVendor struct {
	Enabled bool
	Name string
	Title string
	Description string
	Content string
}

// @security BasicAuth
// CreateVendor godoc
// @Summary Create vendor
// @Accept json
// @Produce json
// @Param vendor body NewVendor true "body"
// @Success 200 {object} VendorView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/vendors [post]
// @Tags vendor
func postVendorHandler(c *fiber.Ctx) error {
	var view VendorView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			var enabled bool
			if v, found := data.Value["Enabled"]; found && len(v) > 0 {
				enabled, err = strconv.ParseBool(v[0])
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var name string
			if v, found := data.Value["Name"]; found && len(v) > 0 {
				name = strings.TrimSpace(v[0])
			}
			var title string
			if v, found := data.Value["Title"]; found && len(v) > 0 {
				title = strings.TrimSpace(v[0])
			}
			var description string
			if v, found := data.Value["Description"]; found && len(v) > 0 {
				description = strings.TrimSpace(v[0])
			}
			var content string
			if v, found := data.Value["Content"]; found && len(v) > 0 {
				content = strings.TrimSpace(v[0])
			}
			vendor := &models.Vendor {
				Enabled: enabled,
				Name:    name,
				Title:   title,
				Description: description,
				Content: content,
			}
			if id, err := models.CreateVendor(common.Database, vendor); err == nil {
				if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
					p := path.Join(dir, "storage", "vendors")
					if _, err := os.Stat(p); err != nil {
						if err = os.MkdirAll(p, 0755); err != nil {
							logger.Errorf("%v", err)
						}
					}
					filename := fmt.Sprintf("%d-%s-thumbnail%s", id, regexp.MustCompile(`(?i)[^-a-z0-9]+`).ReplaceAllString(vendor.Name, "-"), path.Ext(v[0].Filename))
					if p := path.Join(p, filename); len(p) > 0 {
						if in, err := v[0].Open(); err == nil {
							out, err := os.OpenFile(p, os.O_WRONLY | os.O_CREATE, 0644)
							if err != nil {
								c.Status(http.StatusInternalServerError)
								return c.JSON(HTTPError{err.Error()})
							}
							defer out.Close()
							if _, err := io.Copy(out, in); err != nil {
								c.Status(http.StatusInternalServerError)
								return c.JSON(HTTPError{err.Error()})
							}
							vendor.Thumbnail = "/" + path.Join("vendors", filename)
							if err = models.UpdateVendor(common.Database, vendor); err != nil {
								c.Status(http.StatusInternalServerError)
								return c.JSON(HTTPError{err.Error()})
							}
						}
					}
				}
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if bts, err := json.Marshal(vendor); err == nil {
				if err = json.Unmarshal(bts, &view); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}
			return c.JSON(view)
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	return c.JSON(view)
}

type VendorsListResponse struct {
	Data []VendorsListItem
	Filtered int64
	Total int64
}

type VendorsListItem struct {
	ID uint
	Enabled bool
	Name string
	Title string
	Description string
	Content string
}

// @security BasicAuth
// SearchVendors godoc
// @Summary Search vendors
// @Accept json
// @Produce json
// @Param request body ListRequest true "body"
// @Success 200 {object} VendorsListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/vendors/list [post]
// @Tags vendor
func postVendorsListHandler(c *fiber.Ctx) error {
	var response VendorsListResponse
	var request ListRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	if len(request.Sort) == 0 {
		request.Sort["Title"] = "asc"
	}
	if request.Length == 0 {
		request.Length = 10
	}
	// Filter
	var keys1 []string
	var values1 []interface{}
	if len(request.Filter) > 0 {
		for key, value := range request.Filter {
			if key != "" && len(strings.TrimSpace(value)) > 0 {
				switch key {
				default:
					keys1 = append(keys1, fmt.Sprintf("vendors.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	//logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
	//
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				default:
					orders = append(orders, fmt.Sprintf("vendors.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//logger.Infof("order: %+v", order)
	//
	rows, err := common.Database.Debug().Model(&models.Vendor{}).Select("vendors.ID, vendors.Enabled, vendors.Name, vendors.Title, vendors.Description, vendors.Content").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item VendorsListItem
				if err = common.Database.ScanRows(rows, &item); err == nil {
					response.Data = append(response.Data, item)
				} else {
					logger.Errorf("%v", err)
				}
			}
		}else{
			logger.Errorf("%v", err)
		}
		rows.Close()
	}
	rows, err = common.Database.Debug().Model(&models.Vendor{}).Select("vendors.ID, vendors.Enabled, vendors.Name, vendors.Title, vendors.Description, vendors.Content").Where(strings.Join(keys1, " and "), values1...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	if len(keys1) > 0 {
		common.Database.Debug().Model(&models.Vendor{}).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

// @security BasicAuth
// GetVendor godoc
// @Summary Get vendor
// @Accept json
// @Produce json
// @Param id path int true "Vendor ID"
// @Success 200 {object} VendorView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/vendors/{id} [get]
// @Tags vendor
func getVendorHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if vendor, err := models.GetVendor(common.Database, id); err == nil {
		var view VendorView
		if bts, err := json.MarshalIndent(vendor, "", "   "); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				return c.JSON(view)
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// @security BasicAuth
// UpdateVendor godoc
// @Summary update vendor
// @Accept json
// @Produce json
// @Param vendor body VendorView true "body"
// @Param id path int true "Vendor ID"
// @Success 200 {object} TagView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/vendors/{id} [put]
// @Tags vendor
func putVendorHandler(c *fiber.Ctx) error {
	var vendor *models.Vendor
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
		var err error
		if vendor, err = models.GetVendor(common.Database, id); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "ID is not defined"})
	}
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			var enabled bool
			if v, found := data.Value["Enabled"]; found && len(v) > 0 {
				enabled, err = strconv.ParseBool(v[0])
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var title string
			if v, found := data.Value["Title"]; found && len(v) > 0 {
				title = strings.TrimSpace(v[0])
			}
			var description string
			if v, found := data.Value["Description"]; found && len(v) > 0 {
				description = strings.TrimSpace(v[0])
			}
			var content string
			if v, found := data.Value["Content"]; found && len(v) > 0 {
				content = strings.TrimSpace(v[0])
			}
			vendor.Enabled = enabled
			vendor.Title = title
			vendor.Description = description
			vendor.Content = content
			if v, found := data.Value["Thumbnail"]; found && len(v) > 0 && v[0] == "" {
				// To delete existing
				if vendor.Thumbnail != "" {
					if err = os.Remove(path.Join(dir, vendor.Thumbnail)); err != nil {
						logger.Errorf("%v", err)
					}
					vendor.Thumbnail = ""
				}
			}else if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
				p := path.Join(dir, "storage", "vendors")
				if _, err := os.Stat(p); err != nil {
					if err = os.MkdirAll(p, 0755); err != nil {
						logger.Errorf("%v", err)
					}
				}
				filename := fmt.Sprintf("%d-%s-thumbnail%s", id, regexp.MustCompile(`(?i)[^-a-z0-9]+`).ReplaceAllString(vendor.Name, "-"), path.Ext(v[0].Filename))
				if p := path.Join(p, filename); len(p) > 0 {
					if in, err := v[0].Open(); err == nil {
						out, err := os.OpenFile(p, os.O_WRONLY | os.O_CREATE, 0644)
						if err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
						defer out.Close()
						if _, err := io.Copy(out, in); err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
						vendor.Thumbnail = "/" + path.Join("vendors", filename)
						if err = models.UpdateVendor(common.Database, vendor); err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
					}
				}
			}
			// Times
			if err = models.DeleteAllTimesFromVendor(common.Database, vendor); err != nil {
				logger.Errorf("%v", err)
			}
			if v, found := data.Value["Times"]; found && len(v) > 0 {
				for _, vv := range strings.Split(strings.TrimSpace(v[0]), ",") {
					if timeId, err := strconv.Atoi(strings.TrimSpace(vv)); err == nil {
						if time, err := models.GetTime(common.Database, timeId); err == nil {
							if err = models.AddTimeToVendor(common.Database, time, vendor); err != nil {
								logger.Errorf("%v", err)
							}
						}else{
							logger.Errorf("%v", err)
						}
					}else{
						logger.Errorf("%v", err)
					}
				}
			}
			//
			if err := models.UpdateVendor(common.Database, vendor); err == nil {
				if vendor, err := models.GetVendor(common.Database, id); err == nil {
					var view VendorView
					if bts, err := json.Marshal(vendor); err == nil {
						if err = json.Unmarshal(bts, &view); err == nil {
							return c.JSON(view)
						}else{
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
					}else{
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
				}else{
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	c.Status(http.StatusInternalServerError)
	return c.JSON(HTTPError{"Something went wrong"})
}

// @security BasicAuth
// DelVendor godoc
// @Summary Delete vendor
// @Accept json
// @Produce json
// @Param id path int true "Vendor ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/vendors/{id} [delete]
// @Tags vendor
func delVendorHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if vendor, err := models.GetVendor(common.Database, id); err == nil {
		if thumbnail := vendor.Thumbnail; thumbnail != "" {
			if err = os.Remove(path.Join(dir, "storage", thumbnail)); err != nil {
				logger.Warningf("%v", err.Error())
			}
		}
		if err = models.DeleteVendor(common.Database, vendor); err == nil {
			return c.JSON(HTTPMessage{MESSAGE: "OK"})
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// Times

type TimesView []TimeView

// @security BasicAuth
// GetTimes godoc
// @Summary Get times
// @Accept json
// @Produce json
// @Success 200 {object} TimesView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/times [get]
// @Tags time
func getTimesHandler(c *fiber.Ctx) error {
	var times []*models.Time
	var err error
	if v := c.Query("vid"); v != "" {
		id, _ := strconv.Atoi(v)
		if times, err = models.GetTimesByVendorId(common.Database, uint(id)); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		if times, err = models.GetTimes(common.Database); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}
	var view TimesView
	if bts, err := json.MarshalIndent(times, "", "   "); err == nil {
		if err = json.Unmarshal(bts, &view); err == nil {
			return c.JSON(view)
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

type NewTime struct {
	Enabled bool
	Name string
	Title string
}

// @security BasicAuth
// CreateTime godoc
// @Summary Create time
// @Accept json
// @Produce json
// @Param option body NewTime true "body"
// @Success 200 {object} TimeView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/times [post]
// @Tags time
func postTimeHandler(c *fiber.Ctx) error {
	var view TimeView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewTime
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			request.Name = strings.TrimSpace(request.Name)
			if request.Name == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": "Name is not defined"})
			}
			request.Title = strings.TrimSpace(request.Title)
			if request.Title == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": "Title is not defined"})
			}
			time := &models.Time {
				Enabled: request.Enabled,
				Name: request.Name,
				Title: request.Title,
			}
			
			if v := c.Query("vid"); v != "" {
				if id, err := strconv.Atoi(v); err == nil {
					if vendor, err := models.GetVendor(common.Database, id); err == nil {
						if oldTime, err := models.GetTimeByName(common.Database, request.Name); err == nil {
							time = oldTime
							if err = models.AddTimeToVendor(common.Database, oldTime, vendor); err != nil {
								logger.Warningf("%v", err.Error())
							}
						}else{
							if _, err := models.CreateTime(common.Database, time); err == nil {
								if err = models.AddTimeToVendor(common.Database, time, vendor); err != nil {
									logger.Warningf("%v", err.Error())
								}
							}
						}

					}
				}
			}else{
				if _, err := models.GetTimeByName(common.Database, request.Name); err == nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{"already exists"})
				}
				if _, err := models.CreateTime(common.Database, time); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}

			if bts, err := json.Marshal(time); err == nil {
				if err = json.Unmarshal(bts, &view); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}
			return c.JSON(view)
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	return c.JSON(view)
}

type TimesListResponse struct {
	Data []TimeListItem
	Filtered int64
	Total int64
}

type TimeListItem struct {
	ID uint
	Enabled bool
	Name string
	Title string
	Value int
}

// @security BasicAuth
// SearchTimes godoc
// @Summary Search times
// @Accept json
// @Produce json
// @Param request body ListRequest true "body"
// @Success 200 {object} TimesListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/times/list [post]
// @Tags time
func postTimesListHandler(c *fiber.Ctx) error {
	var response TimesListResponse
	var request ListRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	if len(request.Sort) == 0 {
		request.Sort["ID"] = "asc"
	}
	if request.Length == 0 {
		request.Length = 10
	}
	// Filter
	var keys1 []string
	var values1 []interface{}
	if len(request.Filter) > 0 {
		for key, value := range request.Filter {
			if key != "" && len(strings.TrimSpace(value)) > 0 {
				switch key {
				default:
					keys1 = append(keys1, fmt.Sprintf("times.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				default:
					orders = append(orders, fmt.Sprintf("times.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//
	rows, err := common.Database.Debug().Model(&models.Time{}).Select("times.ID, times.Enabled, times.Name, times.Title, times.Value").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item TimeListItem
				if err = common.Database.ScanRows(rows, &item); err == nil {
					response.Data = append(response.Data, item)
				} else {
					logger.Errorf("%v", err)
				}
			}
		}else{
			logger.Errorf("%v", err)
		}
		rows.Close()
	}
	rows, err = common.Database.Debug().Model(&models.Time{}).Select("times.ID, times.Enabled, times.Name, times.Title, times.Value").Where(strings.Join(keys1, " and "), values1...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	if len(keys1) > 0 {
		common.Database.Debug().Model(&models.Coupon{}).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

type TimeView struct {
	ID uint
	Enabled bool
	Name string `json:",omitempty"`
	Title string `json:",omitempty"`
	Value int `json:",omitempty"`
}

// @security BasicAuth
// GetTime godoc
// @Summary Get time
// @Accept json
// @Produce json
// @Param id path int true "Time ID"
// @Success 200 {object} TimeView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/times/{id} [get]
// @Tags time
func getTimeHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if time, err := models.GetTime(common.Database, id); err == nil {
		var view TimeView
		if bts, err := json.MarshalIndent(time, "", "   "); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				return c.JSON(view)
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

type TimeRequest struct {
	TimeView
	Categories string
	Products string
}

// @security BasicAuth
// UpdateTime godoc
// @Summary update time
// @Accept json
// @Produce json
// @Param time body TimeRequest true "body"
// @Param id path int true "Coupon ID"
// @Success 200 {object} TimeView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/times/{id} [put]
// @Tags time
func putTimeHandler(c *fiber.Ctx) error {
	var request TimeRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "ID is not defined"})
	}
	var time *models.Time
	var err error
	if time, err = models.GetTime(common.Database, int(id)); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	time.Enabled = request.Enabled
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Name is not defined"})
	}
	request.Title = strings.TrimSpace(request.Title)
	if request.Title == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Title is not defined"})
	}
	time.Title = request.Title
	if err := models.UpdateTime(common.Database, time); err == nil {
		var view TimeView
		if bts, err := json.MarshalIndent(time, "", "   "); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				return c.JSON(view)
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// @security BasicAuth
// DelTime godoc
// @Summary Delete time
// @Accept json
// @Produce json
// @Param id path int true "Time ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/times/{id} [delete]
// @Tags time
func delTimeHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if time, err := models.GetTime(common.Database, id); err == nil {
		if err = models.DeleteTime(common.Database, time); err == nil {
			return c.JSON(HTTPMessage{MESSAGE: "OK"})
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// Users

type UsersView []UserView

type UserView struct {
	ID uint
	Enabled bool
	Login string
	Email string
	EmailConfirmed bool
	//
	Name string
	Lastname string
	Company string
	Phone string
	Address string
	Zip string
	City string
	Region string
	Country string
	ITN string
	//
	Role int `json:",omitempty"`
	Notification bool
}

// @security BasicAuth
// @Summary Get users
// @Description get string
// @Accept json
// @Produce json
// @Success 200 {object} UsersView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/users [get]
// @Tags user
func getUsersHandler(c *fiber.Ctx) error {
	if users, err := models.GetUsers(common.Database); err == nil {
		var views []UserView
		if bts, err := json.Marshal(users); err == nil {
			if err = json.Unmarshal(bts, &views); err == nil {
				return c.JSON(views)
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

type UsersListResponse struct {
	Data []UsersListItem
	Filtered int64
	Total int64
}

type UsersListItem struct {
	ID uint
	CreatedAt time.Time
	Login string
	Email string
	EmailConfirmed bool
	Role int
	Orders int
	UpdatedAt time.Time
}

// @security BasicAuth
// SearchUsers godoc
// @Summary Search users
// @Accept json
// @Produce json
// @Param request body ListRequest true "body"
// @Success 200 {object} UsersListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/users/list [post]
// @Tags user
func postUsersListHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Query("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var response UsersListResponse
	var request ListRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	if len(request.Sort) == 0 {
		request.Sort["ID"] = "desc"
	}
	if request.Length == 0 {
		request.Length = 10
	}
	// Filter
	var keys1 []string
	var values1 []interface{}
	var keys2 []string
	var values2 []interface{}
	if len(request.Filter) > 0 {
		for key, value := range request.Filter {
			if key != "" && len(strings.TrimSpace(value)) > 0 {
				switch key {
				case "Orders":
					v := strings.TrimSpace(value)
					if strings.Index(v, ">=") == 0 {
						if vv, err := strconv.Atoi(v[2:]); err == nil {
							keys2 = append(keys2, fmt.Sprintf("%v >= ?", key))
							values2 = append(values2, vv)
						}
					} else if strings.Index(v, "<=") == 0 {
						if vv, err := strconv.Atoi(v[2:]); err == nil {
							keys2 = append(keys2, fmt.Sprintf("%v <= ?", key))
							values2 = append(values2, vv)
						}
					} else if strings.Index(v, "!=") == 0 || strings.Index(v, "<>") == 0 {
						if vv, err := strconv.Atoi(v[2:]); err == nil {
							keys2 = append(keys2, fmt.Sprintf("%v <> ?", key))
							values2 = append(values2, vv)
						}
					} else if strings.Index(v, ">") == 0 {
						if vv, err := strconv.Atoi(v[1:]); err == nil {
							keys2 = append(keys2, fmt.Sprintf("%v > ?", key))
							values2 = append(values2, vv)
						}
					} else if strings.Index(v, "<") == 0 {
						if vv, err := strconv.Atoi(v[1:]); err == nil {
							keys2 = append(keys2, fmt.Sprintf("%v < ?", key))
							values2 = append(values2, vv)
						}
					} else {
						if vv, err := strconv.Atoi(v); err == nil {
							keys2 = append(keys2, fmt.Sprintf("%v = ?", key))
							values2 = append(values2, vv)
						}
					}
				default:
					keys1 = append(keys1, fmt.Sprintf("users.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	if id > 0 {
		keys1 = append(keys1, "id = ?")
		values1 = append(values1, id)
	}
	//logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
	//
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				case "CreatedAt":
					orders = append(orders, fmt.Sprintf("users.Created_At %v", value))
				case "UpdatedAt":
					orders = append(orders, fmt.Sprintf("users.Updated_At %v", value))
				default:
					orders = append(orders, fmt.Sprintf("users.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//logger.Infof("order: %+v", order)
	//
	func() {
		rows, err := common.Database.Debug().Model(&models.User{}).Select("users.ID, users.Created_At as CreatedAt, users.Login, users.Email, users.Role, users.Email_Confirmed as EmailConfirmed, count(orders.Id) as Orders, users.Updated_At as UpdatedAt").Joins("left join orders on orders.User_Id = users.id").Group("users.id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
		if err == nil {
			if err == nil {
				for rows.Next() {
					var item UsersListItem
					if err = common.Database.ScanRows(rows, &item); err == nil {
						response.Data = append(response.Data, item)
					} else {
						logger.Errorf("%v", err)
					}
				}
			} else {
				logger.Errorf("%v", err)
			}
			rows.Close()
		}
	}()
	func() {
		rows, err := common.Database.Debug().Model(&models.User{}).Select("users.ID, users.Created_At as CreatedAt, users.Login, users.Email, users.Role, users.Email_Confirmed as EmailConfirmed, users.Updated_At as UpdatedAt").Joins("left join orders on orders.User_Id = users.id").Group("users.id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Order(order).Rows()
		if err == nil {
			for rows.Next() {
				response.Filtered++
			}
			rows.Close()
		}
	}()
	if len(keys1) > 0 || len(keys2) > 0 {
		common.Database.Debug().Model(&models.User{}).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

// @security BasicAuth
// @Summary Get user
// @Description get string
// @Accept json
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {object} UserView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/users/{id} [get]
// @Tags user
func getUserHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "ID is not defined"})
	}
	if user, err := models.GetUser(common.Database, id); err == nil {
		var view UserView
		if bts, err := json.Marshal(user); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				return c.JSON(view)
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}


type ExistingUser struct {
	Password       string
	Email          string
	EmailConfirmed bool
	Role           int
	Notification   bool
}

// @security BasicAuth
// UpdateUser godoc
// @Summary update user
// @Accept json
// @Produce json
// @Param user body ExistingUser true "body"
// @Param id path int true "User ID"
// @Success 200 {object} UserView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/users/{id} [put]
// @Tags user
func putUserHandler(c *fiber.Ctx) error {
	var request ExistingUser
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "ID is not defined"})
	}
	var user *models.User
	var err error
	if user, err = models.GetUser(common.Database, int(id)); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	request.Password = strings.TrimSpace(request.Password)
	if len(request.Password) > 0 {
		if len(request.Password) < 4 {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Password should be at least 4 length"})
		}
		user.Password = models.MakeUserPassword(request.Password)
	}
	request.Email = strings.TrimSpace(request.Email)
	if len(request.Email) > 0 {
		user.Email = request.Email
	}
	user.EmailConfirmed = request.EmailConfirmed
	if request.Role > 0 {
		user.Role = request.Role
	}
	user.Notification = request.Notification
	if err := models.UpdateUser(common.Database, user); err == nil {
		return c.JSON(HTTPMessage{"OK"})
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// @security BasicAuth
// DelUser godoc
// @Summary Delete user
// @Accept json
// @Produce json
// @Param id path int true "User ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/users/{id} [delete]
// @Tags user
func delUserHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if user, err := models.GetUser(common.Database, id); err == nil {
		if err = models.DeleteUser(common.Database, user); err == nil {
			return c.JSON(HTTPMessage{"OK"})
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

type NewCommand struct {

}

type CommandView struct {
	Output string
	Error string `json:"ERROR,omitempty"`
	Status string
}

// @security BasicAuth
// MakePrepare godoc
// @Summary Make prepare
// @Accept json
// @Produce json
// @Param request body NewCommand true "body"
// @Success 200 {object} CommandView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/prepare [post]
func postPrepareHandler(c *fiber.Ctx) error {
	var view CommandView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewCommand
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			//
			cmd := exec.Command(os.Args[0], "render", "-p", path.Join(dir, "hugo", "content"))
			buff := &bytes.Buffer{}
			cmd.Stderr = buff
			cmd.Stdout = buff
			err := cmd.Run()
			if err != nil {
				view.Output = buff.String()
				view.Error = err.Error()
				logger.Errorf("%v\n%v", view.Output, view.Error)
				c.Status(http.StatusInternalServerError)
				return c.JSON(view)
			}
			view.Output = buff.String()
			view.Status = "OK"
			//
			return c.JSON(view)
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	return c.JSON(view)
}

// @security BasicAuth
// MakeRender godoc
// @Summary Make render
// @Accept json
// @Produce json
// @Param request body NewCommand true "body"
// @Success 200 {object} CommandView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/render [post]
func postRenderHandler(c *fiber.Ctx) error {
	var view CommandView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewCommand
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			//
			bin := strings.Split(common.Config.Hugo.Bin, " ")
			//
			var arguments []string
			if len(bin) > 1 {
				for _, x := range bin[1:]{
					x = strings.Replace(x, "%DIR%", dir, -1)
					arguments = append(arguments, x)
				}
			}
			arguments = append(arguments, "--cleanDestinationDir")
			if common.Config.Hugo.Minify {
				arguments = append(arguments, "--minify")
			}
			if len(bin) == 1 {
				arguments = append(arguments, []string{"-s", path.Join(dir, "hugo")}...)
			}
			cmd := exec.Command(bin[0], arguments...)
			buff := &bytes.Buffer{}
			cmd.Stderr = buff
			cmd.Stdout = buff
			err := cmd.Run()
			if err != nil {
				view.Output = buff.String()
				view.Error = err.Error()
				logger.Errorf("%v\n%v", view.Output, view.Error)
				c.Status(http.StatusInternalServerError)
				return c.JSON(view)
			}
			view.Output = buff.String()
			view.Status = "OK"
			//
			return c.JSON(view)
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	return c.JSON(view)
}

type NewPublish struct {

}

type PublishView struct {
	Output string
	Status string
}

// @security BasicAuth
// MakePublish godoc
// @Summary Make publish
// @Accept json
// @Produce json
// @Param request body NewCommand true "body"
// @Success 200 {object} CommandView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/publish [post]
func postPublishHandler(c *fiber.Ctx) error {
	var view CommandView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewCommand
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			//
			if !common.Config.Wrangler.Enabled{
				err := fmt.Errorf("wrangler disabled")
				logger.Errorf("%v", err.Error())
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			//
			bin := strings.Split(common.Config.Wrangler.Bin, " ")
			//
			var arguments []string
			if len(bin) > 1 {
				for _, x := range bin[1:]{
					x = strings.Replace(x, "%DIR%", dir, -1)
					arguments = append(arguments, x)
				}
				if common.Config.Wrangler.ApiToken == "" {
					err := fmt.Errorf("api_token is not specified")
					logger.Errorf("%v", err.Error())
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
				arguments = append(arguments, common.Config.Wrangler.ApiToken)
			}
			//
			logger.Infof("Run: %v %+v", bin[0], strings.Join(arguments, " "))
			cmd := exec.Command(bin[0], arguments...)
			buff := &bytes.Buffer{}
			cmd.Stdout = buff
			cmd.Stderr = buff
			err := cmd.Run()
			if err != nil {
				logger.Infof("Output: %+v", buff.String())
				logger.Errorf("%v", err.Error())
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			view.Output = buff.String()
			view.Status = "OK"
			//
			if _, err := os.Stat(path.Join(dir, HAS_CHANGES)); err == nil {
				if err := os.Remove(path.Join(dir, HAS_CHANGES)); err != nil {
					logger.Errorf("%v", err)
				}
			}
			//
			return c.JSON(view)
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	return c.JSON(view)
}

type AccountView struct {
	Admin bool
	Profiles []ProfileView `json:",omitempty"`
	ProfileId uint `json:",omitempty"`
	UserView
}

// @security BasicAuth
// @Summary Get account
// @Description get account
// @Accept json
// @Produce json
// @Success 200 {object} AccountView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account [get]
// @Tags account
// @Tags frontend
func getAccountHandler(c *fiber.Ctx) error {
	if v := c.Locals("user"); v != nil {
		if user, ok := v.(*models.User); ok {
			var err error
			if user, err = models.GetUserFull(common.Database, user.ID); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			var view AccountView
			if bts, err := json.Marshal(user); err == nil {
				if err = json.Unmarshal(bts, &view); err == nil {
					view.Admin = user.Role < models.ROLE_USER
					if len(user.Profiles) > 0 {
						view.ProfileId = user.Profiles[len(user.Profiles) - 1].ID
					}
					return c.JSON(view)
				}else{
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}
	}
	return c.JSON(HTTPError{"Something went wrong"})
}

type NewAccount struct {
	Email string
	CSRF string
	//
	Name string
	Lastname string
	Company string
	Phone string
	Address string
	Zip string
	City string
	Region string
	Country string
	ITN string
	//
	Profile NewProfile
	//
	OtherShipping bool
}

type Account2View struct {
	AccountView
	Token string `json:",omitempty"`
	Expiration *time.Time `json:",omitempty"`
}

// CreateAccout godoc
// @Summary Create account
// @Accept json
// @Produce json
// @Param profile body NewAccount true "body"
// @Success 200 {object} Account2View
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account [post]
// @Tags account
// @Tags frontend
func postAccountHandler(c *fiber.Ctx) error {
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(c.Request().Header.Header())))
	if err != nil {
		logger.Errorf("%+v", err)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		//var email string
		var request NewAccount
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			if err := c.BodyParser(&request); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		/*} else if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			for key, values := range data.Value {
				if strings.ToLower(key) == "email" {
					email = strings.TrimSpace(values[0])
				}
			}
			if email == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Email is empty"})
			}*/
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
		logger.Infof("Profile1: %+v", request)
		//
		if request.Email == "" {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Email is empty"})
		}
		email := request.Email
		//
		if _, err := models.GetUserByEmail(common.Database, email); err == nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Account already exists"})
		}
		//
		var login string
		if res := regexp.MustCompile(`^([^@]+)@`).FindAllStringSubmatch(email, 1); len(res) > 0 && len(res[0]) > 1 {
			login = fmt.Sprintf("%v-%d", res[0][1], rand.New(rand.NewSource(time.Now().UnixNano())).Intn(8999) + 1000)
		}
		password := NewPassword(12)
		logger.Infof("Create new user %v %v by email %v", login, password, email)
		user := &models.User{
			Enabled: true,
			Email: email,
			EmailConfirmed: true,
			Login: login,
			Password: models.MakeUserPassword(password),
			Role: models.ROLE_USER,
			Notification: true,
		}
		//
		var name = strings.TrimSpace(request.Name)
		if name == "" {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Name is empty"})
		}
		var lastname = strings.TrimSpace(request.Lastname)
		if lastname == "" {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Lastname is empty"})
		}
		var company = strings.TrimSpace(request.Company)
		var phone = strings.TrimSpace(request.Phone)
		if len(phone) > 64 {
			phone = ""
		}
		var address = strings.TrimSpace(request.Address)
		if address == "" {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Address is empty"})
		}
		var zip = strings.TrimSpace(request.Zip)
		if zip == "" {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Zip is empty"})
		}
		var city = strings.TrimSpace(request.City)
		if city == "" {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"City is empty"})
		}
		var region = strings.TrimSpace(request.Region)
		if region == "" {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Region is empty"})
		}
		var country = strings.TrimSpace(request.Country)
		if country == "" {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Country is empty"})
		}
		var itn = strings.TrimSpace(request.ITN)
		if len(itn) > 32 {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"ITN is incorrect"})
		}
		//
		user.Name = name
		user.Lastname = lastname
		user.Company = company
		user.Phone = phone
		user.Address = address
		user.Zip = zip
		user.City = city
		user.Region = region
		user.Country = country
		user.ITN = itn
		id, err := models.CreateUser(common.Database, user)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
		user.ID = id
		// How to create profile?
		var profileId uint
		if request.Profile.Name != "" {
			logger.Infof("Profile2: %+v", request.Profile)
			// create profile from shipping data
			var name = strings.TrimSpace(request.Profile.Name)
			if name == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Name is empty"})
			}
			var lastname = strings.TrimSpace(request.Profile.Lastname)
			if lastname == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Lastname is empty"})
			}
			var company = strings.TrimSpace(request.Profile.Company)
			var phone = strings.TrimSpace(request.Profile.Phone)
			if len(phone) > 64 {
				phone = ""
			}
			var address = strings.TrimSpace(request.Profile.Address)
			if address == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Address is empty"})
			}
			var zip = strings.TrimSpace(request.Profile.Zip)
			if zip == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Zip is empty"})
			}
			var city = strings.TrimSpace(request.Profile.City)
			if city == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"City is empty"})
			}
			var region = strings.TrimSpace(request.Profile.Region)
			if region == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Region is empty"})
			}
			var country = strings.TrimSpace(request.Profile.Country)
			if country == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Country is empty"})
			}
			var itn = strings.TrimSpace(request.Profile.ITN)
			if len(itn) > 32 {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"ITN is incorrect"})
			}
			profile := &models.Profile{
				Name:     name,
				Lastname: lastname,
				Company:  company,
				Phone:    phone,
				Address:  address,
				Zip:      zip,
				City:     city,
				Region:   region,
				Country:  country,
				ITN:      itn,
				UserId:   user.ID,
			}
			if profileId, err = models.CreateProfile(common.Database, profile); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			user.Profiles = []*models.Profile{profile}
		}else{
			// create profile from billing
			profile := &models.Profile{
				Name:     name,
				Lastname: lastname,
				Company:  company,
				Phone:    phone,
				Address:  address,
				Zip:      zip,
				City:     city,
				Region:   region,
				Country:  country,
				ITN:      itn,
				UserId:   user.ID,
				Billing:  true,
			}
			if profileId, err = models.CreateProfile(common.Database, profile); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			user.Profiles = []*models.Profile{profile}
		}
		var view AccountView
		if bts, err := json.Marshal(user); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				view.Admin = user.Role < models.ROLE_USER
				view.ProfileId = profileId
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
		//
		if v := req.Header.Get("Accept"); strings.EqualFold(v, "application/jwt") {
			expiration := time.Now().AddDate(1, 0, 0)
			claims := &JWTClaims{
				Login: user.Login,
				Password: user.Password,
				StandardClaims: jwt.StandardClaims{
					ExpiresAt: expiration.Unix(),
				},
			}
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
			if str, err := token.SignedString(JWTSecret); err == nil {
				c.Status(http.StatusOK)
				return c.JSON(Account2View{
					AccountView: view,
					Token: str,
					Expiration: &expiration,
				})
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			credentials := map[string]string{
				"email": email,
				"login": login,
				"password": password,
			}
			if encoded, err := cookieHandler.Encode(COOKIE_NAME, credentials); err == nil {
				cookie := &fiber.Cookie{
					Name:  COOKIE_NAME,
					Value: encoded,
					Path:  "/",
					Expires: time.Now().AddDate(1, 0, 0),
					SameSite: authMultipleConfig.SameSite,
				}
				c.Cookie(cookie)
			}
			c.Status(http.StatusOK)
			return c.JSON(view)
		}
	}else{
		return c.JSON(HTTPError{"Unsupported Content-Type"})
	}
}

type User2View struct {
	OldPassword string
	NewPassword string
	NewPassword2 string
}

// @security BasicAuth
// UpdateAccount godoc
// @Summary update account
// @Accept json
// @Produce json
// @Param account body AccountView true "body"
// @Success 200 {object} User2View
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account [put]
// @Tags order
func putAccountHandler(c *fiber.Ctx) error {
	if v := c.Locals("user"); v != nil {
		var user *models.User
		var ok bool
		if user, ok = v.(*models.User); ok {
			//
			var request User2View
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			request.OldPassword = strings.TrimSpace(request.OldPassword)
			request.NewPassword = strings.TrimSpace(request.NewPassword)
			request.NewPassword2 = strings.TrimSpace(request.NewPassword2)
			//
			if models.MakeUserPassword(request.OldPassword) != user.Password {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Incorrect password"})
			}
			if len(request.NewPassword) < 6 {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Too short password"})
			}
			if len(request.NewPassword) > 32 {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Too long password"})
			}
			if request.NewPassword != request.NewPassword2 {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Passwords mismatch"})
			}
			user.Password = models.MakeUserPassword(request.NewPassword2)
			if err := models.UpdateUser(common.Database, user); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			c.Status(http.StatusOK)
			return c.JSON(HTTPMessage{"OK"})
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"User not found"})
		}
	}
	c.Status(http.StatusInternalServerError)
	return c.JSON(HTTPError{"Something went wrong"})
}

// @security BasicAuth
// @Summary Get account profiles
// @Description get account profiles
// @Accept json
// @Produce json
// @Success 200 {object} []ProfileView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/profiles [get]
// @Tags account
// @Tags frontend
func getAccountProfilesHandler(c *fiber.Ctx) error {
	if v := c.Locals("user"); v != nil {
		if user, ok := v.(*models.User); ok {
			if profiles, err := models.GetProfilesByUser(common.Database, user.ID); err == nil {
				var views []ProfileView
				if bts, err := json.Marshal(profiles); err == nil {
					if err = json.Unmarshal(bts, &views); err == nil {
						return c.JSON(views)
					}else{
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
				}else{
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}
	}
	return c.JSON(HTTPError{"Something went wrong"})
}

type NewProfile struct {
	Email string
	Name string
	Lastname string
	Company string
	Phone string
	Address string
	Zip string
	City string
	Region string
	Country string
	ITN string
}

// @security BasicAuth
// CreateProfile godoc
// @Summary Create profile in existing account
// @Accept json
// @Produce json
// @Param profile body NewProfile true "body"
// @Success 200 {object} ProfileView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/profiles [post]
// @Tags profile
// @Tags frontend
func postAccountProfileHandler(c *fiber.Ctx) error {
	var view ProfileView
	//
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewProfile
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			logger.Infof("request: %+v", request)
			//
			var userId uint
			if v := c.Locals("user"); v != nil {
				logger.Infof("v: %+v", v)
				var user *models.User
				var ok bool
				if user, ok = v.(*models.User); ok {
					userId = user.ID
				}else{
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{"User not found"})
				}
			}
			//
			var name = strings.TrimSpace(request.Name)
			if name == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Name is empty"})
			}
			var lastname = strings.TrimSpace(request.Lastname)
			if lastname == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Lastname is empty"})
			}
			var company = strings.TrimSpace(request.Company)
			var phone = strings.TrimSpace(request.Phone)
			if len(phone) > 64 {
				phone = ""
			}
			var address = strings.TrimSpace(request.Address)
			if address == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Address is empty"})
			}
			var zip = strings.TrimSpace(request.Zip)
			if zip == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Zip is empty"})
			}
			var city = strings.TrimSpace(request.City)
			if city == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"City is empty"})
			}
			var region = strings.TrimSpace(request.Region)
			if region == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Region is empty"})
			}
			var country = strings.TrimSpace(request.Country)
			if country == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Country is empty"})
			}
			var itn = strings.TrimSpace(request.ITN)
			if len(itn) > 32 {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"ITN is incorrect"})
			}
			profile := &models.Profile{
				Name:     name,
				Lastname: lastname,
				Company:  company,
				Phone: phone,
				Address:  address,
				Zip:      zip,
				City:     city,
				Region:   region,
				Country:  country,
				ITN: itn,
				UserId:   userId,
			}
			//
			if _, err := models.CreateProfile(common.Database, profile); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			//
			if bts, err := json.Marshal(profile); err == nil {
				if err = json.Unmarshal(bts, &view); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}
			return c.JSON(view)
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	//
	return c.JSON(view)
}

type EmailRequest struct {
	Email string
}

// CheckEmail godoc
// @Summary Check email
// @Accept json
// @Produce json
// @Param email body EmailRequest true "body"
// @Success 200 {object} Account2View
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/email [post]
// @Tags account
// @Tags frontend
func postEmailHandler(c *fiber.Ctx) error {
	var request EmailRequest
	if err := c.BodyParser(&request); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	request.Email = strings.TrimSpace(request.Email)
	if request.Email == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Empty email"})
	}
	time.Sleep(1 * time.Second)
	if _, err := models.GetUserByEmail(common.Database, request.Email); err == nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Empty already in use"})
	}
	c.Status(http.StatusOK)
	return c.JSON(HTTPMessage{"OK"})
}

type CartItem struct {
	UUID string // ProductId,VariationId,
	Title string
	Thumbnails []string
	Price float64
	Properties []ItemProperty
	Quantity int
	//
	ProductId int
	VariationId int
}

type ItemProperty struct {
	Name string
	Value string
	Price float64
	//
	ValueId int
	PriceId int
}

// (DEPRECATED) CreateProfile godoc
// @Summary (DEPRECATED) Create profile without having account
// @Accept json
// @Produce json
// @Param profile body NewProfile true "body"
// @Success 200 {object} ProfileView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/profiles [post]
// @Tags profile
func postProfileHandler(c *fiber.Ctx) error {
	var view ProfileView
	//
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewProfile
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			//logger.Infof("request: %+v", request)
			//
			var userId uint
			var email = strings.TrimSpace(request.Email)
			if email == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Email is empty"})
			}
			user, _ := models.GetUserByEmail(common.Database, email)
			if user != nil && user.ID > 0 {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Email already in use, please try to restore password"})
			}
			logger.Infof("create new user")
			var login string
			if res := regexp.MustCompile(`^([^@]+)@`).FindAllStringSubmatch(request.Email, 1); len(res) > 0 && len(res[0]) > 1 {
				login = fmt.Sprintf("%v-%d", res[0][1], rand.New(rand.NewSource(time.Now().UnixNano())).Intn(8999) + 1000)
			}
			password := NewPassword(12)
			logger.Infof("Create new user %v %v by email %v", login, password, email)
			user = &models.User{
				Enabled: true,
				Email: email,
				EmailConfirmed: true,
				Login: login,
				Password: models.MakeUserPassword(password),
				Role: models.ROLE_USER,
				Notification: true,
			}
			var err error
			userId, err = models.CreateUser(common.Database, user)
			if err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			//
			credentials := map[string]string{
				"email": email,
				"login": login,
				"password": password,
			}
			//logger.Infof("credentials: %+v", credentials)
			if encoded, err := cookieHandler.Encode(COOKIE_NAME, credentials); err == nil {
				//logger.Infof("encoded: %+v", encoded)
				cookie := &fiber.Cookie{
					Name:  COOKIE_NAME,
					Value: encoded,
					Path:  "/",
					Expires: time.Now().AddDate(1, 0, 0),
					SameSite: authMultipleConfig.SameSite,
				}
				c.Cookie(cookie)
			}
			//
			var name = strings.TrimSpace(request.Name)
			if name == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Country is empty"})
			}
			var lastname = strings.TrimSpace(request.Lastname)
			if lastname == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Lastname is empty"})
			}
			var company = strings.TrimSpace(request.Company)
			var phone = strings.TrimSpace(request.Phone)
			if len(phone) > 64 {
				phone = ""
			}
			var address = strings.TrimSpace(request.Address)
			if address == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Address is empty"})
			}
			var zip = request.Zip
			if zip == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Zip is empty"})
			}
			var city = strings.TrimSpace(request.City)
			if city == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"City is empty"})
			}
			var region = strings.TrimSpace(request.Region)
			if region == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Region is empty"})
			}
			var country = strings.TrimSpace(request.Country)
			if country == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Country is empty"})
			}
			profile := &models.Profile{
				Name:     name,
				Lastname: lastname,
				Company:  company,
				Phone:  phone,
				Address:  address,
				Zip:      zip,
				City:     city,
				Region:   region,
				Country:  country,
				UserId:   userId,
			}
			//
			if _, err := models.CreateProfile(common.Database, profile); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			//
			if bts, err := json.Marshal(profile); err == nil {
				if err = json.Unmarshal(bts, &view); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}
			return c.JSON(view)
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	//
	return c.JSON(view)
}

func getTestHandler(c *fiber.Ctx) error {
	logger.Infof("getTestHandler")
	time.Sleep(3 * time.Second)
	return return1(c, http.StatusOK, map[string]interface{}{"MESSAGE": "OK", "Status": "paid"})
}

type VATRequest struct {
	VAT string
}

// CheckVAT godoc
// @Summary Check vat, should be solid string without space, eg. ATU40198200
// @Accept json
// @Produce json
// @Param request body VATRequest true "body"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/vat [post]
// @Tags account
// @Tags frontend
func postVATHandler(c *fiber.Ctx) error {
	var request VATRequest
	if err := c.BodyParser(&request); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	request.VAT = strings.TrimSpace(request.VAT)
	if request.VAT == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Empty ITN"})
	}
	time.Sleep(1 * time.Second)
	if status, err := vat.ValidateNumber(request.VAT); err == nil {
		if status {
			c.Status(http.StatusOK)
			return c.JSON(HTTPMessage{"OK"})
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPMessage{"Invalid"})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

type FilterRequest ListRequest

type ProductsFilterResponse struct {
	Data []ProductsFilterItem
	Filtered int64
	Total int64
}

type ProductsFilterItem struct {
	ID uint
	Path string
	Name string
	Title string
	Thumbnail string
	Description string
	Images string
	Variations string
	Price float64
	Width float64
	Height float64
	Depth float64
	Weight float64
	CategoryId uint
}

// @security BasicAuth
// Search and Filter products godoc
// @Summary Search and Filter products. Use fixed word 'Search' to make search, another options according to doc
// @Accept json
// @Produce json
// @Param relPath query string true "Category RelPath"
// @Param category body FilterRequest true "body"
// @Success 200 {object} ProductsFilterResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/filter [post]
// @Tags frontend
func postFilterHandler(c *fiber.Ctx) error {
	var relPath string
	if v := c.Query("relPath"); v != "" {
		relPath = v
	}else{
		return c.JSON(HTTPError{"relPath required"})
	}
	var response ProductsFilterResponse
	var request FilterRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	//logger.Infof("request: %+v", request)
	if len(request.Sort) == 0 {
		request.Sort = map[string]string{"ID": "desc"}
	}
	if request.Length == 0 {
		request.Length = 10
	}
	// Filter
	var keys1 []string
	var values1 []interface{}
	var keys2 []string
	var values2 []interface{}
	if len(request.Filter) > 0 {
		for key, value := range request.Filter {
			if key != "" && len(strings.TrimSpace(value)) > 0 {
				switch key {
				case "Price", "Width", "Height", "Depth", "Weight":
					parts := strings.Split(value, "-")
					if len(parts) == 1 {
						if v, err := strconv.Atoi(parts[0]); err == nil {
							keys1 = append(keys1, "cache_products." + key + " == ?")
							values1 = append(values1, v)
						}
					} else {
						if v, err := strconv.Atoi(parts[0]); err == nil {
							keys1 = append(keys1, "cache_products." + key + " >= ?")
							values1 = append(values1, v)
						}
						if v, err := strconv.Atoi(parts[1]); err == nil {
							keys1 = append(keys1, "cache_products." + key + " <= ?")
							values1 = append(values1, v)
						}
					}
				case "Search":
					keys1 = append(keys1, "(cache_products.Title like ? or cache_products.Description like ?)")
					values1 = append(values1, "%" + value + "%", "%" + value + "%")
				default:
					if strings.Index(key, "Option-") >= -1 {
						if res := regexp.MustCompile(`Option-(\d+)`).FindAllStringSubmatch(key, 1); len(res) > 0 && len(res[0]) > 1 {
							if id, err := strconv.Atoi(res[0][1]); err == nil {
								values := strings.Split(value, ",")
								var keys3 []string
								var values3 []interface{}
								for _, value := range values {
									if v, err := strconv.Atoi(value); err == nil {
										keys3 = append(keys3, "parameters.Value_id = ? or prices.Value_Id = ?")
										values3 = append(values3, v, v)
									}
								}
								keys2 = append(keys2, fmt.Sprintf("(options.Id = ? and (%v))", strings.Join(keys3, " or ")))
								values2 = append(values2, append([]interface{}{id}, values3...)...)
							}
						}
					}else{
						keys1 = append(keys1, fmt.Sprintf("products.%v like ?", key))
						values1 = append(values1, "%"+strings.TrimSpace(value)+"%")
					}
				}
			}
		}
	}
	if len(keys2) > 0 {
		keys1 = append(keys1, fmt.Sprintf("(%v)", strings.Join(keys2, " or ")))
		values1 = append(values1, values2...)
	}
	keys1 = append(keys1, "Path LIKE ?")
	if common.Config.FlatUrl {
		if prefix := "/" + strings.ToLower(common.Config.Products); prefix + "/" == relPath {
			relPath = "/"
		}else{
			relPath = "/" + strings.ToLower(common.Config.Products) + relPath
		}
	}
	values1 = append(values1, relPath + "%")
	////logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
	//
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				case "Price":
					orders = append(orders, fmt.Sprintf("cache_products.%v %v", "Price", value))
				default:
					orders = append(orders, fmt.Sprintf("cache_products.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//logger.Infof("order: %+v", order)
	//
	rows, err := common.Database.Debug().Model(&models.CacheProduct{}).Select("cache_products.ID, cache_products.Name, cache_products.Title, cache_products.Path, cache_products.Description, cache_products.Thumbnail, cache_products.Images, cache_products.Variations, cache_products.Price as Price, cache_products.Width as Width, cache_products.Height as Height, cache_products.Depth as Depth,  cache_products.Weight as Weight, cache_products.Category_Id as CategoryId").Joins("left join parameters on parameters.Product_ID = cache_products.Product_ID").Joins("left join variations on variations.Product_ID = cache_products.Product_ID").Joins("left join properties on properties.Variation_Id = variations.Id").Joins("left join options on options.Id = parameters.Option_Id or options.Id = properties.Option_Id").Joins("left join prices on prices.Property_Id = properties.Id").Where(strings.Join(keys1, " and "), values1...)/*.Having(strings.Join(keys2, " and "), values2...)*/.Group("cache_products.product_id").Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		for rows.Next() {
			var item ProductsFilterItem
			if err = common.Database.ScanRows(rows, &item); err == nil {
				response.Data = append(response.Data, item)
			} else {
				logger.Errorf("%v", err)
			}
		}
		rows.Close()
	}
	//
	common.Database.Debug().Model(&models.CacheProduct{}).Select("Product_ID as ID, Name, Title, Path, Description, Thumbnail, Price, Category_Id as CategoryId").Joins("left join parameters on parameters.Product_ID = cache_products.Product_ID").Joins("left join variations on variations.Product_ID = cache_products.Product_ID").Joins("left join properties on properties.Variation_Id = variations.Id").Joins("left join options on options.Id = parameters.Option_Id or options.Id = properties.Option_Id").Joins("left join prices on prices.Property_Id = properties.Id").Where(strings.Join(keys1, " and "), values1...).Count(&response.Filtered)
	common.Database.Debug().Model(&models.CacheProduct{}).Select("Product_ID as ID, Name, Title, Path, Description, Thumbnail, Price, Category_Id as CategoryId").Joins("left join parameters on parameters.Product_ID = cache_products.Product_ID").Joins("left join variations on variations.Product_ID = cache_products.Product_ID").Joins("left join properties on properties.Variation_Id = variations.Id").Joins("left join options on options.Id = parameters.Option_Id or options.Id = properties.Option_Id").Joins("left join prices on prices.Property_Id = properties.Id").Where("Path LIKE ?", relPath + "%").Count(&response.Total)
	//
	c.Status(http.StatusOK)
	return c.JSON(response)
}

type SearchRequest struct {
	Term string
	Limit int
}

type SearchResult struct {
	Term string
	Products []SearchResultProductView
}

type SearchResultProductView struct {
	ProductView
	Path string `json:",omitempty"`
	BasePrice float64 `json:",omitempty"`
}

type OrdersView []*OrderView

type OrderView struct {
	ID uint
	Description string `json:",omitempty"`
	Items []*ItemView
	Status string
	Sum float64
	Delivery float64
	Total float64
	Comment string `json:",omitempty"`
}

type ItemView struct{
	ID uint
	Uuid string
	Title string
	Description string
	Path string
	Thumbnail string
	//Variation VariationShortView `json:",omitempty"`
	//Properties []PropertyShortView `json:",omitempty"`
	Comment string
	Price float64
	Quantity int
	Total float64
}

// GetOrders godoc
// @Summary Get account orders
// @Accept json
// @Produce json
// @Success 200 {object} OrdersView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders [get]
// @Tags account
// @Tags frontend
func getAccountOrdersHandler(c *fiber.Ctx) error {
	var userId uint
	if v := c.Locals("user"); v != nil {
		if user, ok := v.(*models.User); ok {
			userId = user.ID
		}
	}
	if orders, err := models.GetOrdersByUserId(common.Database, userId); err == nil {
		var view OrdersView
		if bts, err := json.MarshalIndent(orders, "", "   "); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				return c.JSON(view)
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

type CheckoutRequest struct {
	Items []NewItem
	Comment string
	ProfileId uint
	TransportId uint
	TransportServices []TransportServiceView
	PaymentMethod string
	Coupons []string
}

// GetOrders godoc
// @Summary Get checkout information
// @Accept json
// @Produce json
// @Param category body CheckoutRequest true "body"
// @Success 200 {object} OrderShortView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/checkout [post]
// @Tags frontend
func postCheckoutHandler(c *fiber.Ctx) error {
	var request CheckoutRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	_, view, err := Checkout(request)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	return c.JSON(view)
}

type NewOrder struct {
	Created time.Time
	Items []NewItem
	Comment string
	ProfileId uint
	TransportId uint
	Coupons []string
}

type NewItem struct {
	UUID string
	CategoryId uint
	Quantity int
}

/**/

type OrderShortView struct{
	Items []ItemShortView `json:",omitempty"`
	Quantity int `json:",omitempty"`
	Sum float64 `json:",omitempty"`
	Discount float64 `json:",omitempty"`
	Delivery float64 `json:",omitempty"`
	Discount2 float64 `json:",omitempty"`
	VAT float64
	Total float64 `json:",omitempty"`
	Volume float64 `json:",omitempty"`
	Weight float64 `json:",omitempty"`
	Comment string `json:",omitempty"`
	//
	Deliveries []DeliveryView          `json:",omitempty"`
	Transport *TransportOrderView      `json:",omitempty"`
	Coupons []*CouponOrderView         `json:",omitempty"`
	PaymentMethods *PaymentMethodsView `json:",omitempty"`
}

type TransportOrderView struct{
	ID uint
	Name string
	Title string
	Thumbnail string
	Services []TransportServiceView `json:",omitempty"`
}

type CouponOrderView struct {
	ID uint
	Code string
	Title string
	Type string // order, item, shipment
	Amount string
}

type DeliveryView struct {
	ID uint
	Title string
	Thumbnail string
	ByVolume float64
	ByWeight float64
	Services []TransportServiceView `json:",omitempty"`
	Value float64
}

type TransportServiceView struct {
	Name string
	Title string
	Description string
	Price float64
	Selected bool
}

type ItemShortView struct {
	Uuid string                    `json:",omitempty"`
	Title string                   `json:",omitempty"`
	Path string                    `json:",omitempty"`
	Thumbnail string               `json:",omitempty"`
	Variation VariationShortView	`json:",omitempty"`
	Properties []PropertyShortView `json:",omitempty"`
	Price float64                  `json:",omitempty"`
	Discount float64                  `json:",omitempty"`
	Quantity int                   `json:",omitempty"`
	VAT        float64
	Total      float64             `json:",omitempty"`
	Volume float64 `json:",omitempty"`
	Weight float64 `json:",omitempty"`
}

type VariationShortView struct {
	Title string `json:",omitempty"`
	Thumbnail string `json:",omitempty"`
}

type PropertyShortView struct {
	Title string `json:",omitempty"`
	Thumbnail string `json:",omitempty"`
	Value string `json:",omitempty"`
	Price float64 `json:",omitempty"`
}

// CreateOrder godoc
// @Summary Post account order
// @Accept json
// @Produce json
// @Param cart body CheckoutRequest true "body"
// @Success 200 {object} OrderView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders [post]
// @Tags account
// @Tags frontend
func postAccountOrdersHandler(c *fiber.Ctx) error {
	var request CheckoutRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	order, _, err := Checkout(request)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	if v := c.Locals("user"); v != nil {
		if user, ok := v.(*models.User); ok {
			order.UserId = user.ID
		}
	}
	if _, err := models.CreateOrder(common.Database, order); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	return c.JSON(order)
}

// CreateOrder godoc
// @Summary Post account order
// @Accept json
// @Produce json
// @Param cart body NewOrder true "body"
// @Success 200 {object} OrderView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders2 [post]
// @Tags account
// @Tags frontend
/*func postAccountOrdersHandler2(c *fiber.Ctx) error {
	var request NewOrder
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	//logger.Infof("request: %+v", request)
	if request.ProfileId == 0 {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Profile Id is not set"})
	}
	profile, err := models.GetProfile(common.Database, request.ProfileId)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	order := &models.Order{
		Status: models.ORDER_STATUS_NEW,
		ProfileId: request.ProfileId,
		TransportId: request.TransportId,
	}
	if request.TransportId > 0 {
		// 1 Get selected transport company
		transport, err := models.GetTransport(common.Database, int(request.TransportId))
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
		// 2 Get Zone by Country and Zip
		var zoneId uint
		zone, err := models.GetZoneByCountryAndZIP(common.Database, profile.Country, profile.Zip)
		if err == nil {
			zoneId = zone.ID
		}
		// 3 Get Tariff by Transport and Zone
		tariff, _ := models.GetTariffByTransportIdAndZoneId(common.Database, transport.ID, zoneId)
		if cost, err := Delivery(transport, tariff, request.Items); err == nil {
			order.Delivery = cost.Value
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}
	// //
	//if v := c.Locals("user"); v != nil {
	//	if user, ok := v.(*models.User); ok {
	//		order.User = user
	//		//
	//		if orders, err := models.GetOrdersByUserId(common.Database, user.ID); err == nil {
	//			for _, order := range orders {
	//				if order.Status == models.ORDER_STATUS_NEW || order.Status == models.ORDER_STATUS_WAITING_FROM_PAYMENT {
	//					c.Status(http.StatusInternalServerError)
	//					return c.JSON(HTTPError{"Some orders wait for your payment, please finish it before continuing"})
	//				}
	//			}
	//		}
	//	}
	//}
	var orderShortView OrderShortView
	for _, item := range request.Items {
		var arr []int
		if err := json.Unmarshal([]byte(item.UUID), &arr); err == nil && len(arr) >= 2{
			//
			productId := arr[0]
			var product *models.Product
			if product, err = models.GetProduct(common.Database, productId); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			variationId := arr[1]
			var variation *models.Variation
			if variation, err = models.GetVariation(common.Database, variationId); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if product.ID != variation.ProductId {
				err = fmt.Errorf("Products and Variation mismatch")
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}

			categoryId := item.CategoryId

			orderItem := &models.Item{
				Uuid:     item.UUID,
				Title:    product.Title,
				Price:    variation.BasePrice,
				Quantity: item.Quantity,
			}
			if res := reVolume.FindAllStringSubmatch(variation.Dimensions, 1); len(res) > 0 && len(res[0]) > 1 {
				var width float64
				if v, err := strconv.ParseFloat(res[0][1], 10); err == nil {
					width = v
				}
				var height float64
				if v, err := strconv.ParseFloat(res[0][2], 10); err == nil {
					height = v
				}
				var depth float64
				if v, err := strconv.ParseFloat(res[0][3], 10); err == nil {
					depth = v
				}
				orderItem.Volume = width * height * depth / 1000000.0
			}
			orderItem.Weight = variation.Weight
			//
			itemShortView := ItemShortView{
				Uuid: item.UUID,
				Title: product.Title,
				Variation: VariationShortView{
					Title:variation.Title,
				},
				Volume: orderItem.Volume,
				Weight: orderItem.Weight,
			}
			//
			if breadcrumbs := models.GetBreadcrumbs(common.Database, categoryId); len(breadcrumbs) > 0 {
				var chunks []string
				for _, crumb := range breadcrumbs {
					chunks = append(chunks, crumb.Name)
				}
				orderItem.Path = "/" + path.Join(append(chunks, product.Name)...)
			}

			if cache, err := models.GetCacheProductByProductId(common.Database, product.ID); err == nil {
				orderItem.Thumbnail = cache.Thumbnail
			}else{
				logger.Warningf("%v", err.Error())
			}

			if cache, err := models.GetCacheVariationByVariationId(common.Database, variation.ID); err == nil {
				itemShortView.Variation.Thumbnail = cache.Thumbnail
				if orderItem.Thumbnail == "" {
					orderItem.Thumbnail = cache.Thumbnail
				}
			} else {
				logger.Warningf("%v", err.Error())
			}

			if len(arr) > 2 {
				//
				//var propertiesShortView []PropertyShortView
				for _, id := range arr[2:] {
					if price, err := models.GetPrice(common.Database, id); err == nil {
						propertyShortView := PropertyShortView{}
						propertyShortView.Title = price.Property.Title
						//
						if cache, err := models.GetCacheValueByValueId(common.Database, price.Value.ID); err == nil {
							propertyShortView.Thumbnail = cache.Thumbnail
						}
						//
						propertyShortView.Value = price.Value.Value
						if price.Price > 0 {
							propertyShortView.Price = price.Price
						}
						orderItem.Price += price.Price
						itemShortView.Properties = append(itemShortView.Properties, propertyShortView)
					} else {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
				}
				if bts, err := json.Marshal(itemShortView); err == nil {
					orderItem.Description = string(bts)
				}
			}
			orderItem.Total = orderItem.Price * float64(orderItem.Quantity)

			order.Items = append(order.Items, orderItem)
			order.Sum += orderItem.Price * float64(orderItem.Quantity)
			//
			itemShortView.Path = orderItem.Path
			itemShortView.Thumbnail = orderItem.Thumbnail
			itemShortView.Price = orderItem.Price
			itemShortView.Quantity = orderItem.Quantity
			itemShortView.Total = orderItem.Total
			//
			orderShortView.Quantity += itemShortView.Quantity
			orderShortView.Items = append(orderShortView.Items, itemShortView)
			//
			orderShortView.Volume += itemShortView.Volume * float64(itemShortView.Quantity)
			orderShortView.Weight += itemShortView.Weight * float64(itemShortView.Quantity)
		}
	}
	order.Total = order.Sum + order.Delivery
	order.Volume = orderShortView.Volume
	order.Weight = orderShortView.Weight
	orderShortView.Sum = order.Sum
	orderShortView.Delivery = order.Delivery
	orderShortView.Total = order.Total
	if bts, err := json.Marshal(orderShortView); err == nil {
		order.Description = string(bts)
	}
	//logger.Infof("order: %+v", order)
	if _, err := models.CreateOrder(common.Database, order); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	return c.JSON(order)
}*/

func Checkout(request CheckoutRequest) (*models.Order, *OrderShortView, error){
	order := &models.Order{Status: models.ORDER_STATUS_NEW}
	now := time.Now()
	vat := common.Config.Payment.VAT
	tax := 1.0
	// Profile
	var profile *models.Profile
	var err error
	if request.ProfileId > 0 {
		order.ProfileId = request.ProfileId
		profile, err = models.GetProfile(common.Database, request.ProfileId)
		if err == nil {
			if profile.Country != common.Config.Payment.Country {
				var allowed bool
				switch profile.Country {
				case "AT":
					allowed = profile.ITN != ""
				default:
					allowed = true
				}
				if allowed {
					tax -= common.Config.Payment.VAT / 100.0
					vat = 0
				}
			}
		} else {
			return nil, nil, err
		}
	}
	// Coupons
	var couponsView []*CouponOrderView
	var coupons []*models.Coupon
	for _, code := range request.Coupons {
		allowed := true
		for _, coupon := range coupons {
			if coupon.Code == code {
				allowed = false
				break
			}
		}
		if allowed {
			if coupon, err := models.GetCouponByCode(common.Database, code); err == nil {
				coupons = append(coupons, coupon)
			}
		}
	}
	//
	var itemsShortView []ItemShortView
	for _, rItem := range request.Items {
		var arr []int
		if err := json.Unmarshal([]byte(rItem.UUID), &arr); err == nil && len(arr) >= 2{
			//
			productId := arr[0]
			var product *models.Product
			if product, err = models.GetProduct(common.Database, productId); err != nil {
				return nil, nil, err
			}
			variationId := arr[1]

			//var variation *models.Variation
			var vId uint
			var title string
			var basePrice, salePrice, weight float64
			var start, end time.Time
			//var dimensions string
			var width, height, depth float64
			if variationId == 0 {
				title = "default"
				basePrice = product.BasePrice
				salePrice = product.SalePrice
				start = product.Start
				end = product.End
				//dimensions = product.Dimensions
				width = product.Width
				height = product.Height
				depth = product.Depth
				weight = product.Weight
			} else {
				if variation, err := models.GetVariation(common.Database, variationId); err == nil {
					if product.ID != variation.ProductId {
						err = fmt.Errorf("Products and Variation mismatch")
						return nil, nil, err
					}
					vId = variation.ID
					title = variation.Title
					basePrice = variation.BasePrice
					salePrice = variation.SalePrice
					start = variation.Start
					end = variation.End
					width = variation.Width
					height = variation.Height
					depth = variation.Depth
					weight = variation.Weight
				} else {
					return nil, nil, err
				}
			}
			categoryId := rItem.CategoryId
			item := &models.Item{
				Uuid:     rItem.UUID,
				CategoryId: rItem.CategoryId,
				Title:    product.Title,
				BasePrice: basePrice,
				Quantity: rItem.Quantity,
			}
			if salePrice > 0 && start.Before(now) && end.After(now) {
				item.Price = salePrice * tax
				item.SalePrice = salePrice * tax
			}else{
				item.Price = basePrice * tax
			}
			item.VAT = vat
			item.Volume = width * height * depth / 1000000.0
			item.Weight = weight
			//
			if breadcrumbs := models.GetBreadcrumbs(common.Database, categoryId); len(breadcrumbs) > 0 {
				var chunks []string
				for _, crumb := range breadcrumbs {
					chunks = append(chunks, crumb.Name)
				}
				item.Path = "/" + path.Join(append(chunks, product.Name)...)
			}

			if cache, err := models.GetCacheProductByProductId(common.Database, product.ID); err == nil {
				item.Thumbnail = cache.Thumbnail
			}else{
				logger.Warningf("%v", err.Error())
			}
			if cache, err := models.GetCacheVariationByVariationId(common.Database, vId); err == nil {
				if item.Thumbnail == "" {
					item.Thumbnail = cache.Thumbnail
				}
			} else {
				logger.Warningf("%v", err.Error())
			}

			var propertiesShortView []PropertyShortView
			if len(arr) > 2 {
				//
				for _, id := range arr[2:] {
					if price, err := models.GetPrice(common.Database, id); err == nil {
						propertyShortView := PropertyShortView{}
						propertyShortView.Title = price.Property.Title
						//
						if cache, err := models.GetCacheValueByValueId(common.Database, price.Value.ID); err == nil {
							propertyShortView.Thumbnail = cache.Thumbnail
						}
						//
						propertyShortView.Value = price.Value.Value
						if price.Price > 0 {
							propertyShortView.Price = price.Price * tax
						}
						item.Price += price.Price * tax
						propertiesShortView = append(propertiesShortView, propertyShortView)
					} else {
						return nil, nil, err
					}
				}
			}
			// Coupons
			for _, coupon := range coupons {
				if coupon.Enabled && coupon.Start.Before(now) && coupon.End.After(now) {
					var allowed bool
					if coupon.Type == "item" {
						if coupon.ApplyTo == "all" {
							allowed = true
						} else if coupon.ApplyTo == "categories" {
							allowed = false
							for _, category := range coupon.Categories{
								if category.ID == item.CategoryId {
									allowed = true
									break
								}
							}
						} else if coupon.ApplyTo == "products" {
							allowed = false
							for _, product := range coupon.Products{
								if int(product.ID) == productId {
									allowed = true
									break
								}
							}
						} else {
							allowed = false
						}
					}else if coupon.Type == "order" {
						allowed = true
					}
					if allowed {
						couponsView = append(couponsView, &CouponOrderView{
							ID:     coupon.ID,
							Code:   coupon.Code,
							Title:  coupon.Title,
							Type: coupon.Type,
							Amount: coupon.Amount,
						})
						//
						if res := rePercent.FindAllStringSubmatch(coupon.Amount, 1); len(res) > 0 && len(res[0]) > 1 {
							// percent
							if p, err := strconv.ParseFloat(res[0][1], 10); err == nil {
								item.Discount = item.Price * p / 100.0
							}
						}else{
							if n, err := strconv.ParseFloat(coupon.Amount, 10); err == nil {
								item.Discount = n
							}
						}
						if item.Discount > item.Price {
							item.Discount = item.Price
						}
						item.Discount = math.Round(item.Discount * 100) / 100
					}
				}
			}
			// /Coupons
			item.Total = (item.Price - item.Discount) * float64(item.Quantity)
			// [Item Description]
			var itemShortView ItemShortView
			if bts, err := json.Marshal(item); err == nil {
				if err = json.Unmarshal(bts, &itemShortView); err != nil {
					logger.Warningf("%v", err.Error())
				}
			}else{
				logger.Warningf("%v", err.Error())
			}
			itemShortView.Variation = VariationShortView{
				Title: title,
			}
			itemShortView.Properties = propertiesShortView
			if bts, err := json.Marshal(itemShortView); err == nil {
				item.Description = string(bts)
			}
			// [/Item Description]
			itemsShortView = append(itemsShortView, itemShortView)
			order.Items = append(order.Items, item)
			order.Quantity += item.Quantity
			order.Volume += item.Volume
			order.Weight += item.Weight
			order.Sum += item.Total
			order.Discount += item.Discount
		}
	}
	// Transports
	var deliveriesShortView []DeliveryView
	var transportView *TransportOrderView
	if transports, err := models.GetTransports(common.Database); err == nil {
		for _, transport := range transports {
			// All available transports OR selected
			if transport.Enabled && (order.Volume >= transport.Volume || order.Weight >= transport.Weight) && (transport.ID == request.TransportId || request.TransportId == 0) {
				//
				var orderFixed, orderPercent, itemFixed, itemPercent, kg, m3 float64
				var orderIsPercent, itemIsPercent bool
				//
				var tariff *models.Tariff
				if profile != nil {
					// 2 Get Zone by Country, Country and Zip
					var zoneId uint
					if zone, err := models.GetZoneByCountry(common.Database, profile.Country); err == nil {
						zoneId = zone.ID
					}
					for i := 0; i <= len(profile.Zip); i++ {
						n := len(profile.Zip) - i
						zip := profile.Zip[0:n]
						for j := n; j < len(profile.Zip); j++ {
							zip += "X"
						}
						zone, err := models.GetZoneByCountryAndZIP(common.Database, profile.Country, zip)
						if err == nil {
							zoneId = zone.ID
							break
						}
					}
					// 3 Get Tariff by Transport and Zone
					tariff, _ = models.GetTariffByTransportIdAndZoneId(common.Database, transport.ID, zoneId)
				}
				//
				if tariff == nil {
					if res := rePercent.FindAllStringSubmatch(transport.Order, 1); len(res) > 0 && len(res[0]) > 1 {
						if v, err := strconv.ParseFloat(res[0][1], 10); err == nil {
							orderPercent = v
							orderIsPercent = true
						}
					}else{
						if v, err := strconv.ParseFloat(transport.Order, 10); err == nil {
							orderFixed = v
						}
					}
				}else{
					if res := rePercent.FindAllStringSubmatch(tariff.Order, 1); len(res) > 0 && len(res[0]) > 1 {
						if v, err := strconv.ParseFloat(res[0][1], 10); err == nil {
							orderPercent = v
							orderIsPercent = true
						}
					}else{
						if v, err := strconv.ParseFloat(tariff.Order, 10); err == nil {
							orderFixed = v
						}
					}
				}
				// Item
				if tariff == nil {
					if res := rePercent.FindAllStringSubmatch(transport.Item, 1); len(res) > 0 && len(res[0]) > 1 {
						if v, err := strconv.ParseFloat(res[0][1], 10); err == nil {
							itemPercent = v
							itemIsPercent = true
						}
					}else{
						if v, err := strconv.ParseFloat(transport.Item, 10); err == nil {
							itemFixed = v
						}
					}
				}else{
					if res := rePercent.FindAllStringSubmatch(tariff.Item, 1); len(res) > 0 && len(res[0]) > 1 {
						if v, err := strconv.ParseFloat(res[0][1], 10); err == nil {
							orderPercent = v
							itemIsPercent = true
						}
					}else{
						if v, err := strconv.ParseFloat(tariff.Item, 10); err == nil {
							orderFixed = v
						}
					}
				}
				// Kg
				if tariff == nil {
					kg = transport.Kg
				}else{
					kg = tariff.Kg
				}
				// M3
				if tariff == nil {
					m3 = transport.M3
				}else{
					m3 = tariff.M3
				}
				//
				// [Delivery]
				var delivery float64
				for _, item := range order.Items {
					if itemIsPercent {
						delivery += (item.Price * itemPercent / 100.0) * float64(item.Quantity)
					}else{
						delivery += itemFixed * tax * float64(item.Quantity)
					}
				}
				// Delivery: fixed
				if orderIsPercent {
					delivery += order.Sum * orderPercent / 100.0
				}else{
					delivery += orderFixed * tax
				}
				// Delivery: dynamic
				byVolume := delivery + order.Volume * m3
				byWeight := delivery + order.Weight * kg
				var value float64
				if byVolume > byWeight {
					value = byVolume
				} else {
					value = byWeight
				}
				//
				if transport.Free > 0 && order.Sum > transport.Free {
					value = 0
				}
				//
				var services []TransportServiceView
				if transport.Services != "" {
					if err = json.Unmarshal([]byte(transport.Services), &services); err != nil {
						logger.Warningf("%+v", err)
					}
				}
				//
				if request.TransportId > 0 {
					transportView = &TransportOrderView{
						ID: transport.ID,
						Name: transport.Name,
						Title: transport.Title,
						Thumbnail: transport.Thumbnail,
						//Services: services,
					}

					for _, service := range services {
						for j := 0; j < len(request.TransportServices); j++ {
							if request.TransportServices[j].Name == service.Name {
								service.Selected = true
								break
							}
						}
						if service.Selected {
							transportView.Services = append(transportView.Services, service)
							value += service.Price
						}
					}
					//
					order.TransportId = request.TransportId
					order.Delivery = math.Round(value * 100) / 100
				}else{
					deliveriesShortView = append(deliveriesShortView, DeliveryView{
						ID:        transport.ID,
						Title:     transport.Title,
						Thumbnail: transport.Thumbnail,
						ByVolume: math.Round(byVolume * 100) / 100,
						ByWeight: math.Round(byWeight * 100) / 100,
						Services: services,
						Value: math.Round(value * 100) / 100,
					})
				}
			}
		}
	}
	sort.Slice(deliveriesShortView[:], func(i, j int) bool {
		return deliveriesShortView[i].Value < deliveriesShortView[j].Value
	})
	// [/Delivery]
	// [PaymentMethods]
	var paymentMethodsView *PaymentMethodsView
	if request.PaymentMethod == "" {
		paymentMethodsView = &PaymentMethodsView{}
		if common.Config.Payment.Stripe.Enabled {
			paymentMethodsView.Stripe.Enabled = true
			paymentMethodsView.Default = "stripe"
		}
		if common.Config.Payment.Mollie.Enabled {
			paymentMethodsView.Mollie.Enabled = true
			paymentMethodsView.Mollie.Methods = reCSV.Split(common.Config.Payment.Mollie.Methods, -1)
			paymentMethodsView.Default = "mollie"
		}
		if common.Config.Payment.AdvancePayment.Enabled {
			paymentMethodsView.AdvancePayment.Enabled = true
			if tmpl, err := template.New("details").Parse(common.Config.Payment.AdvancePayment.Details); err == nil {
				var tpl bytes.Buffer
				vars := map[string]interface{}{
					/* Something should be here */
				}
				if err := tmpl.Execute(&tpl, vars); err == nil {
					paymentMethodsView.AdvancePayment.Details = tpl.String()
				}else{
					logger.Errorf("%v", err)
				}
			}else{
				logger.Errorf("%v", err)
			}
		}
		if common.Config.Payment.OnDelivery.Enabled {
			paymentMethodsView.OnDelivery.Enabled = true
		}
	}else{
		order.PaymentMethod = request.PaymentMethod
	}
	// [/PaymentMethod]
	// *****************************************************************************************************************
	// Coupons
	for _, coupon := range coupons {
		if coupon.Enabled && coupon.Start.Before(now) && coupon.End.After(now) {
			if coupon.Type == "shipment" {
				couponsView = append(couponsView, &CouponOrderView{
					ID:     coupon.ID,
					Code:   coupon.Code,
					Title:  coupon.Title,
					Type: coupon.Type,
					Amount: coupon.Amount,
				})
				if res := rePercent.FindAllStringSubmatch(coupon.Amount, 1); len(res) > 0 && len(res[0]) > 1 {
					// percent
					if p, err := strconv.ParseFloat(res[0][1], 10); err == nil {
						order.Discount2 = order.Delivery * p / 100.0
					}
				} else {
					if n, err := strconv.ParseFloat(coupon.Amount, 10); err == nil {
						order.Discount2 = n
					}
				}
				if order.Discount2 > order.Delivery {
					order.Discount2 = order.Delivery
				}
				order.Discount2 = math.Round(order.Discount2 * 100) / 100
			}
		}
	}
	//
	order.VAT = vat
	var view *OrderShortView
	if bts, err := json.Marshal(order); err == nil {
		if err = json.Unmarshal(bts, &view); err == nil {
			view.Items = itemsShortView
			view.Deliveries = deliveriesShortView
			//
			view.Transport = transportView
			view.Coupons = couponsView
			//
			view.PaymentMethods = paymentMethodsView
		} else {
			logger.Warningf("%v", err.Error())
		}
	}else{
		logger.Warningf("%v", err.Error())
	}
	order.Total = (order.Sum - order.Discount) + (order.Delivery - order.Discount2)
	// [Order Description]
	if bts, err := json.Marshal(view); err == nil {
		order.Description = string(bts)
	}
	// [/Order Description]
	return order, view, nil
}

// GetOrder godoc
// @Summary Get order
// @Accept json
// @Produce json
// @Param id path int true "Order ID"
// @Success 200 {object} OrderView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders/{id} [get]
// @Tags account
// @Tags frontend
func getAccountOrderHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var userId uint
	if v := c.Locals("user"); v != nil {
		if user, ok := v.(*models.User); ok {
			userId = user.ID
		}
	}
	if order, err := models.GetOrder(common.Database, id); err == nil {
		if order.UserId != userId {
			c.Status(http.StatusForbidden)
			return c.JSON(fiber.Map{"ERROR": "You are not allowed to do that"})
		}
		var view OrderView
		if bts, err := json.MarshalIndent(order, "", "   "); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				return c.JSON(view)
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// @security BasicAuth
// UpdateOrder godoc
// @Summary update order by user
// @Accept json
// @Produce json
// @Param user body ExistingOrder true "body"
// @Param id path int true "Order ID"
// @Success 200 {object} UserView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders/{id} [put]
// @Tags order
func putAccountOrderHandler(c *fiber.Ctx) error {
	var request ExistingOrder
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "ID is not defined"})
	}
	var order *models.Order
	var err error
	if order, err = models.GetOrder(common.Database, int(id)); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	if (order.Status == models.ORDER_STATUS_NEW || order.Status == models.ORDER_STATUS_WAITING_FROM_PAYMENT) && request.Status == models.ORDER_STATUS_CANCELED {
		order.Status = models.ORDER_STATUS_CANCELED
	}
	if err := models.UpdateOrder(common.Database, order); err == nil {
		return c.JSON(HTTPMessage{"OK"})
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

type NewStripePayment struct {
	StripeToken string
	SuccessURL string
	CancelURL string
}

type StripeCheckoutSessionView struct {
	SessionID string `json:"id"`
}

// PostOrder godoc
// @Summary Post order
// @Accept json
// @Produce json
// @Param cart body NewStripePayment true "body"
// @Success 200 {object} StripeCheckoutSessionView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders/checkout/{id}/stripe [post]
// @Tags account
func postAccountOrderCheckoutHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var userId uint
	if v := c.Locals("user"); v != nil {
		if user, ok := v.(*models.User); ok {
			userId = user.ID
		}
	}
	order, err := models.GetOrder(common.Database, id);
	if err == nil {
		if order.UserId != userId {
			c.Status(http.StatusForbidden)
			return c.JSON(fiber.Map{"ERROR": "You are not allowed to do that"})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}

	var request NewStripePayment
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	//logger.Infof("request: %+v", request)

	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{
			"card",
		}),
		Mode: stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			&stripe.CheckoutSessionLineItemParams{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String("usd"),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(fmt.Sprintf("Order #%d", order.ID)),
					},
					UnitAmount: stripe.Int64(int64(math.Round(order.Total * 100))),
				},
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(request.SuccessURL),
		CancelURL:  stripe.String(request.CancelURL),
	}

	session, err := checkout_session.New(params)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}

	data := StripeCheckoutSessionView{SessionID: session.ID}

	c.Status(http.StatusOK)
	return c.JSON(data)
}

type PaymentMethodsView struct {
	Default string
	Stripe struct {
		Enabled bool `json:",omitempty"`
	} `json:",omitempty"`
	Mollie struct {
		Enabled bool `json:",omitempty"`
		Methods []string `json:",omitempty"`
	} `json:",omitempty"`
	AdvancePayment struct {
		Enabled bool `json:",omitempty"`
		Details string `json:",omitempty"`
	}
	OnDelivery struct {
		Enabled bool `json:",omitempty"`
	}
}

// GetAccountPaymentMethods godoc
// @Summary (DEPRECATED) Get account payment methods
// @Accept json
// @Produce json
// @Success 200 {object} PaymentMethodsView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/payment_methods [get]
// @Tags account
// @Tags frontend
func getPaymentMethodsHandler(c *fiber.Ctx) error {
	if !common.Config.Payment.Enabled {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Payment is not enabled, please contact support"})
	}
	var view PaymentMethodsView
	if common.Config.Payment.Stripe.Enabled {
		view.Stripe.Enabled = true
		view.Default = "stripe"
	}
	if common.Config.Payment.Mollie.Enabled {
		view.Mollie.Enabled = true
		view.Mollie.Methods = reCSV.Split(common.Config.Payment.Mollie.Methods, -1)
		view.Default = "mollie"
	}
	if common.Config.Payment.AdvancePayment.Enabled {
		view.AdvancePayment.Enabled = true
		if tmpl, err := template.New("details").Parse(common.Config.Payment.AdvancePayment.Details); err == nil {
			var tpl bytes.Buffer
			vars := map[string]interface{}{
				/* Something should be here */
			}
			if err := tmpl.Execute(&tpl, vars); err == nil {
				view.AdvancePayment.Details = tpl.String()
			}else{
				logger.Errorf("%v", err)
			}
		}else{
			logger.Errorf("%v", err)
		}
	}
	if common.Config.Payment.OnDelivery.Enabled {
		view.OnDelivery.Enabled = true
	}
	//
	return c.JSON(view)
}

// PostAdvancePaymentOrder godoc
// @Summary Post advance-payment order
// @Accept json
// @Produce json
// @Param id path int true "Order ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders/{id}/advance_payment/submit [get]
// @Tags account
// @Tags frontend
func postAccountOrderAdvancePaymentSubmitHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var user *models.User
	if v := c.Locals("user"); v != nil {
		var ok bool
		if user, ok = v.(*models.User); !ok {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"User not found"})
		}
	}
	if order, err := models.GetOrder(common.Database, id); err == nil {
		if order.UserId != user.ID {
			c.Status(http.StatusForbidden)
			return c.JSON(fiber.Map{"ERROR": "You are not allowed to do that"})
		}
		order.Status = models.ORDER_STATUS_WAITING_FROM_PAYMENT
		if err = models.UpdateOrder(common.Database, order); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
		transaction := &models.Transaction{Amount: order.Total, Status: models.TRANSACTION_STATUS_NEW, Order: order}
		transactionPayment := models.TransactionPayment{AdvancePayment: &models.TransactionPaymentAdvancePayment{Total: order.Total}}
		if bts, err := json.Marshal(transactionPayment); err == nil {
			transaction.Payment = string(bts)
		}
		if _, err = models.CreateTransaction(common.Database, transaction); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
		c.Status(http.StatusOK)
		return c.JSON(HTTPMessage{"OK"})
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// PostOnDeliveryOrder godoc
// @Summary Post on-delivery order
// @Accept json
// @Produce json
// @Param id path int true "Order ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders/{id}/on_delivery/submit [get]
// @Tags account
// @Tags frontend
func postAccountOrderOnDeliverySubmitHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var user *models.User
	if v := c.Locals("user"); v != nil {
		var ok bool
		if user, ok = v.(*models.User); !ok {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"User not found"})
		}
	}
	if order, err := models.GetOrder(common.Database, id); err == nil {
		if order.UserId != user.ID {
			c.Status(http.StatusForbidden)
			return c.JSON(fiber.Map{"ERROR": "You are not allowed to do that"})
		}
		order.Status = models.ORDER_STATUS_WAITING_FROM_PAYMENT
		if err = models.UpdateOrder(common.Database, order); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
		transaction := &models.Transaction{Amount: order.Total, Status: models.TRANSACTION_STATUS_NEW, Order: order}
		transactionPayment := models.TransactionPayment{OnDelivery: &models.TransactionPaymentOnDelivery{Total: order.Total}}
		if bts, err := json.Marshal(transactionPayment); err == nil {
			transaction.Payment = string(bts)
		}
		if _, err = models.CreateTransaction(common.Database, transaction); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
		c.Status(http.StatusOK)
		return c.JSON(HTTPMessage{"OK"})
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

type StripeCustomerView struct {

}

// GetStripeCustomer godoc
// @Summary Get stripe customer
// @Accept json
// @Produce json
// @Param id path int true "Order ID"
// @Success 200 {object} StripeCustomerView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders/{id}/stripe/customer [get]
// @Tags account
func getAccountOrderStripeCustomerHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var user *models.User
	if v := c.Locals("user"); v != nil {
		var ok bool
		if user, ok = v.(*models.User); !ok {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"User not found"})
		}
	}
	if order, err := models.GetOrder(common.Database, id); err == nil {
		if order.UserId != user.ID {
			c.Status(http.StatusForbidden)
			return c.JSON(fiber.Map{"ERROR": "You are not allowed to do that"})
		}

		var payment models.ProfilePayment
		var customerId string
		profile, err := models.GetProfile(common.Database, order.ProfileId)
		if err == nil {
			if err := json.Unmarshal([]byte(profile.Payment), &payment); err == nil {
				customerId = payment.Stripe.CustomerId
			}else{
				logger.Warningf("%+v", customerId)
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Profile not found"})
		}

		var customer *stripe.Customer
		if customerId == "" {
			// Create customer
			if customer, err = common.STRIPE.CreateCustomer(&stripe.CustomerParams{
				Name: stripe.String(profile.Name + " " + profile.Lastname),
				Email: stripe.String(user.Email),
				Description: stripe.String(fmt.Sprintf("User #%d %v profile #%d %v %v", user.ID, user.Email, profile.ID, profile.Name, profile.Lastname)),
			}); err == nil {
				customerId = customer.ID
			} else {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			// Get customer
			if customer, err = common.STRIPE.GetCustomer(customerId); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}

		if payment.Stripe.CustomerId != customerId {
			payment.Stripe.CustomerId = customerId
			if bts, err := json.Marshal(models.ProfilePayment{Stripe:models.ProfilePaymentStripe{CustomerId: customer.ID}}); err == nil {
				profile.Payment = string(bts)
				if err = models.UpdateProfile(common.Database, profile); err != nil {
					logger.Warningf("%+v", err.Error())
				}
			}
		}

		c.Status(http.StatusOK)
		return c.JSON(customer)
	} else {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

type StripeCardsView struct {

}


// GetStripeCards godoc
// @Summary Get stripe cards
// @Accept json
// @Produce json
// @Param id path int true "Order ID"
// @Success 200 {object} StripeCardsView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders/{id}/stripe/card [get]
// @Tags account
func getAccountOrderStripeCardHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var user *models.User
	if v := c.Locals("user"); v != nil {
		var ok bool
		if user, ok = v.(*models.User); !ok {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"User not found"})
		}
	}
	if order, err := models.GetOrder(common.Database, id); err == nil {
		if order.UserId != user.ID {
			c.Status(http.StatusForbidden)
			return c.JSON(fiber.Map{"ERROR": "You are not allowed to do that"})
		}

		var payment models.ProfilePayment
		var customerId string
		profile, err := models.GetProfile(common.Database, order.ProfileId)
		if err == nil {
			if err := json.Unmarshal([]byte(profile.Payment), &payment); err == nil {
				customerId = payment.Stripe.CustomerId
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"CustomerId not set"})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Profile not found"})
		}

		cards, err := common.STRIPE.GetCards(customerId)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}

		var card *stripe.Card
		if len(cards) > 0 {
			card = cards[0]
		}

		c.Status(http.StatusOK)
		return c.JSON(card)
	} else {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

type NewStripeCard struct {
	CustomerId string
	Token string
}

type StripeCardView struct {

}

/*type StripeCheckoutSessionView struct {
	SessionID string `json:"id"`
}*/

// PostOrder godoc
// @Summary Post order
// @Description See https://stripe.com/docs/api/cards/create
// @Accept json
// @Produce json
// @Param cart body NewStripeCard true "body"
// @Success 200 {object} StripeCardView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders/checkout/{id}/stripe/card [post]
// @Tags account
func postAccountOrderStripeCardHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var userId uint
	if v := c.Locals("user"); v != nil {
		if user, ok := v.(*models.User); ok {
			userId = user.ID
		}
	}
	order, err := models.GetOrder(common.Database, id)
	if err == nil {
		if order.UserId != userId {
			c.Status(http.StatusForbidden)
			return c.JSON(fiber.Map{"ERROR": "You are not allowed to do that"})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}

	var request NewStripeCard
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	//logger.Infof("request: %+v", request)

	params := &stripe.CardParams{
		Customer: stripe.String(request.CustomerId),
		Token: stripe.String(request.Token),
	}

	if cards, err := common.STRIPE.GetCards(request.CustomerId); err == nil {
		for i := 0; i < len(cards); i++ {
			if _, err = common.STRIPE.DeleteCard(request.CustomerId, cards[i].ID); err != nil {
				logger.Warningf("%+v", err.Error())
			}
		}
	}else{
		logger.Warningf("%+v", err.Error())
	}

	card, err := common.STRIPE.CreateCard(params)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}

	c.Status(http.StatusOK)
	return c.JSON(card)
}


type StripeSecretView struct {
	ClientSecret string
}

// PostStripePayment godoc
// @Summary Post stripe payment
// @Accept json
// @Produce json
// @Param id path int true "Order ID"
// @Success 200 {object} StripeCardsView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders/{id}/stripe/submit [get]
// @Tags account
func postAccountOrderStripeSubmitHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var user *models.User
	if v := c.Locals("user"); v != nil {
		var ok bool
		if user, ok = v.(*models.User); !ok {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"User not found"})
		}
	}
	if order, err := models.GetOrderFull(common.Database, id); err == nil {
		if order.UserId != user.ID {
			c.Status(http.StatusForbidden)
			return c.JSON(fiber.Map{"ERROR": "You are not allowed to do that"})
		}
		var profilePayment models.ProfilePayment
		var customerId string
		profile, err := models.GetProfile(common.Database, order.ProfileId)
		if err == nil {
			if err := json.Unmarshal([]byte(profile.Payment), &profilePayment); err == nil {
				customerId = profilePayment.Stripe.CustomerId
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"CustomerId not set"})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Profile not found"})
		}

		//logger.Infof("Start")
		params := &stripe.PaymentIntentParams{
			Amount: stripe.Int64(int64(math.Round(order.Total * 100))),
			Currency: stripe.String(common.Config.Currency),
			Customer: stripe.String(customerId),
			/*PaymentMethodTypes: []*string{
				stripe.String("card"),
			},*/
			Confirm: stripe.Bool(true),
			//ReturnURL: stripe.String(fmt.Sprintf(CONFIG.Stripe.ConfirmURL, transactionId)),
			/*PaymentMethodOptions: &stripe.PaymentIntentPaymentMethodOptionsParams{
				Card: &stripe.PaymentIntentPaymentMethodOptionsCardParams{
					RequestThreeDSecure: stripe.String("any"),
				},
			},*/
		}
		if v := c.Request().Header.Referer(); len(v) > 0 {
			if u, err := url.Parse(string(v)); err == nil {
				u.Path = fmt.Sprintf("/api/v1/account/orders/%v/stripe/success", order.ID)
				u.RawQuery = ""
				params.ReturnURL = stripe.String(u.String())
			}
		}
		/*if bts, err := json.Marshal(params); err == nil {
			logger.Infof("params: %+v", string(bts))
		}*/
		pi, err := common.STRIPE.CreatePaymentIntent(params)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}

		if bts, err := json.Marshal(pi); err == nil {
			logger.Infof("bts: %+v", string(bts))
		}

		transaction := &models.Transaction{Amount: order.Total, Status: models.TRANSACTION_STATUS_NEW, Order: order}
		transactionPayment := models.TransactionPayment{Stripe: &models.TransactionPaymentStripe{Id: pi.ID}}
		if bts, err := json.Marshal(transactionPayment); err == nil {
			transaction.Payment = string(bts)
		}
		if _, err = models.CreateTransaction(common.Database, transaction); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}

		if pi.Status == stripe.PaymentIntentStatusRequiresPaymentMethod {
			logger.Infof("PaymentIntentStatusRequiresPaymentMethod")
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{pi.LastPaymentError.Error()})
		} else if pi.Status == stripe.PaymentIntentStatusRequiresAction {
			logger.Infof("PaymentIntentStatusRequiresAction")
			transaction.Status = models.TRANSACTION_STATUS_PENDING
			if err = models.UpdateTransaction(common.Database, transaction); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		} else if pi.Status == stripe.PaymentIntentStatusSucceeded {
			logger.Infof("PaymentIntentStatusSucceeded")
			transaction.Status = models.TRANSACTION_STATUS_COMPLETE
			if err = models.UpdateTransaction(common.Database, transaction); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			order.Status = models.ORDER_STATUS_PAID
			if err = models.UpdateOrder(common.Database, order); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			// Notifications
			if common.Config.Notification.Enabled {
				if common.Config.Notification.Email.Enabled {
					// to admin
					//logger.Infof("Time to send admin email")
					if users, err := models.GetUsersByRoleLessOrEqualsAndNotification(common.Database, models.ROLE_ADMIN, true); err == nil {
						//logger.Infof("users found: %v", len(users))
						template, err := models.GetEmailTemplateByType(common.Database, common.NOTIFICATION_TYPE_ADMIN_ORDER_PAID)
						if err == nil {
							//logger.Infof("Template: %+v", template)
							for _, user := range users {
								logger.Infof("Send email admin user: %+v", user.Email)
								if err = SendOrderPaidEmail(mail.NewEmail(user.Login, user.Email), int(order.ID), template); err != nil {
									logger.Errorf("%+v", err)
								}
							}
						}else{
							logger.Warningf("%+v", err)
						}
					}else{
						logger.Warningf("%+v", err)
					}
					// to user
					//logger.Infof("Time to send user email")
					template, err := models.GetEmailTemplateByType(common.Database, common.NOTIFICATION_TYPE_USER_ORDER_PAID)
					if err == nil {
						//logger.Infof("Template: %+v", template)
						if user, err := models.GetUser(common.Database, int(order.UserId)); err == nil {
							if user.EmailConfirmed {
								logger.Infof("Send email to user: %+v", user.Email)
								if err = SendOrderPaidEmail(mail.NewEmail(user.Login, user.Email), int(order.ID), template); err != nil {
									logger.Errorf("%+v", err)
								}
							}else{
								logger.Warningf("User's %v email %v is not confirmed", user.Login, user.Email)
							}
						} else {
							logger.Warningf("%+v", err)
						}
					}
				}
			}
			//
		}else{
			logger.Warningf("Unexpected PaymentIntentStatus: %+v", pi.Status)
		}

		c.Status(http.StatusOK)
		return c.JSON(pi)
	} else {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// GetStripePaymentSuccess godoc
// @Summary Get stripe payment success
// @Accept json
// @Produce html
// @Param id path int true "Order ID"
// @Success 200 {object} StripeCardsView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders/{id}/stripe/success [get]
// @Tags account
func getAccountOrderStripeSuccessHandler(c *fiber.Ctx) error {
	// TODO: Here you can have some business logic
	c.Response().Header.Set("Content-Type", "text/html")
	/*
	EXAMPLE:
	id: 1
	payment_intent: pi_1I0qfWLxvolFmsmRpQATpaRE
	payment_intent_client_secret: pi_1I0qfWLxvolFmsmRpQATpaRE_secret_RkC4BWzSbdOWi31TsHtReNByc
	source_type: card
	*/
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if order, err := models.GetOrderFull(common.Database, id); err == nil {
		paymentIntentClientSecret := c.Query("payment_intent_client_secret")
		if paymentIntentClientSecret != "" {
			if res := regexp.MustCompile(`^(pi_[A-Za-z0-9]+)_secret`).FindAllStringSubmatch(paymentIntentClientSecret, 1); len(res) > 0 && len(res[0]) > 1 {
				paymentIntentId := res[0][1]
				logger.Infof("Success payment callback received, PaymentIntentClientSecret: %s", paymentIntentClientSecret)
				if pi, err := common.STRIPE.GetPaymentIntent(paymentIntentId); err == nil {
					logger.Infof("PaymentIntent ID: %s, Amount: %+v, Currency: %+v, Status: %+v, Customer: %+v", pi.ID, pi.Amount, pi.Currency, pi.Status, pi.Customer)
						if transactions, err := models.GetTransactionsByOrderId(common.Database, id); err == nil {
							for _, transaction := range transactions {
								var payment models.TransactionPayment
								if err = json.Unmarshal([]byte(transaction.Payment), &payment); err == nil {
									if payment.Stripe != nil && payment.Stripe.Id == paymentIntentId {
										if pi.Status == stripe.PaymentIntentStatusSucceeded {
											transaction.Status = models.TRANSACTION_STATUS_COMPLETE
											if err = models.UpdateTransaction(common.Database, transaction); err != nil {
												c.Status(http.StatusInternalServerError)
												return c.JSON(HTTPError{err.Error()})
											}
											order.Status = models.ORDER_STATUS_PAID
											if err = models.UpdateOrder(common.Database, order); err != nil {
												c.Status(http.StatusInternalServerError)
												return c.JSON(HTTPError{err.Error()})
											}
											// Notifications
											if common.Config.Notification.Enabled {
												if common.Config.Notification.Email.Enabled {
													// to admin
													//logger.Infof("Time to send admin email")
													if users, err := models.GetUsersByRoleLessOrEqualsAndNotification(common.Database, models.ROLE_ADMIN, true); err == nil {
														//logger.Infof("users found: %v", len(users))
														template, err := models.GetEmailTemplateByType(common.Database, common.NOTIFICATION_TYPE_ADMIN_ORDER_PAID)
														if err == nil {
															//logger.Infof("Template: %+v", template)
															for _, user := range users {
																logger.Infof("Send email admin user: %+v", user.Email)
																if err = SendOrderPaidEmail(mail.NewEmail(user.Login, user.Email), int(order.ID), template); err != nil {
																	logger.Errorf("%+v", err)
																}
															}
														}else{
															logger.Warningf("%+v", err)
														}
													}else{
														logger.Warningf("%+v", err)
													}
													// to user
													//logger.Infof("Time to send user email")
													template, err := models.GetEmailTemplateByType(common.Database, common.NOTIFICATION_TYPE_USER_ORDER_PAID)
													if err == nil {
														//logger.Infof("Template: %+v", template)
														if user, err := models.GetUser(common.Database, int(order.UserId)); err == nil {
															if user.EmailConfirmed {
																logger.Infof("Send email to user: %+v", user.Email)
																if err = SendOrderPaidEmail(mail.NewEmail(user.Login, user.Email), int(order.ID), template); err != nil {
																	logger.Errorf("%+v", err)
																}
															}else{
																logger.Warningf("User's %v email %v is not confirmed", user.Login, user.Email)
															}
														} else {
															logger.Warningf("%+v", err)
														}
													}
												}
											}
											c.Status(http.StatusOK)
											return c.SendString("OK<script>window.top.postMessage('3DS-authentication-complete:" + c.Query("payment_intent_client_secret") + "');</script>")
										} else {
											payment.Stripe.Error = fmt.Sprintf("RequestID: %v, ChargeID: %v, Msg: %v", pi.LastPaymentError.RequestID, pi.LastPaymentError.ChargeID, pi.LastPaymentError.Msg)
											if bts, err := json.Marshal(payment); err == nil {
												transaction.Payment = string(bts)
											}
											transaction.Status = models.TRANSACTION_STATUS_REJECT
											if err = models.UpdateTransaction(common.Database, transaction); err != nil {
												c.Status(http.StatusInternalServerError)
												return c.JSON(HTTPError{err.Error()})
											}
											err = fmt.Errorf("%v", pi.LastPaymentError.Msg)
											logger.Errorf("%+v", err)
											c.Status(http.StatusInternalServerError)
											return c.SendString("<b>ERROR:</b> " + err.Error())
										}
									}
								}else{
									logger.Errorf("%+v", err)
									c.Status(http.StatusInternalServerError)
									return c.SendString("<b>ERROR:</b> " + err.Error())
								}
							}
							err = fmt.Errorf("no corresponding transaction entity found")
							logger.Errorf("%+v", err)
							c.Status(http.StatusInternalServerError)
							return c.SendString("<b>ERROR:</b> " + err.Error())
						}else{
							logger.Errorf("%+v", err)
							c.Status(http.StatusInternalServerError)
							return c.SendString("<b>ERROR:</b> " + err.Error())
						}
				} else {
					logger.Errorf("%+v", err)
					c.Status(http.StatusInternalServerError)
					return c.SendString("<b>ERROR:</b> " + err.Error())
				}
			}else{
				err = fmt.Errorf("incorrect payment_intent_client_secret")
				logger.Errorf("%+v", err)
				c.Status(http.StatusInternalServerError)
				return c.SendString("<b>ERROR:</b> " + err.Error())
			}
		}else{
			err = fmt.Errorf("missed payment_intent_client_secret")
			logger.Errorf("%+v", err)
			c.Status(http.StatusInternalServerError)
			return c.SendString("<b>ERROR:</b> " + err.Error())
		}
	}else{
		logger.Errorf("%+v", err)
		c.Status(http.StatusInternalServerError)
		return c.SendString("<b>ERROR:</b> " + err.Error())
	}
}

type MollieOrderView struct {
	Id string
	ProfileId string `json:",omitempty"`
	Checkout string `json:",omitempty"`
}

type MollieSubmitRequest struct {
	Language string `json:"language"`
	Method string
}

// PostMollieOrder godoc
// @Summary Post mollie order
// @Accept json
// @Produce json
// @Param id path int true "Order ID"
// @Query method query string true "for example: postMessage"
// @Param form body MollieSubmitRequest true "body"
// @Success 200 {object} MollieOrderView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders/{id}/mollie/submit [post]
// @Tags account
// @Tags frontend
func postAccountOrderMollieSubmitHandler(c *fiber.Ctx) error {
	logger.Infof("postAccountOrderMollieSubmitHandler")
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var user *models.User
	if v := c.Locals("user"); v != nil {
		var ok bool
		if user, ok = v.(*models.User); !ok {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"User not found"})
		}
	}
	if order, err := models.GetOrder(common.Database, id); err == nil {
		if order.UserId != user.ID {
			c.Status(http.StatusForbidden)
			return c.JSON(fiber.Map{"ERROR": "You are not allowed to do that"})
		}
		// PAYLOAD
		//logger.Infof("order: %+v", order)

		var base string
		var redirectUrl string
		if v := c.Request().Header.Referer(); len(v) > 0 {
			if u, err := url.Parse(string(v)); err == nil {
				u.Path = ""
				u.RawQuery = ""
				base = u.String()
				u.Path = fmt.Sprintf("/api/v1/account/orders/%v/mollie/success", order.ID)
				values := url.Values{}
				if v := c.Query("method", ""); v != "" {
					values.Set("method", v)
					u.RawQuery = values.Encode()
				}
				redirectUrl = u.String()
			}
		}

		var request MollieSubmitRequest
		if err := c.BodyParser(&request); err != nil {
			return err
		}
		//
		o := &mollie.Order{
			OrderNumber: fmt.Sprintf("%d", order.ID),
		}
		//
		// Lines
		var orderShortView OrderShortView
		if err := json.Unmarshal([]byte(order.Description), &orderShortView); err == nil {
			re := regexp.MustCompile(`^\[(.*)\]$`)
			for _, item := range orderShortView.Items {
				sku := strings.Replace(re.ReplaceAllString(item.Uuid, ""), ",", ".", -1)
				meta := map[string]string{"variation": item.Variation.Title}
				line := mollie.Line{
					Type:           "physical",
					Category:       "gift",
					Sku:            sku,
					Name:           item.Title,
					ProductUrl:     base + item.Path,
					Metadata:       meta,
					Quantity:       item.Quantity,
					UnitPrice:      mollie.NewAmount(strings.ToUpper(common.Config.Currency), item.Price),
					DiscountAmount: mollie.NewAmount(strings.ToUpper(common.Config.Currency), item.Discount * float64(item.Quantity)),
					TotalAmount:    mollie.NewAmount(strings.ToUpper(common.Config.Currency), item.Total),
					VatAmount:      mollie.NewAmount(strings.ToUpper(common.Config.Currency), item.Total * (common.Config.Payment.VAT / 100.0) / ((100.0 + common.Config.Payment.VAT) / 100.0)),
					VatRate:        fmt.Sprintf("%.2f", common.Config.Payment.VAT),
				}
				/*total := item.Total
				if item.Discount > 0 {
					total = item.Total
					line.DiscountAmount = mollie.NewAmount(strings.ToUpper(common.Config.Currency), item.Discount * float64(item.Quantity))
				}else if order.Discount > 0 {
					discount := 1 - (order.Discount / order.Sum)
					total = (item.Price * discount) * float64(item.Quantity)
					line.DiscountAmount = mollie.NewAmount(strings.ToUpper(common.Config.Currency), item.Price * (1 - discount) * float64(item.Quantity))
				}else{
					line.DiscountAmount = mollie.NewAmount(strings.ToUpper(common.Config.Currency), 0)
				}
				line.TotalAmount = mollie.NewAmount(strings.ToUpper(common.Config.Currency), total)
				line.VatAmount = mollie.NewAmount(strings.ToUpper(common.Config.Currency), total * (common.Config.Payment.ITN / 100.0) / ((100.0 + common.Config.Payment.ITN) / 100.0))*/
				if item.Thumbnail != "" {
					line.ImageUrl = base + strings.Split(item.Thumbnail, ",")[0]
				}
				o.Lines = append(o.Lines, line)
			}
		}
		if order.Delivery > 0 {
			o.Lines = append(o.Lines, mollie.Line{
				Type: "shipping_fee",
				Name: "Shipping Fee",
				Quantity:       1,
				VatRate:        fmt.Sprintf("%.2f", common.Config.Payment.VAT),
				UnitPrice:      mollie.NewAmount(strings.ToUpper(common.Config.Currency), order.Delivery),
				TotalAmount:    mollie.NewAmount(strings.ToUpper(common.Config.Currency), order.Delivery - order.Discount2),
				DiscountAmount: mollie.NewAmount(strings.ToUpper(common.Config.Currency), order.Discount2),
				VatAmount:      mollie.NewAmount(strings.ToUpper(common.Config.Currency), (order.Delivery - order.Discount2) * (common.Config.Payment.VAT / 100.0) / ((100.0 + common.Config.Payment.VAT) / 100.0)),
			})
		}
		//
		o.Amount = mollie.NewAmount(strings.ToUpper(common.Config.Currency), order.Total)
		o.RedirectUrl = redirectUrl
		bts, _ := json.Marshal(o)
		logger.Infof("Bts: %+v", string(bts))
		// Address
		address := mollie.Address{
			Email: user.Email,
		}
		if profile, err := models.GetProfile(common.Database, order.ProfileId); err == nil {
			address.GivenName = profile.Name
			address.FamilyName = profile.Lastname
			address.OrganizationName = profile.Company
			address.StreetAndNumber = profile.Address
			address.Phone = profile.Phone
			address.City = profile.City
			address.PostalCode = profile.Zip
			address.Country = profile.Country
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
		o.BillingAddress = address
		o.ShippingAddress = address
		o.Locale = "en_US"
		// Locale
		if request.Language != "" {
			if request.Language == "de" {
				o.Locale = "de_DE"
			}
		}
		// Method
		if request.Method != "" {
			o.Method = request.Method
		}
		if bts, err := json.Marshal(o); err == nil {
			logger.Infof("Bts: %+v", string(bts))
		}

		if o, links, err := common.MOLLIE.CreateOrder(o); err == nil {
			logger.Infof("o: %+v", o)
			transaction := &models.Transaction{Amount: order.Total, Status: models.TRANSACTION_STATUS_NEW, Order: order}
			transactionPayment := models.TransactionPayment{Mollie: &models.TransactionPaymentMollie{Id: o.Id}}
			if bts, err := json.Marshal(transactionPayment); err == nil {
				transaction.Payment = string(bts)
			}
			if _, err = models.CreateTransaction(common.Database, transaction); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}

			var response MollieOrderView

			response.Id = o.Id
			if link, found := links["checkout"]; found {
				response.Checkout = link.Href
			}

			c.Status(http.StatusOK)
			return c.JSON(response)
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	} else {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// GetMolliePaymentSuccess godoc
// @Summary Get mollie payment success
// @Accept json
// @Produce html
// @Param id path int true "Order ID"
// @Success 200 {object} StripeCardsView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders/{id}/mollie/success [get]
// @Tags account
// @Tags frontend
func getAccountOrderMollieSuccessHandler(c *fiber.Ctx) error {
	c.Response().Header.Set("Content-Type", "text/html")
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if order, err := models.GetOrderFull(common.Database, id); err == nil {
		if transactions, err := models.GetTransactionsByOrderId(common.Database, id); err == nil {
			for _, transaction := range transactions {
				var payment models.TransactionPayment
				if err = json.Unmarshal([]byte(transaction.Payment), &payment); err == nil {
					if payment.Mollie != nil && payment.Mollie.Id != "" {
						if o, err := common.MOLLIE.GetOrder(payment.Mollie.Id); err == nil {
							if o.Status == "paid" {
								transaction.Status = models.TRANSACTION_STATUS_COMPLETE
								if err = models.UpdateTransaction(common.Database, transaction); err != nil {
									c.Status(http.StatusInternalServerError)
									return c.JSON(HTTPError{err.Error()})
								}
								order.Status = models.ORDER_STATUS_PAID
								if err = models.UpdateOrder(common.Database, order); err != nil {
									c.Status(http.StatusInternalServerError)
									return c.JSON(HTTPError{err.Error()})
								}
								// Notifications
								if common.Config.Notification.Enabled {
									if common.Config.Notification.Email.Enabled {
										// to admin
										//logger.Infof("Time to send admin email")
										if users, err := models.GetUsersByRoleLessOrEqualsAndNotification(common.Database, models.ROLE_ADMIN, true); err == nil {
											//logger.Infof("users found: %v", len(users))
											template, err := models.GetEmailTemplateByType(common.Database, common.NOTIFICATION_TYPE_ADMIN_ORDER_PAID)
											if err == nil {
												//logger.Infof("Template: %+v", template)
												for _, user := range users {
													logger.Infof("Send email admin user: %+v", user.Email)
													if err = SendOrderPaidEmail(mail.NewEmail(user.Login, user.Email), int(order.ID), template); err != nil {
														logger.Errorf("%+v", err)
													}
												}
											}else{
												logger.Warningf("%+v", err)
											}
										}else{
											logger.Warningf("%+v", err)
										}
										// to user
										//logger.Infof("Time to send user email")
										template, err := models.GetEmailTemplateByType(common.Database, common.NOTIFICATION_TYPE_USER_ORDER_PAID)
										if err == nil {
											//logger.Infof("Template: %+v", template)
											if user, err := models.GetUser(common.Database, int(order.UserId)); err == nil {
												if user.EmailConfirmed {
													logger.Infof("Send email to user: %+v", user.Email)
													if err = SendOrderPaidEmail(mail.NewEmail(user.Login, user.Email), int(order.ID), template); err != nil {
														logger.Errorf("%+v", err)
													}
												}else{
													logger.Warningf("User's %v email %v is not confirmed", user.Login, user.Email)
												}
											} else {
												logger.Warningf("%+v", err)
											}
										}
									}
								}
								return return1(c, http.StatusOK, map[string]interface{}{"MESSAGE": "OK", "Status": o.Status})
							}else{
								return return1(c, http.StatusInternalServerError, map[string]interface{}{"ERROR": "Unknown status: " + o.Status, "Status": o.Status})
							}
						}else{
							logger.Errorf("%+v", err)
							return return1(c, http.StatusInternalServerError, map[string]interface{}{"ERROR": err.Error()})
						}
					}
				}else{
					logger.Errorf("%+v", err)
					return return1(c, http.StatusInternalServerError, map[string]interface{}{"ERROR": err.Error()})
				}
			}
		}else{
			logger.Errorf("%+v", err)
			return return1(c, http.StatusInternalServerError, map[string]interface{}{"ERROR": err.Error()})
		}
	}else{
		logger.Errorf("%+v", err)
		return return1(c, http.StatusInternalServerError, map[string]interface{}{"ERROR": err.Error()})
	}
	err := fmt.Errorf("something wrong")
	logger.Errorf("%+v", err)
	return return1(c, http.StatusInternalServerError, map[string]interface{}{"ERROR": err.Error()})
}

func return1(c *fiber.Ctx, status int, raw map[string]interface{}) error {
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTML)
	switch c.Query("method", "") {
	case "postMessage":
		if bts, err := json.Marshal(raw); err == nil {
			c.Status(status)
			return c.SendString("<script>window.opener.postMessage(JSON.stringify(" + string(bts) + "),'*');</script>")
		}else{
			c.Status(http.StatusInternalServerError)
			return c.SendString("<b>ERROR:</b> " + err.Error())
		}
	default:
		c.Status(status)
		if status == http.StatusOK {
			return c.SendString(fmt.Sprintf("%+v<script>window.close()</script>", raw["MESSAGE"]))
		}else{
			return c.SendString(fmt.Sprintf("<b>ERROR:</b> %+v", raw["ERROR"]))
		}
	}
}

/* *** */

type CategoriesView []CategoryView

type CategoryView struct {
	ID uint
	Name string
	Title string
	Thumbnail string
	Description string `json:",omitempty"`
	Type string `json:",omitempty"` // "category", "product"
	Children []*CategoryView `json:",omitempty"`
	Parents []*CategoryView `json:",omitempty"`
	Products int64 `json:",omitempty"`
}

func GetCategoriesView(connector *gorm.DB, id int, depth int, noProducts bool) (*CategoryView, error) {
	if id == 0 {
		return getChildrenCategoriesView(connector, &CategoryView{Name: "root", Title: "Root", Type: "category"}, depth, noProducts), nil
	} else {
		if category, err := models.GetCategory(connector, id); err == nil {
			view := getChildrenCategoriesView(connector, &CategoryView{ID: category.ID, Name: category.Name, Title: category.Title, Thumbnail: category.Thumbnail, Description: category.Description, Type: "category"}, depth, noProducts)
			if view != nil {
				if err = getParentCategoriesView(connector, view, category.ParentId); err != nil {
					return nil, err
				}
			}
			return view, nil
		}else{
			return nil, err
		}
	}
}

func getChildrenCategoriesView(connector *gorm.DB, root *CategoryView, depth int, noProducts bool) *CategoryView {
	for _, category := range models.GetChildrenOfCategoryById(connector, root.ID) {
		if depth > 0 {
			child := getChildrenCategoriesView(connector, &CategoryView{ID: category.ID, Name: category.Name, Title: category.Title, Thumbnail: category.Thumbnail, Description: category.Description, Type: "category"}, depth - 1, noProducts)
			root.Children = append(root.Children, child)
		}
	}
	if !noProducts {
		if products, err := models.GetProductsByCategoryId(connector, root.ID); err == nil {
			for _, product := range products {
				var thumbnail string
				if product.Image != nil {
					thumbnail = product.Image.Url
				}
				root.Children = append(root.Children, &CategoryView{ID: product.ID, Name: product.Name, Title: product.Title, Thumbnail: thumbnail, Description: product.Description, Type: "product"})
			}
		}
	}
	return root
}

func getParentCategoriesView(connector *gorm.DB, node *CategoryView, pid uint) error {
	if pid == 0 {
		node.Parents = append([]*CategoryView{{Name: "root", Title: "Root"}}, node.Parents...)
	} else {
		if category, err := models.GetCategory(connector, int(pid)); err == nil {
			node.Parents = append([]*CategoryView{{ID: category.ID, Name: category.Name, Title: category.Title, Thumbnail: category.Thumbnail, Description: category.Description}}, node.Parents...)
			return getParentCategoriesView(connector, node, category.ParentId)
		} else {
			return err
		}
	}
	return nil
}

type ProductView struct {
	ID uint
	Enabled bool
	Name string
	Title string
	Thumbnail string `json:",omitempty"`
	Description string `json:",omitempty"`
	Parameters []ParameterView `json:",omitempty"`
	CustomParameters string `json:",omitempty"`
	Variation string `json:",omitempty"`
	BasePrice float64 `json:",omitempty"`
	SalePrice float64 `json:",omitempty"`
	Start *time.Time `json:",omitempty"`
	End *time.Time `json:",omitempty"`
	Pattern string `json:",omitempty"`
	Dimensions string `json:",omitempty"`
	Width float64 `json:",omitempty"`
	Height float64 `json:",omitempty"`
	Depth float64 `json:",omitempty"`
	Weight float64 `json:",omitempty"`
	Availability string `json:",omitempty"`
	Sending string `json:",omitempty"`
	Sku string
	Content string
	Properties []struct {
		ID uint
		Name string
		Title string
		Filtering bool
		Option struct {
			ID uint
			Name string
			Title string
			Description string `json:",omitempty"`
		}
		Prices []struct {
			ID uint
			Enabled bool
			Value struct {
				ID uint
				Title string
				Thumbnail string `json:",omitempty"`
				Availability string `json:",omitempty"`
				Sending string `json:",omitempty"`
			}
			Price float64
			Availability string `json:",omitempty"`
			Sending string `json:",omitempty"`
		}
	} `json:",omitempty"`
	Variations []VariationView `json:",omitempty"`
	Files []File2View `json:",omitempty"`
	ImageId int `json:",omitempty"`
	Images []ImageView `json:",omitempty"`
	//
	VendorId int `json:",omitempty"`
	TimeId int `json:",omitempty"`
	//
	Categories []CategoryView `json:",omitempty"`
	Tags []TagView `json:",omitempty"`
	RelatedProducts []RelatedProduct `json:",omitempty"`
	//
	Customization string `json:",omitempty"`
	New bool `json:",omitempty"`
}

type RelatedProduct struct {
	ID uint
}

type VariationsView []VariationView

type VariationView struct {
	ID uint
	Name string
	Title string
	Description string `json:",omitempty"`
	Thumbnail string `json:",omitempty"`
	BasePrice float64
	SalePrice float64 `json:",omitempty"`
	Start *time.Time `json:",omitempty"`
	End *time.Time `json:",omitempty"`
	Properties []struct {
		ID uint
		Name string
		Title string
		Filtering bool
		Option struct {
			ID uint
			Name string
			Title string
			Description string `json:",omitempty"`
		}
		Prices []struct {
			ID uint
			Enabled bool
			Value struct {
				ID uint
				Title string
				Thumbnail string `json:",omitempty"`
				Availability string `json:",omitempty"`
				Sending string `json:",omitempty"`
			}
			Price float64
			Availability string `json:",omitempty"`
			Sending string `json:",omitempty"`
		}
	}
	Pattern string `json:",omitempty"`
	Dimensions string `json:",omitempty"`
	Width float64 `json:",omitempty"`
	Height float64 `json:",omitempty"`
	Depth float64 `json:",omitempty"`
	Weight float64 `json:",omitempty"`
	Availability string `json:",omitempty"`
	//Sending string `json:",omitempty"`
	TimeId uint `json:",omitempty"`
	Sku string
	Files []File2View `json:",omitempty"`
	Images []ImageView `json:",omitempty"`
	ProductId uint
	Customization string
	New bool `json:",omitempty"`
}

type HTTPMessage struct {
	MESSAGE  string
}

type HTTPError struct {
	ERROR  string
}

func NewPassword(length int) string {
	var password string
	chars := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < length; i++ {
		password += string(chars[random.Intn(len(chars) - 1)])
	}
	return password
}

/**/
type DeliveriesCosts []DeliveryCost

type DeliveryCost struct {
	ID uint // transport ID
	Title string
	Thumbnail string
	By string
	OrderFee string `json:",omitempty"`
	ItemFee string `json:",omitempty"`
	ByVolume float64
	Volume float64
	ByWeight float64
	Weight float64
	Fees float64
	Sum float64
	Cost string
	Value float64
	//
	Special bool // in case of ZIP match
}

func Delivery(transport *models.Transport, tariff *models.Tariff, items []NewItem) (*DeliveryCost, error) {
	result := &DeliveryCost{
		ID: transport.ID,
		Title: transport.Title,
		Thumbnail: transport.Thumbnail,
		Special: tariff != nil,
	}
	//
	// Order
	var orderFixed, orderPercent float64
	var orderIsPercent bool
	if tariff == nil {
		if res := rePercent.FindAllStringSubmatch(transport.Order, 1); len(res) > 0 && len(res[0]) > 1 {
			if v, err := strconv.ParseFloat(res[0][1], 10); err == nil {
				orderPercent = v
				orderIsPercent = true
			}
		}else{
			if v, err := strconv.ParseFloat(transport.Order, 10); err == nil {
				orderFixed = v
			}
		}
	}else{
		if res := rePercent.FindAllStringSubmatch(tariff.Order, 1); len(res) > 0 && len(res[0]) > 1 {
			if v, err := strconv.ParseFloat(res[0][1], 10); err == nil {
				orderPercent = v
				orderIsPercent = true
			}
		}else{
			if v, err := strconv.ParseFloat(tariff.Order, 10); err == nil {
				orderFixed = v
			}
		}
	}

	// Item
	var itemFixed, itemPercent float64
	var itemIsPercent bool
	if tariff == nil {
		if res := rePercent.FindAllStringSubmatch(transport.Item, 1); len(res) > 0 && len(res[0]) > 1 {
			if v, err := strconv.ParseFloat(res[0][1], 10); err == nil {
				itemPercent = v
				itemIsPercent = true
			}
		}else{
			if v, err := strconv.ParseFloat(transport.Item, 10); err == nil {
				itemFixed = v
			}
		}
	}else{
		if res := rePercent.FindAllStringSubmatch(tariff.Item, 1); len(res) > 0 && len(res[0]) > 1 {
			if v, err := strconv.ParseFloat(res[0][1], 10); err == nil {
				orderPercent = v
				itemIsPercent = true
			}
		}else{
			if v, err := strconv.ParseFloat(tariff.Item, 10); err == nil {
				orderFixed = v
			}
		}
	}
	// Kg
	var kg float64
	if tariff == nil {
		kg = transport.Kg
	}else{
		kg = tariff.Kg
	}
	// M3
	var m3 float64
	if tariff == nil {
		m3 = transport.M3
	}else{
		m3 = tariff.M3
	}

	// 2 Calculate Volume and Weight
	for _, item := range items {
		var arr []int
		if err := json.Unmarshal([]byte(item.UUID), &arr); err == nil && len(arr) >= 2 {
			productId := arr[0]
			var product *models.Product
			if product, err = models.GetProduct(common.Database, productId); err != nil {
				return nil, err
			}
			//
			variationId := arr[1]
			//var vId uint
			//var title string
			var basePrice, /*salePrice,*/ weight float64
			//var start, end time.Time
			//var dimensions string
			var width, height, depth float64
			if variationId == 0 {
				//title = "default"
				basePrice = product.BasePrice
				//salePrice = product.SalePrice
				//start = product.Start
				//end = product.End
				//dimensions = product.Dimensions
				width = product.Width
				height = product.Height
				depth = product.Depth
				weight = product.Weight
			} else {
				var variation *models.Variation
				if variation, err = models.GetVariation(common.Database, variationId); err != nil {
					return nil, err
				}
				if product.ID != variation.ProductId {
					err = fmt.Errorf("Products and Variation mismatch")
					return nil, err
				}
				//vId = variation.ID
				//title = variation.Title
				basePrice = variation.BasePrice
				//salePrice = variation.SalePrice
				//start = variation.Start
				//end = variation.End
				//dimensions = variation.Dimensions
				width = product.Width
				height = product.Height
				depth = product.Depth
				weight = variation.Weight
			}
			// Sum
			sum := basePrice
			for _, id := range arr[2:] {
				if price, err := models.GetPrice(common.Database, id); err == nil {
					sum += price.Price
				}
			}
			result.Sum += sum
			// Fee
			var fee float64
			if itemIsPercent {
				fee += sum * itemPercent / 100.0
			}else{
				fee = itemFixed
			}
			// Volume
			volume := width * height * depth / 1000000.0
			result.Volume += volume
			result.ByVolume += volume * m3 + fee
			// Weight
			result.Weight += weight
			result.ByWeight += weight * kg + fee
			// Calculate
		}
	}
	if result.Weight < transport.Weight && result.Volume < transport.Volume {
		return nil, fmt.Errorf("transport is not avialable for such weight and volume")
	}
	// Order fee
	if orderIsPercent {
		//result.ByVolume += result.ByVolume * orderPercent / 100.0
		//result.ByVolume = result.Volume * m3 + (result.Sum * orderPercent / 100.0)
		result.ByVolume += result.Sum * orderPercent / 100.0
		//result.ByWeight += result.ByWeight * orderPercent / 100.0
		//result.ByWeight = result.Weight * kg + (result.Sum * orderPercent / 100.0)
		result.ByWeight += result.Sum * orderPercent / 100.0
	}else{
		result.ByVolume += orderFixed
		result.ByWeight += orderFixed
	}
	//
	if result.ByVolume > result.ByWeight {
		result.By = "Volume"
		result.Value = result.ByVolume
	}else{
		result.By = "Weight"
		result.Value = result.ByWeight
	}
	result.Cost = fmt.Sprintf("%.2f", result.Value)
	return result, nil
}

type DiscountCost struct {
	Coupons []struct{
		ID uint // coupon id
		Code string
		Sum float64
		Delivery float64
	}
	Sum float64 // affect to sum price
	Delivery float64 // affect to delivery price
}

/*func Discount(coupons []*models.Coupon, items []NewItem) (*DiscountCost, error) {
	// 1
	for _, item := range items {
		var arr []int
		if err := json.Unmarshal([]byte(item.UUID), &arr); err == nil && len(arr) >= 2 {
			productId := arr[0]
			var product *models.Product
			if product, err = models.GetProduct(common.Database, productId); err != nil {
				return nil, err
			}
			//
			variationId := arr[1]
			var variation *models.Variation
			if variation, err = models.GetVariation(common.Database, variationId); err != nil {
				return nil, err
			}
			if product.ID != variation.ProductId {
				err = fmt.Errorf("Products and Variation mismatch")
				return nil, err
			}
			// Sum
			sum := variation.BasePrice
			for _, id := range arr[2:] {
				if price, err := models.GetPrice(common.Database, id); err == nil {
					sum += price.Price
				}
			}
			result.Sum += sum
			// Fee
			var fee float64
			if itemIsPercent {
				fee += sum * itemPercent / 100.0
			}else{
				fee = itemFixed
			}
			// Volume
			if res := reVolume.FindAllStringSubmatch(variation.Dimensions, 1); len(res) > 0 && len(res[0]) > 1 {
				var width float64
				if v, err := strconv.ParseFloat(res[0][1], 10); err == nil {
					width = v
				}
				var height float64
				if v, err := strconv.ParseFloat(res[0][2], 10); err == nil {
					height = v
				}
				var depth float64
				if v, err := strconv.ParseFloat(res[0][3], 10); err == nil {
					depth = v
				}
				volume := width * height * depth / 1000000.0
				result.Volume += volume
				result.ByVolume += volume * m3 + fee
			}
			// Weight
			weight := variation.Weight
			result.Weight += variation.Weight
			result.ByWeight += weight * kg + fee
			// Calculate
		}
	}
	return nil, nil
}*/

func SendOrderPaidEmail(to *mail.Email, orderId int, template *models.EmailTemplate) error {
	vars := &common.NotificationTemplateVariables{ }
	if order, err := models.GetOrderFull(common.Database, orderId); err == nil {
		var orderView struct {
			models.Order
			Items []struct {
				ItemShortView
				Description string
			}
		}
		if bts, err := json.Marshal(order); err == nil {
			if err = json.Unmarshal(bts, &orderView); err == nil {
				for i := 0; i < len(orderView.Items); i++ {
					var itemView ItemShortView
					if err = json.Unmarshal([]byte(orderView.Items[i].Description), &itemView); err == nil {
						orderView.Items[i].Variation = itemView.Variation
						orderView.Items[i].Properties = itemView.Properties
					}
				}
				vars.Order = orderView
			} else {
				logger.Infof("%+v", err)
			}
		}
	}
	//
	return common.NOTIFICATION.SendEmail(mail.NewEmail(common.Config.Notification.Email.Name, common.Config.Notification.Email.Email), to, template.Topic, template.Message, vars)
}

func encrypt(key, text []byte) ([]byte, error) {
	// IMPORTANT: Key should be 32 bytes length, if different make md5sum of key to have exactly 32 bytes!
	if len(key) != 32 {
		key = []byte(fmt.Sprintf("%x", md5.Sum(key)))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	b := base64.StdEncoding.EncodeToString(text)
	ciphertext := make([]byte, aes.BlockSize+len(b))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(crypto_rand.Reader, iv); err != nil {
		return nil, err
	}
	cfb := cipher.NewCFBEncrypter(block, iv)
	cfb.XORKeyStream(ciphertext[aes.BlockSize:], []byte(b))
	return ciphertext, nil
}

func decrypt(key, text []byte) ([]byte, error) {
	// IMPORTANT: Key should be 32 bytes length, if different make md5sum of key to have exactly 32 bytes!
	if len(key) != 32 {
		key = []byte(fmt.Sprintf("%x", md5.Sum(key)))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(text) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}
	iv := text[:aes.BlockSize]
	text = text[aes.BlockSize:]
	cfb := cipher.NewCFBDecrypter(block, iv)
	cfb.XORKeyStream(text, text)
	data, err := base64.StdEncoding.DecodeString(string(text))
	if err != nil {
		return nil, err
	}
	return data, nil
}