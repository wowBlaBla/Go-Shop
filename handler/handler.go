package handler

import (
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/session/v2"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/models"
	"net/http"
	"strconv"
	"time"
)

var (
	sessions = session.New(session.Config{
		Expiration: 24 * 30 * time.Hour,
	})
)


func GetFiber() *fiber.App {
	app, authMulti := CreateFiberAppWithAuthMultiple(nil)

	app.Get("/", authMulti, func(c *fiber.Ctx) error {
		m := fiber.Map{"Hello": "world"}
		if v := c.Locals("user"); v != nil {
			m["User"] = v.(*models.User)
		}
		return c.JSON(m)
	})

	api := app.Group("/api")
	v1 := api.Group("/v1")

	v1.Get("/info", authMulti, func(c *fiber.Ctx) error {
		response := fiber.Map{}
		response["Application"] = fmt.Sprintf("%v v%v %v", common.APPLICATION, common.VERSION, common.COMPILED)
		response["Started"] = common.Started
		if v := c.Locals("authorization"); v != nil {
			response["Authorization"] = v.(string)
		}
		if v := c.Locals("expiration"); v != nil {
			if expiration := v.(int64); expiration > 0 {
				response["ExpirationAt"] = time.Unix(expiration, 0).Format(time.RFC3339)
			}
		}
		if v := c.Locals("username"); v != nil {
			if user, err := models.GetUserByLogin(common.Database, v.(string)); err == nil {
				response["User"] = user
			}
		}
		return c.JSON(response)
	})

	v1.Get("/categories", func(c *fiber.Ctx) error {
		var id int
		if v := c.Query("id"); v != "" {
			id, _ = strconv.Atoi(v)
		}
		if view, err := models.GetCategoriesView(common.Database, id); err == nil {
			return c.JSON(view)
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	})

	v1.Get("/products/:id", func(c *fiber.Ctx) error {
		var id int
		if v := c.Params("id"); v != "" {
			id, _ = strconv.Atoi(v)
		}
		if product, err := models.GetProduct(common.Database, id); err == nil {
			return c.JSON(struct {
				ID uint
				Name string
				Title string
				Description string
				Thumbnail string
			}{
				ID: product.ID,
				Name: product.Name,
				Title: product.Title,
				Description: product.Description,
				Thumbnail: product.Thumbnail,
			})
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	})

	v1.Get("/products/:pid/offers/:id", func(c *fiber.Ctx) error {
		var id int
		if v := c.Params("id"); v != "" {
			id, _ = strconv.Atoi(v)
		}
		if offer, err := models.GetOffer(common.Database, id); err == nil {
			var view struct {
				ID uint
				Name string
				Title string
				Description string
				Thumbnail string
				Properties []struct {
					ID uint
					Option struct {
						ID uint
						Name string
						Title string
						Description string
					}
					Values []struct {
						ID uint
						Title string
						Thumbnail string
						Price float64
						Value string
					}
				}
				Price float64
			}
			if bts, err := json.Marshal(offer); err == nil {
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
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": "Something went wrong"})
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err.Error()})
		}
	})

	return app
}
