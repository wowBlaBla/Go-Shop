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
	"github.com/yonnic/goshop/config"
	"github.com/yonnic/goshop/models"
	"github.com/yonnic/goshop/storage"
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
	rePercent = regexp.MustCompile(`^(\d+(:?\.\d{1,3})?)%$`)
	reName = regexp.MustCompile(`(.+?)-(\d+)$`)
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
	v1.Get("/login", getLoginHandler)
	v1.Post("/login", postLoginHandler)
	v1.Post("/register", csrf, postRegisterHandler)
	v1.Post("/reset", csrf, postResetHandler)
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
	v1.Patch("/categories/:id", authRequired, changed("category updated"), patchCategoryHandler)
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
	v1.Patch("/products/:id", authRequired, patchProductHandler)
	v1.Post("/products/:id/max", postProductMaxHandler)
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
	v1.Patch("/variations/:id", authRequired, patchVariationHandler)
	v1.Put("/variations/:id", authRequired, changed("variation updated"), putVariationHandler)
	v1.Delete("/variations/:id", authRequired, changed("variation deleted"), delVariationHandler)
	//
	v1.Post("/properties", authRequired, changed("property created"), postPropertyHandler)
	v1.Post("/properties/list", authRequired, postPropertiesListHandler)
	v1.Get("/properties/:id", authRequired, getPropertyHandler)
	v1.Put("/properties/:id", authRequired, changed("property updated"), putPropertyHandler)
	v1.Delete("/properties/:id", authRequired, changed("property deleted"), deletePropertyHandler)
	//
	v1.Get("/rates", authRequired, getRatesHandler)
	v1.Post("/rates/list", authRequired, postRatesListHandler)
	v1.Post("/rates", authRequired, changed("rate created"), postRateHandler)
	v1.Get("/rates/:id", authRequired, getRateHandler)
	v1.Put("/rates/:id", authRequired, changed("rate updated"), putRateHandler)
	v1.Delete("/rates/:id", authRequired, changed("rate deleted"), deleteRateHandler)
	//
	v1.Get("/prices", authRequired, getPricesHandler)
	//v1.Post("/prices/list", authRequired, postPricesListHandler)
	v1.Post("/prices", authRequired, changed("price created"), postPriceHandler)
	v1.Post("/prices/all", authRequired, changed("prices created"), postPriceAllHandler)
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
	// Menu
	v1.Post("/menus", authRequired, changed("menu created"), postMenuHandler)
	v1.Post("/menus/list", authRequired, postMenusListHandler)
	v1.Get("/menus/:id", authRequired, getMenuHandler)
	v1.Put("/menus/:id", authRequired, changed("menu updated"), putMenuHandler)
	v1.Delete("/menus/:id", authRequired, changed("menu updated"), delMenuHandler)
	//
	v1.Get("/comments", getCommentsHandler)
	v1.Post("/comments", authRequired, postCommentHandler)
	v1.Get("/comments/:id", authRequired, getCommentHandler)
	v1.Put("/comments/:id", authRequired, putCommentHandler)
	v1.Delete("/comments/:id", authRequired, delCommentHandler)
	//
	v1.Get("/me", authRequired, getMeHandler)
	//
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
	v1.Get("/account/billing_profiles", authRequired, getAccountBillingProfilesHandler)
	v1.Post("/account/billing_profiles", authRequired, postAccountBillingProfileHandler)
	v1.Get("/account/billing_profiles/:id", authRequired, getAccountBillingProfileHandler)
	v1.Put("/account/billing_profiles/:id", authRequired, putAccountBillingProfileHandler)
	v1.Delete("/account/billing_profiles/:id", authRequired, delAccountBillingProfileHandler)
	v1.Get("/account/shipping_profiles", authRequired, getAccountShippingProfilesHandler)
	v1.Post("/account/shipping_profiles", authRequired, postAccountShippingProfileHandler)
	v1.Get("/account/shipping_profiles/:id", authRequired, getAccountShippingProfileHandler)
	v1.Put("/account/shipping_profiles/:id", authRequired, putAccountShippingProfileHandler)
	v1.Delete("/account/shipping_profiles/:id", authRequired, delAccountShippingProfileHandler)
	// Account Orders
	v1.Get("/account/orders", authRequired, getAccountOrdersHandler)
	v1.Post("/account/orders", authRequired, postAccountOrdersHandler)
	v1.Get("/account/orders/:id", authRequired, getAccountOrderHandler)
	v1.Put("/account/orders/:id", authRequired, putAccountOrderHandler)
	v1.Post("/account/orders/:id/checkout", authRequired, postAccountOrderCheckoutHandler)
	// Advance Payment
	v1.Post("/account/orders/:id/advance_payment/submit", authRequired, postAccountOrderAdvancePaymentSubmitHandler)
	// On Delivery
	v1.Post("/account/orders/:id/on_delivery/submit", authRequired, postAccountOrderOnDeliverySubmitHandler)
	// Mollie
	v1.Post("/account/orders/:id/mollie/submit", authRequired, postAccountOrderMollieSubmitHandler)
	v1.Get("/account/orders/:id/mollie/success", authOptional, getAccountOrderMollieSuccessHandler)
	// Account Wishlist
	v1.Get("/account/wishes", authRequired, getAccountWishesHandler)
	v1.Post("/account/wishes", authRequired, postAccountWishHandler)
	v1.Get("/account/wishes/:id", authRequired, getAccountWishHandler)
	v1.Delete("/account/wishes/:id", authRequired, deleteAccountWishHandler)
	//
	v1.Post("/email", postEmailHandler)
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

// Login godoc
// @Summary Get login
// @Accept json
// @Produce json
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/login [get]
// @Tags auth
// @Tags frontend
func getLoginHandler(c *fiber.Ctx) error {
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(c.Request().Header.Header())))
	if err != nil {
		logger.Errorf("%+v", err)
	}
	if code := c.Query("code"); code != "" {
		if user, err := models.GetUserByCode(common.Database, code); err == nil {
			if user.Enabled {
				password := NewPassword(16)
				user.Password = models.MakeUserPassword(password)
				user.Code = ""
				user.Attempt = time.Time{}
				if err = models.UpdateUser(common.Database, user); err != nil {
					return c.Render("login", fiber.Map{
						"Error":    err.Error(),
					})
				}
				// JWT
				expiration := time.Now().Add(JWTLoginDuration)
				claims := &JWTClaims{
					Login: user.Login,
					Password: user.Password,
					StandardClaims: jwt.StandardClaims{
						ExpiresAt: expiration.Unix(),
					},
				}
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
				str, err := token.SignedString(JWTSecret)
				if err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(fiber.Map{"ERROR": err.Error()})
				}
				//
				if v := req.Header.Get("Accept"); strings.EqualFold(v, "application/jwt") {
					c.Status(http.StatusOK)
					return c.JSON(fiber.Map{
						"MESSAGE": "OK",
						"Token": str,
						"Expiration": expiration,
					})
				}else{
					value := map[string]string{
						"email": user.Email,
						"login": user.Login,
						"password": password,
					}
					if encoded, err := cookieHandler.Encode(COOKIE_NAME, value); err == nil {
						expires := time.Time{}
						if authMultipleConfig.CookieDuration > 0 {
							expires = time.Now().Add(authMultipleConfig.CookieDuration)
						}
						cookie := &fiber.Cookie{
							Name:  COOKIE_NAME,
							Value: encoded,
							Path:  "/",
							Expires: expires,
							SameSite: authMultipleConfig.SameSite,
						}
						c.Cookie(cookie)
						c.Status(http.StatusOK)
						c.Set(fiber.HeaderContentType, fiber.MIMETextHTML)
						return c.SendString(`<html><body><script>if (localStorage) {localStorage.setItem('goshop.token', '` + str + `');} setTimeout(function(){window.location = '/'}, 100); </script></body></html>`)
					}else{
						c.Status(http.StatusInternalServerError)
						return c.SendString(err.Error())
					}
				}
			}else{
				if v := req.Header.Get("Accept"); strings.EqualFold(v, "application/jwt") {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}else{
					c.Status(http.StatusInternalServerError)
					return c.SendString(err.Error())
				}
			}
		}else{
			if v := req.Header.Get("Accept"); strings.EqualFold(v, "application/jwt") {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}else{
				c.Status(http.StatusInternalServerError)
				return c.SendString(err.Error())
			}
		}
	}
	return c.Redirect("/login", http.StatusFound)
}

type ResetRequest struct {
	Email string
	Login string
}

// Reset password godoc
// @Summary reset password
// @Accept json
// @Produce json
// @Param form body ResetRequest true "body"
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
		if user.Attempt.Add(time.Duration(15) * time.Minute).After(time.Now()) {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Please try again later"})
		}
		code := fmt.Sprintf("%s-%s-%s-%s", NewPassword(8), NewPassword(8), NewPassword(8), NewPassword(8))
		user.Code = code
		user.Attempt = time.Now()
		if err = models.UpdateUser(common.Database, user); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
		//
		template, err := models.GetEmailTemplateByType(common.Database, common.NOTIFICATION_TYPE_RESET_PASSWORD)
		if err == nil {
			if user, err := models.GetUser(common.Database, int(user.ID)); err == nil {
				if user.EmailConfirmed {
					logger.Infof("Send email to user: %+v", user.Email)
					vars := &common.NotificationTemplateVariables{
						Url: common.Config.Url,
						Code: code,
					}
					if err := common.NOTIFICATION.SendEmail(mail.NewEmail(common.Config.Notification.Email.Name, common.Config.Notification.Email.Email), mail.NewEmail(user.Login, user.Email), template.Topic, template.Message, vars); err != nil {
						logger.Warningf("%+v", err)
					}
				}else{
					logger.Warningf("User's %v email %v is not confirmed", user.Login, user.Email)
				}
			} else {
				logger.Warningf("%+v", err)
			}
		}
		//
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
								if c, err := models.GetCacheCategoryByCategoryId(common.Database, uint(v)); err == nil {
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
						if time.Since(t).Seconds() <= 30 {
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
	Url string
	Debug bool
	Currency string
	Symbol string
	Products string
	FlatUrl bool
	Pattern string
	Preview      string
	Payment      config.PaymentConfig
	Resize       config.ResizeConfig
	Storage       config.StorageConfig
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
	conf.Url = common.Config.Url
	conf.Currency = common.Config.Currency
	conf.Symbol = common.Config.Symbol
	conf.Products = common.Config.Products
	conf.FlatUrl = common.Config.FlatUrl
	conf.Pattern = common.Config.Pattern
	conf.Preview = common.Config.Preview
	conf.Payment = common.Config.Payment
	conf.Resize = common.Config.Resize
	conf.Storage = common.Config.Storage
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
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}
	//
	var message string
	common.Config.Url = request.Url
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
		(request.Resize.Enabled && request.Resize.Image.Enabled && common.Config.Resize.Image.Size != request.Resize.Image.Size){
		render = true
	}
	if (request.Storage.Enabled && !common.Config.Storage.Enabled) ||
		(request.Storage.S3.Enabled && !common.Config.Storage.S3.Enabled) ||
		(common.Config.Storage.S3.Enabled && request.Storage.S3.AccessKeyID != common.Config.Storage.S3.AccessKeyID) ||
		(common.Config.Storage.S3.Enabled && request.Storage.S3.SecretAccessKey != common.Config.Storage.S3.SecretAccessKey) ||
		(common.Config.Storage.S3.Enabled && request.Storage.S3.Region != common.Config.Storage.S3.Region) ||
		(common.Config.Storage.S3.Enabled && request.Storage.S3.Bucket != common.Config.Storage.S3.Bucket) {
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
	if request.Storage.Enabled && request.Storage.S3.Enabled {
		if !common.Config.Storage.Enabled || !common.Config.Storage.S3.Enabled ||
			(common.Config.Storage.S3.Enabled && request.Storage.S3.AccessKeyID != common.Config.Storage.S3.AccessKeyID) ||
			(common.Config.Storage.S3.Enabled && request.Storage.S3.SecretAccessKey != common.Config.Storage.S3.SecretAccessKey) ||
			(common.Config.Storage.S3.Enabled && request.Storage.S3.Region != common.Config.Storage.S3.Region) ||
			(common.Config.Storage.S3.Enabled && request.Storage.S3.Bucket != common.Config.Storage.S3.Bucket){
			if storage, err := storage.NewAWSS3Storage(request.Storage.S3.AccessKeyID,request.Storage.S3.SecretAccessKey, request.Storage.S3.Region, request.Storage.S3.Bucket, request.Storage.S3.Prefix, path.Join(dir, "temp", "s3"), common.Config.Resize.Quality, common.Config.Storage.S3.Rewrite); err == nil {
				filename := fmt.Sprintf("file-%d.txt", rand.Intn(899999) + 100000)
				if err = ioutil.WriteFile(path.Join(dir, "temp", filename), []byte(time.Now().Format(time.RFC3339)), 0755); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
				if _, err = storage.PutFile(path.Join(dir, "temp", filename), path.Join("temp", filename)); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
				if err = storage.DeleteFile(path.Join("temp", filename)); err != nil {
					return c.JSON(HTTPError{err.Error()})
				}
			} else {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}
	}
	common.Config.Storage = request.Storage
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
		FlatUrl bool `toml:"flatUrl"`
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
	if request.Type != "" {
		page.Type = request.Type
	}else{
		page.Type = "post"
	}
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
// @Summary Update order
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
// @Summary Update transaction
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
	Categories []models.CategoryView `json:",omitempty"`
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
// @Summary Update widget
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
	Email string
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
// @Summary Update zone
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
// @Summary Update email template
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
		vars := &common.NotificationTemplateVariables{
			Url: common.Config.Url,
			Symbol: "$",
		}
		if common.Config.Symbol != "" {
			vars.Symbol = common.Config.Symbol
		}
		var orderView struct {
			ID uint
			CreatedAt time.Time
			OrderShortView
			Status string
		}
		if err = json.Unmarshal([]byte(`
{
  "Billing": {
    "ID": 2,
    "Name": "mollie",
    "Title": "Mollie",
    "Profile": {
      "ID": 25,
      "Email": "john0035@mail.com",
      "Name": "John",
      "Lastname": "Smith",
      "Phone": "+49123456789",
      "Address": "First 1/23",
      "Zip": "12345",
      "City": "Berlin",
      "Country": "de"
    },
    "Method": "creditcard"
  },
  "Items": [
    {
      "Uuid": "[21,23,84]",
      "Title": "Product With Variation",
      "Path": "/products/bedroom/bed/product-with-variation",
      "Thumbnail": "/images/products/59-image-1613727567.jpg,/images/products/resize/59-image-1613727567_324x0.jpg 324w,/images/products/resize/59-image-1613727567_512x0.jpg 512w,/images/products/resize/59-image-1613727567_1024x0.jpg 1024w",
      "Variation": {
        "Title": "With Boxes"
      },
      "Properties": [
        {
          "Title": "Boxes count",
          "Value": "4",
          "Rate": 100
        }
      ],
      "Rate": 1300,
      "Quantity": 1,
      "VAT": 19,
      "Total": 1300,
      "Volume": 0.06,
      "Weight": 32
    }
  ],
  "Quantity": 1,
  "Sum": 1300,
  "Delivery": 104.4,
  "VAT": 19,
  "Total": 1404.4,
  "Volume": 0.06,
  "Weight": 32,
  "Shipping": {
    "ID": 3,
    "Name": "fedex",
    "Title": "FedEx",
    "Thumbnail": "/transports/3-fedex-thumbnail.png",
    "Services": [
      {
        "Name": "service1",
        "Title": "Service1",
        "Rate": 1,
        "Selected": true
      }
    ],
    "Profile": {
      "ID": 26,
      "Email": "john0035@mail.com",
      "Name": "John",
      "Lastname": "Smith",
      "Phone": "+49123456789",
      "Address": "First 1/23",
      "Zip": "12345",
      "City": "Berlin",
      "Country": "de"
    },
    "Volume": 0.06,
    "Weight": 32,
    "Value": 104.4,
    "Total": 104.4
  }
}
`), &orderView); err == nil {
			orderView.ID = 0
			orderView.CreatedAt = time.Now()
			orderView.Status = "new"
			vars.Order = orderView
		}
		if bts, err := json.Marshal(vars); err == nil {
			logger.Infof("Bts: %+v", string(bts))
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
// @Summary Update vendor
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
// @Summary Update time
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
	Name string `json:",omitempty"`
	Lastname string `json:",omitempty"`
	Role int `json:",omitempty"`
	Notification bool
	AllowReceiveEmails bool `json:",omitempty"`
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
// @Summary Update user
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
		return c.JSON(HTTPError{"Email email"})
	}
	time.Sleep(1 * time.Second)
	if _, err := models.GetUserByEmail(common.Database, request.Email); err == nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Email already in use"})
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

func getTestHandler(c *fiber.Ctx) error {
	logger.Infof("getTestHandler")
	time.Sleep(3 * time.Second)
	return sendString(c, http.StatusOK, map[string]interface{}{"MESSAGE": "OK", "Status": "paid"})
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
	Categories *models.CategoryView
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
	CategoryName string
	CategoryTitle string
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
	var response ProductsFilterResponse
	var relPath string
	if v := c.Query("relPath"); v != "" {
		relPath = v
	}else{
		return c.JSON(HTTPError{"relPath required"})
	}
	if category, err := models.GetCacheCategoryByLink(common.Database, relPath); err == nil {
		if tree, err := models.GetCategoriesView(common.Database, int(category.CategoryID), 999, true, false); err == nil {
			response.Categories = tree
		}else{
			logger.Warningf("%+v", err)
		}
	} else {
		if tree, err := models.GetCategoriesView(common.Database, 0, 999, true, false); err == nil {
			response.Categories = tree
		}else{
			logger.Warningf("%+v", err)
		}
	}
	//
	var request FilterRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	//logger.Infof("request: %+v", request)
	if len(request.Sort) == 0 {
		request.Sort = map[string]string{"ID": "desc"}
	}
	if request.Length == 0 {
		request.Length = 100
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
				case "Rate", "Width", "Height", "Depth", "Weight":
					parts := strings.Split(value, "-")
					if len(parts) == 1 {
						if v, err := strconv.ParseFloat(parts[0], 64); err == nil {
							keys1 = append(keys1, "cache_products." + key + " == ?")
							values1 = append(values1, math.Round(v * 1000) / 1000)
						}
					} else {
						if v, err := strconv.ParseFloat(parts[0], 64); err == nil {
							//keys1 = append(keys1, "cache_products." + key + " >= ?")
							keys1 = append(keys1, "(cache_products." + key + " >= ? or cache_products." + key + " = ?)")
							//values1 = append(values1, math.Round(v * 1000) / 1000)
							values1 = append(values1, math.Round(v * 1000) / 1000, 0)
						}
						if v, err := strconv.ParseFloat(parts[1], 64); err == nil {
							keys1 = append(keys1, "cache_products." + key + " <= ?")
							values1 = append(values1, math.Round(v * 1000) / 1000)
						}
					}
				case "Search":
					if v, err := url.QueryUnescape(value); err == nil {
						value = v
					}else{
						logger.Warningf("%+v", err)
					}
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
										keys3 = append(keys3, "parameters.Value_id = ? or rates.Value_Id = ?")
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
	keys1 = append(keys1, "cache_products.Path LIKE ?")
	if common.Config.FlatUrl {
		if prefix := "/" + strings.ToLower(common.Config.Products); prefix == relPath || prefix + "/" == relPath {
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
				case "CreatedAt":
					orders = append(orders, fmt.Sprintf("cache_products.%v %v", "created_at", value))
				case "Rate":
					orders = append(orders, fmt.Sprintf("cache_products.%v %v", "Rate", value))
				default:
					orders = append(orders, fmt.Sprintf("cache_products.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//logger.Infof("order: %+v", order)
	rows, err := common.Database.Debug().Model(&models.CacheProduct{}).Select("cache_products.ID, cache_products.Name, cache_products.Title, cache_products.Path, cache_products.Description, cache_products.Thumbnail, cache_products.Images, cache_products.Variations, cache_products.Rate as Rate, cache_products.Width as Width, cache_products.Height as Height, cache_products.Depth as Depth,  cache_products.Weight as Weight, cache_products.Category_Id as CategoryId").Joins("left join parameters on parameters.Product_ID = cache_products.Product_ID").Joins("left join variations on variations.Product_ID = cache_products.Product_ID").Joins("left join properties on properties.Variation_Id = variations.Id").Joins("left join options on options.Id = parameters.Option_Id or options.Id = properties.Option_Id").Joins("left join rates on rates.Property_Id = properties.Id").Where(strings.Join(keys1, " and "), values1...)/*.Having(strings.Join(keys2, " and "), values2...)*/.Rows()
	if err == nil {
		for rows.Next() {
			var item ProductsFilterItem
			if err = common.Database.ScanRows(rows, &item); err == nil {
				//logger.Infof("Item: %+v, %+v, %+v", item.ID, item.Name, item.Path)
				arr := strings.Split(strings.TrimLeft(item.Path, "/"), "/")
				root := response.Categories
				for i, slug := range arr {
					//logger.Infof("\t%d: %+v", i, slug)
					if i == 0 && slug == "" {
						slug = strings.ToLower(common.Config.Products)
					}
					if root.Name == slug {
						//logger.Infof("\t\tcase1")
						root.Count++
					} else {
						//logger.Infof("\t\tcase2")
						for i, child := range root.Children {
							if child.Name == slug {
								root.Children[i].Count++
								//logger.Infof("\t\t\tcase2.1: %d, %+v, %+v", child.ID, child.Name, root.Children[i].Count)
								root = child
								break
							}
						}
					}
				}
			} else {
				logger.Errorf("%v", err)
			}
		}
		rows.Close()
	}
	//
	rows, err = common.Database.Debug().Model(&models.CacheProduct{}).Select("cache_products.ID, cache_products.Name, cache_products.Title, cache_products.Path, cache_products.Description, cache_products.Thumbnail, cache_products.Images, cache_products.Variations, cache_products.Rate as Rate, cache_products.Width as Width, cache_products.Height as Height, cache_products.Depth as Depth,  cache_products.Weight as Weight, cache_products.Category_Id as CategoryId").Joins("left join parameters on parameters.Product_ID = cache_products.Product_ID").Joins("left join variations on variations.Product_ID = cache_products.Product_ID").Joins("left join properties on properties.Variation_Id = variations.Id").Joins("left join options on options.Id = parameters.Option_Id or options.Id = properties.Option_Id").Joins("left join rates on rates.Property_Id = properties.Id").Where(strings.Join(keys1, " and "), values1...)/*.Having(strings.Join(keys2, " and "), values2...)*/.Group("cache_products.product_id").Order(order).Limit(request.Length).Offset(request.Start).Rows()
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
	common.Database.Debug().Model(&models.CacheProduct{}).Select("Product_ID as ID, Name, Title, Path, Description, Thumbnail, Rate, Category_Id as CategoryId").Joins("left join parameters on parameters.Product_ID = cache_products.Product_ID").Joins("left join variations on variations.Product_ID = cache_products.Product_ID").Joins("left join properties on properties.Variation_Id = variations.Id").Joins("left join options on options.Id = parameters.Option_Id or options.Id = properties.Option_Id").Joins("left join rates on rates.Property_Id = properties.Id").Where(strings.Join(keys1, " and "), values1...).Count(&response.Filtered)
	common.Database.Debug().Model(&models.CacheProduct{}).Select("Product_ID as ID, Name, Title, Path, Description, Thumbnail, Rate, Category_Id as CategoryId").Joins("left join parameters on parameters.Product_ID = cache_products.Product_ID").Joins("left join variations on variations.Product_ID = cache_products.Product_ID").Joins("left join properties on properties.Variation_Id = variations.Id").Joins("left join options on options.Id = parameters.Option_Id or options.Id = properties.Option_Id").Joins("left join rates on rates.Property_Id = properties.Id").Where("Path LIKE ?", relPath + "%").Count(&response.Total)
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

type NewStripePayment struct {
	StripeToken string
	SuccessURL string
	CancelURL string
}

type StripeCheckoutSessionView struct {
	SessionID string `json:"id"`
}

// CheckoutStripe godoc
// @Summary Checkout stripe
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

/* *** */

type ProductView struct {
	ID uint
	Enabled bool
	Name string
	Title string
	Thumbnail string `json:",omitempty"`
	Description string `json:",omitempty"`
	Notes string `json:",omitempty"`
	Parameters []ParameterView `json:",omitempty"`
	CustomParameters string `json:",omitempty"`
	Variation string `json:",omitempty"`
	BasePrice float64
	SalePrice float64 `json:",omitempty"`
	Start *time.Time `json:",omitempty"`
	End *time.Time `json:",omitempty"`
	Prices []*PriceView
	Pattern string `json:",omitempty"`
	Dimensions string `json:",omitempty"`
	Width float64 `json:",omitempty"`
	Height float64 `json:",omitempty"`
	Depth float64 `json:",omitempty"`
	Volume float64 `json:",omitempty"`
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
		Rates []struct {
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
	Categories []models.CategoryView `json:",omitempty"`
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
	Notes string `json:",omitempty"`
	Thumbnail string `json:",omitempty"`
	BasePrice float64
	SalePrice float64 `json:",omitempty"`
	Start *time.Time `json:",omitempty"`
	End *time.Time `json:",omitempty"`
	Prices []*PriceView
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
		Rates []struct {
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
	Volume float64 `json:",omitempty"`
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
				if rate, err := models.GetRate(common.Database, id); err == nil {
					sum += rate.Price
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

func SendOrderPaidEmail(to *mail.Email, orderId int, template *models.EmailTemplate) error {
	vars := &common.NotificationTemplateVariables{ }
	if order, err := models.GetOrderFull(common.Database, orderId); err == nil {
		vars = &common.NotificationTemplateVariables{
			Url: common.Config.Url,
			Symbol: "$",
		}
		if common.Config.Symbol != "" {
			vars.Symbol = common.Config.Symbol
		}
		var orderView struct {
			ID uint
			CreatedAt time.Time
			OrderShortView
			Status string
		}
		if err = json.Unmarshal([]byte(order.Description), &orderView); err == nil {
			orderView.ID = order.ID
			orderView.CreatedAt = order.CreatedAt
			orderView.Status = order.Status
			vars.Order = orderView
		}
		logger.Infof("View: %+v", orderView)
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