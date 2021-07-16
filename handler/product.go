package handler

import (
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/google/logger"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/models"
	"image"
	"io"
	"math"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Products
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
// @Tags product
func getProductsHandler(c *fiber.Ctx) error {
	if products, err := models.GetProductsWithImages(common.Database); err == nil {
		var views ProductsShortView
		for _, product := range products {
			var view ProductShortView
			if bts, err := json.MarshalIndent(product, "", "   "); err == nil {
				if err = json.Unmarshal(bts, &view); err == nil {
					if product.Image != nil {
						view.Thumbnail = product.Image.Url
					}else if len(product.Images) > 0{
						view.Thumbnail = product.Images[0].Url
					}
					views = append(views, view)
				}else{
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}
		c.Status(http.StatusOK)
		return c.JSON(views)
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
// @Tags product
func postProductsHandler(c *fiber.Ctx) error {
	var view ProductView
	//
	data, err := c.Request().MultipartForm()
	if err != nil {
		return err
	}
	var enabled bool
	if v, found := data.Value["Enabled"]; found && len(v) > 0 {
		enabled, _ = strconv.ParseBool(v[0])
	}
	var name string
	if v, found := data.Value["Name"]; found && len(v) > 0 {
		name = strings.TrimSpace(v[0])
	}
	/*if name == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Invalid name"})
	}*/
	for ;; {
		if _, err := models.GetProductByName(common.Database, name); err == nil {
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
	var parameters []*models.Parameter
	if options, err := models.GetOptionsByStandard(common.Database, true); err == nil {
		for _, option := range options {
			parameter := &models.Parameter{
				Name: option.Name,
				Title: option.Title,
				Option: option,
			}
			if option.Value != nil {
				parameter.Value = option.Value
			}
			parameters = append(parameters, parameter)
		}
	}
	var customParameters string
	if v, found := data.Value["CustomParameters"]; found && len(v) > 0 {
		customParameters = strings.TrimSpace(v[0])
	}
	var container bool
	if v, found := data.Value["Container"]; found && len(v) > 0 {
		container, _ = strconv.ParseBool(v[0])
	}
	var variation = "Default"
	if v, found := data.Value["Variation"]; found && len(v) > 0 {
		variation = strings.TrimSpace(v[0])
	}
	var size = common.Config.Size
	if v, found := data.Value["Size"]; found && len(v) > 0 {
		size = strings.TrimSpace(v[0])
	}
	var basePrice float64
	if v, found := data.Value["BasePrice"]; found && len(v) > 0 {
		if vv, _ := strconv.ParseFloat(v[0], 10); err == nil {
			basePrice = vv
		}
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
		if vv, _ := strconv.ParseFloat(v[0], 10); err == nil {
			volume = vv
		}
	}
	var weight float64
	if v, found := data.Value["Weight"]; found && len(v) > 0 {
		if vv, _ := strconv.ParseFloat(v[0], 10); err == nil {
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
	var stock uint
	if v, found := data.Value["TimeId"]; found && len(v) > 0 {
		if vv, _ := strconv.Atoi(v[0]); err == nil {
			stock = uint(vv)
		}
	}
	var content string
	if v, found := data.Value["Content"]; found && len(v) > 0 {
		content = strings.TrimSpace(v[0])
	}
	var customization string
	if v, found := data.Value["Customization"]; found && len(v) > 0 {
		customization = strings.TrimSpace(v[0])
	}
	product := &models.Product{Enabled: enabled, Name: name, Title: title, Description: description, Notes: notes, Parameters: parameters, CustomParameters: customParameters, Container: container, Variation: variation, Size: size, BasePrice: basePrice, Pattern: pattern, Dimensions: dimensions, Width: width, Height: height, Depth: depth, Volume: volume, Weight: weight, Packages: packages, Availability: availability, TimeId: timeId, Sku: sku, Stock: stock, Content: content, Customization: customization}
	if _, err := models.CreateProduct(common.Database, product); err == nil {
		// Create new product automatically
		if name == "" {
			product.Name = fmt.Sprintf("new-product-%d", product.ID)
			product.Title = fmt.Sprintf("New Product %d", product.ID)
			if err = models.UpdateProduct(common.Database, product); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}
		if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
			p := path.Join(dir, "storage", "products")
			if _, err := os.Stat(p); err != nil {
				if err = os.MkdirAll(p, 0755); err != nil {
					logger.Errorf("%v", err)
				}
			}
			// Image
			var p1 string
			img := &models.Image{Name: name, Size: v[0].Size}
			//filename = fmt.Sprintf("%d-%s%s", id, img.Name, path.Ext(v[0].Filename))
			if id, err := models.CreateImage(common.Database, img); err == nil {
				p := path.Join(dir, "storage", "images")
				if _, err := os.Stat(p); err != nil {
					if err = os.MkdirAll(p, 0755); err != nil {
						logger.Errorf("%v", err)
					}
				}
				filename := fmt.Sprintf("%d-%s%s", id, img.Name, path.Ext(v[0].Filename))
				if p := path.Join(p, filename); len(p) > 0 {
					if err = common.Copy(p1, p); err == nil {
						img.Url = common.Config.Base + "/" + path.Join("storage", "images", filename)
						img.Path = "/" + path.Join("storage", "images", filename)
						if reader, err := os.Open(p); err == nil {
							defer reader.Close()
							if config, _, err := image.DecodeConfig(reader); err == nil {
								img.Height = config.Height
								img.Width = config.Width
							} else {
								logger.Errorf("%v", err.Error())
							}
						}
						if err = models.UpdateImage(common.Database, img); err != nil {
							logger.Errorf("%v", err.Error())
						}
						if err = models.AddImageToProduct(common.Database, product, img); err != nil {
							logger.Errorf("%v", err.Error())
						}
					}else{
						logger.Errorf("%v", err.Error())
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
		/*if _, err = models.CreateVariation(common.Database, &models.Variation{Title: "Default", Name: "default", Description: "", BasePrice: basePrice, ProductId: product.ID}); err != nil {
			logger.Errorf("%v", err)
		}*/
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
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
	Enabled bool
	Name string
	Title string
	Thumbnail string
	Description string
	BasePrice float64
	Sku string
	Stock uint
	Variations int
	CategoryId uint `json:",omitempty"`
	Sort int
}

// @security BasicAuth
// SearchProducts godoc
// @Summary Search products
// @Accept json
// @Produce json
// @Param id query int false "Category id"
// @Param request body ListRequest true "body"
// @Success 200 {object} ProductsListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/products/list [post]
// @Tags product
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
		if id > 0 {
			request.Sort = map[string]string{"Sort": "desc"}
		}else{
			request.Sort = map[string]string{"Id": "desc"}
		}
	}
	if request.Length == 0 {
		request.Length = 10
	}
	// Filter
	var keys1 []string
	var values1 []interface{}
	var keys2 []string
	var values2 []interface{}
	if request.Search != "" {
		term := strings.TrimSpace(request.Search)
		term2 := "%" + term + "%"
		keys1 = append(keys1, "(products.ID = ? OR products.Title like ? OR products.Description like ? OR products.Sku like ?)")
		values1 = append(values1, term, term2, term2, term2)
	}
	if len(request.Filter) > 0 {
		for key, value := range request.Filter {
			if key != "" && len(strings.TrimSpace(value)) > 0 {
				switch key {
				case "Enabled":
					keys1 = append(keys1, fmt.Sprintf("products.%v = ?", key))
					if strings.EqualFold(value, "true") {
						values1 = append(values1, "1")
					}else{
						values1 = append(values1, "0")
					}
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
		keys1 = append(keys1, "categories_products.category_id = ?")
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
				case "Sort":
					orders = append(orders, fmt.Sprintf("%v %v", key, value))
				case "Variations":
					orders = append(orders, fmt.Sprintf("%v %v", key, value))
				default:
					orders = append(orders, fmt.Sprintf("products.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//logger.Infof("order: %+v", order)
	//
	rows, err := common.Database.Debug().Model(&models.Product{}).Select("products.ID, products.Enabled, products.Name, products.Title, cache_images.Thumbnail as Thumbnail, products.Description, products.base_price as BasePrice, products.Sku, products.Stock, count(variations.ID) as Variations, categories_products_sort.Value as Sort").Joins("left join categories_products on categories_products.product_id = products.id").Joins("left join categories_products_sort on categories_products_sort.productId = products.id").Joins("left join cache_products on products.id = cache_products.product_id").Joins("left join cache_images on products.image_id = cache_images.image_id").Joins("left join variations on variations.product_id = products.id").Group("products.id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
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
	rows, err = common.Database.Debug().Model(&models.Product{}).Select("products.ID, products.Enabled, products.Name, products.Title, cache_images.Thumbnail as Thumbnail, products.Description, products.base_price as BasePrice, products.Sku, products.Stock, count(variations.ID) as Variations").Joins("left join categories_products on categories_products.product_id = products.id").Joins("left join cache_images on products.image_id = cache_images.image_id").Joins("left join variations on variations.product_id = products.id").Group("variations.product_id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	//
	if len(keys1) > 0 || len(keys2) > 0 {
		common.Database.Debug().Model(&models.Product{}).Where("category_id = ?", id).Count(&response.Total)
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
// @Param id path int true "Products ID"
// @Success 200 {object} ProductView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/products/{id} [get]
// @Tags product
func getProductHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if product, err := models.GetProductFull(common.Database, id); err == nil {
		var view ProductView
		if bts, err := json.MarshalIndent(product, "", "   "); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				view.New = product.UpdatedAt.Sub(product.CreatedAt).Seconds() < 1.0
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
				for i := 0; i < len(view.Images); i++ {
					if cache, err := models.GetCacheImageByImageId(common.Database, view.Images[i].ID); err == nil {
						view.Images[i].Thumbnail = cache.Thumbnail
					}
				}
				// Related Products 2
				if rows, err := common.Database.Debug().Table("products_relations").Select("products_relations.ProductIdL as ProductIdL, products_relations.ProductIdR as ProductIdR").Where("products_relations.ProductIdL = ? or products_relations.ProductIdR = ?", product.ID, product.ID).Rows(); err == nil {
					for rows.Next() {
						var r struct {
							ProductIdL uint
							ProductIdR uint
						}
						if err = common.Database.ScanRows(rows, &r); err == nil {
							if r.ProductIdL == product.ID {
								var found bool
								for _, p := range view.RelatedProducts {
									if p.ID == r.ProductIdR {
										found = true
										break
									}
								}
								if !found {
									view.RelatedProducts = append(view.RelatedProducts, RelatedProduct{ID: r.ProductIdR})
								}
							}else{
								var found bool
								for _, p := range view.RelatedProducts {
									if p.ID == r.ProductIdL {
										found = true
										break
									}
								}
								if !found {
									view.RelatedProducts = append(view.RelatedProducts, RelatedProduct{ID: r.ProductIdL})
								}
							}
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
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

type ProductPatchRequest struct {
	Enabled string
	AddFile uint
	AddImage uint
	Properties PropertiesView
}

// @security BasicAuth
// PatchProduct godoc
// @Summary patch product
// @Accept json
// @Produce json
// @Param option body ProductPatchRequest true "body"
// @Param id path int true "Product ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/products/{id} [patch]
// @Tags product
func patchProduct0Handler(c *fiber.Ctx) error {
	var request ProductPatchRequest
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
	var product *models.Product
	var err error
	if product, err = models.GetProductFull(common.Database, id); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		logger.Infof("contentType: %+v", contentType)
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON){
			if request.Enabled == "true" {
				product.Enabled = true
			}else if request.Enabled == "false" {
				product.Enabled = false
			}
			if request.AddFile > 0 {
				for _, file := range product.Files {
					if file.ID == request.AddFile {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{"File already added"})
					}
				}
				if file, err := models.GetFile(common.Database, int(request.AddFile)); err == nil {
					product.Files = append(product.Files, file)
				}else{
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}
			if request.AddImage > 0 {
				for _, image := range product.Images {
					if image.ID == request.AddImage {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{"Image already added"})
					}
				}
				if image, err := models.GetImage(common.Database, int(request.AddImage)); err == nil {
					product.Images = append(product.Images, image)
				}else{
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}
		} else if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				return err
			}
			var properties PropertiesView
			if v, found := data.Value["Properties"]; found {
				if err = json.Unmarshal([]byte(v[0]), &properties); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Properties missed"})
			}
			//
			if c.Query("action", "") == "setProperties" {
				logger.Infof("setProperties: %+v", len(properties))
				var properties2 []*models.Property
				for i, p := range properties {
					logger.Infof("Property #%d: %+v", i, p)
					var property *models.Property
					if p.ID == 0 {
						// new
						logger.Infof("New Property")
						property = &models.Property{
							ProductId: product.ID,
							OptionId:  p.OptionId,
						}
					}else{
						// existing
						if property, err = models.GetPropertyFull(common.Database, int(p.ID)); err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
						//
						logger.Infof("Existing Property: %+v", property)
					}

					property.Type = p.Type
					property.Size = p.Size
					property.Mode = p.Mode
					property.Name = p.Name
					property.Title = p.Title
					property.Sku = p.Sku
					property.Stock = p.Stock
					property.Filtering = p.Filtering

					for j, r := range p.Rates {
						logger.Infof("Rate #%d: %+v", j, r)
						var rate *models.Rate
						if r.ID == 0 {
							// new
							logger.Infof("New Rate")
							rate = &models.Rate{
								Enabled: r.Enabled,
								ValueId: r.ValueId,
							}
						} else {
							// existing
							if rate, err = models.GetRate(common.Database, int(r.ID)); err != nil {
								c.Status(http.StatusInternalServerError)
								return c.JSON(HTTPError{err.Error()})
							}
							logger.Infof("Existing Rate: %+v", rate)
						}

						rate.Price = r.Price
						rate.Availability = r.Availability
						rate.Sending = r.Sending
						rate.Sku = r.Sku
						rate.Stock = r.Stock

						//var value *models.Value
						if r.ValueId == 0 {
							// new
							logger.Infof("New Value")
							rate.Value = &models.Value{}
						}else{
							// existing
							if rate.Value, err = models.GetValue(common.Database, int(r.ValueId)); err == nil {
								r.ValueId = rate.Value.ID
							}
							logger.Infof("Existing Value: %+v", rate.Value)
						}

						rate.Value.Title = r.Value.Title
						rate.Value.Thumbnail = r.Value.Thumbnail
						rate.Value.Description = r.Value.Description
						rate.Value.Color = r.Value.Color
						rate.Value.Value = r.Value.Value
						rate.Value.OptionId = r.Value.OptionId
						rate.Value.Availability = r.Value.Availability
						rate.Value.Sort = r.Value.Sort

						property.Rates = append(property.Rates, rate)
					}

					properties2 = append(properties2, property)
					if property.ID == 0 {
						logger.Infof("To create")
						if _, err := models.CreateProperty(common.Database, property); err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
					}else {
						logger.Infof("To update")
						if err := models.UpdateProperty(common.Database, property); err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
					}

				}
				// Properties
				for _, property := range properties2 {
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
			}
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Missed Content-Type header"})
	}
	//
	if err = models.UpdateProduct(common.Database, product); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	return c.JSON(HTTPMessage{"OK"})
}

func patchProductHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "ID is not defined"})
	}
	var product *models.Product
	var err error
	if product, err = models.GetProductFull(common.Database, id); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON){

			if action := c.Query("action", ""); action != "" {
				switch action {
				case "setEnable":
					var request struct {
						Enabled bool
					}
					if err := c.BodyParser(&request); err != nil {
						return err
					}
					product.Enabled = request.Enabled
					if err = models.UpdateProduct(common.Database, product); err != nil {
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
					for _, file := range product.Files {
						if file.ID == request.File {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{"File already added"})
						}
					}
					if file, err := models.GetFile(common.Database, int(request.File)); err == nil {
						product.Files = append(product.Files, file)
					}else{
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
					if err = models.UpdateProduct(common.Database, product); err != nil {
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
					for _, image := range product.Images {
						if image.ID == request.Image {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{"Image already added"})
						}
					}
					if image, err := models.GetImage(common.Database, int(request.Image)); err == nil {
						product.Images = append(product.Images, image)
					}else{
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
					if err = models.UpdateProduct(common.Database, product); err != nil {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
					c.Status(http.StatusOK)
					return c.JSON(HTTPMessage{"OK"})
				default:
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{"Unknown action"})
				}
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"No action defined"})
			}
		} else if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				return err
			}
			var propertyViews PropertiesView
			if v, found := data.Value["Properties"]; found {
				if err = json.Unmarshal([]byte(v[0]), &propertyViews); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Properties missed"})
			}
			logger.Infof("propertyViews: %+v", propertyViews)
			var properties, properties2 []*models.Property
			if properties, err = models.GetPropertiesByProductId(common.Database, int(product.ID)); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			var resized bool
			logger.Infof("Existing Properties: %+v", len(properties))
			for _, property := range properties{
				logger.Infof("Existing Property: %+v", property)
				var found bool
				for i, p := range propertyViews {
					if property.ID == p.ID {
						logger.Infof("Match to incoming property #%+v", p)
						// TODO: To update
						property.Type = p.Type
						property.Size = p.Size
						property.Mode = p.Mode
						property.Name = p.Name
						property.Title = p.Title
						property.Sku = p.Sku
						property.Stock = p.Stock
						property.Filtering = p.Filtering

						// TODO: Check rates
						logger.Infof("Existing Rates: %+v", len(property.Rates))
						//for i, rate := range property.Rates {
						for i := 0; i < len(property.Rates); i++ {
							rate := property.Rates[i]
							if bts, err := json.Marshal(rate); err == nil {
								logger.Infof("%d: %+v", i, string(bts))
							}
							var found bool
							for j := 0; j < len(p.Rates); j++ {
								r := p.Rates[j]
								if rate.ID == r.ID {
									logger.Infof("Match to incoming rate #%+v", r.ID)
									// TODO: Update
									rate.Enabled = r.Enabled
									rate.Price = r.Price
									rate.Availability = r.Availability
									rate.Sending = r.Sending
									rate.Sku = r.Sku
									rate.Stock = r.Stock
									if rate.Value.OptionId == 0 {
										if value, err := models.GetValue(common.Database, int(rate.ValueId)); err == nil {
											value.Title = r.Value.Title
											value.Description = r.Value.Description
											value.Color = r.Value.Color
											value.Value = r.Value.Value
											value.OptionId = r.Value.OptionId
											value.Availability = r.Value.Availability
											value.Sort = r.Value.Sort
											if value.Thumbnail != "" && r.Value.Thumbnail == "" {
												// TODO: To delete thumbnail
												logger.Infof("Existing thumbnail should be %v deleted", value.Thumbnail)
												value.Thumbnail = ""
												if err = models.DeleteCacheValueByValueId(common.Database, value.ID); err != nil {
													logger.Warningf("%+v", err)
												}
											}else if _, found := data.File[r.Value.Thumbnail]; r.Value.Thumbnail != "" && found {
												logger.Infof("New thumbnail will be loaded: %v", r.Value.Thumbnail)
												value.Thumbnail = r.Value.Thumbnail
											}else{
												logger.Infof("Thumbnail %v rejected", r.Value.Thumbnail)
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
									logger.Infof("Updating rate %+v", rate)
									if err := models.UpdateRate(common.Database, rate); err != nil {
										c.Status(http.StatusInternalServerError)
										return c.JSON(HTTPError{err.Error()})
									}

									//p.Rates = append(p.Rates[:j], p.Rates[j + 1:]...)
									copy(p.Rates[j:], p.Rates[j+1:])
									p.Rates[len(p.Rates) - 1] = nil
									p.Rates = p.Rates[:len(p.Rates)-1]
									j--
									found = true
									break
								}
							}
							if !found {
								// TODO: Delete rate
								logger.Infof("Delete rate %+v", rate)
								copy(property.Rates[i:], property.Rates[i+1:])
								property.Rates[len(property.Rates)-1] = nil
								property.Rates = property.Rates[:len(property.Rates)-1]
								i--
								if err := models.DeleteRate(common.Database, rate); err != nil {
									c.Status(http.StatusInternalServerError)
									return c.JSON(HTTPError{err.Error()})
								}
							}
						}

						for _, r := range p.Rates {
							// TODO: Add rate
							rate := &models.Rate{
								Enabled: r.Enabled,
							}
							if r.ValueId == 0 {
								rate.Value = &models.Value{
									Title: r.Value.Title,
									Thumbnail: r.Value.Thumbnail,
									Description: r.Value.Description,
									Color: r.Value.Color,
									Value: r.Value.Value,
									OptionId: r.Value.OptionId,
									Availability: r.Value.Availability,
									Sort: r.Value.Sort,
								}
							}else{
								rate.ValueId = r.ValueId
							}
							rate.Price = r.Price
							rate.Availability = r.Availability
							rate.Sending = r.Sending
							rate.Sku = r.Sku
							rate.Stock = r.Stock

							if _, err := models.CreateRate(common.Database, rate); err != nil {
								c.Status(http.StatusInternalServerError)
								return c.JSON(HTTPError{err.Error()})
							}

							property.Rates = append(property.Rates, rate)
						}

						logger.Infof("Updating property %+v", property)
						if err := models.UpdateProperty(common.Database, property); err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}

						properties2 = append(properties2, property)
						if i == len(propertyViews) - 1 {
							propertyViews = propertyViews[:i]
						}else{
							propertyViews = append(propertyViews[:i], propertyViews[i + 1:]...)
						}
						found = true
						break
					}
				}
				if !found {
					// TODO: Delete property
					logger.Infof("Delete property %+v", property)
					for _, rate := range property.Rates {
						if rate.Value.OptionId == 0 {
							if err = models.DeleteValue(common.Database, rate.Value); err != nil {
								logger.Warningf("%+v", err)
							}
						}
						if err = models.DeleteRate(common.Database, rate); err != nil {
							logger.Warningf("%+v", err)
						}
					}
					if err := models.DeleteProperty(common.Database, property); err != nil {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}

					resized = true
				}
			}
			logger.Infof("New properties: %+v", len(propertyViews))
			for _, p := range propertyViews {
				logger.Infof("New property: %+v", p)
				property := &models.Property{
					Type: p.Type,
					Size: p.Size,
					Mode: p.Mode,
					Name: p.Name,
					Title: p.Title,
					Sku: p.Sku,
					Stock: p.Stock,
					OptionId: p.OptionId,
					Filtering: p.Filtering,
				}

				property.ProductId = product.ID

				// TODO: Add rates
				for _, r := range p.Rates {
					// TODO: Add rate
					rate := &models.Rate{
						Enabled: r.Enabled,
					}
					if r.ValueId == 0 {
						rate.Value = &models.Value{
							Title: r.Value.Title,
							Thumbnail: r.Value.Thumbnail,
							Description: r.Value.Description,
							Color: r.Value.Color,
							Value: r.Value.Value,
							OptionId: r.Value.OptionId,
							Availability: r.Value.Availability,
							Sort: r.Value.Sort,
						}
					}else{
						rate.ValueId = r.ValueId
					}
					rate.Price = r.Price
					rate.Availability = r.Availability
					rate.Sending = r.Sending
					rate.Sku = r.Sku
					rate.Stock = r.Stock

					property.Rates = append(property.Rates, rate)
				}
				logger.Infof("Creating Property: %+v", property)
				if _, err := models.CreateProperty(common.Database, property); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
				properties2 = append(properties2, property)
				resized = true
			}

			// Properties
			for _, property := range properties2 {
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
				logger.Infof("Prices reset REQUIRED")
				logger.Infof("Delete old prices")
				if prices, err := models.GetPricesByProductId(common.Database, product.ID); err == nil {
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
			//
			var table [][]uint
			for i := 0; i < len(properties2); i++ {
				var row []uint
				for j := 0; j < len(properties2[i].Rates); j++ {
					row = append(row, properties2[i].Rates[j].ID)
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
			//
			var existingPrices []*models.Price
			if existingPrices, err = models.GetPricesByProductId(common.Database, product.ID); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			logger.Infof("existingPrices: %+v", len(existingPrices))
			logger.Infof("matrix: %+v", matrix)
			logger.Infof("===========================")
			//
			for _, row := range matrix {
				logger.Infof("row: %+v", row)
				price := &models.Price{
					Enabled: true,
					ProductId: product.ID,
					Availability: common.AVAILABILITY_AVAILABLE,
				}
				for _, rateId := range row {
					if rateId > 0 {
						if rate, err := models.GetRate(common.Database, int(rateId)); err == nil {
							price.Rates = append(price.Rates, rate)
						}
					}
				}
				logger.Infof("price: %+v", price)

				var match bool
				for _, existingPrice := range existingPrices {
					logger.Infof("existingPrice: %+v", existingPrice)
					match = len(price.Rates) == len(existingPrice.Rates)
					logger.Infof("match1: %+v", match)
					if match {
						for _, rate1 := range price.Rates {
							var found bool
							for _, rate2 := range existingPrice.Rates {
								if rate1.ID == rate2.ID {
									logger.Infof("Match found %+v and %+v", rate1, rate2)
									found = true
									break
								}
							}
							if !found {
								logger.Infof("Match not found %+v", rate1)
								match = false
								break
							}
						}
					}
					logger.Infof("match2: %+v", match)

					if match {
						logger.Infof("Price exists: %+v", existingPrice)
						break
					}
				}

				if !match {
					if _, err = models.CreatePrice(common.Database, price); err != nil {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
					logger.Infof("New price created: %+v", price)
				}
			}
		}
	}

	c.Status(http.StatusOK)
	return c.JSON(HTTPMessage{"OK"})
}

// @security BasicAuth
// UpdateProduct godoc
// @Summary Update product
// @Accept multipart/form-data
// @Produce json
// @Param id query int false "Products id"
// @Param product body NewProduct true "body"
// @Success 200 {object} ProductView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/products/{id} [put]
// @Tags product
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
			var enabled bool
			if v, found := data.Value["Enabled"]; found && len(v) > 0 {
				enabled, _ = strconv.ParseBool(v[0])
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
			if product.Name != name {
				for ; ; {
					if _, err := models.GetProductByName(common.Database, name); err == nil {
						if res := reName.FindAllStringSubmatch(name, 1); len(res) > 0 && len(res[0]) > 2 {
							if n, err := strconv.Atoi(res[0][2]); err == nil {
								name = fmt.Sprintf("%s-%d", res[0][1], n+1)
							}
						} else {
							name = fmt.Sprintf("%s-%d", name, 2)
						}
					} else {
						break
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
			var notes string
			if v, found := data.Value["Notes"]; found && len(v) > 0 {
				notes = strings.TrimSpace(v[0])
			}
			var customParameters string
			if v, found := data.Value["CustomParameters"]; found && len(v) > 0 {
				customParameters = strings.TrimSpace(v[0])
			}
			var container bool
			if v, found := data.Value["Container"]; found && len(v) > 0 {
				container, _ = strconv.ParseBool(v[0])
			}
			var variation string
			if v, found := data.Value["Variation"]; found && len(v) > 0 {
				variation = strings.TrimSpace(v[0])
			}
			var size string
			if v, found := data.Value["Size"]; found && len(v) > 0 {
				size = strings.TrimSpace(v[0])
			}
			var basePrice float64
			if v, found := data.Value["BasePrice"]; found && len(v) > 0 {
				if vv, _ := strconv.ParseFloat(v[0], 10); err == nil {
					basePrice = vv
				}
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
			var pattern string
			if v, found := data.Value["Pattern"]; found && len(v) > 0 {
				pattern = strings.TrimSpace(v[0])
			}
			var dimensions string
			if v, found := data.Value["Dimensions"]; found && len(v) > 0 {
				dimensions = strings.TrimSpace(v[0])
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
				if vv, _ := strconv.ParseFloat(v[0], 10); err == nil {
					volume = vv
				}
			}
			var weight float64
			if v, found := data.Value["Weight"]; found && len(v) > 0 {
				if vv, _ := strconv.ParseFloat(v[0], 10); err == nil {
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
			var sku string
			if v, found := data.Value["Sku"]; found && len(v) > 0 {
				sku = strings.TrimSpace(v[0])
			}
			var content string
			if v, found := data.Value["Content"]; found && len(v) > 0 {
				content = strings.TrimSpace(v[0])
			}
			var imageId uint
			if v, found := data.Value["ImageId"]; found && len(v) > 0 {
				if vv, _ := strconv.Atoi(v[0]); err == nil {
					imageId = uint(vv)
				}
			}
			var vendorId uint
			if v, found := data.Value["VendorId"]; found && len(v) > 0 {
				if vv, _ := strconv.Atoi(v[0]); err == nil {
					vendorId = uint(vv)
				}
			}
			var timeId uint
			if v, found := data.Value["TimeId"]; found && len(v) > 0 {
				if vv, _ := strconv.Atoi(v[0]); err == nil {
					timeId = uint(vv)
				}
			}
			var stock uint
			if v, found := data.Value["Stock"]; found && len(v) > 0 {
				if vv, _ := strconv.Atoi(v[0]); err == nil {
					stock = uint(vv)
				}
			}
			var customization string
			if v, found := data.Value["Customization"]; found && len(v) > 0 {
				customization = strings.TrimSpace(v[0])
			}
			product.Enabled = enabled
			product.Name = name
			product.Title = title
			product.Description = description
			product.Notes = notes
			product.CustomParameters = customParameters
			oldBasePrice := product.BasePrice
			product.Container = container
			product.Variation = variation
			product.Size = size
			product.BasePrice = basePrice
			oldSalePrice := product.SalePrice
			product.SalePrice = salePrice
			oldStart := product.Start
			product.Start = start
			oldEnd := product.End
			product.End = end
			product.Pattern = pattern
			product.Dimensions = dimensions
			product.Width = width
			product.Height = height
			product.Depth = depth
			oldVolume := product.Volume
			product.Volume = volume
			oldWeight := product.Weight
			product.Weight = weight
			oldPackages := product.Packages
			product.Packages = packages
			oldAvailability := product.Availability
			product.Availability = availability
			oldSku := product.Sku
			product.Sku = sku
			oldStock := product.Stock
			product.Stock = stock
			product.ImageId = imageId
			product.VendorId = vendorId
			product.TimeId = timeId
			product.Content = content
			product.Customization = customization
			if variations, err := models.GetProductVariations(common.Database, int(product.ID)); err == nil {
				for _, variation := range variations {
					if variation.Name == "default" {
						if math.Abs(oldBasePrice - basePrice) > 0.01 {
							variation.BasePrice = product.BasePrice
						}
						if math.Abs(oldSalePrice - salePrice) > 0.01 {
							variation.SalePrice = product.SalePrice
						}
						if !oldStart.Equal(start) {
							variation.Start = product.Start
						}
						if !oldEnd.Equal(end) {
							variation.End = product.End
						}
						if math.Abs(oldVolume - volume) > 0.01 {
							variation.Volume = product.Volume
						}
						if math.Abs(oldWeight - weight) > 0.01 {
							variation.Weight = product.Weight
						}
						if oldPackages != packages {
							variation.Packages = product.Packages
						}
						if oldAvailability != availability {
							variation.Availability = product.Availability
						}
						if oldSku != sku {
							variation.Sku = product.Sku
						}
						if (oldStock != stock) {
							variation.Stock = product.Stock
						}
						if err := models.UpdateVariation(common.Database, variation); err != nil {
							logger.Warningf("%+v", err)
						}
					}
				}
			}
			if v, found := data.Value["Thumbnail"]; found && len(v) > 0 && v[0] == "" {
				// To delete existing
				if product.Thumbnail != "" {
					if err = os.Remove(path.Join(dir, product.Thumbnail)); err != nil {
						logger.Errorf("%v", err)
					}
					product.Thumbnail = ""
				}
			}else if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
				p := path.Join(dir, "storage", "variations")
				if _, err := os.Stat(p); err != nil {
					if err = os.MkdirAll(p, 0755); err != nil {
						logger.Errorf("%v", err)
					}
				}
				filename := fmt.Sprintf("%d-%s-thumbnail%s", product.ID, regexp.MustCompile(`(?i)[^-a-z0-9]+`).ReplaceAllString("default", "-"), path.Ext(v[0].Filename))
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
						product.Thumbnail = "/" + path.Join("variations", filename)
					}
				}
			}
			if err = models.UpdateProduct(common.Database, product); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
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
					if tagId, err := strconv.Atoi(strings.TrimSpace(vv)); err == nil {
						if tag, err := models.GetTag(common.Database, tagId); err == nil {
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
			// Related Products
			if err = models.DeleteAllProductsFromProduct(common.Database, product); err != nil {
				logger.Errorf("%v", err)
			}
			if v, found := data.Value["RelatedProducts"]; found && len(v) > 0 {
				for _, vv := range strings.Split(strings.TrimSpace(v[0]), ",") {
					if productId, err := strconv.Atoi(strings.TrimSpace(vv)); err == nil {
						if p, err := models.GetProduct(common.Database, productId); err == nil {
							if err = models.AddProductToProduct(common.Database, product, p); err != nil {
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
			// Related Products 2
			if err := common.Database.Exec("delete from products_relations where ProductIdL = ? or ProductIdR = ?", product.ID, product.ID).Error; err != nil {
				logger.Errorf("%+v", err)
			}
			if v, found := data.Value["RelatedProducts"]; found && len(v) > 0 {
				for _, vv := range strings.Split(strings.TrimSpace(v[0]), ",") {
					if productId, err := strconv.Atoi(strings.TrimSpace(vv)); err == nil {
						if p, err := models.GetProduct(common.Database, productId); err == nil {
							if err := common.Database.Exec("insert into products_relations (ProductIdL, ProductIdR) values (?, ?)", product.ID, p.ID).Error; err != nil {
								logger.Errorf("%+v", err)
							}
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
// @Param id path int true "Products ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/products/{id} [delete]
// @Tags product
func delProductHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if product, err := models.GetProductFull(common.Database, id); err == nil {
		//
		for _, variation := range product.Variations {
			for _, property := range variation.Properties {
				for _, price := range property.Rates {
					if err = models.DeleteRate(common.Database, price); err != nil {
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