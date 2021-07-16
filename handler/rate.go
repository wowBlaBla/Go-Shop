package handler

import (
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/google/logger"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/models"
	"net/http"
	"strconv"
	"strings"
)

type RatesListResponse struct {
	Data []RatesListItem
	Filtered int64
	Total int64
}

type RatesListItem struct {
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
// SearchRates godoc
// @Summary Search rates
// @Accept json
// @Produce json
// @Param request body ListRequest true "body"
// @Success 200 {object} RatesListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/rates/list [post]
// @Tags price
func postRatesListHandler(c *fiber.Ctx) error {
	var response RatesListResponse
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
				case "Rate":
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
					keys1 = append(keys1, fmt.Sprintf("rates.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	/*keys1 = append(keys1, "property_id = ?", "rates.deleted_at is NULL")
	values1 = append(values1, id)*/
	//logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
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
					orders = append(orders, fmt.Sprintf("rates.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//logger.Infof("order: %+v", order)
	//
	rows, err := common.Database.Debug().Model(&models.Rate{}).Select("rates.ID, rates.Enabled, rates.Rate, products.Title as ProductTitle, variations.Title as VariationTitle, properties.ID as PropertyId, properties.Title as PropertyTitle, options.ID as OptionId, `values`.ID as ValueId, `values`.Title as ValueTitle").Joins("left join `values` on `values`.id = rates.value_id").Joins("left join options on options.id = `values`.option_id").Joins("left join properties on properties.ID = rates.Property_Id").Joins("left join variations on variations.ID = properties.Variation_Id").Joins("left join products on products.ID = variations.Product_Id").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item RatesListItem
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
	rows, err = common.Database.Debug().Model(&models.Rate{}).Select("rates.ID, rates.Enabled, rates.Rate, products.Title as ProductTitle, variations.Title as VariationTitle, properties.ID as PropertyId, properties.Title as PropertyTitle, options.ID as OptionId, `values`.ID as ValueId, `values`.Title as ValueTitle").Joins("left join `values` on `values`.id = rates.value_id").Joins("left join options on options.id = `values`.option_id").Joins("left join properties on properties.ID = rates.Property_Id").Joins("left join variations on variations.ID = properties.Variation_Id").Joins("left join products on products.ID = variations.Product_Id").Where(strings.Join(keys1, " and "), values1...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	/*if len(keys1) > 0 {
		common.Database.Preview().Model(&models.Rate{}).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}*/
	response.Total = response.Filtered
	c.Status(http.StatusOK)
	return c.JSON(response)
}

type NewRate struct {
	Enabled      bool
	ID           uint
	PropertyId   uint
	ValueId      uint
	Price        float64
	Availability string
	Sending      string
	Sku          string
	Stock        uint
}

type RateView struct {
	ID uint
	Enabled bool
	PropertyId uint
	Property PropertyView `json:",omitempty"`
	Prices []*PriceView `json:",omitempty"`
	Value ValueView `json:",omitempty"`
	ValueId uint
	Price float64
	Availability string
	Sending string
	Sku string
	Stock uint
}

// @security BasicAuth
// CreateRate godoc
// @Summary Create rates
// @Accept json
// @Produce json
// @Param property_id query int true "Property id"
// @Param price body NewRate true "body"
// @Success 200 {object} RateView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/rates [post]
// @Tags price
func postRateHandler(c *fiber.Ctx) error {
	var view RateView
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
			var request NewRate
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			//logger.Infof("request: %+v", request)
			//
			if rates, err := models.GetRatesByPropertyAndValue(common.Database, request.PropertyId, request.ValueId); err == nil {
				if len(rates) > 0 {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{"Rate already define, edit existing"})
				}
			}
			//
			price := &models.Rate{
				Enabled: request.Enabled,
				PropertyId: property.ID,
				ValueId: request.ValueId,
				Price: request.Price,
				Availability: request.Availability,
				Sending: request.Sending,
				Sku: request.Sku,
				Stock: request.Stock,
			}
			logger.Infof("price: %+v", price)
			//
			if _, err := models.CreateRate(common.Database, price); err != nil {
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

type RatesView []*RateView

// @security BasicAuth
// GetRates godoc
// @Summary Get rates
// @Accept json
// @Produce json
// @Param property_id path int true "Property ID"
// @Success 200 {object} RateView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/rates [get]
// @Tags price
func getRatesHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Query("property_id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if rates, err := models.GetRatesByProperty(common.Database, uint(id)); err == nil {
		var view RatesView
		if bts, err := json.MarshalIndent(rates, "", "   "); err == nil {
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
// GetRate godoc
// @Summary Get price
// @Accept json
// @Produce json
// @Param id path int true "Rate ID"
// @Success 200 {object} RateView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/rates/{id} [get]
// @Tags price
func getRateHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if price, err := models.GetRate(common.Database, id); err == nil {
		var view RateView
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
// UpdateRate godoc
// @Summary Update price
// @Accept json
// @Produce json
// @Param id path int true "Rate ID"
// @Param request body NewRate true "body"
// @Success 200 {object} RateView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/rates/{id} [put]
// @Tags price
func putRateHandler(c *fiber.Ctx) error {
	var view RateView
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var price *models.Rate
	var err error
	if price, err = models.GetRate(common.Database, id); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewRate
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			price.Price = request.Price
			price.Availability = request.Availability
			price.Sending = request.Sending
			price.Sku = request.Sku
			price.Stock = request.Stock
			if err = models.UpdateRate(common.Database, price); err != nil {
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
// DelRate godoc
// @Summary Delete price
// @Accept json
// @Produce json
// @Param id path int true "Rate ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/rates/{id} [delete]
// @Tags price
func deleteRateHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if rate, err := models.GetRate(common.Database, id); err == nil {
		if err = models.DeleteRate(common.Database, rate); err != nil {
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