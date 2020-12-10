package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	swagger "github.com/arsmn/fiber-swagger/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/session/v2"
	"github.com/gofiber/session/v2/provider/redis"
	"github.com/google/logger"
	"github.com/jinzhu/now"
	"github.com/stripe/stripe-go/v71"
	"github.com/stripe/stripe-go/v71/card"
	checkout_session "github.com/stripe/stripe-go/v71/checkout/session"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/models"
	"gorm.io/gorm"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
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

var (
	reNotAbc = regexp.MustCompile("(?i)[^a-z0-9]+")
	sessions = session.New(session.Config{
		//Expiration: 24 * 30 * time.Hour,
	})
)


func GetFiber() *fiber.App {
	app, authMulti := CreateFiberAppWithAuthMultiple(AuthMultipleConfig{
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
	/*app.Get("/", authMulti, func(c *fiber.Ctx) error {
		m := fiber.Map{"Hello": "world"}
		if v := c.Locals("user"); v != nil {
			m["User"] = v.(*models.User)
		}
		return c.JSON(m)
	})*/
	// Sessions
	if provider, err := redis.New(redis.Config{
	  KeyPrefix:   "session",
	  Addr:        "127.0.0.1:6379",
	  PoolSize:    8,
	  IdleTimeout: 30 * time.Second,
	}); err == nil {
		sessions = session.New(session.Config{
			Lookup: "cookie:sid",
			Expiration: 24 * 30 * time.Hour,
			Provider: provider,
			SameSite: "None",
			/*Secure: true,*/
		})
	}else{
		logger.Errorf("%v", err)
	}
	//
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
	v1.Get("/logout", authMulti, getLogoutHandler)
	v1.Get("/info", authMulti, hasRole(models.ROLE_ROOT, models.ROLE_ADMIN, models.ROLE_MANAGER), getInfoHandler)
	v1.Get("/dashboard", authMulti, hasRole(models.ROLE_ROOT, models.ROLE_ADMIN, models.ROLE_MANAGER), getDashboardHandler)
	//
	v1.Get("/categories", authMulti, getCategoriesHandler)
	v1.Post("/categories", authMulti, postCategoriesHandler)
	v1.Post("/categories/autocomplete", authMulti, postCategoriesAutocompleteHandler)
	v1.Post("/categories/list", authMulti, postCategoriesListHandler)
	v1.Get("/categories/:id", authMulti, getCategoryHandler)
	v1.Put("/categories/:id", authMulti, putCategoryHandler)
	v1.Delete("/categories/:id", authMulti, delCategoryHandler)
	//
	v1.Get("/contents", authMulti, getContentsHandler)
	v1.Get("/contents/*", authMulti, getContentHandler)
	v1.Patch("/contents/*", authMulti, patchContentHandler)
	v1.Post("/contents/*", authMulti, postContentHandler)
	v1.Put("/contents/*", authMulti, putContentHandler)
	//
	v1.Get("/products", authMulti, getProductsHandler)
	v1.Post("/products", authMulti, postProductsHandler)
	v1.Post("/products/list", authMulti, postProductsListHandler)
	v1.Get("/products/:id", authMulti, getProductHandler)
	v1.Put("/products/:id", authMulti, putProductHandler)
	v1.Delete("/products/:id", authMulti, delProductHandler)
	//
	v1.Get("/variations", authMulti, getVariationsHandler)
	v1.Post("/variations", authMulti, postVariationHandler)
	v1.Post("/variations/list", authMulti, postVariationsListHandler)
	v1.Get("/variations/:id", authMulti, getVariationHandler)
	v1.Put("/variations/:id", authMulti, putVariationHandler)
	v1.Delete("/variations/:id", authMulti, delVariationHandler)
	//
	v1.Post("/properties", authMulti, postPropertyHandler)
	v1.Post("/properties/list", authMulti, postPropertiesListHandler)
	v1.Get("/properties/:id", authMulti, getPropertyHandler)
	v1.Put("/properties/:id", authMulti, putPropertyHandler)
	v1.Delete("/properties/:id", authMulti, deletePropertyHandler)
	//
	v1.Get("/prices", authMulti, getPricesHandler)
	v1.Post("/prices/list", authMulti, postPricesListHandler)
	v1.Post("/prices", authMulti, postPriceHandler)
	v1.Get("/prices/:id", authMulti, getPriceHandler)
	v1.Put("/prices/:id", authMulti, putPriceHandler)
	v1.Delete("/prices/:id", authMulti, deletePriceHandler)
	//
	v1.Get("/tags", authMulti, getTagsHandler)
	v1.Post("/tags", authMulti, postTagHandler)
	v1.Post("/tags/list", authMulti, postTagsListHandler)
	v1.Get("/tags/:id", authMulti, getTagHandler)
	v1.Put("/tags/:id", authMulti, putTagHandler)
	v1.Delete("/tags/:id", authMulti, delTagHandler)
	//
	v1.Get("/options", authMulti, getOptionsHandler)
	v1.Post("/options", authMulti, postOptionHandler)
	v1.Post("/options/list", authMulti, postOptionsListHandler)
	v1.Get("/options/:id", authMulti, getOptionHandler)
	v1.Put("/options/:id", authMulti, putOptionHandler)
	v1.Delete("/options/:id", authMulti, delOptionHandler)
	//
	v1.Get("/values", authMulti, getValuesHandler)
	v1.Post("/values", authMulti, postValueHandler)
	v1.Post("/values/list", authMulti, postValuesListHandler)
	v1.Get("/values/:id", authMulti, getValueHandler)
	v1.Put("/values/:id", authMulti, putValueHandler)
	v1.Delete("/values/:id", authMulti, delValueHandler)
	//
	v1.Post("/images", authMulti, postImageHandler)
	v1.Post("/images/list", authMulti, postImagesListHandler)
	v1.Get("/images/:id", authMulti, getImageHandler)
	v1.Put("/images/:id", authMulti, putImageHandler)
	v1.Delete("/images/:id", authMulti, delImageHandler)
	//
	v1.Post("/orders/list", authMulti, postOrdersListHandler)
	v1.Get("/orders/:id", authMulti, getOrderHandler)
	v1.Put("/orders/:id", authMulti, putOrderHandler)
	v1.Delete("/orders/:id", authMulti, delOrderHandler)
	//
	v1.Post("/transactions/list", authMulti, postTransactionsListHandler)
	v1.Get("/transactions/:id", authMulti, getTransactionHandler)
	v1.Put("/transactions/:id", authMulti, putTransactionHandler)
	v1.Delete("/transactions/:id", authMulti, delTransactionHandler)
	//
	v1.Get("/me", authMulti, getMeHandler)
	//
	v1.Get("/users", authMulti, getUsersHandler)
	v1.Post("/users/list", authMulti, postUsersListHandler)
	v1.Get("/users/:id", authMulti, getUserHandler)
	v1.Put("/users/:id", authMulti, putUserHandler)
	v1.Delete("/users/:id", authMulti, delUserHandler)
	//
	v1.Post("/prepare", authMulti, postPrepareHandler)
	v1.Post("/render", authMulti, postRenderHandler)
	v1.Post("/publish", authMulti, postPublishHandler)
	//
	v1.Get("/account", authMulti, getAccountHandler)
	v1.Post("/account/profiles", authMulti, postAccountProfileHandler)
	//
	v1.Get("/account/orders", authMulti, getAccountOrdersHandler)
	v1.Post("/account/orders", authMulti, postAccountOrdersHandler)
	v1.Get("/account/orders/:id", authMulti, getAccountOrderHandler)
	v1.Post("/account/orders/:id/checkout", authMulti, postAccountOrderCheckoutHandler)
	v1.Get("/account/orders/:id/stripe/customer", authMulti, getAccountOrderStripeCustomerHandler)
	v1.Get("/account/orders/:id/stripe/card", authMulti, getAccountOrderStripeCardHandler)
	v1.Post("/account/orders/:id/stripe/card", authMulti, postAccountOrderStripeCardHandler)
	//v1.Get("/account/orders/:id/stripe/secret", authMulti, getAccountOrderStripeSecretHandler)
	v1.Post("/account/orders/:id/stripe/submit", authMulti, postAccountOrderStripeSubmitHandler)
	//
	v1.Post("/profiles", postProfileHandler)
	v1.Post("/delivery/calculate", postDeliveryCalculateHandler)
	//
	v1.Post("/filter", postFilterHandler)
	v1.Post("/search", postSearchHandler)

	//app.Get("/session", func(c *fiber.Ctx) error {
	//	store := sessions.Get(c) // get/create new session
	//	defer store.Save()
	//
	//	var counter int64
	//	if v := store.Get("counter"); v != nil {
	//		counter = v.(int64)
	//	}
	//	counter++
	//	store.Set("counter", counter)
	//	return c.JSON(fiber.Map{"Counter": counter})
	//})
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
	app.Use("/swagger", swagger.Handler) // default
	app.Use("/swagger", swagger.New(swagger.Config{ // custom
		URL: fmt.Sprintf("http://localhost:%d/doc.json", common.Config.Port),
		DeepLinking: false,
	}))
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
	/*static := path.Join(dir, "storage")
	if _, err := os.Stat(static); err != nil {
		if err = os.MkdirAll(static, 0755); err != nil {
			logger.Errorf("%v", err)
		}
	}
	app.Static("/static", static)
	//
	htdocs := path.Join(dir, "htdocs")
	if _, err := os.Stat(htdocs); err != nil {
		if err = os.MkdirAll(htdocs, 0755); err != nil {
			logger.Errorf("%v", err)
		}
	}
	if p := path.Join(htdocs, "index.html"); len(p) > 0 {
		if _, err := os.Stat(p); err != nil {
			if err = ioutil.WriteFile(p, []byte(`Hello world`), 0644); err != nil {
				logger.Errorf("%v", err)
			}
		}
	}
	app.Static("/", htdocs)
	app.Static("*", path.Join(htdocs, "index.html"))
	 */
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

type InfoView struct {
	Application string
	Started string
	Authorization string `json:",omitempty"`
	ExpirationAt string `json:",omitempty"`
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
	if v := c.Locals("authorization"); v != nil {
		view.Application = v.(string)
	}
	if v := c.Locals("expiration"); v != nil {
		if expiration := v.(int64); expiration > 0 {
			view.ExpirationAt = time.Unix(expiration, 0).Format(time.RFC3339)
		}
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
func getCategoriesHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Query("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var depth = 1
	if v := c.Query("depth"); v != "" {
		depth, _ = strconv.Atoi(v)
	}
	view, err := GetCategoriesView(common.Database, id, depth)
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
			if category, err := models.GetCategoryByName(common.Database, name); err == nil {
				if parentCategory, err := models.GetCategory(common.Database, pid); err == nil {
					if category.ParentId == parentCategory.ID {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{"Name is already in use"})
					}
				}
			}
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
							category.Thumbnail = "/" + path.Join("storage", "categories", filename)
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
	logger.Infof("request: %+v", request)
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
// @Param category body ListRequest true "body"
// @Success 200 {object} CategoriesListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/categories/list [post]
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
		request.Sort["ID"] = "desc"
	}
	if request.Length == 0 {
		request.Length = 10
	}
	logger.Infof("request: %+v", request)
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
	logger.Infof("order: %+v", order)
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
// GetProduct godoc
// @Summary Get category
// @Accept json
// @Produce json
// @Param id path int true "Category ID"
// @Success 200 {object} CategoryFullView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/products/{id} [get]
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
// CreateCategory godoc
// @Summary Update category
// @Accept json
// @Produce json
// @Param category body NewCategory true "body"
// @Success 200 {object} CategoryView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/categories/{id} [put]
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
			if parentCategory, err := models.GetCategory(common.Database, pid); err == nil {
				for _, category := range models.GetChildrenOfCategoryById(common.Database, parentCategory.ID) {
					if int(category.ID) != id && category.Name == request.Name {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{"Name is already in use"})
					}
				}
			}
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
						category.Thumbnail = "/" + path.Join("storage", "categories", filename)
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
// @Router /api/v1/options/{id} [delete]
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
									if page.Meta != nil {
										logger.Infof("Meta: %+v", page.Meta)
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
// @Router /api/v1/contents/* [get]
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
// @Router /api/v1/contents/* [post]
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
	c.Status(http.StatusInternalServerError)
	return c.JSON(HTTPError{"Something wrong"})
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
// @Router /api/v1/contents/* [post]
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
// PutContent godoc
// @Summary Put content
// @Accept json
// @Produce json
// @Success 200 {object} FileView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/contents/* [put]
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

type ProductsShortView []ProductShortView

type ProductShortView struct {
	ID uint
	Name string
	Title string
	Description string `json:",omitempty"`
	Thumbnail string `json:",omitempty"`
}

// @security BasicAuth
// GetProducts godoc
// @Summary Get products
// @Accept json
// @Produce json
// @Success 200 {object} ProductsShortView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/products [get]
func getProductsHandler(c *fiber.Ctx) error {
	if products, err := models.GetProducts(common.Database); err == nil {
		var view ProductsShortView
		if bts, err := json.MarshalIndent(products, "", "   "); err == nil {
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

type NewProduct struct {
	Name string
	Title string
	Description string
	Categories string
}

// @security BasicAuth
// CreateProduct godoc
// @Summary Create product
// @Accept multipart/form-data
// @Produce json
// @Param category body NewProduct true "body"
// @Success 200 {object} ProductView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/products [post]
func postProductsHandler(c *fiber.Ctx) error {
	var view ProductView
	//
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				return err
			}
			var name string
			if v, found := data.Value["Name"]; found && len(v) > 0 {
				name = strings.TrimSpace(v[0])
			}
			if name == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Invalid name"})
			}
			if _, err := models.GetProductByName(common.Database, name); err == nil {
				return c.JSON(HTTPError{"Name is already in use"})
			}
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
			product := &models.Product{Name: name, Title: title, Description: description, Content: content}
			if id, err := models.CreateProduct(common.Database, product); err == nil {
				if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
					p := path.Join(dir, "storage", "products")
					if _, err := os.Stat(p); err != nil {
						if err = os.MkdirAll(p, 0755); err != nil {
							logger.Errorf("%v", err)
						}
					}
					filename := fmt.Sprintf("%d-%s-thumbnail%s", id, product.Name, path.Ext(v[0].Filename))
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
							product.Thumbnail = "/" + path.Join("storage", "products", filename)
							if err = models.UpdateProduct(common.Database, product); err != nil {
								c.Status(http.StatusInternalServerError)
								return c.JSON(HTTPError{err.Error()})
							}
						}
					}
				}
				if bts, err := json.Marshal(product); err == nil {
					if err = json.Unmarshal(bts, &view); err != nil {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
				}
				if v, found := data.Value["Categories"]; found && len(v) > 0 {
					for _, vv := range strings.Split(strings.TrimSpace(v[0]), ",") {
						if categoryId, err := strconv.Atoi(strings.TrimSpace(vv)); err == nil {
							if category, err := models.GetCategory(common.Database, categoryId); err == nil {
								if err = models.AddProductToCategory(common.Database, category, product); err != nil {
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
				if _, err = models.CreateVariation(common.Database, &models.Variation{Title: "Default", Name: "default", Description: "", ProductId: product.ID}); err != nil {
					logger.Errorf("%v", err)
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

type ProductsListResponse struct {
	Data []ProductsListItem
	Filtered int64
	Total int64
}

type ProductsListItem struct {
	ID uint
	Name string
	Title string
	Thumbnail string
	Description string
	VariationsIds string
	VariationsTitles string
	CategoryId uint `json:",omitempty"`
}

// @security BasicAuth
// SearchCategories godoc
// @Summary Search products
// @Accept json
// @Produce json
// @Param id query int false "Category id"
// @Param category body ListRequest true "body"
// @Success 200 {object} ProductsListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/products/list [post]
func postProductsListHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Query("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var response ProductsListResponse
	var request ListRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
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
				case "Variations":
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
					keys1 = append(keys1, fmt.Sprintf("products.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	if id > 0 {
		keys1 = append(keys1, "category_id = ?")
		values1 = append(values1, id)
	}
	logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
	//
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				case "Variations":
					orders = append(orders, fmt.Sprintf("%v %v", key, value))
				default:
					orders = append(orders, fmt.Sprintf("products.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	logger.Infof("order: %+v", order)
	//
	rows, err := common.Database.Debug().Model(&models.Product{}).Select("products.ID, products.Name, products.Title, products.Thumbnail, products.Description, group_concat(variations.ID, ', ') as VariationsIds, group_concat(variations.Title, ', ') as VariationsTitles, category_id as CategoryId").Joins("left join categories_products on categories_products.product_id = products.id").Joins("left join variations on variations.product_id = products.id").Group("products.id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item ProductsListItem
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
	rows, err = common.Database.Debug().Model(&models.Product{}).Select("products.ID, products.Name, products.Title, products.Thumbnail, products.Description, group_concat(variations.ID, ', ') as VariationsIds, group_concat(variations.Title, ', ') as VariationsTitles, category_id as CategoryId").Joins("left join categories_products on categories_products.product_id = products.id").Joins("left join variations on variations.product_id = products.id").Group("variations.product_id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	//
	if len(keys1) > 0 || len(keys2) > 0 {
		common.Database.Debug().Model(&models.Product{}).Joins("left join categories_products on categories_products.product_id = products.id").Where("category_id = ?", id).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	//
	c.Status(http.StatusOK)
	return c.JSON(response)
}

// @security BasicAuth
// GetProduct godoc
// @Summary Get product
// @Accept json
// @Produce json
// @Param id path int true "Product ID"
// @Success 200 {object} ProductView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/products/{id} [get]
func getProductHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if product, err := models.GetProductFull(common.Database, id); err == nil {
		var view ProductView
		if bts, err := json.MarshalIndent(product, "", "   "); err == nil {
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
// UpdateProduct godoc
// @Summary Update product
// @Accept multipart/form-data
// @Produce json
// @Param id query int false "Product id"
// @Param product body NewProduct true "body"
// @Success 200 {object} ProductView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/products/{id} [put]
func putProductHandler(c *fiber.Ctx) error {
	var view VariationView
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var product *models.Product
	var err error
	if product, err = models.GetProduct(common.Database, id); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	var request NewProduct
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
			product.Title = title
			product.Description = description
			product.Content = content
			if v, found := data.Value["Thumbnail"]; found && len(v) > 0 && v[0] == "" {
				// To delete existing
				if product.Thumbnail != "" {
					if err = os.Remove(path.Join(dir, product.Thumbnail)); err != nil {
						logger.Errorf("%v", err)
					}
					product.Thumbnail = ""
				}
			}else if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
				// New file
				p := path.Join(dir, "storage", "products")
				if _, err := os.Stat(p); err != nil {
					if err = os.MkdirAll(p, 0755); err != nil {
						logger.Errorf("%v", err)
					}
				}
				filename := fmt.Sprintf("%d-%s-thumbnail%s", id, regexp.MustCompile(`(?i)[^-a-z0-9]+`).ReplaceAllString(product.Name, "-"), path.Ext(v[0].Filename))
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
						product.Thumbnail = "/" + path.Join("storage", "products", filename)
					}
				}
			}

			if err = models.UpdateProduct(common.Database, product); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if bts, err := json.Marshal(product); err == nil {
				if err = json.Unmarshal(bts, &view); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}
			// Categories
			if err = models.DeleteAllCategoriesFromProduct(common.Database, product); err != nil {
				logger.Errorf("%v", err)
			}
			if v, found := data.Value["Categories"]; found && len(v) > 0 {
				for _, vv := range strings.Split(strings.TrimSpace(v[0]), ",") {
					if categoryId, err := strconv.Atoi(strings.TrimSpace(vv)); err == nil {
						if category, err := models.GetCategory(common.Database, categoryId); err == nil {
							if err = models.AddProductToCategory(common.Database, category, product); err != nil {
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
			// Tags
			if err = models.DeleteAllTagsFromProduct(common.Database, product); err != nil {
				logger.Errorf("%v", err)
			}
			if v, found := data.Value["Tags"]; found && len(v) > 0 {
				for _, vv := range strings.Split(strings.TrimSpace(v[0]), ",") {
					logger.Infof("vv: %+v", vv)
					if tagId, err := strconv.Atoi(strings.TrimSpace(vv)); err == nil {
						logger.Infof("tagId: %+v", tagId)
						if tag, err := models.GetTag(common.Database, tagId); err == nil {
							logger.Infof("tag: %+v", tag)
							if err = models.AddProductToTag(common.Database, tag, product); err != nil {
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
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	return c.JSON(view)
}

// @security BasicAuth
// DelProduct godoc
// @Summary Delete product
// @Accept json
// @Produce json
// @Param id path int true "Product ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/products/{id} [delete]
func delProductHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if product, err := models.GetProductFull(common.Database, id); err == nil {
		//
		for _, variation := range product.Variations {
			for _, property := range variation.Properties {
				for _, price := range property.Prices {
					if err = models.DeletePrice(common.Database, price); err != nil {
						logger.Errorf("%v", err.Error())
					}
				}
				if err = models.DeleteProperty(common.Database, property); err != nil {
					logger.Errorf("%v", err.Error())
				}
			}
			//
			if variation.Thumbnail != "" {
				p := path.Join(dir, "hugo", variation.Thumbnail)
				if _, err := os.Stat(p); err == nil {
					if err = os.Remove(p); err != nil {
						logger.Errorf("%v", err.Error())
					}
				}
			}
			if err = models.DeleteVariation(common.Database, variation); err != nil {
				logger.Errorf("%v", err.Error())
			}
		}
		//
		if product.Thumbnail != "" {
			p := path.Join(dir, "hugo", product.Thumbnail)
			if _, err := os.Stat(p); err == nil {
				if err = os.Remove(p); err != nil {
					logger.Errorf("%v", err.Error())
				}
			}
		}
		if err = models.DeleteProduct(common.Database, product); err != nil {
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

// @security BasicAuth
// SearchVariations godoc
// @Summary Search variations
// @Accept json
// @Produce json
// @Param id query int false "Product ID"
// @Param category body ListRequest true "body"
// @Success 200 {object} VariationsListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/variations/list [post]
func postVariationsListHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Query("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var response VariationsListResponse
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
				case "ProductTitle":
					keys2 = append(keys2, fmt.Sprintf("%v like ?", key))
					values2 = append(values2, "%" + strings.TrimSpace(value) + "%")
				case "Options":
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
					keys1 = append(keys1, fmt.Sprintf("variations.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	if id > 0 {
		keys1 = append(keys1, "product_id = ?")
		values1 = append(values1, id)
	}
	logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
	//
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				case "Options":
					orders = append(orders, fmt.Sprintf("%v %v", key, value))
				default:
					orders = append(orders, fmt.Sprintf("properties.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	logger.Infof("order: %+v", order)
	//
	rows, err := common.Database.Debug().Model(&models.Variation{}).Select("variations.ID, variations.Name, variations.Title, variations.Thumbnail, variations.Description, variations.Base_Price, variations.Product_id, products.Title as ProductTitle, group_concat(properties.ID, ', ') as PropertiesIds, group_concat(properties.Title, ', ') as PropertiesTitles").Joins("left join products on products.id = variations.product_id").Joins("left join properties on properties.variation_id = variations.id").Group("variations.id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item VariationsListItem
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
	rows, err = common.Database.Debug().Model(&models.Variation{}).Select("variations.ID, variations.Name, variations.Title, variations.Thumbnail, variations.Description, variations.Base_Price, variations.Product_id, group_concat(properties.ID, ', ') as PropertiesIds, group_concat(properties.Title, ', ') as PropertiesTitles").Joins("left join properties on properties.variation_id = variations.id").Group("variations.id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	if len(keys1) > 0 || len(keys2) > 0 {
		common.Database.Debug().Model(&models.Variation{}).Where("product_id = ?", id).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

// @security BasicAuth
// GetProductVariations godoc
// @Summary Get product variations
// @Accept json
// @Produce json
// @Param id path int false "Product ID"
// @Success 200 {object} VariationsView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/variations [get]
func getVariationsHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Query("product_id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if variations, err := models.GetProductVariations(common.Database, id); err == nil {
		var view []*VariationView
		if bts, err := json.MarshalIndent(variations, "", "   "); err == nil {
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
	c.Status(http.StatusInternalServerError)
	return c.JSON(fiber.Map{"ERROR": "Something went wrong"})
}

type NewVariation struct {
	Title string
	Name string
	Sku string
}

// @security BasicAuth
// CreateVariation godoc
// @Summary Create variation
// @Accept multipart/form-data
// @Produce json
// @Query product_id query int true "Product id"
// @Param category body NewVariation true "body"
// @Success 200 {object} VariationView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/variations [post]
func postVariationHandler(c *fiber.Ctx) error {
	var view VariationView
	var id int
	if v := c.Query("product_id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var err error
	var product *models.Product
	if product, err = models.GetProduct(common.Database, id); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				return err
			}
			var name string
			if v, found := data.Value["Name"]; found && len(v) > 0 {
				name = strings.TrimSpace(v[0])
			}
			if name == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Invalid name"})
			}
			if variations, err := models.GetVariationsByProductAndName(common.Database, product.ID, name); err == nil && len(variations) > 0 {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Name is already in use"})
			}
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
			var basePrice float64
			if v, found := data.Value["BasePrice"]; found && len(v) > 0 {
				if basePrice, err = strconv.ParseFloat(v[0], 10); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{"Invalid base price"})
				}
			}
			var sku string
			if v, found := data.Value["Sku"]; found && len(v) > 0 {
				sku = strings.TrimSpace(v[0])
			}
			variation := &models.Variation{Name: name, Title: title, Description: description, BasePrice: basePrice, ProductId: product.ID, Sku: sku}
			if id, err := models.CreateVariation(common.Database, variation); err == nil {
				if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
					p := path.Join(dir, "storage", "variations")
					if _, err := os.Stat(p); err != nil {
						if err = os.MkdirAll(p, 0755); err != nil {
							logger.Errorf("%v", err)
						}
					}
					filename := fmt.Sprintf("%d-%s-thumbnail%s", id, product.Name, path.Ext(v[0].Filename))
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
							variation.Thumbnail = "/" + path.Join("storage", "variations", filename)
							if err = models.UpdateVariation(common.Database, variation); err != nil {
								c.Status(http.StatusInternalServerError)
								return c.JSON(HTTPError{err.Error()})
							}
						}
					}
				}
				if bts, err := json.Marshal(variation); err == nil {
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

// @security BasicAuth
// UpdateProductsVariation godoc
// @Summary Update variation
// @Accept multipart/form-data
// @Produce json
// @Param id query int false "Variation id"
// @Param category body NewVariation true "body"
// @Success 200 {object} VariationView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/variations/{id} [put]
func putVariationHandler(c *fiber.Ctx) error {
	var view VariationView
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var variation *models.Variation
	var err error
	if variation, err = models.GetVariation(common.Database, id); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	var request NewVariation
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
			var basePrice float64
			if v, found := data.Value["BasePrice"]; found && len(v) > 0 {
				if basePrice, err = strconv.ParseFloat(v[0], 10); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{"Invalid base price"})
				}
			}
			var sku string
			if v, found := data.Value["Sku"]; found && len(v) > 0 {
				sku = strings.TrimSpace(v[0])
			}
			variation.Title = title
			variation.Description = description
			variation.BasePrice = basePrice
			variation.Sku = sku
			if v, found := data.Value["Thumbnail"]; found && len(v) > 0 && v[0] == "" {
				// To delete existing
				if variation.Thumbnail != "" {
					if err = os.Remove(path.Join(dir, variation.Thumbnail)); err != nil {
						logger.Errorf("%v", err)
					}
					variation.Thumbnail = ""
				}
			}else if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
				p := path.Join(dir, "storage", "variations")
				if _, err := os.Stat(p); err != nil {
					if err = os.MkdirAll(p, 0755); err != nil {
						logger.Errorf("%v", err)
					}
				}
				filename := fmt.Sprintf("%d-%s-thumbnail%s", id, regexp.MustCompile(`(?i)[^-a-z0-9]+`).ReplaceAllString(variation.Name, "-"), path.Ext(v[0].Filename))
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
						variation.Thumbnail = "/" + path.Join("storage", "variations", filename)
					}
				}
			}
			if err = models.UpdateVariation(common.Database, variation); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if bts, err := json.Marshal(variation); err == nil {
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
// DelVariation godoc
// @Summary Delete variation
// @Accept json
// @Produce json
// @Param id path int true "Variation ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/variations/{id} [delete]
func delVariationHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if variation, err := models.GetVariation(common.Database, id); err == nil {
		for _, property := range variation.Properties {
			for _, price := range property.Prices{
				if err = models.DeletePrice(common.Database, price); err != nil {
					logger.Errorf("%v", err)
				}
			}
			if err = models.DeleteProperty(common.Database, property); err != nil {
				logger.Errorf("%v", err)
			}
		}
		if variation.Thumbnail != "" {
			p := path.Join(dir, "hugo", variation.Thumbnail)
			if _, err := os.Stat(p); err == nil {
				if err = os.Remove(p); err != nil {
					logger.Errorf("%v", err.Error())
				}
			}
		}
		if err = models.DeleteVariation(common.Database, variation); err == nil {
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

type VariationsListResponse struct {
	Data []VariationsListItem
	Filtered int64
	Total int64
}

type VariationsListItem struct {
	ID uint
	Name string
	Title string
	Thumbnail string
	Description string
	BasePrice float64
	ProductId uint
	ProductTitle string
	PropertiesIds string
	PropertiesTitles string
	//Options int
}

type ValuesListResponse struct {
	Data []ValuesListItem
	Filtered int64
	Total int64
}

type ValuesListItem struct {
	ID uint
	OptionTitle string
	Title string
	Thumbnail string
	Value string
}

// @security BasicAuth
// SearchOptionValues godoc
// @Summary Search option values
// @Accept json
// @Produce json
// @Param option_id query int true "Option ID"
// @Param category body ListRequest true "body"
// @Success 200 {object} ValuesListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/values/list [post]
func postValuesListHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Query("option_id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var response ValuesListResponse
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
				case "OptionTitle":
					keys1 = append(keys1, fmt.Sprintf("%v = ?", key))
					values1 = append(values1, strings.TrimSpace(value))
				default:
					keys1 = append(keys1, fmt.Sprintf("%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	if id > 0 {
		keys1 = append(keys1, "option_id = ?")
		values1 = append(values1, id)
	}
	logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
	//
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				case "Options":
					orders = append(orders, fmt.Sprintf("%v %v", key, value))
				default:
					orders = append(orders, fmt.Sprintf("`values`.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	logger.Infof("order: %+v", order)
	//
	rows, err := common.Database.Debug().Model(&models.Value{}).Select("`values`.ID, `values`.Title, `values`.Thumbnail, `values`.Value, options.Title as OptionTitle").Joins("left join options on options.id = `values`.Option_Id").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item ValuesListItem
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
	rows, err = common.Database.Debug().Model(&models.Value{}).Select("`values`.ID, `values`.Title, `values`.Thumbnail, `values`.Value, options.Title as OptionTitle").Joins("left join options on options.id = `values`.Option_Id").Where(strings.Join(keys1, " and "), values1...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	if len(keys1) > 0 {
		if id == 0 {
			common.Database.Debug().Model(&models.Value{}).Count(&response.Total)
		}else{
			common.Database.Debug().Model(&models.Value{}).Where("option_id = ?", id).Count(&response.Total)
		}
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

// @security BasicAuth
// GetVariation godoc
// @Summary Get variation
// @Accept json
// @Produce json
// @Param id path int true "Variation ID"
// @Success 200 {object} VariationView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/variations/{id} [get]
func getVariationHandler(c *fiber.Ctx) error {
	var variation *models.Variation
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
		var err error
		if variation, err = models.GetVariation(common.Database, id); err == nil {
			var view VariationView
			if bts, err := json.MarshalIndent(variation, "", "   "); err == nil {
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
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Variation ID is not defined"})
	}
}

type NewProperty struct {
	Type string
	Name string
	Title string
	OptionId uint
	Sku string
	Filtering bool
}

// @security BasicAuth
// CreateProperty godoc
// @Summary Create property
// @Accept json
// @Produce json
// @Param variation_id query int true "Variation id"
// @Param property body NewProperty true "body"
// @Success 200 {object} PropertyView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /properties [post]
func postPropertyHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Query("variation_id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var err error
	var variation *models.Variation
	if variation, err = models.GetVariation(common.Database, id); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Option already define, edit existing"})
	}
	var view PropertyView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewProperty
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			//
			if properties, err := models.GetPropertiesByVariationAndName(common.Database, id, request.Name); err == nil {
				if len(properties) > 0 {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{"Option already define, edit existing"})
				}
			}
			//
			property := &models.Property{
				Type: request.Type,
				Name:        request.Name,
				Title:       request.Title,
				OptionId:    request.OptionId,
				VariationId: variation.ID,
				Sku: request.Sku,
				Filtering: request.Filtering,
			}
			logger.Infof("property: %+v", request)
			//
			if _, err := models.CreateProperty(common.Database, property); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if bts, err := json.Marshal(property); err == nil {
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

type PropertiesListResponse struct {
	Data []PropertiesListItem
	Filtered int64
	Total int64
}

type PropertiesListItem struct {
	ID uint
	Name string
	Title string
	ProductId uint
	ProductTitle string
	VariationId uint
	VariationTitle string
	OptionId uint
	OptionTitle string
	PricesIds string
	PricesPrices string
	ValuesValues string
}

// @security BasicAuth
// SearchProductVariations godoc
// @Summary Search product variations
// @Accept json
// @Produce json
// @Param variation_id path int false "Variation ID"
// @Param request body ListRequest true "body"
// @Success 200 {object} VariationsListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/properties/list [post]
func postPropertiesListHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("variation_id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var response PropertiesListResponse
	var request ListRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	logger.Infof("request: %+v", request)
	if len(request.Sort) == 0 {
		request.Sort = make(map[string]string)
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
				case "ProductTitle":
					keys2 = append(keys2, fmt.Sprintf("%v like ?", key))
					values2 = append(values2, "%" + strings.TrimSpace(value) + "%")
				case "VariationTitle":
					keys2 = append(keys2, fmt.Sprintf("%v like ?", key))
					values2 = append(values2, "%" + strings.TrimSpace(value) + "%")
				case "Options":
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
					keys1 = append(keys1, fmt.Sprintf("properties.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	if id > 0 {
		keys1 = append(keys1, "variation_id = ?", "properties.deleted_at is NULL")
		values1 = append(values1, id)
	}
	logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
	logger.Infof("keys2: %+v, values2: %+v", keys2, values2)
	//
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				case "ProductTitle":
					orders = append(orders, fmt.Sprintf("products.Title %v", value))
				case "VariationTitle":
					orders = append(orders, fmt.Sprintf("variations.Title %v", value))
				default:
					orders = append(orders, fmt.Sprintf("properties.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}else{
		order = strings.Join([]string{"ProductTitle", "VariationTitle"}, ", ")
	}
	logger.Infof("order: %+v", order)
	//
	rows, err := common.Database.Debug().Model(&models.Property{}).Select("properties.ID, properties.Name, properties.Title, products.Id as ProductId, products.Title as ProductTitle, variations.Id as VariationId, variations.Title as VariationTitle, group_concat(prices.ID, ', ') as PricesIds, group_concat(`values`.Value, ', ') as ValuesValues, group_concat(prices.Price, ', ') as PricesPrices, options.ID as OptionId, options.Title as OptionTitle").Joins("left join prices on prices.property_id = properties.id").Joins("left join options on options.id = properties.option_id").Joins("left join `values` on `values`.id = prices.value_id").Joins("left join variations on variations.id = properties.variation_id").Joins("left join products on products.id = variations.product_id").Group("prices.property_id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item PropertiesListItem
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
	rows, err = common.Database.Debug().Model(&models.Property{}).Select("properties.ID, properties.Name, properties.Title, products.Id as ProductId, products.Title as ProductTitle, variations.Id as VariationId, variations.Title as VariationTitle, count(prices.ID) as Prices").Joins("left join prices on prices.property_id = properties.id").Joins("left join variations on variations.id = properties.variation_id").Joins("left join products on products.id = variations.product_id").Group("prices.property_id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	// ???
	if len(keys1) > 0 || len(keys2) > 0 {
		common.Database.Debug().Model(&models.Property{}).Where("variation_id = ?", id).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

type PropertyView struct {
	ID uint
	Type string
	Name string
	Title string
	OptionId uint
	Sku string
	Filtering bool
}

// @security BasicAuth
// GetProduct godoc
// @Summary Get property
// @Accept json
// @Produce json
// @Param pid path int true "Product ID"
// @Param oid path int true "Variation ID"
// @Param id path int true "Option ID"
// @Success 200 {object} PropertyView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/properties/{id} [get]
func getPropertyHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if property, err := models.GetProperty(common.Database, id); err == nil {
		var view PropertyView
		if bts, err := json.MarshalIndent(property, "", "   "); err == nil {
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
// UpdatePrice godoc
// @Summary Update price
// @Accept json
// @Produce json
// @Param id path int true "Option ID"
// @Param category body NewPrice true "body"
// @Success 200 {object} PriceView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/properties/{id} [put]
func putPropertyHandler(c *fiber.Ctx) error {
	var view PriceView
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var property *models.Property
	var err error
	if property, err = models.GetProperty(common.Database, id); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewProperty
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			property.Type = request.Type
			property.Title = request.Title
			property.Sku = request.Sku
			property.Filtering = request.Filtering
			if err = models.UpdateProperty(common.Database, property); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if bts, err := json.Marshal(property); err == nil {
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

// @security BasicAuth
// DelPrice godoc
// @Summary Delete price
// @Accept json
// @Produce json
// @Param id query int true "Option id"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/properties/{id} [delete]
func deletePropertyHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if property, err := models.GetProperty(common.Database, id); err == nil {
		if err = models.DeleteProperty(common.Database, property); err != nil {
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

type PricesListResponse struct {
	Data []PricesListItem
	Filtered int64
	Total int64
}

type PricesListItem struct {
	ID         uint
	Enabled    bool
	ProductTitle string
	VariationTitle string
	PropertyId uint
	PropertyTitle string
	OptionId uint
	ValueId    uint
	ValueTitle string
	Price      float64
}

// @security BasicAuth
// SearchProductVariationPropertyValues godoc
// @Summary Search product variation property prices
// @Accept json
// @Produce json
// @Param property_id path int false "Option ID"
// @Param request body ListRequest true "body"
// @Success 200 {object} PricesListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/prices/list [post]
func postPricesListHandler(c *fiber.Ctx) error {
	var response PricesListResponse
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
				case "ProductTitle":
					keys1 = append(keys1, fmt.Sprintf("%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				case "VariationTitle":
					keys1 = append(keys1, fmt.Sprintf("%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				case "PropertyTitle":
					keys1 = append(keys1, fmt.Sprintf("%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				case "Price":
					v := strings.TrimSpace(value)
					if strings.Index(v, ">=") == 0 {
						if vv, err := strconv.Atoi(v[2:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("%v >= ?", key))
							values1 = append(values1, vv)
						}
					} else if strings.Index(v, "<=") == 0 {
						if vv, err := strconv.Atoi(v[1:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("%v <= ?", key))
							values1 = append(values1, vv)
						}
					} else if strings.Index(v, "!=") == 0 || strings.Index(v, "<>") == 0 {
						if vv, err := strconv.Atoi(v[1:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("%v <> ?", key))
							values1 = append(values1, vv)
						}
					} else if strings.Index(v, ">") == 0 {
						if vv, err := strconv.Atoi(v[1:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("%v > ?", key))
							values1 = append(values1, vv)
						}
					} else if strings.Index(v, "<") == 0 {
						if vv, err := strconv.Atoi(v[1:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("%v < ?", key))
							values1 = append(values1, vv)
						}
					} else {
						if vv, err := strconv.Atoi(v); err == nil {
							keys1 = append(keys1, fmt.Sprintf("%v = ?", key))
							values1 = append(values1, vv)
						}
					}
				case "ValueTitle":
					keys1 = append(keys1, fmt.Sprintf("%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				default:
					keys1 = append(keys1, fmt.Sprintf("prices.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	/*keys1 = append(keys1, "property_id = ?", "prices.deleted_at is NULL")
	values1 = append(values1, id)*/
	logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
	//
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				case "ValueTitle":
					orders = append(orders, fmt.Sprintf("%v %v", key, value))
				case "Variations":
					orders = append(orders, fmt.Sprintf("%v %v", key, value))
				default:
					orders = append(orders, fmt.Sprintf("prices.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	logger.Infof("order: %+v", order)
	//
	rows, err := common.Database.Debug().Model(&models.Price{}).Select("prices.ID, prices.Enabled, prices.Price, products.Title as ProductTitle, variations.Title as VariationTitle, properties.ID as PropertyId, properties.Title as PropertyTitle, options.ID as OptionId, `values`.ID as ValueId, `values`.Title as ValueTitle").Joins("left join `values` on `values`.id = prices.value_id").Joins("left join options on options.id = `values`.option_id").Joins("left join properties on properties.ID = prices.Property_Id").Joins("left join variations on variations.ID = properties.Variation_Id").Joins("left join products on products.ID = variations.Product_Id").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item PricesListItem
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
	rows, err = common.Database.Debug().Model(&models.Price{}).Select("prices.ID, prices.Enabled, prices.Price, products.Title as ProductTitle, variations.Title as VariationTitle, properties.ID as PropertyId, properties.Title as PropertyTitle, options.ID as OptionId, `values`.ID as ValueId, `values`.Title as ValueTitle").Joins("left join `values` on `values`.id = prices.value_id").Joins("left join options on options.id = `values`.option_id").Joins("left join properties on properties.ID = prices.Property_Id").Joins("left join variations on variations.ID = properties.Variation_Id").Joins("left join products on products.ID = variations.Product_Id").Where(strings.Join(keys1, " and "), values1...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	/*if len(keys1) > 0 {
		common.Database.Debug().Model(&models.Price{}).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}*/
	response.Total = response.Filtered
	c.Status(http.StatusOK)
	return c.JSON(response)
}

type NewPrice struct {
	Enabled bool
	PropertyId uint
	ValueId uint
	Price float64
	Sku string
}

type PriceView struct {
	ID uint
	Enabled bool
	PropertyId uint
	ValueId uint
	Price float64
	Sku string
}

// @security BasicAuth
// CreatePrice godoc
// @Summary Create categories
// @Accept json
// @Produce json
// @Param property_id query int true "Option id"
// @Param category body NewPrice true "body"
// @Success 200 {object} PriceView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/prices [post]
func postPriceHandler(c *fiber.Ctx) error {
	var view PriceView
	var id int
	if v := c.Query("property_id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var property *models.Property
	var err error
	if property, err = models.GetProperty(common.Database, id); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//logger.Infof("property: %+v", property)
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewPrice
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			//logger.Infof("request: %+v", request)
			//
			if prices, err := models.GetPricesByPropertyAndValue(common.Database, request.PropertyId, request.ValueId); err == nil {
				if len(prices) > 0 {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{"Price already define, edit existing"})
				}
			}
			//
			price := &models.Price{
				Enabled: request.Enabled,
				PropertyId: property.ID,
				ValueId: request.ValueId,
				Price: request.Price,
				Sku: request.Sku,
			}
			logger.Infof("price: %+v", price)
			//
			if _, err := models.CreatePrice(common.Database, price); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if bts, err := json.Marshal(price); err == nil {
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

type PricesView []*PriceView

// @security BasicAuth
// GetPrices godoc
// @Summary Get prices
// @Accept json
// @Produce json
// @Param property_id path int true "Option ID"
// @Success 200 {object} PriceView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/prices [get]
func getPricesHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Query("property_id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if prices, err := models.GetPricesByProperty(common.Database, uint(id)); err == nil {
		var view PricesView
		if bts, err := json.MarshalIndent(prices, "", "   "); err == nil {
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
// GetPrices godoc
// @Summary Get price
// @Accept json
// @Produce json
// @Param id path int true "Price ID"
// @Success 200 {object} PricesView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/prices/{id} [get]
func getPriceHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if price, err := models.GetPrice(common.Database, id); err == nil {
		var view PriceView
		if bts, err := json.MarshalIndent(price, "", "   "); err == nil {
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
// UpdatePrice godoc
// @Summary Update price
// @Accept json
// @Produce json
// @Param id path int true "Price ID"
// @Param request body NewPrice true "body"
// @Success 200 {object} PriceView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/prices/{id} [put]
func putPriceHandler(c *fiber.Ctx) error {
	var view PriceView
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var price *models.Price
	var err error
	if price, err = models.GetPrice(common.Database, id); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewPrice
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			price.Price = request.Price
			price.Sku = request.Sku
			if err = models.UpdatePrice(common.Database, price); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if bts, err := json.Marshal(price); err == nil {
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

// @security BasicAuth
// DelPrice godoc
// @Summary Delete price
// @Accept json
// @Produce json
// @Param id path int true "Price ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/prices/{id} [delete]
func deletePriceHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if price, err := models.GetPrice(common.Database, id); err == nil {
		if err = models.DeletePrice(common.Database, price); err != nil {
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

type TagsView []TagView

type TagView struct{
	ID uint
	Enabled bool
	Name string
	Title string
	Description string
	Hidden bool
}

// @security BasicAuth
// GetTags godoc
// @Summary Get tags
// @Accept json
// @Produce json
// @Success 200 {object} TagsView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/tags [get]
func getTagsHandler(c *fiber.Ctx) error {
	if tags, err := models.GetTags(common.Database); err == nil {
		var view TagsView
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


type NewTag struct {
	Enabled bool
	Hidden bool
	Name string
	Title string
	Description string
}

// @security BasicAuth
// CreateTag godoc
// @Summary Create tag
// @Accept json
// @Produce json
// @Param option body NewTag true "body"
// @Success 200 {object} TagView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/options [post]
func postTagHandler(c *fiber.Ctx) error {
	var view TagView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewTag
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			request.Title = strings.TrimSpace(request.Title)
			if request.Title == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": "Title is not defined"})
			}
			if request.Name == "" {
				request.Name = strings.TrimSpace(request.Name)
				request.Name = reNotAbc.ReplaceAllString(strings.ToLower(request.Title), "-")
			}
			if len(request.Description) > 256 {
				request.Description = request.Description[0:255]
			}
			if options, err := models.GetOptionsByName(common.Database, request.Name); err == nil && len(options) > 0 {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Option exists"})
			}
			tag := &models.Tag {
				Enabled: request.Enabled,
				Hidden: request.Hidden,
				Name: request.Name,
				Title: request.Title,
				Description: request.Description,
			}
			if _, err := models.CreateTag(common.Database, tag); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if bts, err := json.Marshal(tag); err == nil {
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

type TagsListResponse struct {
	Data []TagsListItem
	Filtered int64
	Total int64
}

type TagsListItem struct {
	ID uint
	Enabled bool
	Hidden bool
	Name string
	Title string
	Description string
}

// @security BasicAuth
// SearchOptions godoc
// @Summary Search options
// @Accept json
// @Produce json
// @Param category body ListRequest true "body"
// @Success 200 {object} TagsListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/tags/list [post]
func postTagsListHandler(c *fiber.Ctx) error {
	var response TagsListResponse
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
					keys1 = append(keys1, fmt.Sprintf("tags.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
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
					orders = append(orders, fmt.Sprintf("tags.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	logger.Infof("order: %+v", order)
	//
	rows, err := common.Database.Debug().Model(&models.Tag{}).Select("tags.ID, tags.Enabled, tags.Hidden, tags.Name, tags.Title, tags.Description").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item TagsListItem
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
	rows, err = common.Database.Debug().Model(&models.Tag{}).Select("tags.ID, tags.Enabled, tags.Hidden, tags.Name, tags.Title, tags.Description").Where(strings.Join(keys1, " and "), values1...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	if len(keys1) > 0 {
		common.Database.Debug().Model(&models.Tag{}).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

// @security BasicAuth
// GetOption godoc
// @Summary Get tag
// @Accept json
// @Produce json
// @Param id path int true "Tag ID"
// @Success 200 {object} TagView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/tags/{id} [get]
func getTagHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if tag, err := models.GetTag(common.Database, id); err == nil {
		var view TagView
		if bts, err := json.MarshalIndent(tag, "", "   "); err == nil {
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
// UpdateTag godoc
// @Summary update tag
// @Accept json
// @Produce json
// @Param option body TagView true "body"
// @Param id path int true "Tag ID"
// @Success 200 {object} TagView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/tags/{id} [put]
func putTagHandler(c *fiber.Ctx) error {
	var request TagView
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
	var tag *models.Tag
	var err error
	if tag, err = models.GetTag(common.Database, int(id)); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	request.Title = strings.TrimSpace(request.Title)
	if request.Title == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Title is not defined"})
	}
	if len(request.Description) > 256 {
		request.Description = request.Description[0:255]
	}
	tag.Enabled = request.Enabled
	tag.Title = request.Title
	tag.Description = request.Description
	tag.Hidden = request.Hidden
	if err := models.UpdateTag(common.Database, tag); err == nil {
		return c.JSON(TagView{ID: tag.ID, Name: tag.Name, Title: tag.Title, Description: tag.Description})
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// @security BasicAuth
// DelTag godoc
// @Summary Delete tag
// @Accept json
// @Produce json
// @Param id path int true "Tag ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/tags/{id} [delete]
func delTagHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if tag, err := models.GetTag(common.Database, id); err == nil {
		if err = models.DeleteTag(common.Database, tag); err == nil {
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

type OptionsShortView []OptionShortView

type OptionShortView struct {
	ID uint
	Name string `json:",omitempty"`
	Title string `json:",omitempty"`
	Description string `json:",omitempty"`
}

type NewOption struct {
	Name string
	Title string
	Description string
}

// @security BasicAuth
// CreateOption godoc
// @Summary Create option
// @Accept json
// @Produce json
// @Param option body NewOption true "body"
// @Success 200 {object} OptionView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/options [post]
func postOptionHandler(c *fiber.Ctx) error {
	var view OptionView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewOption
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			request.Title = strings.TrimSpace(request.Title)
			if request.Title == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": "Title is not defined"})
			}
			if request.Name == "" {
				request.Name = strings.TrimSpace(request.Name)
				request.Name = reNotAbc.ReplaceAllString(strings.ToLower(request.Title), "-")
			}
			if len(request.Description) > 256 {
				request.Description = request.Description[0:255]
			}
			if options, err := models.GetOptionsByName(common.Database, request.Name); err == nil && len(options) > 0 {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Option exists"})
			}
			option := &models.Option {
				Name: request.Name,
				Title: request.Title,
				Description: request.Description,
			}
			if _, err := models.CreateOption(common.Database, option); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if bts, err := json.Marshal(option); err == nil {
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

type OptionsListResponse struct {
	Data []OptionsListItem
	Filtered int64
	Total int64
}

type OptionsListItem struct {
	ID uint
	Name string
	Title string
	Description string
	ValuesValues string
}

// @security BasicAuth
// SearchOptions godoc
// @Summary Search options
// @Accept json
// @Produce json
// @Param category body ListRequest true "body"
// @Success 200 {object} OptionsListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/options/list [post]
func postOptionsListHandler(c *fiber.Ctx) error {
	var response OptionsListResponse
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
				case "Values":
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
					keys1 = append(keys1, fmt.Sprintf("options.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
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
					orders = append(orders, fmt.Sprintf("options.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	logger.Infof("order: %+v", order)
	//
	rows, err := common.Database.Debug().Model(&models.Option{}).Select("options.ID, options.Name, options.Title, options.Description, group_concat(`values`.Value, ', ') as ValuesValues").Joins("left join `values` on `values`.option_id = options.id").Group("options.id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item OptionsListItem
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
	rows, err = common.Database.Debug().Model(&models.Option{}).Select("options.ID, options.Name, options.Title, options.Description, group_concat(`values`.Value, ', ') as ValuesValues").Joins("left join `values` on `values`.option_id = options.id").Group("options.id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	if len(keys1) > 0 || len(keys2) > 0 {
		common.Database.Debug().Model(&models.Option{}).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

type OptionsFullView []OptionFullView

type OptionFullView struct {
	OptionShortView
	Values []ValueView `json:",omitempty"`
}


// @security BasicAuth
// GetOptions godoc
// @Summary Get options
// @Accept json
// @Produce json
// @Success 200 {object} OptionsFullView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/options [get]
func getOptionsHandler(c *fiber.Ctx) error {
	if options, err := models.GetOptionsFull(common.Database); err == nil {
		var view OptionsFullView
		if bts, err := json.MarshalIndent(options, "", "   "); err == nil {
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

type OptionView struct {
	ID uint
	Name string `json:",omitempty"`
	Title string `json:",omitempty"`
	Description string `json:",omitempty"`
	Values []ValueView
}

type ValuesView []ValueView

type ValueView struct {
	ID uint
	Title string `json:",omitempty"`
	Thumbnail string `json:",omitempty"`
	Value string `json:",omitempty"`
}

// @security BasicAuth
// GetOption godoc
// @Summary Get option
// @Accept json
// @Produce json
// @Param id path int true "Option ID"
// @Success 200 {object} OptionView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/options/{id} [get]
func getOptionHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if option, err := models.GetOption(common.Database, id); err == nil {
		var view OptionView
		if bts, err := json.MarshalIndent(option, "", "   "); err == nil {
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
// UpdateOption godoc
// @Summary update option
// @Accept json
// @Produce json
// @Param option body OptionShortView true "body"
// @Param id path int true "Option ID"
// @Success 200 {object} OptionShortView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/options/{id} [put]
func putOptionHandler(c *fiber.Ctx) error {
	var request OptionShortView
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
	var option *models.Option
	var err error
	if option, err = models.GetOption(common.Database, int(id)); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	request.Title = strings.TrimSpace(request.Title)
	if request.Title == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Title is not defined"})
	}
	if request.Name == "" {
		request.Name = strings.TrimSpace(request.Name)
		request.Name = reNotAbc.ReplaceAllString(strings.ToLower(request.Title), "-")
	}
	if len(request.Description) > 256 {
		request.Description = request.Description[0:255]
	}
	option.Name = request.Name
	option.Title = request.Title
	option.Description = request.Description
	if err := models.UpdateOption(common.Database, option); err == nil {
		return c.JSON(OptionShortView{ID: option.ID, Name: option.Name, Title: option.Title, Description: option.Description})
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// @security BasicAuth
// GetValues godoc
// @Summary get option values
// @Accept json
// @Produce json
// @Param option_id query int true "Option ID"
// @Success 200 {object} ValuesView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/options/{id}/values [get]
func getValuesHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Query("option_id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var values []*models.Value
	var err error
	if id == 0 {
		values, err = models.GetValues(common.Database)
	}else{
		values, err = models.GetValuesByOptionId(common.Database, id)
	}
	if err == nil {
		var view []*ValueView
		if bts, err := json.MarshalIndent(values, "", "   "); err == nil {
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
	c.Status(http.StatusInternalServerError)
	return c.JSON(fiber.Map{"ERROR": "Something went wrong"})
}

type NewValue struct {
	Title string
	Thumbnail string
	Value string
}

// @security BasicAuth
// CreateOptionValues godoc
// @Tag.name options
// @Summary Create option value
// @Accept json
// @Produce json
// @Param option_id query int true "Option ID"
// @Param option body NewValue true "body"
// @Success 200 {object} ValueView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/values [post]
func postValueHandler(c *fiber.Ctx) error {
	var option *models.Option
	var id int
	if v := c.Query("option_id"); v != "" {
		id, _ = strconv.Atoi(v)
		var err error
		if option, err = models.GetOption(common.Database, id); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Option ID is not defined"})
	}
	var view ValueView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				return err
			}
			var title string
			if v, found := data.Value["Title"]; found && len(v) > 0 {
				title = strings.TrimSpace(v[0])
			}
			if title == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Invalid title"})
			}
			var val string
			if v, found := data.Value["Value"]; found && len(v) > 0 {
				val = strings.TrimSpace(v[0])
			}
			if val == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Invalid value"})
			}
			value := &models.Value{Title: title, Value: val, OptionId: option.ID}
			if id, err := models.CreateValue(common.Database, value); err == nil {
				if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
					p := path.Join(dir, "storage", "values")
					if _, err := os.Stat(p); err != nil {
						if err = os.MkdirAll(p, 0755); err != nil {
							logger.Errorf("%v", err)
						}
					}
					filename := fmt.Sprintf("%d-%s-thumbnail%s", id, regexp.MustCompile(`(?i)[^-a-z0-9]+`).ReplaceAllString(value.Title, "-"), path.Ext(v[0].Filename))
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
							value.Thumbnail = "/" + path.Join("storage", "values", filename)
							if err = models.UpdateValue(common.Database, value); err != nil {
								c.Status(http.StatusInternalServerError)
								return c.JSON(HTTPError{err.Error()})
							}
						}
					}
				}
				if bts, err := json.Marshal(value); err == nil {
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

// @security BasicAuth
// GetOptionValue godoc
// @Summary Get option value
// @Accept json
// @Produce json
// @Param id path int true "Value ID"
// @Success 200 {object} ValueView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/values/{id} [get]
func getValueHandler(c *fiber.Ctx) error {
	var value *models.Value
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
		var err error
		if value, err = models.GetValue(common.Database, id); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Value ID is not defined"})
	}
	var view ValueView
	if bts, err := json.MarshalIndent(value, "", "   "); err == nil {
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

// @security BasicAuth
// UpdateOptionValue godoc
// @Summary update option value
// @Accept json
// @Produce json
// @Param option body ValueView true "body"
// @Param id path int true "Value ID"
// @Success 200 {object} ValueView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/values/{id} [put]
func putValueHandler(c *fiber.Ctx) error {
	var value *models.Value
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
		var err error
		if value, err = models.GetValue(common.Database, id); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Value ID is not defined"})
	}
	var view ValueView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				return err
			}
			var title string
			if v, found := data.Value["Title"]; found && len(v) > 0 {
				title = strings.TrimSpace(v[0])
			}
			if title == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Invalid title"})
			}
			var val string
			if v, found := data.Value["Value"]; found && len(v) > 0 {
				val = strings.TrimSpace(v[0])
			}
			if val == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Invalid value"})
			}
			value.Title = title
			value.Value = val
			//
			if v, found := data.Value["Thumbnail"]; found && len(v) > 0 && v[0] == "" {
				// To delete existing
				if value.Thumbnail != "" {
					if err = os.Remove(path.Join(dir, value.Thumbnail)); err != nil {
						logger.Errorf("%v", err)
					}
					value.Thumbnail = ""
				}
			}else if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
				p := path.Join(dir, "storage", "values")
				if _, err := os.Stat(p); err != nil {
					if err = os.MkdirAll(p, 0755); err != nil {
						logger.Errorf("%v", err)
					}
				}
				filename := fmt.Sprintf("%d-%s-thumbnail%s", id, regexp.MustCompile(`(?i)[^-a-z0-9]+`).ReplaceAllString(value.Title, "-"), path.Ext(v[0].Filename))
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
						value.Thumbnail = "/" + path.Join("storage", "values", filename)
						if err = models.UpdateValue(common.Database, value); err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
					}
				}
			}
			//
			if err := models.UpdateValue(common.Database, value); err == nil {
				return c.JSON(ValueView{ID: value.ID, Title: value.Title, Thumbnail: value.Thumbnail, Value: value.Value})
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

// @security BasicAuth
// DelOption godoc
// @Summary Delete option
// @Accept json
// @Produce json
// @Param id path int true "Value ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/values/{id} [delete]
func delValueHandler(c *fiber.Ctx) error {
	var value *models.Value
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
		var err error
		if value, err = models.GetValue(common.Database, id); err == nil {
			//
			if value.Thumbnail != "" {
				p := path.Join(dir, "hugo", value.Thumbnail)
				if _, err := os.Stat(p); err == nil {
					if err = os.Remove(p); err != nil {
						logger.Errorf("%v", err.Error())
					}
				}
			}
			//
			if err = models.DeleteValue(common.Database, value); err == nil {
				return c.JSON(HTTPMessage{MESSAGE: "OK"})
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Value ID is not defined"})
	}
}

// @security BasicAuth
// DelOption godoc
// @Summary Delete option
// @Accept json
// @Produce json
// @Param id path int true "Option ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/options/{id} [delete]
func delOptionHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if option, err := models.GetOption(common.Database, id); err == nil {
		for _, value := range option.Values {
			if err = models.DeleteValue(common.Database, value); err != nil {
				logger.Errorf("%v", err)
			}
		}
		if err = models.DeleteOption(common.Database, option); err == nil {
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

type NewImage struct {
	Name string
	Image string
}

// @security BasicAuth
// CreateImage godoc
// @Summary Create image
// @Accept multipart/form-data
// @Produce json
// @Param pid query int false "Product id"
// @Param category body NewImage true "body"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/categories [post]
func postImageHandler(c *fiber.Ctx) error {
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				return err
			}
			var name string
			if v, found := data.Value["Name"]; found && len(v) > 0 {
				name = strings.TrimSpace(v[0])
			}
			if v, found := data.File["Image"]; found && len(v) > 0 {
				for _, vv := range v {
					if name == "" {
						name = strings.TrimSuffix(vv.Filename, filepath.Ext(vv.Filename))
					}
					img := &models.Image{Name: name, Size: vv.Size}
					if id, err := models.CreateImage(common.Database, img); err == nil {
						p := path.Join(dir, "storage", "images")
						if _, err := os.Stat(p); err != nil {
							if err = os.MkdirAll(p, 0755); err != nil {
								logger.Errorf("%v", err)
							}
						}
						filename := fmt.Sprintf("%d-%s%s", id, img.Name, path.Ext(vv.Filename))
						if p := path.Join(p, filename); len(p) > 0 {
							if in, err := vv.Open(); err == nil {
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
								img.Url = common.Config.Base + "/" + path.Join("storage", "images", filename)
								img.Path = "/" + path.Join("storage", "images", filename)
								if reader, err := os.Open(p); err != nil {
									defer reader.Close()
									if config, _, err := image.DecodeConfig(reader); err == nil {
										img.Height = config.Height
										img.Width = config.Width
									} else {
										logger.Errorf("%v", err.Error())
									}
								}
								if err = models.UpdateImage(common.Database, img); err != nil {
									c.Status(http.StatusInternalServerError)
									return c.JSON(HTTPError{err.Error()})
								}
								if v := c.Query("pid"); len(v) > 0 {
									if id, err := strconv.Atoi(v); err == nil {
										if product, err := models.GetProduct(common.Database, id); err == nil {
											if err = models.AddImageToProduct(common.Database, product, img); err != nil {
												logger.Errorf("%v", err.Error())
											}
										}else{
											logger.Errorf("%v", err.Error())
										}
									}else{
										logger.Errorf("%v", err.Error())
									}
								}
								c.Status(http.StatusOK)
								return c.JSON(img)
							}else{
								c.Status(http.StatusInternalServerError)
								return c.JSON(HTTPError{err.Error()})
							}
						}
					}else{
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
				}
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Image missed"})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	return c.JSON(HTTPMessage{"OK"})
}

type ImagesListResponse struct {
	Data []ImagesListItem
	Filtered int64
	Total int64
}

type ImagesListItem struct {
	ID uint
	Created time.Time
	Url string
	Name string
	Height int
	Width int
	Size int
	Updated time.Time
}

// @security BasicAuth
// SearchImages godoc
// @Summary Search images
// @Accept json
// @Produce json
// @Param category body ListRequest true "body"
// @Success 200 {object} ImagesListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/images/list [post]
func postImagesListHandler(c *fiber.Ctx) error {
	var response ImagesListResponse
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
				case "Height":
				case "Weight":
				case "Size":
					v := strings.TrimSpace(value)
					if strings.Index(v, ">=") == 0 {
						if vv, err := strconv.Atoi(v[2:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("%v >= ?", key))
							values1 = append(values1, vv)
						}
					} else if strings.Index(v, "<=") == 0 {
						if vv, err := strconv.Atoi(v[2:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("%v <= ?", key))
							values1 = append(values1, vv)
						}
					} else if strings.Index(v, "!=") == 0 || strings.Index(v, "<>") == 0 {
						if vv, err := strconv.Atoi(v[2:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("%v <> ?", key))
							values1 = append(values1, vv)
						}
					} else if strings.Index(v, ">") == 0 {
						if vv, err := strconv.Atoi(v[1:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("%v > ?", key))
							values1 = append(values1, vv)
						}
					} else if strings.Index(v, "<") == 0 {
						if vv, err := strconv.Atoi(v[1:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("%v < ?", key))
							values1 = append(values1, vv)
						}
					} else {
						if vv, err := strconv.Atoi(v); err == nil {
							keys1 = append(keys1, fmt.Sprintf("%v = ?", key))
							values1 = append(values1, vv)
						}
					}
				default:
					keys1 = append(keys1, fmt.Sprintf("images.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
	logger.Infof("keys2: %+v, values2: %+v", keys2, values2)
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
					orders = append(orders, fmt.Sprintf("images.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	logger.Infof("order: %+v", order)
	if v := c.Query("product_id"); v != "" {
		//id, _ = strconv.Atoi(v)
		keys1 = append(keys1, fmt.Sprintf("products_images.product_id = ?"))
		values1 = append(values1, v)
		rows, err := common.Database.Debug().Model(&models.Image{}).Select("images.ID, images.Created_At as Created, images.Name, images.Url, images.Height, images.Width, images.Size, images.Updated_At as Updated").Joins("left join products_images on products_images.image_id = images.id").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
		if err == nil {
			if err == nil {
				for rows.Next() {
					var item ImagesListItem
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
		rows, err = common.Database.Debug().Model(&models.Image{}).Select("images.ID, images.Created_At as Created, images.Name, images.Url, images.Height, images.Width, images.Size, images.Updated_At as Updated").Joins("left join products_images on products_images.image_id = images.id").Where(strings.Join(keys1, " and "), values1...).Rows()
		if err == nil {
			for rows.Next() {
				response.Filtered ++
			}
			rows.Close()
		}
		if len(keys1) > 0 || len(keys2) > 0 {
			common.Database.Debug().Model(&models.Image{}).Select("images.ID").Joins("left join products_images on products_images.image_id = images.id").Where("products_images.product_id = ?", v).Count(&response.Total)
		}else{
			response.Total = response.Filtered
		}
	}else{
		rows, err := common.Database.Debug().Model(&models.Image{}).Select("images.ID, images.Created_At as Created, images.Name, images.Url, images.Height, images.Width, images.Size, images.Updated_At as Updated").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
		if err == nil {
			if err == nil {
				for rows.Next() {
					var item ImagesListItem
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
		rows, err = common.Database.Debug().Model(&models.Image{}).Select("images.ID, images.Created_At as Created, images.Name, images.Url, images.Height, images.Width, images.Size, images.Updated_At as Updated").Where(strings.Join(keys1, " and "), values1...).Rows()
		if err == nil {
			for rows.Next() {
				response.Filtered ++
			}
			rows.Close()
		}
		if len(keys1) > 0 || len(keys2) > 0 {
			common.Database.Debug().Model(&models.Image{}).Count(&response.Total)
		}else{
			response.Total = response.Filtered
		}
	}
	//

	c.Status(http.StatusOK)
	return c.JSON(response)
}

type ImageView struct {
	ID uint
	Created time.Time `json:",omitempty"`
	Name string `json:",omitempty"`
	Path string `json:",omitempty"`
	Url string `json:",omitempty"`
	Height int `json:",omitempty"`
	Width int `json:",omitempty"`
	Size int `json:",omitempty"`
	Updated time.Time `json:",omitempty"`
}

// @security BasicAuth
// GetOption godoc
// @Summary Get image
// @Accept json
// @Produce json
// @Param id path int true "Image ID"
// @Success 200 {object} ImageView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/images/{id} [get]
func getImageHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if option, err := models.GetImage(common.Database, id); err == nil {
		var view ImageView
		if bts, err := json.MarshalIndent(option, "", "   "); err == nil {
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

type ExistingImage struct {

}

// @security BasicAuth
// UpdateImage godoc
// @Summary update image
// @Accept json
// @Produce json
// @Param option body ExistingImage true "body"
// @Param id path int true "Image ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/images/{id} [put]
func putImageHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var img *models.Image
	var err error
	if img, err = models.GetImage(common.Database, id); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	if img.Path == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Image does not exists, please create new"})
	}
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				return err
			}
			if v, found := data.File["Image"]; found && len(v) > 0 {
				p := path.Dir(path.Join(dir, img.Path))
				if _, err := os.Stat(p); err != nil {
					if err = os.MkdirAll(p, 0755); err != nil {
						logger.Errorf("%v", err)
					}
				}
				if err = os.Remove(img.Path); err != nil {
					logger.Errorf("%v", err)
				}
				if img.Name == "" {
					img.Name = strings.TrimSuffix(v[0].Filename, filepath.Ext(v[0].Filename))
				}
				img.Size = v[0].Size
				//
				filename := fmt.Sprintf("%d-%s-%d%s", id, regexp.MustCompile(`(?i)[^-a-z0-9]+`).ReplaceAllString(img.Name, "-"), time.Now().Unix(), path.Ext(v[0].Filename))
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
						img.Path = path.Join(path.Dir(img.Path), filename)
						img.Url = common.Config.Base + path.Join(path.Dir(strings.Replace(img.Path, "/hugo/", "/", 1)), filename)
						//
						if reader, err := os.Open(p); err != nil {
							defer reader.Close()
							if config, _, err := image.DecodeConfig(reader); err == nil {
								img.Height = config.Height
								img.Width = config.Width
							} else {
								logger.Errorf("%v", err.Error())
							}
						}
					}
				}
			}
			if err = models.UpdateImage(common.Database, img); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			//
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
// DelImage godoc
// @Summary Delete image
// @Accept json
// @Produce json
// @Param id path int true "Image ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/images/{id} [delete]
func delImageHandler(c *fiber.Ctx) error {
	var oid int
	if v := c.Params("id"); v != "" {
		oid, _ = strconv.Atoi(v)
		if image, err := models.GetImage(common.Database, oid); err == nil {
			if err = os.Remove(path.Join(dir, image.Path)); err != nil {
				logger.Errorf("%v", err.Error())
			}
			if err = models.DeleteImage(common.Database, image); err == nil {
				return c.JSON(HTTPMessage{MESSAGE: "OK"})
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Image ID is not defined"})
	}
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
					keys1 = append(keys1, fmt.Sprintf("users.Email like ?"))
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
	logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
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
	logger.Infof("order: %+v", order)
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
// @Param option body ExistingOrder true "body"
// @Param id path int true "Order ID"
// @Success 200 {object} OrderShortView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/orders/{id} [put]
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
// @Param category body ListRequest true "body"
// @Success 200 {object} TransactionsListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/transactions/list [post]
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
	var keys2 []string
	var values2 []interface{}
	if len(request.Filter) > 0 {
		for key, value := range request.Filter {
			if key != "" && len(strings.TrimSpace(value)) > 0 {
				switch key {
				case "Values":
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
					keys1 = append(keys1, fmt.Sprintf("options.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
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
	logger.Infof("order: %+v", order)
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
	if len(keys1) > 0 || len(keys2) > 0 {
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
// @Param option body ExistingTransaction true "body"
// @Param id path int true "Transaction ID"
// @Success 200 {object} TransactionView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/transactions/{id} [put]
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

type MeView struct {
	ID uint
	Enabled bool
	Login string
	Email string
	EmailConfirmed bool
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
	Zip int
	City string
	Region string
	Country string
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

type UsersView []UserView

type UserView struct {
	ID uint
	Enabled bool
	Login string
	Email string
	EmailConfirmed bool
	Role int `json:",omitempty"`
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
	Orders int
	UpdatedAt time.Time
}

// @security BasicAuth
// SearchUsers godoc
// @Summary Search users
// @Accept json
// @Produce json
// @Param category body ListRequest true "body"
// @Success 200 {object} UsersListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/users/list [post]
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
	logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
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
	logger.Infof("order: %+v", order)
	//
	func() {
		rows, err := common.Database.Debug().Model(&models.User{}).Select("users.ID, users.Created_At as CreatedAt, users.Login, users.Email, count(orders.Id) as Orders, users.Updated_At as UpdatedAt").Joins("left join orders on orders.User_Id = users.id").Group("users.id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
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
		rows, err := common.Database.Debug().Model(&models.User{}).Select("users.ID, users.Created_At as CreatedAt, users.Login, users.Email, users.Updated_At as UpdatedAt").Joins("left join orders on orders.User_Id = users.id").Group("users.id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Order(order).Rows()
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
// @Param id path int true "Order ID"
// @Success 200 {object} UserView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/users/{id} [get]
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
	Password string
	EmailConfirmed bool
}

// @security BasicAuth
// UpdateUser godoc
// @Summary update user
// @Accept json
// @Produce json
// @Param option body ExistingUser true "body"
// @Param id path int true "User ID"
// @Success 200 {object} UserView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/users/{id} [put]
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
	user.EmailConfirmed = request.EmailConfirmed
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

type NewRender struct {

}

type RenderView struct {
	Output string
	Status string
}

// @security BasicAuth
// MakeRender godoc
// @Summary Make render
// @Accept json
// @Produce json
// @Param category body NewRender true "body"
// @Success 200 {object} RenderView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /render [post]
func postPrepareHandler(c *fiber.Ctx) error {
	var view RenderView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewRender
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
				logger.Errorf("%v", err.Error())
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			view.Output = string(buff.Bytes())
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

type NewBuild struct {

}

type BuildView struct {
	Output string
	Status string
}

// @security BasicAuth
// MakeRender godoc
// @Summary Make build
// @Accept json
// @Produce json
// @Param category body NewBuild true "body"
// @Success 200 {object} BuildView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /render [post]
func postRenderHandler(c *fiber.Ctx) error {
	var view BuildView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewBuild
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			//
			arguments := []string{"--cleanDestinationDir"}
			if common.Config.Hugo.Minify {
				arguments = append(arguments, "--minify")
			}
			arguments = append(arguments, []string{"-s", path.Join(dir, "hugo")}...)
			cmd := exec.Command(common.Config.Hugo.Home, arguments...)
			buff := &bytes.Buffer{}
			cmd.Stderr = buff
			cmd.Stdout = buff
			err := cmd.Run()
			if err != nil {
				logger.Errorf("%v", err.Error())
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			view.Output = string(buff.Bytes())
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
// MakeRender godoc
// @Summary Make build
// @Accept json
// @Produce json
// @Param category body NewBuild true "body"
// @Success 200 {object} BuildView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /publish [post]
func postPublishHandler(c *fiber.Ctx) error {
	var view BuildView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewBuild
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			//
			p := path.Join(dir, "worker", "publish.sh")
			if _, err := os.Stat(p); err != nil {
				logger.Errorf("%v", err.Error())
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			//
			cmd := exec.Command(p)
			buff := &bytes.Buffer{}
			cmd.Stdout = buff
			cmd.Stderr = buff
			err := cmd.Run()
			if err != nil {
				logger.Errorf("%v", err.Error())
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			view.Output = string(buff.Bytes())
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

type AccountView struct {
	Admin bool
	Profiles []ProfileView `json:",omitempty"`
	UserView
}

// @security BasicAuth
// @Summary Get account
// @Description get string
// @Accept json
// @Produce json
// @Success 200 {object} AccountView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account [get]
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

type NewProfile struct {
	Email string
	Name string
	Lastname string
	Company string
	Address string
	Zip int
	City string
	Region string
	Country string
}

// @security BasicAuth
// CreateProfile godoc
// @Summary Create profile
// @Accept json
// @Produce json
// @Param profile body NewProfile true "body"
// @Success 200 {object} ProfileView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/profiles [post]
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
			var address = strings.TrimSpace(request.Address)
			if address == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Address is empty"})
			}
			var zip = request.Zip
			if zip == 0 {
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

type Cart struct {
	Created time.Time
	Items []CartItem
	Total float64
	Updated time.Time
}

func (c *Cart) AddItem(uuid string) error {
	logger.Infof("AddItem: %v", uuid)
	// Already in cart?
	for i, item := range c.Items {
		if item.UUID == uuid {
			logger.Info("qty")
			c.Items[i].Quantity ++
			c.Total += item.Price
			return nil
		}
	}
	logger.Info("not qty")
	// Add new item
	var arr []int
	if err := json.Unmarshal([]byte(uuid), &arr); err == nil && len(arr) > 2{
		productId := arr[0]
		var product *models.Product
		if product, err = models.GetProduct(common.Database, productId); err != nil {
			return err
		}
		variationId := arr[1]
		var variation *models.Variation
		if variation, err = models.GetVariation(common.Database, variationId); err != nil {
			return err
		}
		if product.ID != variation.ProductId {
			return fmt.Errorf("Product and Variation mismatch")
		}
		item := CartItem{
			UUID:  uuid,
			Title: product.Title + " " + variation.Title,
			Price: variation.BasePrice,
		}
		if product.Thumbnail != "" {
			item.Thumbnails = append(item.Thumbnails, product.Thumbnail)
		}
		if variation.Thumbnail != "" {
			item.Thumbnails = append(item.Thumbnails, variation.Thumbnail)
		}
		for _, id := range arr[2:] {
			if price, err := models.GetPrice(common.Database, id); err == nil {
				item.Properties = append(item.Properties, ItemProperty{
					Name: price.Property.Title,
					Value: price.Value.Value,
					Price: price.Price,
				})
				item.Price += price.Price
			} else {
				return err
			}
		}
		item.Quantity = 1
		
		c.Items = append(c.Items, item)
		c.Total += item.Price
		return nil
	}
	return nil
}


func (c *Cart) RemoveItem(uuid string, count int) error {
	logger.Infof("RemoveItem: %+v", uuid)
	for i, item := range c.Items {
		if item.UUID == uuid {
			logger.Info("found")
			if item.Quantity > count {
				logger.Info("case1")
				c.Items[i].Quantity -= count
			}else{
				logger.Info("case2")
				c.Items = append(c.Items[:i], c.Items[i+1:]...)
			}
			c.Total -= item.Price
			break
		}
	}
	return nil
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

func (c *Cart) Save(store *session.Store) error {
	if bts, err := json.Marshal(c); err == nil {
		store.Set("cart", bts)
		return nil
	}else{
		return err
	}
	return fmt.Errorf("something went wrong")
}

func LoadCart(store *session.Store) (*Cart, error) {
	var cart Cart
	v := store.Get("cart")
	if v != nil {
		if bts := v.([]byte); len(bts) > 0 {
			if err := json.Unmarshal(bts, &cart); err != nil {
				return nil, err
			}
		}else{
			return nil, fmt.Errorf("not found")
		}
	}else{
		cart = Cart{
			Created: time.Now(),
		}
	}
	return &cart, nil
}

// @security BasicAuth
// CreateProfile godoc
// @Summary Create profile
// @Accept json
// @Produce json
// @Param profile body NewProfile true "body"
// @Success 200 {object} ProfileView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/profiles [post]
func postProfileHandler(c *fiber.Ctx) error {
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
				Login: login,
				Password: models.MakeUserPassword(password),
				Role: models.ROLE_USER,
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
			logger.Infof("credentials: %+v", credentials)
			if encoded, err := cookieHandler.Encode(COOKIE_NAME, credentials); err == nil {
				logger.Infof("encoded: %+v", encoded)
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
				return c.JSON(HTTPError{"Name is empty"})
			}
			var lastname = strings.TrimSpace(request.Lastname)
			if lastname == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Lastname is empty"})
			}
			var company = strings.TrimSpace(request.Company)
			var address = strings.TrimSpace(request.Address)
			if address == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Address is empty"})
			}
			var zip = request.Zip
			if zip == 0 {
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

type ShippingView struct {
	Price float64
}

// @security BasicAuth
// CalculateShippingPrice godoc
// @Summary Calculate shipping price
// @Accept json
// @Produce json
// @Param option body Address true "body"
// @Success 200 {object} ShippingView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/delivery/calculate [post]
func postDeliveryCalculateHandler(c *fiber.Ctx) error {
	var view ShippingView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request ProfileView
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			logger.Infof("request: %+v", request)
			// TODO: Calculation
			rand.Seed(time.Now().UnixNano())
			view.Price = math.Round(rand.Float64() * 10000) / 100
			return c.JSON(view)
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	return c.JSON(view)
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
	BasePrice float64
	CategoryId uint
}

// @security BasicAuth
// SearchCategories godoc
// @Summary Filter products
// @Accept json
// @Produce json
// @Param relPath query int true "Category RelPath"
// @Param category body FilterRequest true "body"
// @Success 200 {object} ProductsFilterResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/filter [post]
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
	logger.Infof("request: %+v", request)
	if len(request.Sort) == 0 {
		request.Sort = map[string]string{"ID": "desc"}
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
				case "BasePrice":
					parts := strings.Split(value, "-")
					if len(parts) == 1 {
						if v, err := strconv.Atoi(parts[0]); err == nil {
							keys1 = append(keys1, "cache_products.Base_Price == ?")
							values1 = append(values1, v)
						}
					} else {
						if v, err := strconv.Atoi(parts[0]); err == nil {
							keys1 = append(keys1, "cache_products.Base_Price >= ?")
							values1 = append(values1, v)
						}
						if v, err := strconv.Atoi(parts[1]); err == nil {
							keys1 = append(keys1, "cache_products.Base_Price <= ?")
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
								//logger.Infof("Search for list #%d with values %+v", id, values)
								//
								//keys2 = append(keys2, fmt.Sprintf("options.List_Id = ?"))
								//values2 = append(values2, id)
								//
								var keys3 []string
								var values3 []interface{}
								for _, value := range values {
									if v, err := strconv.Atoi(value); err == nil {
										keys3 = append(keys3, fmt.Sprintf("prices.Value_Id = ?"))
										values3 = append(values3, v)
									}
								}
								keys1 = append(keys1, fmt.Sprintf("(options.Id = ? and (%v))", strings.Join(keys3, " or ")))
								values1 = append(values1, append([]interface{}{id}, values3...)...)
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
	keys1 = append(keys1, "Path LIKE ?")
	values1 = append(values1, relPath + "%")
	logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
	//
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				case "BasePrice":
					orders = append(orders, fmt.Sprintf("cache_products.%v %v", "Base_Price", value))
				default:
					orders = append(orders, fmt.Sprintf("cache_products.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	logger.Infof("order: %+v", order)
	//
	rows, err := common.Database.Debug().Model(&models.CacheProduct{}).Select("cache_products.ID, cache_products.Name, cache_products.Title, cache_products.Path, cache_products.Description, cache_products.Thumbnail, cache_products.Images, cache_products.Variations, cache_products.Base_Price as BasePrice, cache_products.Category_Id as CategoryId").Joins("inner join variations on variations.Product_ID = cache_products.Product_ID").Joins("inner join properties on properties.Variation_Id = variations.Id").Joins("inner join options on options.Id = properties.Option_Id").Joins("inner join prices on prices.Property_Id = properties.Id").Where(strings.Join(keys1, " and "), values1...)/*.Having(strings.Join(keys2, " and "), values2...)*/.Group("cache_products.id").Order(order).Limit(request.Length).Offset(request.Start).Rows()
	//rows, err := common.Database.Debug().Model(&models.CacheProduct{}).Select("Product_ID as ID, Name, Title, Path, Description, Thumbnail, Base_Price as BasePrice, Category_Id as CategoryId").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
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
	common.Database.Debug().Model(&models.CacheProduct{}).Select("Product_ID as ID, Name, Title, Path, Description, Thumbnail, Base_Price as BasePrice, Category_Id as CategoryId").Where(strings.Join(keys1, " and "), values1...).Count(&response.Filtered)
	common.Database.Debug().Model(&models.CacheProduct{}).Select("Product_ID as ID, Name, Title, Path, Description, Thumbnail, Base_Price as BasePrice, Category_Id as CategoryId").Where("Path LIKE ?", relPath + "%").Count(&response.Total)
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

// Search godoc
// @Summary Post search request
// @Accept json
// @Produce json
// @Param search body SearchRequest true "body"
// @Success 200 {object} SearchResult
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /search [post]
func postSearchHandler(c *fiber.Ctx) error {
	var request SearchRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	request.Term = strings.TrimSpace(request.Term)
	if request.Term == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Empty request"})
	}
	//
	result := SearchResult{Term: request.Term}
	term := "%" + request.Term + "%"
	limit := 20
	if request.Limit > 0 && request.Limit < 100 {
		limit = request.Limit
	}
	if products, err := models.SearchProducts(common.Database, term, limit); err == nil {
		for _, product := range products {
			if bts, err := json.Marshal(product); err == nil {
				var view SearchResultProductView
				if err = json.Unmarshal(bts, &view); err == nil {
					if len(product.Categories) > 0 {
						if breadcrumbs := models.GetBreadcrumbs(common.Database, product.Categories[0].ID); len(breadcrumbs) > 0 {
							var names []string
							for _, crumb := range breadcrumbs {
								names = append(names, crumb.Name)
							}
							view.Path = "/" + path.Join(names...) + "/" + product.Name + "/"
						}
					}
					if len(product.Variations) > 0 {
						view.BasePrice = product.Variations[0].BasePrice
					}
					result.Products = append(result.Products, view)
				}
			}

		}
	}
	return c.JSON(result)
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
// @Summary Get orders
// @Accept json
// @Produce json
// @Success 200 {object} OrdersView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders [get]
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

type NewOrder struct {
	Created time.Time
	Items []NewItem
	Comment string
	ProfileId uint
}

type NewItem struct {
	UUID string
	CategoryId uint
	Quantity int
}

/**/

type OrderShortView struct{
	Items []ItemShortView `json:",omitempty"`
	Sum float64
	Delivery float64
	Total float64
	Comment string `json:",omitempty"`
}

type ItemShortView struct {
	Uuid string                    `json:",omitempty"`
	Title string                   `json:",omitempty"`
	Path string                    `json:",omitempty"`
	Thumbnail string               `json:",omitempty"`
	Variation VariationShortView	`json:",omitempty"`
	Properties []PropertyShortView `json:",omitempty"`
	Price float64                  `json:",omitempty"`
	Quantity int                   `json:",omitempty"`
	Total      float64             `json:",omitempty"`
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
// @Summary Post cart
// @Accept json
// @Produce json
// @Param cart body NewOrder true "body"
// @Success 200 {object} OrderView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders [post]
func postAccountOrdersHandler(c *fiber.Ctx) error {
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
	}
	// TODO: Here will be calculation delivery price
	if profile.ID > 0 {
		rand.Seed(time.Now().UnixNano())
		order.Delivery = math.Round(rand.Float64() * 10000) / 100
	}
	// //
	if v := c.Locals("user"); v != nil {
		if user, ok := v.(*models.User); ok {
			order.User = user
		}
	}
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
			if len(product.Categories) > 0 {
				if breadcrumbs := models.GetBreadcrumbs(common.Database, product.Categories[0].ID); len(breadcrumbs) > 0 {

				}
			}
			variationId := arr[1]
			var variation *models.Variation
			if variation, err = models.GetVariation(common.Database, variationId); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if product.ID != variation.ProductId {
				err = fmt.Errorf("Product and Variation mismatch")
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
			//
			itemShortView := ItemShortView{
				Uuid: item.UUID,
				Title: product.Title,
				Variation: VariationShortView{
					Title:variation.Title,
				},
			}
			//
			if breadcrumbs := models.GetBreadcrumbs(common.Database, categoryId); len(breadcrumbs) > 0 {
				var chunks []string
				for _, crumb := range breadcrumbs {
					chunks = append(chunks, crumb.Name)
				}
				orderItem.Path = "/" + path.Join(append(chunks, product.Name)...)
			}

			var thumbnails []string

			if product.Thumbnail != "" {
				if orderItem.Thumbnail == "" {
					orderItem.Thumbnail = product.Thumbnail
				}
				thumbnails = append(thumbnails, product.Thumbnail)
			}

			/*if variation.Thumbnail != "" {
				if orderItem.Thumbnail == "" {
					orderItem.Thumbnail = variation.Thumbnail
				}
				thumbnails = append(thumbnails, variation.Thumbnail)
			}
			orderItem.Thumbnails = strings.Join(thumbnails, ",")
			*/

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
						logger.Infof("price: %+v, value: %+v", price, price.Value)
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
			orderShortView.Items = append(orderShortView.Items, itemShortView)
		}
	}
	order.Total = order.Sum + order.Delivery
	orderShortView.Sum = order.Sum
	orderShortView.Delivery = order.Delivery
	orderShortView.Total = order.Total
	if bts, err := json.Marshal(orderShortView); err == nil {
		order.Description = string(bts)
	}
	logger.Infof("order: %+v", order)
	if _, err := models.CreateOrder(common.Database, order); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	/*transaction := &models.Transaction{Amount: order.Total, Status: models.TRANSACTION_STATUS_NEW, OrderId: order.ID}
	if _, err := models.CreateTransaction(common.Database, transaction); err != nil {
		logger.Errorf("%v", err.Error())
	}
	logger.Infof("transaction: %+v", transaction)*/
	return c.JSON(order)
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
// @Success 200 {object} SessionView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders/checkout/{id}/stripe [post]
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
	logger.Infof("request: %+v", request)

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
						Name: stripe.String(fmt.Sprintf("Order #%d", order.Total)),
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


// GetStripeCards godoc
// @Summary Get stripe cards
// @Accept json
// @Produce json
// @Param id path int true "Order ID"
// @Success 200 {object} StripeCardsView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders/{id}/stripe/card [get]
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

/*type StripeCheckoutSessionView struct {
	SessionID string `json:"id"`
}*/

// PostOrder godoc
// @Summary Post order
// @Accept json
// @Produce json
// @Param cart body NewStripeCard true "body"
// @Success 200 {object} SomethingView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders/checkout/{id}/stripe/card [post]
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
	logger.Infof("request: %+v", request)

	params := &stripe.CardParams{
		Customer: stripe.String(request.CustomerId),
		Token: stripe.String(request.Token),
	}

	card, err := card.New(params)
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

// GetSecret godoc
// @Summary Get secret
// @Accept json
// @Produce json
// @Param id path int true "Order ID"
// @Success 200 {object} StripeSecretView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders/{id}/stripe/secret [get]
/*func getAccountOrderStripeSecretHandler(c *fiber.Ctx) error {
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

		pi, err := common.STRIPE.CreatePaymentIntent(order.Total, string(stripe.CurrencyUSD))

		if err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}

		data := StripeSecretView{
			ClientSecret: pi.ClientSecret,
		}
		c.Status(http.StatusOK)
		return c.JSON(data)
	} else {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}*/

// PostStripePayment godoc
// @Summary Post stripe payment
// @Accept json
// @Produce json
// @Param id path int true "Order ID"
// @Success 200 {object} StripeCardsView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders/{id}/stripe/submit [get]
func postAccountOrderStripeSubmitHandler(c *fiber.Ctx) error {
	logger.Infof("postAccountOrderStripeSubmitHandler")
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

		logger.Infof("Start")
		params := &stripe.PaymentIntentParams{
			Amount: stripe.Int64(int64(math.Round(order.Total * 100))),
			Currency: stripe.String(common.Config.Currency),
			Customer: stripe.String(customerId),
			/*PaymentMethodTypes: []*string{
				stripe.String("card"),
			},*/
			Confirm: stripe.Bool(true),
			//ReturnURL: stripe.String(fmt.Sprintf(CONFIG.Stripe.ConfirmURL, transactionId)),
		}
		if bts, err := json.Marshal(params); err == nil {
			logger.Infof("params: %+v", string(bts))
		}
		pi, err := common.STRIPE.CreatePaymentIntent(params)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}

		if bts, err := json.Marshal(pi); err == nil {
			logger.Infof("bts: %+v", string(bts))
		}

		transaction := &models.Transaction{Amount: order.Total, Status: models.TRANSACTION_STATUS_NEW, Order: order}
		transactionPayment := models.TransactionPayment{Stripe: models.TransactionPaymentStripe{Id: pi.ID}}
		if bts, err := json.Marshal(transactionPayment); err == nil {
			transaction.Payment = string(bts)
		}
		if _, err = models.CreateTransaction(common.Database, transaction); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}

		if pi.Status == "requires_payment_method" {
			logger.Infof("case 1")
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{pi.LastPaymentError.Error()})
		} else if pi.Status == "requires_action" {
			logger.Infof("case 2")
			transaction.Status = models.TRANSACTION_STATUS_PENDING
			if err = models.UpdateTransaction(common.Database, transaction); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		} else {
			logger.Infof("case 3")
			// succeeded
			transaction.Status = models.TRANSACTION_STATUS_COMPLETE
			if err = models.UpdateTransaction(common.Database, transaction); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			order.Status = models.ORDER_STATUS_PAYED
			if err = models.UpdateOrder(common.Database, order); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}

		c.Status(http.StatusOK)
		return c.JSON(pi)
	} else {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
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

func GetCategoriesView(connector *gorm.DB, id int, depth int) (*CategoryView, error) {
	if id == 0 {
		return getChildrenCategoriesView(connector, &CategoryView{Name: "root", Title: "Root", Type: "category"}, depth), nil
	} else {
		if category, err := models.GetCategory(connector, id); err == nil {
			view := getChildrenCategoriesView(connector, &CategoryView{ID: category.ID, Name: category.Name, Title: category.Title, Thumbnail: category.Thumbnail, Description: category.Description, Type: "category"}, depth)
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

func getChildrenCategoriesView(connector *gorm.DB, root *CategoryView, depth int) *CategoryView {
	for _, category := range models.GetChildrenOfCategoryById(connector, root.ID) {
		if depth > 0 {
			child := getChildrenCategoriesView(connector, &CategoryView{ID: category.ID, Name: category.Name, Title: category.Title, Thumbnail: category.Thumbnail, Description: category.Description, Type: "category"}, depth - 1)
			root.Children = append(root.Children, child)
		}
	}
	if products, err := models.GetProductsByCategoryId(connector, root.ID); err == nil {
		for _, product := range products {
			root.Children = append(root.Children, &CategoryView{ID: product.ID, Name: product.Name, Title: product.Title, Thumbnail: product.Thumbnail, Description: product.Description, Type: "product"})
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
	Name string
	Title string
	Thumbnail string `json:",omitempty"`
	Description string `json:",omitempty"`
	Content string
	Upsale string
	Variations []VariationView `json:",omitempty"`
	Images []ImageView `json:",omitempty"`
	//
	Categories []CategoryView `json:",omitempty"`
	Tags []TagView `json:",omitempty"`
}

type VariationsView []VariationView

type VariationView struct {
	ID uint
	Name string
	Title string
	Description string `json:",omitempty"`
	Thumbnail string `json:",omitempty"`
	BasePrice float64
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
			}
			Price float64
		}
	}
	Sku string
	ProductId uint
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
