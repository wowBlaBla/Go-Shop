package handler

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/google/logger"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/models"
	"net/http"
	"strconv"
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
	var request NewPrice
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	//
	price := &models.Price{
		Enabled: request.Enabled,
		Price: request.Price,
		Availability: request.Availability,
		Sending: request.Sending,
		Sku: request.Sku,
	}
	if request.VariationId > 0 {
		price.VariationId = request.VariationId
	}else if request.ProductId > 0 {
		price.ProductId = request.ProductId
	}
	for _, rate := range request.Rates {
		if rate.ID > 0 {
			price.Rates = append(price.Rates, rate)
		}
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
}

type NewPrices []NewPrice

// @security BasicAuth
// CreatePrices godoc
// @Summary Create prices
// @Accept json
// @Produce json
// @Param price body NewPrices true "body"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/prices/all [post]
// @Tags price
func postPriceAllHandler(c *fiber.Ctx) error {
	var requests []NewPrice
	if err := c.BodyParser(&requests); err != nil {
		return err
	}
	//
	for _, request := range requests {
		logger.Infof("request: %+v", request)
		price := &models.Price{
			Enabled: request.Enabled,
			Price: request.Price,
			Availability: request.Availability,
			Sending: request.Sending,
			Sku: request.Sku,
		}
		if request.VariationId > 0 {
			price.VariationId = request.VariationId
		}else if request.ProductId > 0 {
			price.ProductId = request.ProductId
		}
		for _, rate := range request.Rates {
			if rate.ID > 0 {
				price.Rates = append(price.Rates, rate)
			}
		}
		//
		logger.Infof("price: %+v", price)
		//
		var id uint
		var err error
		if id, err = models.CreatePrice(common.Database, price); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
		logger.Infof("id: %+v, err: %+v", id, err)
	}
	return c.JSON(HTTPMessage{"OK"})
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