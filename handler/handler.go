package handler

import (
	"encoding/json"
	"fmt"
	swagger "github.com/arsmn/fiber-swagger/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/session/v2"
	"github.com/gofiber/session/v2/provider/redis"
	"github.com/google/logger"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/models"
	"gorm.io/gorm"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"regexp"
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
			AllowHeaders:  "Accept, Authorization, Content-Type, Cookie, Ignoreloadingbar, Origin, Set-Cookie",
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
	v1.Get("/logout", authMulti, getLogoutHandler)
	v1.Get("/info", authMulti, hasRole(models.ROLE_ROOT, models.ROLE_ADMIN, models.ROLE_MANAGER), getInfoHandler)
	v1.Get("/categories", authMulti, getCategoriesHandler)
	v1.Post("/categories", authMulti, postCategoriesHandler)
	v1.Post("/categories/list", authMulti, postCategoriesListHandler)
	v1.Get("/categories/:id", authMulti, getCategoryHandler)
	v1.Put("/categories/:id", authMulti, putCategoryHandler)
	v1.Delete("/categories/:id", authMulti, delCategoryHandler)
	v1.Get("/products", authMulti, getProductsHandler)
	v1.Post("/products", authMulti, postProductsHandler)
	v1.Post("/products/list", authMulti, postProductsListHandler)
	v1.Get("/products/:id", authMulti, getProductHandler)
	v1.Get("/products/:id/offers", authMulti, getProductOffersHandler)
	v1.Post("/products/:id/offers/list", authMulti, postProductOffersListHandler)
	v1.Get("/products/:pid/offers/:id", authMulti, getProductOfferHandler)
	v1.Post("/products/:pid/offers/:id/properties", authMulti, postProductOfferPropertyHandler)
	v1.Post("/products/:pid/offers/:id/properties/list", authMulti, postProductOfferPropertiesListHandler)
	v1.Get("/products/:pid/offers/:oid/properties/:id", authMulti, getProductOfferPropertyHandler)
	v1.Put("/products/:pid/offers/:oid/properties/:id", authMulti, putProductOfferPropertyHandler)
	v1.Delete("/products/:pid/offers/:oid/properties/:id", authMulti, deleteProductOfferPropertyHandler)
	v1.Post("/products/:pid/offers/:oid/properties/:id/list", authMulti, postProductOfferPropertyPricesListHandler)
	v1.Get("/products/:pid/offers/:oid/properties/:id/prices", authMulti, getProductOfferPropertyPricesHandler)
	v1.Post("/products/:pid/offers/:oid/properties/:id/prices", authMulti, postProductOfferPropertyPriceHandler)
	v1.Get("/products/:pid/offers/:oid/properties/:rid/prices/:id", authMulti, getProductOfferPropertyPriceHandler)
	v1.Put("/products/:pid/offers/:oid/properties/:rid/prices/:id", authMulti, putProductOfferPropertyPriceHandler)
	v1.Delete("/products/:pid/offers/:oid/properties/:rid/prices/:id", authMulti, deleteProductOfferPropertyPriceHandler)
	v1.Get("/options", authMulti, getOptionsHandler)
	v1.Post("/options", authMulti, postOptionsHandler)
	v1.Post("/options/list", authMulti, postOptionsListHandler)
	v1.Get("/options/:id", authMulti, getOptionHandler)
	v1.Put("/options/:id", authMulti, putOptionHandler)
	v1.Get("/options/:id/values", authMulti, getOptionValuesHandler)
	v1.Post("/options/:id/values", authMulti, postOptionValuesHandler)
	v1.Post("/options/:id/values/list", authMulti, postOptionValuesListHandler)
	v1.Get("/options/:oid/values/:id", authMulti, getOptionValueHandler)
	v1.Put("/options/:oid/values/:id", authMulti, putOptionValueHandler)
	v1.Delete("/options/:oid/values/:id", authMulti, delOptionValueHandler)
	v1.Delete("/options/:id", authMulti, delOptionHandler)
	v1.Get("/users", authMulti, getUsersHandler)

	app.Get("/profile", authMulti, getProfileHandler)

	app.Get("/orders", authMulti, getOrdersHandler)
	app.Post("/orders", authMulti, postOrdersHandler)
	app.Get("/orders/:id", authMulti, getOrderHandler)

	app.Post("/search", postSearchHandler)

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
	//
	static := path.Join(dir, "static")
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
	return app
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
		return c.JSON(fiber.Map{"ERROR": err.Error()})
	}
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
			category := &models.Category{Name: name, Title: title, Description: description, ParentId: request.ParentId}
			if id, err := models.CreateCategory(common.Database, category); err == nil {
				if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
					p := path.Join(dir, "static", "categories")
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
							category.Thumbnail = "/" + path.Join("static", "categories", filename)
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
		request.Sort["ID"] = "asc"
	}
	if request.Length == 0 {
		request.Length = 10
	}
	logger.Infof("request: %+v", request)
	// Filter
	var keys []string
	var values []interface{}
	if len(request.Filter) > 0 {
		for key, value := range request.Filter {
			if key != "" && len(strings.TrimSpace(value)) > 0 {
				keys = append(keys, fmt.Sprintf("%v like ?", key))
				values = append(values, "%" + strings.TrimSpace(value) + "%")
			}
		}
	}
	var filtering = true
	if len(keys) == 0 {
		filtering = false
		keys = append(keys, "parent_id = ?")
		values = append(values, id)
	}
	logger.Infof("keys: %+v, values: %+v", strings.Join(keys, " and "), values)
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
	var children []*models.Category
	if err := common.Database.Debug().Where(strings.Join(keys, " and "), values...).Order(order).Limit(request.Length).Offset(request.Start).Find(&children).Error; err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": err})
	}
	if bts, err := json.Marshal(children); err == nil {
		if err = json.Unmarshal(bts, &response.Data); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": err})
	}
	if filtering {
		common.Database.Debug().Model(&models.Category{}).Where(strings.Join(keys, " and "), values...).Count(&response.Filtered)
		common.Database.Debug().Model(&models.Category{}).Count(&response.Total)
	}else{
		common.Database.Debug().Model(&models.Category{}).Count(&response.Filtered)
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}



type CategoryFullView struct {
	ID uint
	Name string
	Title string
	Description string
	Thumbnail string
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
				return c.JSON(fiber.Map{"ERROR": err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": err.Error()})
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
			category.Name = name
			category.Title = title
			category.Description = description
			if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
				p := path.Join(dir, "static", "categories")
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
						category.Thumbnail = "/" + path.Join("static", "categories", filename)
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
				p := path.Join(dir, category.Thumbnail)
				if _, err := os.Stat(p); err == nil {
					if err = os.Remove(p); err != nil {
						logger.Errorf("%v", err.Error())
					}
				}
			}
			if err = models.DeleteCategory(common.Database, category); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": err.Error()})
			}
		}
		//
		if category.Thumbnail != "" {
			p := path.Join(dir, category.Thumbnail)
			if _, err := os.Stat(p); err == nil {
				if err = os.Remove(p); err != nil {
					logger.Errorf("%v", err.Error())
				}
			}
		}
		if err = models.DeleteCategory(common.Database, category); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": err.Error()})
	}
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
				return c.JSON(fiber.Map{"ERROR": err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": err.Error()})
	}
}

type NewProduct struct {
	Name string
	Title string
	Description string
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
// @Router /api/v1/categories [post]
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
			product := &models.Product{Name: name, Title: title, Description: description}
			if id, err := models.CreateProduct(common.Database, product); err == nil {
				if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
					p := path.Join(dir, "static", "products")
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
							// TODO: Update product with Thumbnail
							product.Thumbnail = "/" + path.Join("static", "products", filename)
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
	Offers int
}

// @security BasicAuth
// SearchCategories godoc
// @Summary Search products
// @Accept json
// @Produce json
// @Param category body ListRequest true "body"
// @Success 200 {object} ProductsListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/categories/list [post]
func postProductsListHandler(c *fiber.Ctx) error {
	var response ProductsListResponse
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
	var keys2 []string
	var values2 []interface{}
	if len(request.Filter) > 0 {
		for key, value := range request.Filter {
			if key != "" && len(strings.TrimSpace(value)) > 0 {
				switch key {
				case "Offers":
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
	logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
	//
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				case "Offers":
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
	rows, err := common.Database.Debug().Model(&models.Product{}).Select("products.ID, products.Name, products.Title, products.Thumbnail, products.Description, count(offers.ID) as Offers").Joins("left join offers on offers.product_id = products.id").Group("offers.product_id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item ProductsListItem
				if err = common.Database.ScanRows(rows, &item); err == nil {
					logger.Infof("item: %+v", item)
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
	rows, err = common.Database.Debug().Model(&models.Product{}).Select("products.ID, products.Name, products.Title, products.Thumbnail, products.Description, count(offers.ID) as Offers").Joins("left join offers on offers.product_id = products.id").Group("offers.product_id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	if len(keys1) > 0 || len(keys2) > 0 {
		common.Database.Debug().Model(&models.Product{}).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
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
				return c.JSON(fiber.Map{"ERROR": err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": err.Error()})
	}
}

// @security BasicAuth
// GetProductOffers godoc
// @Summary Get product offers
// @Accept json
// @Produce json
// @Param id path int true "Product ID"
// @Success 200 {object} OffersView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/products/{id}/offers [get]
func getProductOffersHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if offers, err := models.GetProductOffers(common.Database, id); err == nil {
		var view []*OfferView
		if bts, err := json.MarshalIndent(offers, "", "   "); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				return c.JSON(view)
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	}
	c.Status(http.StatusInternalServerError)
	return c.JSON(fiber.Map{"ERROR": "Something went wrong"})
}

type ProductOffersListResponse struct {
	Data []ProductOffersListItem
	Filtered int64
	Total int64
}

type ProductOffersListItem struct {
	ID uint
	Name string
	Title string
	Thumbnail string
	Description string
	BasePrice float64
	Properties int
}

// @security BasicAuth
// SearchProductOffers godoc
// @Summary Search product offers
// @Accept json
// @Produce json
// @Param id path int true "Product ID"
// @Param category body ListRequest true "body"
// @Success 200 {object} ProductOffersListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/products/{id}/offers/list [post]
func postProductOffersListHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var response ProductOffersListResponse
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
	var keys2 []string
	var values2 []interface{}
	if len(request.Filter) > 0 {
		for key, value := range request.Filter {
			if key != "" && len(strings.TrimSpace(value)) > 0 {
				switch key {
				case "Properties":
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
					keys1 = append(keys1, fmt.Sprintf("offers.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	keys1 = append(keys1, "product_id = ?")
	values1 = append(values1, id)
	logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
	//
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				case "Properties":
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
	rows, err := common.Database.Debug().Model(&models.Offer{}).Select("offers.ID, offers.Name, offers.Title, offers.Thumbnail, offers.Description, offers.Base_Price, count(properties.ID) as Properties").Joins("left join properties on properties.offer_id = offers.id").Group("properties.offer_id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item ProductOffersListItem
				if err = common.Database.ScanRows(rows, &item); err == nil {
					logger.Infof("item: %+v", item)
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
	rows, err = common.Database.Debug().Model(&models.Offer{}).Select("offers.ID, offers.Name, offers.Title, offers.Thumbnail, offers.Description, offers.Base_Price, count(properties.ID) as Properties").Joins("left join properties on properties.offer_id = offers.id").Group("properties.offer_id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	if len(keys1) > 0 || len(keys2) > 0 {
		common.Database.Debug().Model(&models.Offer{}).Where("product_id = ?", id).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

type OptionValuesListResponse struct {
	Data []OptionValuesListItem
	Filtered int64
	Total int64
}

type OptionValuesListItem struct {
	ID uint
	Title string
	Thumbnail string
	Value string
}

// @security BasicAuth
// SearchOptionValues godoc
// @Summary Search option values
// @Accept json
// @Produce json
// @Param id path int true "Option ID"
// @Param category body ListRequest true "body"
// @Success 200 {object} OptionValuesListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/options/{id}/values/list [post]
func postOptionValuesListHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var response OptionValuesListResponse
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
				keys1 = append(keys1, fmt.Sprintf("%v like ?", key))
				values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
			}
		}
	}
	keys1 = append(keys1, "option_id = ?")
	values1 = append(values1, id)
	logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
	//
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				case "Properties":
					orders = append(orders, fmt.Sprintf("%v %v", key, value))
				default:
					orders = append(orders, fmt.Sprintf("%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	logger.Infof("order: %+v", order)
	//
	var values []*models.Value
	if err := common.Database.Debug().Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Find(&values).Error; err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": err})
	}
	for _, value := range values {
		logger.Infof("item: %+v", value)
		response.Data = append(response.Data, OptionValuesListItem{
			ID:        value.ID,
			Title:     value.Title,
			Thumbnail: value.Thumbnail,
			Value:     value.Value,
		})
	}

	common.Database.Debug().Model(&models.Value{}).Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Count(&response.Filtered)
	if len(keys1) > 0 {
		common.Database.Debug().Model(&models.Value{}).Where("option_id = ?", id).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

// @security BasicAuth
// GetProductOffer godoc
// @Summary Get product offer
// @Accept json
// @Produce json
// @Param pid path int true "Product ID"
// @Param id path int true "Offer ID"
// @Success 200 {object} OfferView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/products/{pid}/offers/{id} [get]
func getProductOfferHandler(c *fiber.Ctx) error {
	var pid int
	if v := c.Params("pid"); v != "" {
		pid, _ = strconv.Atoi(v)
		var err error
		if _, err = models.GetProduct(common.Database, pid); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Option ID is not defined"})
	}
	var offer *models.Offer
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
		var err error
		if offer, err = models.GetOffer(common.Database, id); err == nil {
			var view OfferView
			if bts, err := json.MarshalIndent(offer, "", "   "); err == nil {
				if err = json.Unmarshal(bts, &view); err == nil {
					return c.JSON(view)
				}else{
					c.Status(http.StatusInternalServerError)
					return c.JSON(fiber.Map{"ERROR": err.Error()})
				}
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": err.Error()})
			}
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Offer ID is not defined"})
	}
}

type NewProperty struct {
	Name string
	Title string
	OptionId uint
}

// @security BasicAuth
// CreateProperty godoc
// @Summary Create property
// @Accept json
// @Produce json
// @Param pid query int false "Product id"
// @Param id query int false "Offer id"
// @Param property body NewProperty true "body"
// @Success 200 {object} PropertyView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /products/{pid}/offers/{id}/properties [post]
func postProductOfferPropertyHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var err error
	var offer *models.Offer
	if offer, err = models.GetOffer(common.Database, id); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Property already define, edit existing"})
	}
	var view PropertyView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewProperty
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			logger.Infof("request: %+v", request)
			//
			if properties, err := models.GetPropertiesByOfferAndName(common.Database, id, request.Name); err == nil {
				if len(properties) > 0 {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{"Property already define, edit existing"})
				}
			}
			//
			property := &models.Property{
				Name: request.Name,
				Title: request.Title,
				OptionId: request.OptionId,
				OfferId: offer.ID,
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

type ProductOfferPropertiesListResponse struct {
	Data []ProductOfferPropertiesListItem
	Filtered int64
	Total int64
}

type ProductOfferPropertiesListItem struct {
	ID uint
	Name string
	Title string
	OptionId uint
	OptionTitle string
	Prices int
}

// @security BasicAuth
// SearchProductOffers godoc
// @Summary Search product offers
// @Accept json
// @Produce json
// @Param pid path int true "Product ID"
// @Param id path int true "Offer ID"
// @Param category body ListRequest true "body"
// @Success 200 {object} ProductOffersListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/products/{pid}/offers/{id}/properties/list [post]
func postProductOfferPropertiesListHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var response ProductOfferPropertiesListResponse
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
	var keys2 []string
	var values2 []interface{}
	if len(request.Filter) > 0 {
		for key, value := range request.Filter {
			if key != "" && len(strings.TrimSpace(value)) > 0 {
				switch key {
				case "Properties":
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
	keys1 = append(keys1, "offer_id = ?", "properties.deleted_at is NULL")
	values1 = append(values1, id)
	logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
	//
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				case "Offers":
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
	rows, err := common.Database.Debug().Model(&models.Property{}).Select("properties.ID, properties.Name, properties.Title, count(prices.ID) as Prices, options.ID as OptionId, options.Title as OptionTitle").Joins("left join prices on prices.property_id = properties.id").Joins("left join options on options.id = properties.option_id").Group("prices.property_id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item ProductOfferPropertiesListItem
				if err = common.Database.ScanRows(rows, &item); err == nil {
					logger.Infof("item: %+v", item)
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
	rows, err = common.Database.Debug().Model(&models.Property{}).Select("properties.ID, properties.Name, properties.Title, count(prices.ID) as Prices").Joins("left join prices on prices.property_id = properties.id").Group("prices.property_id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	if len(keys1) > 0 || len(keys2) > 0 {
		common.Database.Debug().Model(&models.Property{}).Where("offer_id = ?", id).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

type PropertyView struct {
	ID uint
	Name string
	Title string
	OptionId uint
}

// @security BasicAuth
// GetProduct godoc
// @Summary Get property
// @Accept json
// @Produce json
// @Param pid path int true "Product ID"
// @Param oid path int true "Offer ID"
// @Param id path int true "Property ID"
// @Success 200 {object} PropertyView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/products/{pid}/offers/{oid}/properties/{id} [get]
func getProductOfferPropertyHandler(c *fiber.Ctx) error {
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
				return c.JSON(fiber.Map{"ERROR": err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": err.Error()})
	}
}

// @security BasicAuth
// UpdatePrice godoc
// @Summary Update price
// @Accept json
// @Produce json
// @Param pid query int true "Product id"
// @Param oid query int true "Offer id"
// @Param rid query int true "Property id"
// @Param id path int true "Price ID"
// @Param category body NewPrice true "body"
// @Success 200 {object} PriceView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/products/{pid}/offers/{oid}/properties/{id} [put]
func putProductOfferPropertyHandler(c *fiber.Ctx) error {
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
			property.Title = request.Title
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
// @Param pid query int true "Product id"
// @Param oid query int true "Offer id"
// @Param id query int true "Property id"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /products/{pid}/offers/{oid}/properties/{id} [delete]
func deleteProductOfferPropertyHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if property, err := models.GetProperty(common.Database, id); err == nil {
		if err = models.DeleteProperty(common.Database, property); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": err.Error()})
	}
	c.Status(http.StatusOK)
	return c.JSON(HTTPMessage{"OK"})
}

type ProductOfferPropertyPricesListResponse struct {
	Data []ProductOfferPropertyPricesListItem
	Filtered int64
	Total int64
}

type ProductOfferPropertyPricesListItem struct {
	ID         uint
	Enabled    bool
	ValueId    uint
	ValueTitle string
	Price      float64
}

// @security BasicAuth
// SearchProductOfferPropertyValues godoc
// @Summary Search product offer property prices
// @Accept json
// @Produce json
// @Param pid path int true "Product ID"
// @Param oid path int true "Offer ID"
// @Param id path int true "Property ID"
// @Param category body ListRequest true "body"
// @Success 200 {object} ProductOfferPropertyPricesListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/products/{pid}/offers/{oid}/properties/{id}/prices/list [post]
func postProductOfferPropertyPricesListHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var response ProductOfferPropertyPricesListResponse
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
	var keys2 []string
	var values2 []interface{}
	if len(request.Filter) > 0 {
		for key, value := range request.Filter {
			if key != "" && len(strings.TrimSpace(value)) > 0 {
				switch key {
				case "Properties":
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
	keys1 = append(keys1, "property_id = ?", "prices.deleted_at is NULL")
	values1 = append(values1, id)
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
				case "Offers":
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
	rows, err := common.Database.Debug().Model(&models.Price{}).Select("prices.ID, prices.Enabled, prices.Price, `values`.ID as ValueId, `values`.Title as ValueTitle").Joins("left join `values` on `values`.id = prices.value_id").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item ProductOfferPropertyPricesListItem
				if err = common.Database.ScanRows(rows, &item); err == nil {
					logger.Infof("item: %+v", item)
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
	rows, err = common.Database.Debug().Model(&models.Price{}).Select("prices.ID, prices.Enabled, prices.Price, `values`.ID as ValueId, `values`.Title as ValueTitle").Joins("left join `values` on `values`.id = prices.value_id").Where(strings.Join(keys1, " and "), values1...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	if len(keys1) > 0 || len(keys2) > 0 {
		common.Database.Debug().Model(&models.Price{}).Where("property_id = ?", id).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

type NewPrice struct {
	Enabled bool
	PropertyId uint
	ValueId uint
	Price float64
}

type PriceView struct {
	ID uint
	Enabled bool
	PropertyId uint
	ValueId uint
	Price float64
}

// @security BasicAuth
// CreateCategory godoc
// @Summary Create categories
// @Accept json
// @Produce json
// @Param pid query int false "Product id"
// @Param oid query int false "Offer id"
// @Param id query int false "Property id"
// @Param category body NewPrice true "body"
// @Success 200 {object} PriceView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /products/{pid}/offers/{oid}/properties/{id}/prices [post]
func postProductOfferPropertyPriceHandler(c *fiber.Ctx) error {
	var view PriceView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewPrice
			if err := c.BodyParser(&request); err != nil {
				return err
			}
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
				PropertyId: request.PropertyId,
				ValueId: request.ValueId,
				Price: request.Price,
			}
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
// @Param pid path int true "Product ID"
// @Param oid path int true "Offer ID"
// @Param id path int true "Property ID"
// @Success 200 {object} PriceView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/products/{pid}/offers/{oid}/properties/{id}/prices [get]
func getProductOfferPropertyPricesHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if prices, err := models.GetPricesByProperty(common.Database, uint(id)); err == nil {
		var view PricesView
		if bts, err := json.MarshalIndent(prices, "", "   "); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				return c.JSON(view)
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": err.Error()})
	}
}

// @security BasicAuth
// GetPrices godoc
// @Summary Get prices
// @Accept json
// @Produce json
// @Param pid path int true "Product ID"
// @Param oid path int true "Offer ID"
// @Param rid path int true "Property ID"
// @Param id path int true "Price ID"
// @Success 200 {object} PricesView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/products/{pid}/offers/{oid}/properties/{id}/prices [get]
func getProductOfferPropertyPriceHandler(c *fiber.Ctx) error {
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
				return c.JSON(fiber.Map{"ERROR": err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": err.Error()})
	}
}

// @security BasicAuth
// UpdatePrice godoc
// @Summary Update price
// @Accept json
// @Produce json
// @Param pid query int true "Product id"
// @Param oid query int true "Offer id"
// @Param rid query int true "Property id"
// @Param id path int true "Price ID"
// @Param category body NewPrice true "body"
// @Success 200 {object} PriceView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/products/{pid}/offers/{oid}/properties/{rid}/prices/{id} [put]
func putProductOfferPropertyPriceHandler(c *fiber.Ctx) error {
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
// @Param pid query int true "Product id"
// @Param oid query int true "Offer id"
// @Param rid query int true "Property id"
// @Param id path int true "Price ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /products/{pid}/offers/{oid}/properties/{rid}/prices/{id} [delete]
func deleteProductOfferPropertyPriceHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if price, err := models.GetPrice(common.Database, id); err == nil {
		if err = models.DeletePrice(common.Database, price); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": err.Error()})
	}
	c.Status(http.StatusOK)
	return c.JSON(HTTPMessage{"OK"})
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
	Values []*NewValue
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
func postOptionsHandler(c *fiber.Ctx) error {
	var request NewOption
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	request.Title = strings.TrimSpace(request.Title)
	if request.Title == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Title is not defined"})
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" {
		request.Name = reNotAbc.ReplaceAllString(strings.ToLower(request.Title), "-")
	}
	option := &models.Option{
		Name: request.Name,
		Title: request.Title,
	}
	if len(request.Values) > 0 {
		for _, v := range request.Values {
			v.Title = strings.TrimSpace(v.Title)
			v.Value = strings.TrimSpace(v.Value)
			if v.Title != "" && v.Value != "" {
				option.Values = append(option.Values, &models.Value{Title: v.Title, Thumbnail: v.Thumbnail, Value: v.Value})
			}
		}
	}
	if id, err := models.CreateOption(common.Database, option); err == nil {
		if option, err := models.GetOption(common.Database, int(id)); err == nil {
			return c.JSON(OptionShortView{ID: option.ID, Name: option.Name, Title: option.Title})
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": err.Error()})
	}
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
	Values int
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
		request.Sort["ID"] = "asc"
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
	rows, err := common.Database.Debug().Model(&models.Option{}).Select("options.ID, options.Name, options.Title, options.Description, count(`values`.ID) as `Values`").Joins("left join `values` on `values`.option_id = options.id").Group("`values`.option_id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item OptionsListItem
				if err = common.Database.ScanRows(rows, &item); err == nil {
					logger.Infof("item: %+v", item)
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
	rows, err = common.Database.Debug().Model(&models.Option{}).Select("options.ID, options.Name, options.Title, options.Description, count(`values`.ID) as `Values`").Joins("left join `values` on `values`.option_id = options.id").Group("`values`.option_id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Rows()
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

// @security BasicAuth
// GetOptions godoc
// @Summary Get options
// @Accept json
// @Produce json
// @Success 200 {object} OptionsShortView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/options [get]
func getOptionsHandler(c *fiber.Ctx) error {
	if options, err := models.GetOptions(common.Database); err == nil {
		var view OptionsShortView
		if bts, err := json.MarshalIndent(options, "", "   "); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				return c.JSON(view)
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": err.Error()})
	}
}

type OptionView struct {
	ID uint
	Name string `json:",omitempty"`
	Title string `json:",omitempty"`
	Description string `json:",omitempty"`
	Values []ValueView
}

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
				return c.JSON(fiber.Map{"ERROR": err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": err.Error()})
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
		return c.JSON(fiber.Map{"ERROR": err})
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
		return c.JSON(fiber.Map{"ERROR": err.Error()})
	}
}

// @security BasicAuth
// GetOptionValues godoc
// @Summary get option values
// @Accept json
// @Produce json
// @Param id path int true "Option ID"
// @Success 200 {object} OptionValuesView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/options/{id}/values [get]
func getOptionValuesHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if values, err := models.GetValuesByOptionId(common.Database, id); err == nil {
		var view []*ValueView
		if bts, err := json.MarshalIndent(values, "", "   "); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				return c.JSON(view)
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
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
// @Param id path int true "Option ID"
// @Param option body NewValue true "body"
// @Success 200 {object} ValueView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/options/{id}/values [post]
func postOptionValuesHandler(c *fiber.Ctx) error {
	var option *models.Option
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
		var err error
		if option, err = models.GetOption(common.Database, id); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
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
					p := path.Join(dir, "static", "values")
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
							value.Thumbnail = "/" + path.Join("static", "values", filename)
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
// @Param oid path int true "Option ID"
// @Param id path int true "Value ID"
// @Success 200 {object} ValueView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/options/{oid}/values/{id} [get]
func getOptionValueHandler(c *fiber.Ctx) error {
	var oid int
	if v := c.Params("oid"); v != "" {
		oid, _ = strconv.Atoi(v)
		var err error
		if _, err = models.GetOption(common.Database, oid); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Option ID is not defined"})
	}
	var value *models.Value
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
		var err error
		if value, err = models.GetValue(common.Database, id); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
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
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": err.Error()})
	}
}

// @security BasicAuth
// UpdateOptionValue godoc
// @Summary update option value
// @Accept json
// @Produce json
// @Param option body ValueView true "body"
// @Param oid path int true "Option ID"
// @Param id path int true "Value ID"
// @Success 200 {object} ValueView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/options/{oid}/values/{id} [put]
func putOptionValueHandler(c *fiber.Ctx) error {
	var oid int
	if v := c.Params("oid"); v != "" {
		oid, _ = strconv.Atoi(v)
		var err error
		if _, err = models.GetOption(common.Database, oid); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Option ID is not defined"})
	}
	var value *models.Value
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
		var err error
		if value, err = models.GetValue(common.Database, id); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
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
			if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
				p := path.Join(dir, "static", "values")
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
						value.Thumbnail = "/" + path.Join("static", "values", filename)
						if err = models.UpdateValue(common.Database, value); err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
					}
				}
			}else{
				if value.Thumbnail != "" {
					if err = os.Remove(path.Join(dir, value.Thumbnail)); err != nil {
						logger.Errorf("%v", err)
					}
					value.Thumbnail = ""
				}
			}
			//
			if err := models.UpdateValue(common.Database, value); err == nil {
				return c.JSON(ValueView{ID: value.ID, Title: value.Title, Thumbnail: value.Thumbnail, Value: value.Value})
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": err.Error()})
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
// @Param oid path int true "Option ID"
// @Param id path int true "Value ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/options/{oid}/values/{id} [delete]
func delOptionValueHandler(c *fiber.Ctx) error {
	var oid int
	if v := c.Params("oid"); v != "" {
		oid, _ = strconv.Atoi(v)
		var err error
		if _, err = models.GetOption(common.Database, oid); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Option ID is not defined"})
	}
	var value *models.Value
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
		var err error
		if value, err = models.GetValue(common.Database, id); err == nil {
			//
			if value.Thumbnail != "" {
				p := path.Join(dir, value.Thumbnail)
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
				return c.JSON(fiber.Map{"ERROR": err.Error()})
			}
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
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
		if err = models.DeleteOption(common.Database, option); err == nil {
			return c.JSON(HTTPMessage{MESSAGE: "OK"})
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": err.Error()})
	}
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
				return c.JSON(fiber.Map{"ERROR": err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": err.Error()})
	}
}

type ProfileView struct {
	User UserView
}

// @security BasicAuth
// @Summary Get profile
// @Description get string
// @Accept json
// @Produce json
// @Success 200 {object} ProfileView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /profile [get]
func getProfileHandler(c *fiber.Ctx) error {
	if v := c.Locals("user"); v != nil {
		if user, ok := v.(*models.User); ok {
			var view UserView
			if bts, err := json.Marshal(user); err == nil {
				if err = json.Unmarshal(bts, &view); err == nil {
					return c.JSON(view)
				}else{
					c.Status(http.StatusInternalServerError)
					return c.JSON(fiber.Map{"ERROR": err.Error()})
				}
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": err.Error()})
			}
		}
	}
	return c.JSON(HTTPError{"Something went wrong"})
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
		offerId := arr[1]
		var offer *models.Offer
		if offer, err = models.GetOffer(common.Database, offerId); err != nil {
			return err
		}
		if product.ID != offer.ProductId {
			return fmt.Errorf("Product and Offer mismatch")
		}
		item := CartItem{
			UUID: uuid,
			Title: product.Title + " " + offer.Title,
			Price: offer.BasePrice,
		}
		if product.Thumbnail != "" {
			item.Thumbnails = append(item.Thumbnails, product.Thumbnail)
		}
		if offer.Thumbnail != "" {
			item.Thumbnails = append(item.Thumbnails, offer.Thumbnail)
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
		logger.Infof("item: %+v", item)
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
	UUID string // ProductId,OfferId,
	Title string
	Thumbnails []string
	Price float64
	Properties []ItemProperty
	Quantity int
	//
	ProductId int
	OfferId int
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
					if len(product.Offers) > 0 {
						view.BasePrice = product.Offers[0].BasePrice
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
	Items []*ItemView
	Status string
	Total float64
}

type ItemView struct{
	ID uint
	Uuid string
	Title string
	Description string
	Path string
	Thumbnail string
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
// @Router /orders [get]
func getOrdersHandler(c *fiber.Ctx) error {
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
				return c.JSON(fiber.Map{"ERROR": err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": err.Error()})
	}
}

type NewOrder struct {
	Created time.Time
	Items []NewItem
}

type NewItem struct {
	UUID string
	CategoryId uint
	Quantity int
}

// CreateOrder godoc
// @Summary Post cart
// @Accept json
// @Produce json
// @Param cart body NewOrder true "body"
// @Success 200 {object} OrderView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /order [post]
func postOrdersHandler(c *fiber.Ctx) error {
	var request NewOrder
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	logger.Infof("request: %+v", request)
	order := &models.Order{
		Status: models.ORDER_STATUS_NEW,
	}
	if v := c.Locals("user"); v != nil {
		if user, ok := v.(*models.User); ok {
			order.User = user
		}
	}
	for _, item := range request.Items {
		var arr []int
		if err := json.Unmarshal([]byte(item.UUID), &arr); err == nil && len(arr) >= 2{
			productId := arr[0]
			var product *models.Product
			if product, err = models.GetProduct(common.Database, productId); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": err.Error()})
			}
			if len(product.Categories) > 0 {
				if breadcrumbs := models.GetBreadcrumbs(common.Database, product.Categories[0].ID); len(breadcrumbs) > 0 {

				}
			}
			offerId := arr[1]
			var offer *models.Offer
			if offer, err = models.GetOffer(common.Database, offerId); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": err.Error()})
			}
			if product.ID != offer.ProductId {
				err = fmt.Errorf("Product and Offer mismatch")
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": err.Error()})
			}

			categoryId := item.CategoryId

			item := &models.Item{
				Uuid: item.UUID,
				Title: product.Title + " " + offer.Title,
				Price: offer.BasePrice,
				Quantity: item.Quantity,
			}

			if breadcrumbs := models.GetBreadcrumbs(common.Database, categoryId); len(breadcrumbs) > 0 {
				logger.Infof("breadcrumbs: %+v", breadcrumbs)
				var chunks []string
				for _, crumb := range breadcrumbs {
					chunks = append(chunks, crumb.Name)
				}
				item.Path = "/" + path.Join(append(chunks, product.Name)...)
				logger.Infof("item.Path: %+v", item.Path)
			}

			var thumbnails []string

			if product.Thumbnail != "" {
				if item.Thumbnail == "" {
					item.Thumbnail = product.Thumbnail
				}
				thumbnails = append(thumbnails, product.Thumbnail)
			}

			if offer.Thumbnail != "" {
				if item.Thumbnail == "" {
					item.Thumbnail = offer.Thumbnail
				}
				thumbnails = append(thumbnails, offer.Thumbnail)
			}

			item.Thumbnails = strings.Join(thumbnails, ",")

			if len(arr) > 2 {
				var lines []string
				for _, id := range arr[2:] {
					if price, err := models.GetPrice(common.Database, id); err == nil {
						var line = fmt.Sprintf("%v: %v", price.Property.Title, price.Value.Value)
						if price.Price > 0 {
							line += fmt.Sprintf(" +$%.2f", price.Price)
						}
						lines = append(lines, line)
						item.Price += price.Price
					} else {
						c.Status(http.StatusInternalServerError)
						return c.JSON(fiber.Map{"ERROR": err.Error()})
					}
				}
				item.Description = strings.Join(lines, "\n")
			}

			order.Items = append(order.Items, item)
			order.Total += item.Price * float64(item.Quantity)
		}
	}
	logger.Infof("order: %+v", order)
	if _, err := models.CreateOrder(common.Database, order); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": err.Error()})
	}

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
// @Router /orders/{id} [get]
func getOrderHandler(c *fiber.Ctx) error {
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
				return c.JSON(fiber.Map{"ERROR": err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": err.Error()})
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
	Children []*CategoryView `json:",omitempty"`
	Parents []*CategoryView `json:",omitempty"`
}

func GetCategoriesView(connector *gorm.DB, id int, depth int) (*CategoryView, error) {
	if id == 0 {
		return getChildrenCategoriesView(connector, &CategoryView{Name: "root", Title: "Root"}, depth), nil
	} else {
		if category, err := models.GetCategory(connector, id); err == nil {
			view := getChildrenCategoriesView(connector, &CategoryView{ID: category.ID, Name: category.Name, Title: category.Title, Thumbnail: category.Thumbnail, Description: category.Description}, depth)
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
			child := getChildrenCategoriesView(connector, &CategoryView{ID: category.ID, Name: category.Name, Title: category.Title, Thumbnail: category.Thumbnail, Description: category.Description}, depth - 1)
			root.Children = append(root.Children, child)
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
	Description string `json:",omitempty"`
	Thumbnail string `json:",omitempty"`
	Offers []OfferView `json:",omitempty"`
	Images []ImageView `json:",omitempty"`
}

type OffersView []OfferView

type OfferView struct {
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
}

type ImageView struct {
	ID uint
	Path string `json:",omitempty"`
	Url string `json:",omitempty"`
	Width int `json:",omitempty"`
	Height int `json:",omitempty"`
	Size int `json:",omitempty"`
}

type HTTPMessage struct {
	MESSAGE  string
}

type HTTPError struct {
	ERROR  string
}