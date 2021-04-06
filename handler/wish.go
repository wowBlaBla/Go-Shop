package handler

import (
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/google/logger"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/models"
	"net/http"
	"path"
	"strconv"
)

type WishesView[]WishView

type WishView struct {
	WishWrapperView
	WishItemView
}

// GetWishes godoc
// @Summary Get account wishes
// @Accept json
// @Produce json
// @Success 200 {object} WishesView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/wishes [get]
// @Tags account
// @Tags frontend
func getAccountWishesHandler(c *fiber.Ctx) error {
	var userId uint
	if v := c.Locals("user"); v != nil {
		if user, ok := v.(*models.User); ok {
			userId = user.ID
		}
	}
	if wishes, err := models.GetWishesByUserId(common.Database, userId); err == nil {
		var views []WishWrapperView
		if bts, err := json.Marshal(wishes); err == nil {
			if err = json.Unmarshal(bts, &views); err == nil {
				views2 := make([]WishView, len(views))
				for i := 0; i < len(views); i++ {
					views2[i].WishWrapperView = views[i]
					if err = json.Unmarshal([]byte(wishes[i].Description), &views2[i].WishItemView); err == nil {
						views2[i].Description = ""
					}else{
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
				}
				return c.JSON(views2)
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

type WithRequest struct {
	Uuid string
	CategoryId uint
}

type WishWrapperView struct {
	ID uint
	Uuid string
	CategoryId uint
	Path        string
	Name        string
	Title       string
	Description string `json:",omitempty"`
	Thumbnail   string
	Price       float64
}

type WishItemView struct {
	Variation VariationShortView   `json:",omitempty"`
	Properties []PropertyShortView `json:",omitempty"`
}

// @security BasicAuth
// CreateWish godoc
// @Summary Create account wish
// @Accept json
// @Produce json
// @Param option body WithRequest true "body"
// @Success 200 {object} WishWrapperView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/wishes [post]
// @Tags account
// @Tags frontend
func postAccountWishHandler(c *fiber.Ctx) error {
	var request WishWrapperView
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	if request.Uuid == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Uuid is not defined"})
	}
	//
	var arr []int
	if err := json.Unmarshal([]byte(request.Uuid), &arr); err == nil && len(arr) >= 2 {
		wish := &models.Wish{
			Uuid:     request.Uuid,
			CategoryId: request.CategoryId,
		}
		var item WishItemView
		//
		productId := arr[0]
		var product *models.Product
		if product, err = models.GetProduct(common.Database, productId); err != nil || !product.Enabled {
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": fmt.Sprintf("Product #%+v not exists", productId)})
		}
		wish.Name = product.Name
		wish.Title = product.Title
		variationId := arr[1]
		if variationId == 0 {
			wish.Price = product.BasePrice
		} else {
			if variation, err := models.GetVariation(common.Database, variationId); err == nil {
				if product.ID != variation.ProductId {
					c.Status(http.StatusInternalServerError)
					return c.JSON(fiber.Map{"ERROR": fmt.Sprintf("Product #%+v and Variation #%+v mismatch", productId, variationId)})
				}
				item.Variation = VariationShortView{
					Title: variation.Title,
				}
				wish.Price = variation.BasePrice
			} else {
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": fmt.Sprintf("Variation #%+v not exists", variationId)})
			}
		}
		var userId uint
		if v := c.Locals("user"); v != nil {
			if user, ok := v.(*models.User); ok {
				userId = user.ID
			}
		}
		wish.UserId = userId
		//
		if breadcrumbs := models.GetBreadcrumbs(common.Database, request.CategoryId); len(breadcrumbs) > 0 {
			var chunks []string
			for _, crumb := range breadcrumbs {
				chunks = append(chunks, crumb.Name)
			}
			wish.Path = "/" + path.Join(append(chunks, product.Name)...)
		}

		if cache, err := models.GetCacheProductByProductId(common.Database, product.ID); err == nil {
			wish.Thumbnail = cache.Thumbnail
		}else{
			logger.Warningf("%v", err.Error())
		}
		if cache, err := models.GetCacheVariationByVariationId(common.Database, uint(variationId)); err == nil {
			if wish.Thumbnail == "" {
				wish.Thumbnail = cache.Thumbnail
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
						propertyShortView.Price = price.Price
					}
					wish.Price += price.Price
					propertiesShortView = append(propertiesShortView, propertyShortView)
				} else {
					c.Status(http.StatusInternalServerError)
					return c.JSON(fiber.Map{"ERROR": err})
				}
			}
		}
		item.Properties = propertiesShortView
		if bts, err := json.Marshal(item); err == nil {
			wish.Description = string(bts)
		}
		if _, err := models.CreateWish(common.Database, wish); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(fiber.Map{"ERROR": err})
		}
		var wishView struct {
			ID uint
			WishWrapperView
			WishItemView
		}
		wishView.ID = wish.ID
		if bts, err := json.Marshal(wish); err == nil {
			if err = json.Unmarshal(bts, &wishView.WishWrapperView); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": err})
			}
		}
		wishView.Description = ""
		wishView.WishItemView = item
		c.Status(http.StatusOK)
		return c.JSON(wishView)
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Incorrect Uuid"})
	}
}

// @security BasicAuth
// GetWish godoc
// @Summary Get account wish
// @Accept json
// @Produce json
// @Param id path int true "Wish ID"
// @Success 200 {object} WishWrapperView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/wishes/{id} [get]
// @Tags account
// @Tags frontend
func getAccountWishHandler(c *fiber.Ctx) error {
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
	if wish, err := models.GetWish(common.Database, id); err == nil {
		if wish.UserId != userId {
			c.Status(http.StatusForbidden)
			return c.JSON(fiber.Map{"ERROR": "You are not allowed to do that"})
		}
		var view WishWrapperView
		if bts, err := json.Marshal(wish); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				var view2 WishView
				view2.WishWrapperView = view
				if err = json.Unmarshal([]byte(wish.Description), &view2.WishItemView); err == nil {
					logger.Infof("WishItemView: %+v", view2.WishItemView)
					view2.Description = ""
					return c.JSON(view2)
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

// @security BasicAuth
// DelWish godoc
// @Summary Delete account wish
// @Accept json
// @Produce json
// @Param id path int true "Wish ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/wishes/{id} [delete]
// @Tags account
// @Tags frontend
func deleteAccountWishHandler(c *fiber.Ctx) error {
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
	if wish, err := models.GetWish(common.Database, id); err == nil {
		if wish.UserId != userId {
			c.Status(http.StatusForbidden)
			return c.JSON(fiber.Map{"ERROR": "You are not allowed to do that"})
		}
		if err = models.DeleteWish(common.Database, wish); err == nil {
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