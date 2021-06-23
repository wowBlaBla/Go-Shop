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

type NewProperty struct {
	Type      string
	Size string
	Name      string
	Title     string
	OptionId  uint
	Sku       string
	Filtering bool
	Rates     []NewRate
	Stock 	  uint
}

// @security BasicAuth
// CreateProperty godoc
// @Description Create property binding to product or variation
// @Summary Create property
// @Accept json
// @Produce json
// @Param product_id query int false "Product Id"
// @Param variation_id query int false "Variation Id"
// @Param property body NewProperty true "body"
// @Success 200 {object} PropertyView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /properties [post]
// @Tags property
func postPropertyHandler(c *fiber.Ctx) error {
	var view PropertyView
	var request NewProperty
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	//
	property := &models.Property{
		Type: request.Type,
		Size: request.Size,
		Name:        request.Name,
		Title:       request.Title,
		OptionId:    request.OptionId,
		Sku: request.Sku,
		Stock: request.Stock,
		Filtering: request.Filtering,
	}
	if v := c.Query("product_id"); v != "" {
		if id, err := strconv.Atoi(v); err == nil {
			if product, err := models.GetProduct(common.Database, id); err == nil {
				if properties, err := models.GetPropertiesByProductAndName(common.Database, id, request.Name); err == nil {
					if len(properties) > 0 {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{"Option already define"})
					}
				}
				property.ProductId = product.ID
			} else {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Product not found"})
			}
		}
	}else if v := c.Query("variation_id"); v != "" {
		if id, err := strconv.Atoi(v); err == nil {
			if variation, err := models.GetVariation(common.Database, id); err == nil {
				if properties, err := models.GetPropertiesByVariationAndName(common.Database, id, request.Name); err == nil {
					if len(properties) > 0 {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{"Option already defined"})
					}
				}
				property.VariationId = variation.ID
			} else {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Variation not found"})
			}
		}
	}
	// logger.Infof("property: %+v", request)
	for _, p := range request.Rates {
		if value, err := models.GetValue(common.Database, int(p.ValueId)); err == nil {
			property.Rates = append(property.Rates, &models.Rate{
				Enabled: true,
				Availability: p.Availability,
				Sku: p.Sku,
				Stock: p.Stock,
				Price:   p.Price,
				Value:   value,
			})
		}
	}
	logger.Infof("property.Rates: %+v", property.Rates)
	//
	if _, err := models.CreateProperty(common.Database, property); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	if bts, err := json.Marshal(property); err == nil {
		if err = json.Unmarshal(bts, &view); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
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
// SearchProperties godoc
// @Summary Search properties
// @Accept json
// @Produce json
// @Param variation_id path int false "Variation ID"
// @Param request body ListRequest true "body"
// @Success 200 {object} VariationsListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/properties/list [post]
// @Tags property
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
	//logger.Infof("request: %+v", request)
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
	//logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
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
	//logger.Infof("order: %+v", order)
	//
	rows, err := common.Database.Debug().Model(&models.Property{}).Select("properties.ID, properties.Name, properties.Title, products.Id as ProductId, products.Title as ProductTitle, variations.Id as VariationId, variations.Title as VariationTitle, replace(group_concat(prices.ID), ',', ', ') as PricesIds, replace(group_concat(`values`.Value), ',', ', ') as ValuesValues, replace(group_concat(prices.Rate), ',', ', ') as PricesPrices, options.ID as OptionId, options.Title as OptionTitle").Joins("left join prices on prices.property_id = properties.id").Joins("left join options on options.id = properties.option_id").Joins("left join `values` on `values`.id = prices.value_id").Joins("left join variations on variations.id = properties.variation_id").Joins("left join products on products.id = variations.product_id").Group("prices.property_id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item PropertiesListItem
				if err = common.Database.ScanRows(rows, &item); err == nil {
					item.ValuesValues = strings.TrimRight(item.ValuesValues, ", ")
					item.PricesIds = strings.TrimRight(item.PricesIds, ", ")
					item.PricesPrices = strings.TrimRight(item.PricesPrices, ", ")
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
	rows, err = common.Database.Debug().Model(&models.Property{}).Select("properties.ID, properties.Name, properties.Title, products.Id as ProductId, products.Title as ProductTitle, variations.Id as VariationId, variations.Title as VariationTitle, count(prices.ID) as Rates").Joins("left join prices on prices.property_id = properties.id").Joins("left join variations on variations.id = properties.variation_id").Joins("left join products on products.id = variations.product_id").Group("prices.property_id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Rows()
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
	Size string
	Name string
	Title string
	OptionId uint
	Sku string
	Filtering bool
	Stock uint
}

// @security BasicAuth
// GetProperty godoc
// @Summary Get property
// @Accept json
// @Produce json
// @Param id path int true "Property ID"
// @Success 200 {object} PropertyView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/properties/{id} [get]
// @Tags property
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
// UpdateProperty godoc
// @Summary Update property
// @Accept json
// @Produce json
// @Param id path int true "Property ID"
// @Param category body NewProperty true "body"
// @Success 200 {object} PropertyView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/properties/{id} [put]
// @Tags property
func putPropertyHandler(c *fiber.Ctx) error {
	var view PropertyView
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
			property.Size = request.Size
			property.Title = request.Title
			property.Sku = request.Sku
			property.Filtering = request.Filtering
			property.Stock = request.Stock
			// Update prices
			for _, p := range request.Rates {
				if price, err := models.GetRate(common.Database, int(p.ID)); err == nil {
					price.Availability = p.Availability
					price.Sku = p.Sku
					price.Price = p.Price
					price.Stock = p.Stock
					if err = models.UpdateRate(common.Database, price); err != nil {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
				}
			}
			//
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
// DelProperty godoc
// @Summary Delete property
// @Accept json
// @Produce json
// @Param id query int true "Property id"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/properties/{id} [delete]
// @Tags property
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