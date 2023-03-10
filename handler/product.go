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
	var manufacturerPrice float64
	if v, found := data.Value["ManufacturerPrice"]; found && len(v) > 0 {
		if vv, _ := strconv.ParseFloat(v[0], 10); err == nil {
			manufacturerPrice = vv
		}
	}
	var salePrice float64
	if v, found := data.Value["SalePrice"]; found && len(v) > 0 {
		if vv, _ := strconv.ParseFloat(v[0], 10); err == nil {
			salePrice = vv
		}
	}
	var itemPrice float64
	if v, found := data.Value["ItemPrice"]; found && len(v) > 0 {
		if vv, _ := strconv.ParseFloat(v[0], 10); err == nil {
			itemPrice = vv
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
	product := &models.Product{
		Enabled: enabled, Name: name, Title: title, Description: description, Notes: notes, Parameters: parameters,
		CustomParameters: customParameters, Container: container, Variation: variation, Size: size,
		BasePrice: basePrice, ManufacturerPrice: manufacturerPrice, SalePrice: salePrice, ItemPrice: itemPrice,
		MinQuantity: minQuantity, MaxQuantity: maxQuantity, PurchasableMultiply: purchasableMultiply,
		Pattern: pattern, Dimensions: dimensions, DimensionUnit: dimensionUnit, Width: width, Height: height,
		Depth: depth, Volume: volume, Weight: weight, WeightUnit: weightUnit, Packages: packages, Availability: availability, TimeId: timeId,
		Sku: sku, Stock: stock, Content: content, Customization: customization,
	}
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
	SalePrice float64 `json:",omitempty"`
	Sku string
	Stock uint
	VariationIds string `json:",omitempty"`
	Variations string `json:",omitempty"`
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
	var cid int
	if v := c.Query("cid"); v != "" {
		cid, _ = strconv.Atoi(v)
		if rows, err := common.Database.Debug().Raw("select products.id as productId, categories_products_sort.Value as sortValue from products left join categories_products on categories_products.product_id = products.id left join categories_products_sort on categories_products_sort.productId = products.id where categories_products.category_id = ? order by categories_products_sort.Value asc, products.title asc", cid).Rows(); err == nil {
			defer rows.Close()
			var index = 1
			for rows.Next() {
				var productId uint
				var sortValue *uint
				if err = rows.Scan(&productId, &sortValue); err == nil {
					if sortValue == nil {
						if err := common.Database.Debug().Exec("insert into categories_products_sort (CategoryId, ProductId, Value) values (?, ?, ?)", cid, productId, index).Error; err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
					}
					index ++
				}else{
					logger.Warningf("%+v", err)
				}
			}
		}else{
			logger.Warningf("%+v", err)
		}
	}
	var response ProductsListResponse
	var request ListRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	if len(request.Sort) == 0 {
		if cid > 0 {
			request.Sort = map[string]string{"Sort": "asc"}
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
		var term, term2 string
		if res := regexp.MustCompile(`^(['"]?)(.+?)(['"]?)$`).FindStringSubmatch(strings.TrimSpace(request.Search)); len(res) > 1 {
			if res[1] == "" {
				// no quote
				term = res[2]
				term2 = "%" + res[2] + "%"
			}else{
				// quoted
				term = res[2]
				term2 = res[2]
			}
		}
		keys1 = append(keys1, "(products.ID = ? OR products.Title like ? OR products.Description like ? OR variations.Title like ? OR products.Sku like ? OR variations.ID = ? OR variations.Title like ? OR variations.Description like ? OR variations.Title like ? OR variations.Sku like ?)")
		values1 = append(values1, term, term2, term2, term2, term2, term, term2, term2, term2, term2)
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
	if cid > 0 {
		keys1 = append(keys1, "categories_products.category_id = ?")
		values1 = append(values1, cid)
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
	rows, err := common.Database.Debug().Model(&models.Product{}).Select("products.ID, products.Enabled, products.Name, products.Title, cache_images.Thumbnail as Thumbnail, products.Description, products.base_price as BasePrice, products.sale_price as SalePrice, products.Sku, products.Stock, group_concat(distinct variations.Id) as VariationIds, group_concat(distinct variations.Title) as Variations, group_concat(distinct variations.Sku) as VariationSkus, categories_products_sort.Value as Sort").Joins("left join categories_products on categories_products.product_id = products.id").Joins("left join categories_products_sort on categories_products_sort.productId = products.id")/*.Joins("left join cache_products on products.id = cache_products.product_id")*/.Joins("left join cache_images on products.image_id = cache_images.image_id").Joins("left join variations on variations.product_id = products.id").Group("products.id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
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
	rows, err = common.Database.Debug().Model(&models.Product{}).Select("products.ID, products.Enabled, products.Name, products.Title, cache_images.Thumbnail as Thumbnail, products.Description, products.base_price as BasePrice, products.sale_price as SalePrice, products.Sku, products.Stock, group_concat(distinct variations.Id) as VariationIds, group_concat(distinct variations.Title) as Variations, group_concat(distinct variations.Sku) as VariationSkus").Joins("left join categories_products on categories_products.product_id = products.id").Joins("left join cache_images on products.image_id = cache_images.image_id").Joins("left join variations on variations.product_id = products.id").Group("products.id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	//
	if cid > 0 {
		common.Database.Debug().Model(&models.Product{}).Joins("left join categories_products on categories_products.product_id = products.id").Where("categories_products.category_id = ?", cid).Count(&response.Total)
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
				if cache, err := models.GetCacheProductByProductId(common.Database, product.ID); err == nil {
					view.Rendered = &cache.UpdatedAt
				}
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
				case "setSort":
					var request struct {
						CategoryId uint
						Sort int
					}
					if err := c.BodyParser(&request); err != nil {
						return err
					}
					if request.CategoryId > 0 && request.Sort > 0 {
						if err := common.Database.Exec("delete from categories_products_sort where CategoryId = ? and ProductId = ?", request.CategoryId, id).Error; err != nil {
							logger.Warningf("%+v", err)
						}
						if err := common.Database.Exec("insert into categories_products_sort (CategoryId, ProductId, Value) values (?, ?, ?)", request.CategoryId, id, request.Sort).Error; err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
						c.Status(http.StatusOK)
						return c.JSON(HTTPMessage{"OK"})
					}else{
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{"Incorrect values"})
					}
				default:
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{"Unknown action"})
				}
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"No action defined"})
			}
		} else if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			/*data, err := c.Request().MultipartForm()
			if err != nil {
				return err
			}*/

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
			var basePrice float64
			if v, found := data.Value["BasePrice"]; found && len(v) > 0 {
				if vv, _ := strconv.ParseFloat(v[0], 10); err == nil {
					basePrice = vv
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
			// Parameters
			if v, found := data.Value["Parameters"]; found && len(v) > 0 {
				var newParameters []*ParameterView
				if err = json.Unmarshal([]byte(v[0]), &newParameters); err == nil {
					var existingParameters []*models.Parameter
					if existingParameters, err = models.GetParametersByProductId(common.Database, int(product.ID)); err != nil {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
					for _, existingParameter := range existingParameters {
						var found bool
						for i, newParameter := range newParameters {
							if existingParameter.ID == newParameter.ID {
								existingParameter.Title = newParameter.Title
								existingParameter.ValueId =  newParameter.ValueId
								existingParameter.CustomValue = newParameter.CustomValue
								if err := models.UpdateParameter(common.Database, existingParameter); err != nil {
									c.Status(http.StatusInternalServerError)
									return c.JSON(HTTPError{err.Error()})
								}
								if i == len(newParameters) - 1 {
									newParameters = newParameters[:i]
								}else{
									newParameters = append(newParameters[:i], newParameters[i + 1:]...)
								}
								found = true
								break
							}
						}
						if !found {
							if err := models.DeleteParameter(common.Database, existingParameter); err != nil {
								c.Status(http.StatusInternalServerError)
								return c.JSON(HTTPError{err.Error()})
							}
						}
					}
					for _, newParameter := range newParameters {
						parameter := &models.Parameter{
							Name: newParameter.Name,
							Title: newParameter.Title,
							Type: newParameter.Type,
							OptionId:  newParameter.OptionId,
							ValueId: newParameter.ValueId,
							CustomValue: newParameter.CustomValue,
							ProductId: product.ID,
						}
						if _, err := models.CreateParameter(common.Database, parameter); err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
					}
				}else{
					logger.Warningf("%+v", err)
				}
			}
			// Properties
			var newProperties PropertiesView
			if v, found := data.Value["Properties"]; found {
				if err = json.Unmarshal([]byte(v[0]), &newProperties); err == nil {
					var count = 1
					for _, property := range newProperties {
						count *= len(property.Rates)
					}
					if count > common.MAX_PRICES {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{fmt.Sprintf("Too many prices: %d, limit %d", count, common.MAX_PRICES)})
					}
					var existingProperties, updatedProperties []*models.Property
					if existingProperties, err = models.GetPropertiesByProductId(common.Database, int(product.ID)); err != nil {
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

						property.ProductId = product.ID

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
					for i := 0; i < len(updatedProperties); i++ {
						var row []uint
						for j := 0; j < len(updatedProperties[i].Rates); j++ {
							row = append(row, updatedProperties[i].Rates[j].ID)
						}
						table = append(table, row)
					}

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
					if len(matrix) > common.MAX_PRICES {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{fmt.Sprintf("Too many prices: %d, limit %d", len(matrix), common.MAX_PRICES)})
					}
					var existingPrices []*models.Price
					if existingPrices, err = models.GetPricesByProductId(common.Database, product.ID); err != nil {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
					//
					for _, row := range matrix {
						if len(row) > 0 {

							price := &models.Price{
								Enabled:      true,
								ProductId:    product.ID,
								Availability: common.AVAILABILITY_AVAILABLE,
							}
							var ids = []uint{product.ID, 0}
							for _, rateId := range row {
								var rate *models.Rate
								key := fmt.Sprintf("%v", rateId)
								if v, found := RATES.Get(key); !found {
									if rate, err = models.GetRate(common.Database, int(rateId)); err == nil {
										RATES.Set(key, rate, time.Minute)
									}
								}else {
									rate = v.(*models.Rate)
								}
								if rate != nil {
									ids = append(ids, rate.ID)
									price.Rates = append(price.Rates, rate)
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
							}
						}
					}
				}else{
					logger.Warningf("%+v", err)
				}
			}
			// Prices
			if v, found := data.Value["Prices"]; found && len(v) > 0 {
				var newPrices []*PriceView
				if err = json.Unmarshal([]byte(v[0]), &newPrices); err == nil {
					product.Prices = []*models.Price{}
					var existingPrices []*models.Price
					if existingPrices, err = models.GetPricesByProductId(common.Database, product.ID); err != nil {
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
								product.Prices = append(product.Prices, existingPrice)
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
							ProductId: product.ID,
						}
						if _, err := models.CreatePrice(common.Database, price); err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
						product.Prices = append(product.Prices, price)
					}
				}else{
					logger.Warningf("%+v", err)
				}
			}
			//
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
			var typ string
			if v, found := data.Value["Type"]; found && len(v) > 0 {
				typ = strings.TrimSpace(v[0])
			}
			var size string
			if v, found := data.Value["Size"]; found && len(v) > 0 {
				size = strings.TrimSpace(v[0])
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
			oldContainer := product.Container
			product.Container = container
			product.Variation = variation
			product.Type = typ
			product.Size = size
			product.BasePrice = basePrice
			oldManufacturerPrice := product.ManufacturerPrice
			product.ManufacturerPrice = oldManufacturerPrice
			product.SalePrice = salePrice
			product.Start = start
			product.End = end
			oldItemPrice := product.ItemPrice
			product.ItemPrice = oldItemPrice
			product.MinQuantity = minQuantity
			product.MaxQuantity = maxQuantity
			product.PurchasableMultiply = purchasableMultiply
			product.Pattern = pattern
			product.Dimensions = dimensions
			product.DimensionUnit = dimensionUnit
			product.Width = width
			product.Height = height
			product.Depth = depth
			product.Volume = volume
			product.Weight = weight
			product.WeightUnit = weightUnit
			product.Packages = packages
			product.Availability = availability
			product.Sku = sku
			product.Stock = stock
			product.ImageId = imageId
			product.VendorId = vendorId
			product.TimeId = timeId
			product.Content = content
			product.Customization = customization
			// Off => On
			if container && !oldContainer {
				if variations, err := models.GetProductVariations(common.Database, int(product.ID)); err == nil {
					if len(variations) == 0 {
						variation := &models.Variation{
							Enabled: true,
							Name: fmt.Sprintf("variation-1"), Title: fmt.Sprintf("Variation 1"),
							BasePrice: basePrice, ManufacturerPrice: manufacturerPrice, SalePrice: salePrice,
							ItemPrice: itemPrice, MinQuantity: minQuantity, MaxQuantity: maxQuantity,
							PurchasableMultiply: purchasableMultiply, ProductId: product.ID, Pattern: pattern,
							Dimensions: dimensions, DimensionUnit: dimensionUnit, Width: width, Height: height,
							Depth: depth, Volume: volume, Weight: weight, WeightUnit: weightUnit, Packages: packages,
							Availability: availability, TimeId: timeId, Sku: sku, Stock: stock,
						}
						if vid, err := models.CreateVariation(common.Database, variation); err == nil {
							m := make(map[uint]uint)
							if properties, err := models.GetPropertiesByProductId(common.Database, int(product.ID)); err == nil {
								var alpha []uint
								for i := 0; i < len(properties); i++ {
									properties[i].ID = 0
									properties[i].ProductId = 0
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
								var beta []uint
								for i := 0; i < len(properties); i++ {
									for j := 0; j < len(properties[i].Rates); j++ {
										beta = append(beta, properties[i].Rates[j].ID)
									}
								}
								// this map contains old => new match
								for i := 0; i < len(alpha); i++ {
									m[alpha[i]] = beta[i]
								}
							}

							if prices, err := models.GetPricesByProductId(common.Database, product.ID); err == nil {
								for i := 0; i < len(prices); i++ {
									prices[i].ID = 0
									prices[i].ProductId = 0
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
							}

							if err = models.UpdateVariation(common.Database, variation); err != nil {
								c.Status(http.StatusInternalServerError)
								return c.JSON(HTTPError{err.Error()})
							}
						} else {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
					}
				}
			}
			//
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
					if vv != "" {
						if tagId, err := strconv.Atoi(strings.TrimSpace(vv)); err == nil {
							if tag, err := models.GetTag(common.Database, tagId); err == nil {
								if err = models.AddProductToTag(common.Database, tag, product); err != nil {
									logger.Errorf("%v", err)
								}
							} else {
								logger.Errorf("%v", err)
							}
						} else {
							logger.Errorf("%v", err)
						}
					}
				}
			}
			// Related Products
			if err = models.DeleteAllProductsFromProduct(common.Database, product); err != nil {
				logger.Errorf("%v", err)
			}
			if v, found := data.Value["RelatedProducts"]; found && len(v) > 0 {
				for _, vv := range strings.Split(strings.TrimSpace(v[0]), ",") {
					if vv != "" {
						if productId, err := strconv.Atoi(strings.TrimSpace(vv)); err == nil {
							if p, err := models.GetProduct(common.Database, productId); err == nil {
								if err = models.AddProductToProduct(common.Database, product, p); err != nil {
									logger.Errorf("%v", err)
								}
							} else {
								logger.Errorf("%v", err)
							}
						} else {
							logger.Errorf("%v", err)
						}
					}
				}
			}
			// Related Products 2
			if err := common.Database.Exec("delete from products_relations where ProductIdL = ? or ProductIdR = ?", product.ID, product.ID).Error; err != nil {
				logger.Errorf("%+v", err)
			}
			if v, found := data.Value["RelatedProducts"]; found && len(v) > 0 {
				for _, vv := range strings.Split(strings.TrimSpace(v[0]), ",") {
					if vv != "" {
						if productId, err := strconv.Atoi(strings.TrimSpace(vv)); err == nil {
							if p, err := models.GetProduct(common.Database, productId); err == nil {
								if err := common.Database.Exec("insert into products_relations (ProductIdL, ProductIdR) values (?, ?)", product.ID, p.ID).Error; err != nil {
									logger.Errorf("%+v", err)
								}
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