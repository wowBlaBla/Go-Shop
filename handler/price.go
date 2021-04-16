package handler

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/google/logger"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/models"
	"net/http"
	"strconv"
	"strings"
)

type NewPrice struct {
	Enabled bool
	ID uint
	ProductId uint
	VariationId uint
	//RateIds string
	Rates []*models.Rate // ID matter
	Price float64
	Availability string
	Sending string
	Sku string
}

type PriceView struct {
	ID uint
	Enabled bool
	ProductId uint
	VariationId uint
	//RateIds string
	Rates []*RateView
	Price float64
	Availability string
	Sending string
	Sku string
}

// @security BasicAuth
// CreatePrice godoc
// @Summary Create prices
// @Accept json
// @Produce json
// @Param product_id query int true "Product id"
// @Param variation_id query int true "Variation id"
// @Param price body NewPrice true "body"
// @Success 200 {object} PriceView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/prices [post]
// @Tags price
func postPriceHandler(c *fiber.Ctx) error {
	var view PriceView
	/*var pid int
	if v := c.Query("property_id"); v != "" {
		pid, _ = strconv.Atoi(v)
	}
	var vid int
	if v := c.Query("variation_id"); v != "" {
		vid, _ = strconv.Atoi(v)
	}
	var property *models.Property
	var err error
	if property, err = models.GetProperty(common.Database, pid); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}*/
	//
	var request NewPrice
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	//
	/*if prices, err := models.GetPricesByPropertyAndValue(common.Database, request.PropertyId, request.ValueId); err == nil {
		if len(prices) > 0 {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Price already define, edit existing"})
		}
	}*/
	//
	price := &models.Price{
		Enabled: request.Enabled,
		ProductId: request.ProductId,
		VariationId: request.VariationId,
		Rates: request.Rates,
		Price: request.Price,
		Availability: request.Availability,
		Sending: request.Sending,
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
}

type PricesView []*PriceView

// @security BasicAuth
// GetPrices godoc
// @Summary Get prices
// @Accept json
// @Produce json
// @Param product_id path int true "Product ID"
// @Param variation_id path int true "Variation ID"
// @Success 200 {object} PriceView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/prices [get]
// @Tags price
func getPricesHandler(c *fiber.Ctx) error {
	var pid int
	if v := c.Query("product_id"); v != "" {
		pid, _ = strconv.Atoi(v)
	}
	var vid int
	if v := c.Query("variation_id"); v != "" {
		vid, _ = strconv.Atoi(v)
	}
	var prices []*models.Price
	var err error
	if pid > 0 {
		if prices, err = models.GetPricesByProductId(common.Database, uint(pid)); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else if vid > 0 {
		if prices, err = models.GetPricesByVariationId(common.Database, uint(vid)); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Either product_id or variation_id required"})
	}
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
}

// @security BasicAuth
// GetPrice godoc
// @Summary Get price
// @Accept json
// @Produce json
// @Param id path int true "Price ID"
// @Success 200 {object} PriceView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/prices/{id} [get]
// @Tags price
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
// @Tags price
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
			price.Availability = request.Availability
			price.Sending = request.Sending
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
// @Tags price
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