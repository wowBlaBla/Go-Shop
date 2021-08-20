package handler

import (
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/google/logger"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/models"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// @security BasicAuth
// SearchVariations godoc
// @Summary Search variations
// @Accept json
// @Produce json
// @Param id query int false "Products ID"
// @Param request body ListRequest true "body"
// @Success 200 {object} VariationsListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/variations/list [post]
// @Tags variation
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
		keys1 = append(keys1, "variations.product_id = ?")
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
				case "Options":
					orders = append(orders, fmt.Sprintf("%v %v", key, value))
				default:
					orders = append(orders, fmt.Sprintf("variations.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//logger.Infof("order: %+v", order)
	//
	rows, err := common.Database.Debug().Model(&models.Variation{}).Select("variations.ID, variations.Name, variations.Title, cache_variation.Thumbnail as Thumbnail, variations.Description, variations.Base_Price, variations.Stock, variations.Product_id as ProductId, products.Title as ProductTitle, cache_products.Thumbnail as ProductThumbnail, group_concat(properties.ID) as PropertiesIds, group_concat(properties.Title) as PropertiesTitles").Joins("left join cache_variation on variations.id = cache_variation.variation_id").Joins("left join products on products.id = variations.product_id").Joins("left join cache_products on variations.product_id = cache_products.product_id").Joins("left join properties on properties.variation_id = variations.id").Group("variations.id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item VariationsListItem
				if err = common.Database.ScanRows(rows, &item); err == nil {
					item.PropertiesIds = strings.TrimRight(item.PropertiesIds, ", ")
					item.PropertiesTitles = strings.TrimRight(item.PropertiesTitles, ", ")
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
	rows, err = common.Database.Debug().Model(&models.Variation{}).Select("variations.ID, variations.Name, variations.Title, variations.Thumbnail, variations.Description, variations.Base_Price, variations.Stock, variations.Product_id as ProductId, group_concat(properties.ID) as PropertiesIds, group_concat(properties.Title) as PropertiesTitles").Joins("left join properties on properties.variation_id = variations.id").Group("variations.id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Rows()
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
// GetVariations godoc
// @Summary Get variations
// @Accept json
// @Produce json
// @Param id path int false "Products ID"
// @Success 200 {object} VariationsView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/variations [get]
// @Tags variation
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
// @Query product_id query int true "Products id"
// @Param category body NewVariation true "body"
// @Success 200 {object} VariationView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/variations [post]
// @Tags variation
func postVariationHandler(c *fiber.Ctx) error {
	var view VariationView
	var id int
	if v := c.Query("product_id"); v != "" {
		id, _ = strconv.Atoi(v)
	}else if v := c.Query("pid"); v != "" {
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
			var enabled = true
			if v, found := data.Value["Enabled"]; found && len(v) > 0 {
				if vv, err := strconv.ParseBool(v[0]); err == nil {
					enabled = vv
				}
			}
			var name string
			if v, found := data.Value["Name"]; found && len(v) > 0 {
				name = strings.TrimSpace(v[0])
			}
			/*if name == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Invalid name"})
			}*/
			if variations, err := models.GetVariationsByProductAndName(common.Database, product.ID, name); err == nil && len(variations) > 0 {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Name is already in use"})
			}
			var title string
			if v, found := data.Value["Title"]; found && len(v) > 0 {
				title = strings.TrimSpace(v[0])
			}
			/*if title == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Invalid title"})
			}*/
			var description string
			if v, found := data.Value["Description"]; found && len(v) > 0 {
				description = strings.TrimSpace(v[0])
			}
			var notes string
			if v, found := data.Value["Notes"]; found && len(v) > 0 {
				notes = strings.TrimSpace(v[0])
			}
			var basePrice float64
			if v, found := data.Value["BasePrice"]; found && len(v) > 0 {
				if basePrice, err = strconv.ParseFloat(v[0], 10); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{"Invalid base price"})
				}
			}
			var manufacturerPrice float64
			if v, found := data.Value["ManufacturerPrice"]; found && len(v) > 0 {
				if manufacturerPrice, err = strconv.ParseFloat(v[0], 10); err != nil {
					logger.Warningf("%+v", err)
				}
			}
			var salePrice float64
			if v, found := data.Value["SalePrice"]; found && len(v) > 0 {
				salePrice, _ = strconv.ParseFloat(v[0], 10)
			}
			var itemPrice float64
			if v, found := data.Value["ItemPrice"]; found && len(v) > 0 {
				if itemPrice, err = strconv.ParseFloat(v[0], 10); err != nil {
					logger.Warningf("%+v", err)
				}
			}
			var minQuantity int
			if v, found := data.Value["MinQuantity"]; found && len(v) > 0 {
				minQuantity, _ = strconv.Atoi(v[0])
			}
			var maxQuantity int
			if v, found := data.Value["MaxQuantity"]; found && len(v) > 0 {
				maxQuantity, _ = strconv.Atoi(v[0])
			}
			var purchasableMultiply int
			if v, found := data.Value["PurchasableMultiply"]; found && len(v) > 0 {
				purchasableMultiply, _ = strconv.Atoi(v[0])
			}
			var pattern string
			if v, found := data.Value["Pattern"]; found && len(v) > 0 {
				pattern = strings.TrimSpace(v[0])
			}else if common.Config.Pattern != "" {
				pattern = common.Config.Pattern
			}else{
				pattern = "whd"
			}
			var dimensions string
			if v, found := data.Value["Dimensions"]; found && len(v) > 0 {
				dimensions = strings.TrimSpace(v[0])
			}
			var dimensionUnit string
			if v, found := data.Value["DimensionUnit"]; found && len(v) > 0 {
				dimensionUnit = strings.TrimSpace(v[0])
			}else if common.Config.DimensionUnit != "" {
				dimensionUnit = common.Config.DimensionUnit
			}else{
				dimensionUnit = "cm"
			}
			var width float64
			if v, found := data.Value["Width"]; found && len(v) > 0 {
				if vv, _ := strconv.ParseFloat(v[0], 10); err == nil {
					width = vv
				}
			}
			var height float64
			if v, found := data.Value["Height"]; found && len(v) > 0 {
				if vv, _ := strconv.ParseFloat(v[0], 10); err == nil {
					height = vv
				}
			}
			var depth float64
			if v, found := data.Value["Depth"]; found && len(v) > 0 {
				if vv, _ := strconv.ParseFloat(v[0], 10); err == nil {
					depth = vv
				}
			}
			var volume float64
			if v, found := data.Value["Volume"]; found && len(v) > 0 {
				if vv, err := strconv.ParseFloat(v[0], 10); err == nil {
					volume = vv
				}
			}
			var weight float64
			if v, found := data.Value["Weight"]; found && len(v) > 0 {
				if vv, err := strconv.ParseFloat(v[0], 10); err == nil {
					weight = vv
				}
			}
			var weightUnit string
			if v, found := data.Value["WeightUnit"]; found && len(v) > 0 {
				weightUnit = strings.TrimSpace(v[0])
			}else if common.Config.WeightUnit != "" {
				weightUnit = common.Config.WeightUnit
			}else{
				weightUnit = "kg"
			}
			var packages int
			if v, found := data.Value["Packages"]; found && len(v) > 0 {
				if vv, _ := strconv.Atoi(v[0]); err == nil {
					packages = vv
				}
			}
			var availability string
			if v, found := data.Value["Availability"]; found && len(v) > 0 {
				availability = strings.TrimSpace(v[0])
			}
			var timeId uint
			if v, found := data.Value["TimeId"]; found && len(v) > 0 {
				if vv, _ := strconv.Atoi(v[0]); err == nil {
					timeId = uint(vv)
				}
			}
			var sku string
			if v, found := data.Value["Sku"]; found && len(v) > 0 {
				sku = strings.TrimSpace(v[0])
			}

			var stock uint
			if v, found := data.Value["Stock"]; found && len(v) > 0 {
				if vv, _ := strconv.Atoi(v[0]); err == nil {
					stock = uint(vv)
				}
			}
			variation := &models.Variation{
				Enabled: enabled, Name: name, Title: title, Description: description, Notes: notes,
				BasePrice: basePrice, ManufacturerPrice: manufacturerPrice, SalePrice: salePrice, ItemPrice: itemPrice,
				MinQuantity: minQuantity, MaxQuantity: maxQuantity, PurchasableMultiply: purchasableMultiply,
				ProductId: product.ID, Pattern: pattern, Dimensions: dimensions, DimensionUnit: dimensionUnit,
				Width: width, Height: height, Depth: depth, Volume: volume, Weight: weight, WeightUnit: weightUnit,
				Packages: packages, Availability: availability, TimeId: timeId, Sku: sku, Stock: stock}

			if id, err := models.CreateVariation(common.Database, variation); err == nil {
				if name == "" {
					variation.Name = fmt.Sprintf("new-variation-%d", variation.ID)
					variation.Title = fmt.Sprintf("New Variation %d", variation.ID)
					if err = models.UpdateVariation(common.Database, variation); err != nil {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
				}
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
							variation.Thumbnail = "/" + path.Join("variations", filename)
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
// UpdateVariation godoc
// @Summary Update variation
// @Accept multipart/form-data
// @Produce json
// @Param id query int false "Variation id"
// @Param category body NewVariation true "body"
// @Success 200 {object} VariationView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/variations/{id} [put]
// @Tags variation
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
			var enabled = true
			if v, found := data.Value["Enabled"]; found && len(v) > 0 {
				if vv, err := strconv.ParseBool(v[0]); err == nil {
					enabled = vv
				}
			}
			var name string
			if v, found := data.Value["Name"]; found && len(v) > 0 {
				name = strings.TrimSpace(v[0])
			}
			if name == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Invalid title"})
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
			var notes string
			if v, found := data.Value["Notes"]; found && len(v) > 0 {
				notes = strings.TrimSpace(v[0])
			}
			var basePrice float64
			if v, found := data.Value["BasePrice"]; found && len(v) > 0 {
				if basePrice, err = strconv.ParseFloat(v[0], 10); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{"Invalid base price"})
				}
			}
			var manufacturerPrice float64
			if v, found := data.Value["ManufacturerPrice"]; found && len(v) > 0 {
				manufacturerPrice, _ = strconv.ParseFloat(v[0], 10)
			}
			var salePrice float64
			if v, found := data.Value["SalePrice"]; found && len(v) > 0 {
				salePrice, _ = strconv.ParseFloat(v[0], 10)
			}
			var start time.Time
			if v, found := data.Value["Start"]; found && len(v) > 0 {
				start, _ = time.Parse(time.RFC3339, v[0])
			}
			var end time.Time
			if v, found := data.Value["End"]; found && len(v) > 0 {
				end, _ = time.Parse(time.RFC3339, v[0])
			}
			var itemPrice float64
			if v, found := data.Value["ItemPrice"]; found && len(v) > 0 {
				itemPrice, _ = strconv.ParseFloat(v[0], 10)
			}
			var minQuantity int
			if v, found := data.Value["MinQuantity"]; found && len(v) > 0 {
				minQuantity, _ = strconv.Atoi(v[0])
			}
			var maxQuantity int
			if v, found := data.Value["MaxQuantity"]; found && len(v) > 0 {
				maxQuantity, _ = strconv.Atoi(v[0])
			}
			var purchasableMultiply int
			if v, found := data.Value["PurchasableMultiply"]; found && len(v) > 0 {
				purchasableMultiply, _ = strconv.Atoi(v[0])
			}
			// Properties
			if v, found := data.Value["Properties"]; found {
				var newProperties PropertiesView
				if err = json.Unmarshal([]byte(v[0]), &newProperties); err == nil {
					variation.Properties = []*models.Property{}
					variation.Prices = []*models.Price{}
					var existingProperties, updatedProperties []*models.Property
					if existingProperties, err = models.GetPropertiesByVariationId(common.Database, int(variation.ID)); err != nil {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
					var resized bool
					for _, existingProperty := range existingProperties{
						var found bool
						for i, newProperty := range newProperties {
							if existingProperty.ID == newProperty.ID {
								logger.Infof("Existing property #%+v", existingProperty.ID)
								existingProperty.Type = newProperty.Type
								existingProperty.Size = newProperty.Size
								existingProperty.Mode = newProperty.Mode
								existingProperty.Name = newProperty.Name
								existingProperty.Title = newProperty.Title
								existingProperty.Sku = newProperty.Sku
								existingProperty.Stock = newProperty.Stock
								existingProperty.Filtering = newProperty.Filtering

								// TODO: Check rates
								logger.Infof("Existing Rates: %+v", len(existingProperty.Rates))
								//for i, rate := range property.Rates {
								for i := 0; i < len(existingProperty.Rates); i++ {
									existingRate := existingProperty.Rates[i]
									var found bool
									for j := 0; j < len(newProperty.Rates); j++ {
										newRate := newProperty.Rates[j]
										if existingRate.ID == newRate.ID {
											existingRate.Enabled = newRate.Enabled
											existingRate.Price = newRate.Price
											existingRate.Availability = newRate.Availability
											existingRate.Sending = newRate.Sending
											existingRate.Sku = newRate.Sku
											existingRate.Stock = newRate.Stock
											if existingRate.Value.OptionId == 0 {
												if value, err := models.GetValue(common.Database, int(existingRate.ValueId)); err == nil {
													value.Title = newRate.Value.Title
													value.Description = newRate.Value.Description
													value.Color = newRate.Value.Color
													value.Value = newRate.Value.Value
													value.OptionId = newRate.Value.OptionId
													value.Availability = newRate.Value.Availability
													value.Sort = newRate.Value.Sort
													if value.Thumbnail != "" && newRate.Value.Thumbnail == "" {
														// TODO: To delete thumbnail
														logger.Infof("Existing thumbnail should be %v deleted", value.Thumbnail)
														value.Thumbnail = ""
														if err = models.DeleteCacheValueByValueId(common.Database, value.ID); err != nil {
															logger.Warningf("%+v", err)
														}
													}else if _, found := data.File[newRate.Value.Thumbnail]; newRate.Value.Thumbnail != "" && found {
														logger.Infof("New thumbnail will be loaded: %v", newRate.Value.Thumbnail)
														value.Thumbnail = newRate.Value.Thumbnail
													}
													logger.Infof("Updating value %+v", value)
													if err := models.UpdateValue(common.Database, value); err != nil {
														c.Status(http.StatusInternalServerError)
														return c.JSON(HTTPError{err.Error()})
													}
												}else{
													c.Status(http.StatusInternalServerError)
													return c.JSON(HTTPError{err.Error()})
												}
											}
											if err := models.UpdateRate(common.Database, existingRate); err != nil {
												c.Status(http.StatusInternalServerError)
												return c.JSON(HTTPError{err.Error()})
											}
											copy(newProperty.Rates[j:], newProperty.Rates[j+1:])
											newProperty.Rates[len(newProperty.Rates) - 1] = nil
											newProperty.Rates = newProperty.Rates[:len(newProperty.Rates)-1]
											j--
											found = true
											break
										}
									}
									if !found {
										copy(existingProperty.Rates[i:], existingProperty.Rates[i+1:])
										existingProperty.Rates[len(existingProperty.Rates)-1] = nil
										existingProperty.Rates = existingProperty.Rates[:len(existingProperty.Rates)-1]
										i--
										if err := models.DeleteRate(common.Database, existingRate); err != nil {
											c.Status(http.StatusInternalServerError)
											return c.JSON(HTTPError{err.Error()})
										}
									}
								}

								for _, newRate := range newProperty.Rates {
									// TODO: Add rate
									rate := &models.Rate{
										Enabled: newRate.Enabled,
									}
									if newRate.ValueId == 0 {
										rate.Value = &models.Value{
											Title: newRate.Value.Title,
											Thumbnail: newRate.Value.Thumbnail,
											Description: newRate.Value.Description,
											Color: newRate.Value.Color,
											Value: newRate.Value.Value,
											OptionId: newRate.Value.OptionId,
											Availability: newRate.Value.Availability,
											Sort: newRate.Value.Sort,
										}
									}else{
										rate.ValueId = newRate.ValueId
									}
									rate.Price = newRate.Price
									rate.Availability = newRate.Availability
									rate.Sending = newRate.Sending
									rate.Sku = newRate.Sku
									rate.Stock = newRate.Stock

									if _, err := models.CreateRate(common.Database, rate); err != nil {
										c.Status(http.StatusInternalServerError)
										return c.JSON(HTTPError{err.Error()})
									}

									existingProperty.Rates = append(existingProperty.Rates, rate)
								}

								logger.Infof("Updating property %+v", existingProperty)
								if err := models.UpdateProperty(common.Database, existingProperty); err != nil {
									c.Status(http.StatusInternalServerError)
									return c.JSON(HTTPError{err.Error()})
								}

								updatedProperties = append(updatedProperties, existingProperty)
								if i == len(newProperties) - 1 {
									newProperties = newProperties[:i]
								}else{
									newProperties = append(newProperties[:i], newProperties[i + 1:]...)
								}
								found = true
								break
							}
						}
						if !found {
							// TODO: Delete property
							logger.Infof("Delete property %+v", existingProperty)
							for _, rate := range existingProperty.Rates {
								if rate.Value.OptionId == 0 {
									if err = models.DeleteValue(common.Database, rate.Value); err != nil {
										logger.Warningf("%+v", err)
									}
								}
								if err = models.DeleteRate(common.Database, rate); err != nil {
									logger.Warningf("%+v", err)
								}
							}
							if err := models.DeleteProperty(common.Database, existingProperty); err != nil {
								c.Status(http.StatusInternalServerError)
								return c.JSON(HTTPError{err.Error()})
							}

							resized = true
						}
					}
					logger.Infof("New properties: %+v", len(newProperties))
					for _, newProperty := range newProperties {
						logger.Infof("New property: %+v", newProperty)
						property := &models.Property{
							Type: newProperty.Type,
							Size: newProperty.Size,
							Mode: newProperty.Mode,
							Name: newProperty.Name,
							Title: newProperty.Title,
							Sku: newProperty.Sku,
							Stock: newProperty.Stock,
							OptionId: newProperty.OptionId,
							Filtering: newProperty.Filtering,
						}

						property.VariationId = variation.ID

						// TODO: Add rates
						for _, newRate := range newProperty.Rates {
							// TODO: Add rate
							rate := &models.Rate{
								Enabled: newRate.Enabled,
							}
							if newRate.ValueId == 0 {
								rate.Value = &models.Value{
									Title: newRate.Value.Title,
									Thumbnail: newRate.Value.Thumbnail,
									Description: newRate.Value.Description,
									Color: newRate.Value.Color,
									Value: newRate.Value.Value,
									OptionId: newRate.Value.OptionId,
									Availability: newRate.Value.Availability,
									Sort: newRate.Value.Sort,
								}
							}else{
								rate.ValueId = newRate.ValueId
							}
							rate.Price = newRate.Price
							rate.Availability = newRate.Availability
							rate.Sending = newRate.Sending
							rate.Sku = newRate.Sku
							rate.Stock = newRate.Stock

							property.Rates = append(property.Rates, rate)
						}
						if _, err := models.CreateProperty(common.Database, property); err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
						updatedProperties = append(updatedProperties, property)
						resized = true
					}

					// Properties
					for _, property := range updatedProperties {
						for _, rate := range property.Rates {
							if value := rate.Value; value != nil {
								if thumbnail := value.Thumbnail; thumbnail != "" {
									if v, found := data.File[thumbnail]; found && len(v) > 0 {
										p := path.Join(dir, "storage", "values")
										if _, err := os.Stat(p); err != nil {
											if err = os.MkdirAll(p, 0755); err != nil {
												logger.Errorf("%v", err)
											}
										}
										filename := fmt.Sprintf("%d-%s-thumbnail%s", value.ID, regexp.MustCompile(`(?i)[^-a-z0-9]+`).ReplaceAllString(value.Title, "-"), path.Ext(v[0].Filename))
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
												value.Thumbnail = "/" + path.Join("values", filename)
												if err = models.UpdateValue(common.Database, value); err != nil {
													c.Status(http.StatusInternalServerError)
													return c.JSON(HTTPError{err.Error()})
												}
												//
												if p1 := path.Join(dir, "storage", "values", filename); len(p1) > 0 {
													if fi, err := os.Stat(p1); err == nil {
														filename := filepath.Base(p1)
														filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
														logger.Infof("Copy %v => %v %v bytes", p1, path.Join("images", "values", filename), fi.Size())
														var paths string
														if thumbnails, err := common.STORAGE.PutImage(p1, path.Join("images", "values", filename), common.Config.Resize.Thumbnail.Size); err == nil {
															paths = strings.Join(thumbnails, ",")
														} else {
															logger.Warningf("%v", err)
														}
														// Cache
														if err = models.DeleteCacheValueByValueId(common.Database, value.ID); err != nil {
															logger.Warningf("%v", err)
														}
														if _, err = models.CreateCacheValue(common.Database, &models.CacheValue{
															ValueID: value.ID,
															Title:     value.Title,
															Thumbnail: paths,
															Value: value.Value,
														}); err != nil {
															logger.Warningf("%v", err)
														}
													}
												}
											}
										}
									}
								}
							}
						}
					}

					// Prices
					if resized {
						logger.Infof("Resize %v", resized)
						variation.Prices = []*models.Price{}
						if prices, err := models.GetPricesByVariationId(common.Database, variation.ID); err == nil {
							for _, price := range prices {
								if err = models.DeletePrice(common.Database, price); err != nil {
									c.Status(http.StatusInternalServerError)
									return c.JSON(HTTPError{err.Error()})
								}
							}
						} else {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
					}
					logger.Infof("updatedProperties: %+v", updatedProperties)
					//
					var table [][]uint
					for i := 0; i < len(updatedProperties); i++ {
						logger.Infof("i: %d", i)
						var row []uint
						for j := 0; j < len(updatedProperties[i].Rates); j++ {
							logger.Infof("j: %d", j)
							row = append(row, updatedProperties[i].Rates[j].ID)
						}
						table = append(table, row)
					}
					logger.Infof("table: %+v", table)
					var matrix = [][]uint{}
					vector := make([]int, len(table))
					for counter := 0; ; counter++ {
						var line []uint
						for i, index := range vector {
							line = append(line, table[i][index])
						}
						var done bool
						for i := len(vector) - 1; i >= 0; i-- {
							if vector[i] < len(table[i]) - 1 {
								vector[i]++
								done = true
								break
							}else{
								vector[i] = 0
							}
						}
						matrix = append(matrix, line)
						if !done {
							break
						}
					}
					if len(matrix) > 250 {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{"Too many properties combinations, please reduce property values count"})
					}
					var existingPrices []*models.Price
					if existingPrices, err = models.GetPricesByVariationId(common.Database, variation.ID); err != nil {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
					logger.Infof("matrix: %+v", matrix)
					//
					for _, row := range matrix {
						logger.Infof("row: %+v", row)
						if len(row) > 0 {
							price := &models.Price{
								Enabled: true,
								VariationId: variation.ID,
								Availability: common.AVAILABILITY_AVAILABLE,
							}
							var ids = []uint{variation.ProductId, variation.ID}
							for _, rateId := range row {
								if rateId > 0 {
									if rate, err := models.GetRate(common.Database, int(rateId)); err == nil {
										ids = append(ids, rate.ID)
										price.Rates = append(price.Rates, rate)
									}
								}
							}

							var sku []string
							for _, id := range ids {
								sku = append(sku, fmt.Sprintf("%d", id))
							}
							price.Sku = strings.Join(sku, ".")

							var match bool
							for _, existingPrice := range existingPrices {
								match = len(price.Rates) == len(existingPrice.Rates)
								if match {
									for _, rate1 := range price.Rates {
										var found bool
										for _, rate2 := range existingPrice.Rates {
											if rate1.ID == rate2.ID {
												found = true
												break
											}
										}
										if !found {
											match = false
											break
										}
									}
								}

								if match {
									break
								}
							}

							if !match {
								if _, err = models.CreatePrice(common.Database, price); err != nil {
									c.Status(http.StatusInternalServerError)
									return c.JSON(HTTPError{err.Error()})
								}
								variation.Prices = append(variation.Prices, price)
							}
						}
					}
					variation.Properties = updatedProperties
				}else{
					logger.Warningf("%+v", err)
				}
			}
			logger.Infof("AFTER Properties: %+v", variation.Prices)
			// Prices
			if v, found := data.Value["Prices"]; found && len(v) > 0 {
				var newPrices []*PriceView
				if err = json.Unmarshal([]byte(v[0]), &newPrices); err == nil {
					variation.Prices = []*models.Price{}
					var existingPrices []*models.Price
					if existingPrices, err = models.GetPricesByVariationId(common.Database, variation.ID); err != nil {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
					for _, existingPrice := range existingPrices {
						var found bool
						for i, newPrice := range newPrices {
							if existingPrice.ID == newPrice.ID {
								existingPrice.BasePrice = newPrice.BasePrice
								existingPrice.SalePrice = newPrice.SalePrice
								existingPrice.Availability =  newPrice.Availability
								existingPrice.Sku =  newPrice.Sku
								existingPrice.Stock = newPrice.Stock
								if err := models.UpdatePrice(common.Database, existingPrice); err != nil {
									c.Status(http.StatusInternalServerError)
									return c.JSON(HTTPError{err.Error()})
								}
								variation.Prices = append(variation.Prices, existingPrice)
								if i == len(newPrices) - 1 {
									newPrices = newPrices[:i]
								}else{
									newPrices = append(newPrices[:i], newPrices[i + 1:]...)
								}
								found = true
								break
							}
						}
						if !found {
							if err := models.DeletePrice(common.Database, existingPrice); err != nil {
								c.Status(http.StatusInternalServerError)
								return c.JSON(HTTPError{err.Error()})
							}
						}
					}
					for _, newPrice := range newPrices {
						price := &models.Price{
							BasePrice: newPrice.BasePrice,
							SalePrice: newPrice.SalePrice,
							Availability: newPrice.Availability,
							Sku: newPrice.Sku,
							Stock:  newPrice.Stock,
							VariationId: variation.ID,
						}
						if _, err := models.CreatePrice(common.Database, price); err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
						variation.Prices = append(variation.Prices, price)
					}
				}else{
					logger.Warningf("%+v", err)
				}
			}
			var pattern string
			if v, found := data.Value["Pattern"]; found && len(v) > 0 {
				pattern = strings.TrimSpace(v[0])
			}
			var dimensions string
			if v, found := data.Value["Dimensions"]; found && len(v) > 0 {
				dimensions = strings.TrimSpace(v[0])
			}
			var dimensionUnit string
			if v, found := data.Value["DimensionUnit"]; found && len(v) > 0 {
				dimensionUnit = strings.TrimSpace(v[0])
			}else if common.Config.DimensionUnit != "" {
				dimensionUnit = common.Config.DimensionUnit
			}else{
				dimensionUnit = "cm"
			}
			var width float64
			if v, found := data.Value["Width"]; found && len(v) > 0 {
				if vv, _ := strconv.ParseFloat(v[0], 10); err == nil {
					width = vv
				}
			}
			var height float64
			if v, found := data.Value["Height"]; found && len(v) > 0 {
				if vv, _ := strconv.ParseFloat(v[0], 10); err == nil {
					height = vv
				}
			}
			var depth float64
			if v, found := data.Value["Depth"]; found && len(v) > 0 {
				if vv, _ := strconv.ParseFloat(v[0], 10); err == nil {
					depth = vv
				}
			}
			var volume float64
			if v, found := data.Value["Volume"]; found && len(v) > 0 {
				if vv, err := strconv.ParseFloat(v[0], 10); err == nil {
					volume = vv
				}
			}
			var weight float64
			if v, found := data.Value["Weight"]; found && len(v) > 0 {
				if vv, err := strconv.ParseFloat(v[0], 10); err == nil {
					weight = vv
				}
			}
			var packages int
			if v, found := data.Value["Packages"]; found && len(v) > 0 {
				if vv, _ := strconv.Atoi(v[0]); err == nil {
					packages = vv
				}
			}
			var availability string
			if v, found := data.Value["Availability"]; found && len(v) > 0 {
				availability = strings.TrimSpace(v[0])
			}
			var timeId uint
			if v, found := data.Value["TimeId"]; found && len(v) > 0 {
				if vv, _ := strconv.Atoi(v[0]); err == nil {
					timeId = uint(vv)
				}
			}
			var sku string
			if v, found := data.Value["Sku"]; found && len(v) > 0 {
				sku = strings.TrimSpace(v[0])
			}
			var customization string
			if v, found := data.Value["Customization"]; found && len(v) > 0 {
				customization = strings.TrimSpace(v[0])
			}
			var stock uint
			if v, found := data.Value["Stock"]; found && len(v) > 0 {
				if vv, _ := strconv.Atoi(v[0]); err == nil {
					stock = uint(vv)
				}
			}
			variation.Enabled = enabled
			variation.Name = name
			variation.Title = title
			variation.Description = description
			variation.Notes = notes
			variation.BasePrice = basePrice
			variation.ManufacturerPrice = manufacturerPrice
			variation.SalePrice = salePrice
			variation.Start = start
			variation.End = end
			variation.ItemPrice = itemPrice
			variation.MinQuantity = minQuantity
			variation.MaxQuantity = maxQuantity
			variation.PurchasableMultiply = purchasableMultiply
			variation.Pattern = pattern
			variation.Dimensions = dimensions
			variation.DimensionUnit = dimensionUnit
			variation.Width = width
			variation.Height = height
			variation.Depth = depth
			variation.Volume = volume
			variation.Weight = weight
			variation.Packages = packages
			variation.Availability = availability
			variation.TimeId = timeId
			variation.Sku = sku
			variation.Stock = stock
			variation.Customization = customization
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
						variation.Thumbnail = "/" + path.Join("variations", filename)
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
// @Tags variation
func delVariationHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if variation, err := models.GetVariation(common.Database, id); err == nil {
		if variations, err := models.GetVariationsByProduct(common.Database, variation.ProductId); err == nil {
			if len(variations) == 1 && id == int(variations[0].ID) {
				if product, err := models.GetProduct(common.Database, int(variation.ProductId)); err == nil {
					product.BasePrice = variation.BasePrice
					product.ManufacturerPrice = variation.ManufacturerPrice
					product.SalePrice = variation.SalePrice
					product.Start = variation.Start
					product.End = variation.End
					product.ItemPrice = variation.ItemPrice
					product.MinQuantity = variation.MinQuantity
					product.MaxQuantity = variation.MaxQuantity
					product.PurchasableMultiply = variation.PurchasableMultiply
					product.Pattern = variation.Pattern
					product.Dimensions = variation.Dimensions
					product.DimensionUnit = variation.DimensionUnit
					product.Width = variation.Width
					product.Height = variation.Height
					product.Depth = variation.Depth
					product.Volume = variation.Volume
					product.Weight = variation.Weight
					product.WeightUnit = variation.WeightUnit
					product.Packages = variation.Packages
					product.Availability = variation.Availability
					product.TimeId = variation.TimeId
					product.Sku = variation.Sku
					product.Stock = variation.Stock

					product.Container = false
					if err = models.UpdateProduct(common.Database, product); err != nil {
						logger.Warningf("%+v", err)
					}
					if err := common.Database.Exec("delete from properties where product_id = ?", product.ID).Error; err != nil {
						logger.Warningf("%+v", err)
					}
					if properties, err := models.GetPropertiesByVariationId(common.Database, int(variation.ID)); err == nil {
						for i := 0; i < len(properties); i++ {
							properties[i].ProductId = product.ID
							properties[i].VariationId = 0
							if err = models.UpdateProperty(common.Database, properties[i]); err != nil {
								logger.Warningf("%+v", err)
							}
						}
					}
					if err := common.Database.Exec("delete from prices where product_id = ?", product.ID).Error; err != nil {
						logger.Warningf("%+v", err)
					}
					if prices, err := models.GetPricesByVariationId(common.Database, variation.ID); err == nil {
						logger.Infof("Prices: %+v", len(prices))
						for i := 0; i < len(prices); i++ {
							prices[i].ProductId = product.ID
							prices[i].VariationId = 0
							if err = models.UpdatePrice(common.Database, prices[i]); err != nil {
								logger.Warningf("%+v", err)
							}
							logger.Infof("%d: price: %+v", i, prices[i])
						}
					}
				}
			}else{
				for _, property := range variation.Properties {
					for _, rate := range property.Rates {
						if err = models.DeleteRate(common.Database, rate); err != nil {
							logger.Errorf("%v", err)
						}
					}
					if err = models.DeleteProperty(common.Database, property); err != nil {
						logger.Errorf("%v", err)
					}
				}
				for _, price := range variation.Prices {
					if err = models.DeletePrice(common.Database, price); err != nil {
						logger.Errorf("%v", err)
					}
				}
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
	Sku string `json:",omitempty"`
	Stock uint `json:",omitempty"`
	ProductId uint
	ProductTitle string
	ProductThumbnail string
	PropertiesIds string
	PropertiesTitles string
	//Options int
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
// @Tags variation
func getVariationHandler(c *fiber.Ctx) error {
	var variation *models.Variation
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
		var err error
		if variation, err = models.GetVariation(common.Database, id); err == nil {
			var view VariationView
			if bts, err := json.Marshal(variation); err == nil {
				if err = json.Unmarshal(bts, &view); err == nil {
					if cache, err := models.GetCacheVariationByVariationId(common.Database, variation.ID); err == nil {
						view.Rendered = &cache.UpdatedAt
					}
					view.New = variation.UpdatedAt.Sub(variation.CreatedAt).Seconds() < 1.0
					for i, property := range view.Properties {
						for j, rate := range property.Rates{
							if cache, err := models.GetCacheValueByValueId(common.Database, rate.Value.ID); err == nil {
								arr := strings.Split(cache.Thumbnail, ",")
								if len(arr) > 1 {
									view.Properties[i].Rates[j].Value.Thumbnail = strings.Split(arr[1], " ")[0]
								}else{
									view.Properties[i].Rates[j].Value.Thumbnail = arr[0]
								}
							}
						}
					}
					for i, price := range view.Prices {
						if price.Thumbnail != "" {
							if cache, err := models.GetCachePriceByPriceId(common.Database, price.ID); err == nil {
								arr := strings.Split(cache.Thumbnail, ",")
								if len(arr) > 1 {
									view.Prices[i].Thumbnail = strings.Split(arr[1], " ")[0]
								}else{
									view.Prices[i].Thumbnail = strings.Split(arr[0], " ")[0]
								}
							}
						}
					}
					for i := 0; i < len(view.Images); i++ {
						if cache, err := models.GetCacheImageByImageId(common.Database, view.Images[i].ID); err == nil {
							view.Images[i].Thumbnail = cache.Thumbnail
						}
					}
					if variations, err := models.GetVariationsByProduct(common.Database, variation.ProductId); err == nil {
						for _, variation := range variations {
							view.Siblings = append(view.Siblings, VariationShortView{
								ID: variation.ID,
								Name: variation.Name,
								Title: variation.Title,
							})
						}
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
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Variation ID is not defined"})
	}
}

type PatchVariationRequest struct {
	Action string
	AddFile uint
	AddImage uint
}

// @security BasicAuth
// PatchVariation godoc
// @Summary Patch variation
// @Accept json
// @Produce json
// @Param id path int true "Variation ID"
// @Success 200 {object} PatchVariationRequest
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/variations/{id} [post]
// @Tags variation
func patchVariationHandler(c *fiber.Ctx) error {
	var variation *models.Variation
	var id int
	var err error
	if v := c.Params("id"); v != "" {
		if id, err = strconv.Atoi(v); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}
	if variation, err = models.GetVariation(common.Database, id); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			if action := c.Query("action", ""); action != "" {
				switch action {
				case "setEnable":
					var request struct {
						Enabled bool
					}
					if err := c.BodyParser(&request); err != nil {
						return err
					}
					variation.Enabled = request.Enabled
					if err = models.UpdateVariation(common.Database, variation); err != nil {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
					c.Status(http.StatusOK)
					return c.JSON(HTTPMessage{"OK"})
				case "addFile":
					var request struct {
						File uint
					}
					if err := c.BodyParser(&request); err != nil {
						return err
					}
					for _, file := range variation.Files {
						if file.ID == request.File {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{"File already added"})
						}
					}
					if file, err := models.GetFile(common.Database, int(request.File)); err == nil {
						variation.Files = append(variation.Files, file)
					}else{
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
					if err = models.UpdateVariation(common.Database, variation); err != nil {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
					c.Status(http.StatusOK)
					return c.JSON(HTTPMessage{"OK"})
				case "addImage":
					var request struct {
						Image uint
					}
					if err := c.BodyParser(&request); err != nil {
						return err
					}
					for _, image := range variation.Images {
						if image.ID == request.Image {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{"Image already added"})
						}
					}
					if image, err := models.GetImage(common.Database, int(request.Image)); err == nil {
						variation.Images = append(variation.Images, image)
					}else{
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
					if err = models.UpdateVariation(common.Database, variation); err != nil {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
					c.Status(http.StatusOK)
					return c.JSON(HTTPMessage{"OK"})
				case "clone":
					variation.ID = 0
					name := variation.Name
					for ; ; {
						if variations, err := models.GetVariationsByProductAndName(common.Database, variation.ProductId, name); err == nil && len(variations) > 0 {
							if res := reName.FindAllStringSubmatch(name, 1); len(res) > 0 && len(res[0]) > 2 {
								if n, err := strconv.Atoi(res[0][2]); err == nil {
									name = fmt.Sprintf("%s-%d", res[0][1], n + 1)
								}
							}else{
								name = fmt.Sprintf("%s-%d", name, 2)
							}
						} else {
							break
						}
					}
					variation.Name = name
					variation.Thumbnail = ""
					properties := variation.Properties
					variation.Properties = []*models.Property{}
					prices := variation.Prices
					variation.Prices = []*models.Price{}
					variation.Images = []*models.Image{}
					variation.Files = []*models.File{}
					variation.Time = nil
					variation.TimeId = 0
					var vid uint
					if vid, err = models.CreateVariation(common.Database, variation); err == nil {
						// We need to replace old properties ids to new one, because of this first save old ids
						alpha := []uint{}
						for i := 0; i < len(properties); i++ {
							properties[i].ID = 0
							properties[i].VariationId = vid
							rates := properties[i].Rates
							properties[i].Rates = []*models.Rate{}
							for _, rate := range rates {
								alpha = append(alpha, rate.ID)
								rate.ID = 0
								rate.Property = nil
								rate.PropertyId = 0
								properties[i].Rates = append(properties[i].Rates, rate)
							}
							variation.Properties = append(variation.Properties, properties[i])
						}
						if err = models.UpdateVariation(common.Database, variation); err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
						// then create save new ids
						beta := []uint{}
						for i := 0; i < len(properties); i++ {
							for j := 0; j < len(properties[i].Rates); j++ {
								beta = append(beta, properties[i].Rates[j].ID)
							}
						}
						// this map contains old => new match
						m := make(map[uint]uint)
						for i := 0; i < len(alpha); i++ {
							m[alpha[i]] = beta[i]
						}
						//
						for i := 0; i < len(prices); i++ {
							prices[i].ID = 0
							prices[i].Variation = nil
							prices[i].VariationId = vid
							rates := prices[i].Rates
							prices[i].Rates = []*models.Rate{}
							for _, rate := range rates {
								// update old ids to new
								if v, found := m[rate.ID]; found {
									rate.ID = v
								}
								prices[i].Rates = append(prices[i].Rates, rate)
							}
							variation.Prices = append(variation.Prices, prices[i])
						}
						if err = models.UpdateVariation(common.Database, variation); err == nil {
							var view VariationView
							if bts, err := json.MarshalIndent(variation, "", "   "); err == nil {
								if err = json.Unmarshal(bts, &view); err == nil {
									view.New = variation.UpdatedAt.Sub(variation.CreatedAt).Seconds() < 1.0
									for i, property := range view.Properties {
										for j, rate := range property.Rates{
											if cache, err := models.GetCacheValueByValueId(common.Database, rate.Value.ID); err == nil {
												view.Properties[i].Rates[j].Value.Thumbnail = strings.Split(cache.Thumbnail, ",")[0]
											}
										}
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
						} else {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
					}else{
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
				default:
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{"Unknown action"})
				}
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"No action defined"})
			}
		}else if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {

		}
	}
	//
	return c.JSON(HTTPMessage{"OK"})
}