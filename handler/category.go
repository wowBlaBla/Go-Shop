package handler

import (
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/google/logger"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/models"
	"gorm.io/gorm"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
)

// @security BasicAuth
// GetCategories godoc
// @Summary Get categories
// @Description get string
// @Accept json
// @Produce json
// @Param id query int false "Root ID"
// @Param depth query int false "Depth, default 1"
// @Success 200 {object} models.CategoryView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/categories [get]
// @Tags category
func getCategoriesHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Query("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var depth = 1
	if v := c.Query("depth"); v != "" {
		depth, _ = strconv.Atoi(v)
	}
	var noProducts = false
	if v := c.Query("no-products"); v != "" {
		if vv, err := strconv.ParseBool(v); err == nil {
			noProducts = vv
		}
	}
	view, err := models.GetCategoriesView(common.Database, id, depth, noProducts, true)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	common.Database.Debug().Model(&models.Product{}).Select("count(ID)").Joins("left join categories_products on categories_products.product_id = products.id").Group("categories_products.category_id").Where("categories_products.category_id = ?", id).Count(&view.Products)
	//
	return c.JSON(view)
}

type NewCategory struct {
	Name string
	Title string
	Description string
	ParentId uint
	Sort int
}

// @security BasicAuth
// CreateCategory godoc
// @Summary Create categories
// @Accept json
// @Produce json
// @Param parent_id query int false "Parent id"
// @Param category body NewCategory true "body"
// @Success 200 {object} models.CategoriesView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/categories [post]
// @Tags category
func postCategoriesHandler(c *fiber.Ctx) error {
	var view models.CategoryView
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
			/*if category, err := models.GetCategoryByName(common.Database, name); err == nil {
				if parentCategory, err := models.GetCategory(common.Database, pid); err == nil {
					if category.ParentId == parentCategory.ID {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{"Name is already in use"})
					}
				}
			}*/
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
			var content string
			if v, found := data.Value["Content"]; found && len(v) > 0 {
				content = strings.TrimSpace(v[0])
			}
			var customization string
			if v, found := data.Value["Customization"]; found && len(v) > 0 {
				customization = strings.TrimSpace(v[0])
			}
			category := &models.Category{Name: name, Title: title, Description: description, Content: content, Customization: customization, ParentId: request.ParentId, Sort: request.Sort}
			if id, err := models.CreateCategory(common.Database, category); err == nil {
				category.Sort = int(id)
				if err = models.UpdateCategory(common.Database, category); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
				if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
					p := path.Join(dir, "storage", "categories")
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
							category.Thumbnail = "/" + path.Join("categories", filename)
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

type AutocompleteRequest struct {
	Search string
	Include string
}

type CategoriesAutocompleteResponse []*CategoriesAutocompleteItem

type CategoriesAutocompleteItem struct {
	ID uint
	Path string
	Name string
	Title string
}

// @security BasicAuth
// SearchCategoriesAutocomplete godoc
// @Summary Search categories autocomplete
// @Accept json
// @Produce json
// @Param category body AutocompleteRequest true "body"
// @Success 200 {object} CategoriesAutocompleteResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/categories/autocomplete [post]
// @Tags category
func postCategoriesAutocompleteHandler(c *fiber.Ctx) error {
	var response CategoriesAutocompleteResponse
	var request AutocompleteRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	request.Search = strings.TrimSpace(request.Search)
	var keys []string
	var values []interface{}
	if request.Search == "" {
		if request.Include == "" {
			keys = append(keys, "parent_id = ?")
			values = append(values, 0)
		} else {
			for _, id := range strings.Split(request.Include, ",") {
				keys = append(keys, "id = ?")
				values = append(values, id)
			}
		}
	} else {
		keys = append(keys, "title like ?")
		values = append(values, "%" + request.Search + "%")
	}
	//logger.Infof("request: %+v", request)
	//
	var categories []*models.Category
	if err := common.Database.Debug().Where(strings.Join(keys, " or "), values...).Limit(10).Find(&categories).Error; err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	for _, category := range categories {
		item := &CategoriesAutocompleteItem {
			ID: category.ID,
			Name: category.Name,
			Title: category.Title,
		}
		//
		if category.ParentId != 0 {
			breadcrumbs := &[]*models.Category{}
			var f3 func(connector *gorm.DB, id uint)
			f3 = func(connector *gorm.DB, id uint) {
				if id != 0 {
					if category, err := models.GetCategory(connector, int(id)); err == nil {
						*breadcrumbs = append([]*models.Category{category}, *breadcrumbs...)
						f3(connector, category.ParentId)
					}
				}
			}
			f3(common.Database, category.ParentId)
			var titles []string
			for _, breadcrumb := range *breadcrumbs {
				titles = append(titles, breadcrumb.Title)
			}
			//
			item.Path = "/ " + strings.Join(titles, " / ") + " /"
		}
		response = append(response, item)
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
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
	Products int
	Sort int
}

// @security BasicAuth
// SearchCategories godoc
// @Summary Search categories
// @Accept json
// @Produce json
// @Param parent_id query int false "Parent id"
// @Param request body ListRequest true "body"
// @Success 200 {object} CategoriesListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/categories/list [post]
// @Tags category
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
		request.Sort["Sort"] = "asc"
		request.Sort["Title"] = "asc"
	}
	if request.Length == 0 {
		request.Length = 10
	}
	//logger.Infof("request: %+v", request)
	// Filter
	var keys1 []string
	var values1 []interface{}
	var keys2 []string
	var values2 []interface{}
	if len(request.Filter) > 0 {
		for key, value := range request.Filter {
			if key != "" && len(strings.TrimSpace(value)) > 0 {

				switch key {
				case "Products":
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
					keys1 = append(keys1, fmt.Sprintf("categories.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	var filtering = true
	if len(keys1) == 0 {
		filtering = false
		keys1 = append(keys1, "parent_id = ?")
		values1 = append(values1, id)
	}
	//logger.Infof("keys1: %+v, values1: %+v", strings.Join(keys1, " and "), values1)
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
	//logger.Infof("order: %+v", order)
	//
	rows, err := common.Database.Debug().Model(&models.Category{}).Select("categories.ID, categories.Name, categories.Title, categories.Thumbnail, categories.Description, count(categories_products.product_id) as Products, categories.Sort").Joins("left join categories_products on categories_products.category_id = categories.id").Group("categories.id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item CategoriesListItem
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
	//
	rows, err = common.Database.Debug().Model(&models.Category{}).Select("categories.ID, categories.Name, categories.Title, categories.Thumbnail, categories.Description, count(categories_products.product_id) as Products, categories.Sort").Joins("left join categories_products on categories_products.category_id = categories.id").Group("categories.id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}

	if filtering {
		common.Database.Debug().Model(&models.Category{}).Where(strings.Join(keys1, " and "), values1...).Count(&response.Filtered)
		common.Database.Debug().Model(&models.Category{}).Count(&response.Total)
	}else{
		common.Database.Debug().Model(&models.Category{}).Where("parent_id = ?", id).Count(&response.Filtered)
		response.Total = response.Filtered
	}
	//
	c.Status(http.StatusOK)
	return c.JSON(response)
}



type CategoryFullView struct {
	ID uint
	Name string
	Title string
	Description string
	Thumbnail string
	Content string
	Customization string
	ParentId uint
}

// @security BasicAuth
// GetCategory godoc
// @Summary Get category
// @Accept json
// @Produce json
// @Param id path int true "Category ID"
// @Success 200 {object} CategoryFullView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/categories/{id} [get]
// @Tags category
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

type CategoryPatchRequest struct {
	Product struct {
		ID uint
		Sort int
	}
	Sort int
}

// @security BasicAuth
// PatchCategory godoc
// @Summary patch category
// @Accept json
// @Produce json
// @Param option body CategoryPatchRequest true "body"
// @Param id path int true "Category ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/category/{id} [patch]
// @Tags category
func patchCategoryHandler(c *fiber.Ctx) error {
	var request CategoryPatchRequest
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
	var category *models.Category
	var err error
	if category, err = models.GetCategory(common.Database, int(id)); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	if request.Product.ID > 0 {
		if err := common.Database.Exec("delete from categories_products_sort where CategoryId = ? and ProductId = ?", category.ID, request.Product.ID).Error; err != nil {
			logger.Errorf("%+v", err)
		}
		if err := common.Database.Exec("insert into categories_products_sort (CategoryId, ProductId, Value) values (?, ?, ?)", category.ID, request.Product.ID, request.Product.Sort).Error; err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		category.Sort = request.Sort
		if err := models.UpdateCategory(common.Database, category); err == nil {
			return c.JSON(HTTPMessage{"OK"})
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}
	return c.JSON(HTTPMessage{"OK"})
}

// @security BasicAuth
// UpdateCategory godoc
// @Summary Update category
// @Accept json
// @Produce json
// @Param category body NewCategory true "body"
// @Success 200 {object} models.CategoryView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/categories/{id} [put]
// @Tags category
func putCategoryHandler(c *fiber.Ctx) error {
	var view models.CategoryView
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
		if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
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
			/*if parentCategory, err := models.GetCategory(common.Database, pid); err == nil {
				for _, category := range models.GetChildrenOfCategoryById(common.Database, parentCategory.ID) {
					if int(category.ID) != id && category.Name == request.Name {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{"Name is already in use"})
					}
				}
			}*/
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
			var content string
			if v, found := data.Value["Content"]; found && len(v) > 0 {
				content = strings.TrimSpace(v[0])
			}
			var customization string
			if v, found := data.Value["Customization"]; found && len(v) > 0 {
				customization = strings.TrimSpace(v[0])
			}
			var parentId uint
			if v, found := data.Value["ParentId"]; found && len(v) > 0 {
				if id, err := strconv.Atoi(v[0]); err == nil {
					parentId = uint(id)
				}
			}
			var sort int
			if v, found := data.Value["Sort"]; found && len(v) > 0 {
				if sort, err = strconv.Atoi(v[0]); err != nil {
					logger.Warningf("%+v", err)
				}
			}
			category.Name = name
			category.Title = title
			category.Description = description
			category.Content = content
			category.Customization = customization
			category.ParentId = parentId
			category.Sort = sort
			if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
				p := path.Join(dir, "storage", "categories")
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
						category.Thumbnail = "/" + path.Join("categories", filename)
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
// @Router /api/v1/category/{id} [delete]
// @Tags category
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
				p := path.Join(dir, "hugo", category.Thumbnail)
				if _, err := os.Stat(p); err == nil {
					if err = os.Remove(p); err != nil {
						logger.Errorf("%v", err.Error())
					}
				}
			}
			if err = models.DeleteCategory(common.Database, category); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}
		//
		if category.Thumbnail != "" {
			p := path.Join(dir, "hugo", category.Thumbnail)
			if _, err := os.Stat(p); err == nil {
				if err = os.Remove(p); err != nil {
					logger.Errorf("%v", err.Error())
				}
			}
		}
		if err = models.DeleteCategory(common.Database, category); err != nil {
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