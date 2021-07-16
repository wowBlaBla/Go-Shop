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
// GetValues godoc
// @Summary get option values
// @Accept json
// @Produce json
// @Param option_id query int true "Option ID"
// @Success 200 {object} ValuesView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/values [get]
// @Tags value
func getValuesHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Query("option_id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var values []*models.Value
	var err error
	if id == 0 {
		values, err = models.GetValues(common.Database)
	}else{
		values, err = models.GetValuesByOptionId(common.Database, id)
	}
	if err == nil {
		var view []*ValueView
		if bts, err := json.MarshalIndent(values, "", "   "); err == nil {
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

type ValuesView []ValueView

type ValueView struct {
	ID uint
	Title string `json:",omitempty"`
	Description string `json:",omitempty"`
	Color string `json:",omitempty"`
	Thumbnail string `json:",omitempty"`
	Value string `json:",omitempty"`
	Availability string `json:",omitempty"`
	Sending string `json:",omitempty"`
	Sort int `json:",omitempty"`
	OptionId uint `json:",omitempty"`
}

type NewValue struct {
	Title string
	Color string
	Thumbnail string
	Value string
	Sort int
}

// @security BasicAuth
// CreateValue godoc
// @Tag.name values
// @Summary Create value
// @Accept json
// @Produce json
// @Param option_id query int true "Option ID"
// @Param value body NewValue true "body"
// @Success 200 {object} ValueView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/values [post]
// @Tags value
func postValueHandler(c *fiber.Ctx) error {
	var option *models.Option
	var id int
	if v := c.Query("option_id"); v != "" {
		id, _ = strconv.Atoi(v)
		var err error
		if option, err = models.GetOption(common.Database, id); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Option ID is not defined"})
	}
	var view ValueView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				return err
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
			var color string
			if v, found := data.Value["Color"]; found && len(v) > 0 {
				color = strings.TrimSpace(v[0])
			}
			var val string
			if v, found := data.Value["Value"]; found && len(v) > 0 {
				val = strings.TrimSpace(v[0])
			}
			if val == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Invalid value"})
			}
			var availability string
			if v, found := data.Value["Availability"]; found && len(v) > 0 {
				availability = strings.TrimSpace(v[0])
			}
			var sort int
			if v, found := data.Value["Sort"]; found && len(v) > 0 {
				sort, _ = strconv.Atoi(v[0])
			}
			value := &models.Value{Title: title, Description: description, Color: color, Value: val, OptionId: option.ID, Availability: availability, Sort: sort}
			if id, err := models.CreateValue(common.Database, value); err == nil {
				value.Sort = int(id)
				if err = models.UpdateValue(common.Database, value); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
				if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
					p := path.Join(dir, "storage", "values")
					if _, err := os.Stat(p); err != nil {
						if err = os.MkdirAll(p, 0755); err != nil {
							logger.Errorf("%v", err)
						}
					}
					filename := fmt.Sprintf("%d-%s-thumbnail%s", id, regexp.MustCompile(`(?i)[^-a-z0-9]+`).ReplaceAllString(value.Title, "-"), path.Ext(v[0].Filename))
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
				if bts, err := json.Marshal(value); err == nil {
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

type ValuesListResponse struct {
	Data []ValuesListItem
	Filtered int64
	Total int64
}

type ValuesListItem struct {
	ID uint
	OptionTitle string
	Title string
	Thumbnail string
	Value string
	Sort int `json:",omitempty"`
}

// @security BasicAuth
// SearchValues godoc
// @Summary Search option values
// @Accept json
// @Produce json
// @Param option_id query int true "Option ID"
// @Param request body ListRequest true "body"
// @Success 200 {object} ValuesListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/values/list [post]
// @Tags value
func postValuesListHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Query("option_id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var response ValuesListResponse
	var request ListRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	if len(request.Sort) == 0 {
		request.Sort["Sort"] = "asc"
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
				case "OptionTitle":
					keys1 = append(keys1, fmt.Sprintf("%v = ?", key))
					values1 = append(values1, strings.TrimSpace(value))
				default:
					keys1 = append(keys1, fmt.Sprintf("`values`.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	if id > 0 {
		keys1 = append(keys1, "option_id = ?")
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
					orders = append(orders, fmt.Sprintf("`values`.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//logger.Infof("order: %+v", order)
	//
	rows, err := common.Database.Debug().Model(&models.Value{}).Select("`values`.ID, `values`.Title, cache_values.Thumbnail as Thumbnail, `values`.Value, options.Title as OptionTitle, `values`.Sort").Joins("left join cache_values on cache_values.value_id = `values`.ID").Joins("left join options on options.id = `values`.Option_Id").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item ValuesListItem
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
	rows, err = common.Database.Debug().Model(&models.Value{}).Select("`values`.ID, `values`.Title, `values`.Thumbnail, `values`.Value, options.Title as OptionTitle, `values`.Sort").Joins("left join options on options.id = `values`.Option_Id").Where(strings.Join(keys1, " and "), values1...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	if len(keys1) > 0 {
		if id == 0 {
			common.Database.Debug().Model(&models.Value{}).Count(&response.Total)
		}else{
			common.Database.Debug().Model(&models.Value{}).Where("option_id = ?", id).Count(&response.Total)
		}
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

// @security BasicAuth
// GetValue godoc
// @Summary Get value
// @Accept json
// @Produce json
// @Param id path int true "Value ID"
// @Success 200 {object} ValueView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/values/{id} [get]
// @Tags value
func getValueHandler(c *fiber.Ctx) error {
	var value *models.Value
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
		var err error
		if value, err = models.GetValue(common.Database, id); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Value ID is not defined"})
	}
	var view ValueView
	if bts, err := json.Marshal(value); err == nil {
		if err = json.Unmarshal(bts, &view); err == nil {
			if cache, err := models.GetCacheValueByValueId(common.Database, value.ID); err == nil {
				view.Thumbnail = strings.Split(cache.Thumbnail, ",")[0]
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
}

type ValuePatchRequest struct {
	Sort int
}

// @security BasicAuth
// PatchValue godoc
// @Summary patch value
// @Accept json
// @Produce json
// @Param value body ValuePatchRequest true "body"
// @Param id path int true "Value ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/values/{id} [put]
// @Tags value
func patchValueHandler(c *fiber.Ctx) error {
	var request ValuePatchRequest
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
	var value *models.Value
	var err error
	if value, err = models.GetValue(common.Database, int(id)); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	value.Sort = request.Sort
	if err := models.UpdateValue(common.Database, value); err == nil {
		return c.JSON(HTTPMessage{"OK"})
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// @security BasicAuth
// UpdateValue godoc
// @Summary update option value
// @Accept json
// @Produce json
// @Param value body ValueView true "body"
// @Param id path int true "Value ID"
// @Success 200 {object} ValueView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/values/{id} [put]
// @Tags value
func putValueHandler(c *fiber.Ctx) error {
	var value *models.Value
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
		var err error
		if value, err = models.GetValue(common.Database, id); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Value ID is not defined"})
	}
	var view ValueView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
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
			var color string
			if v, found := data.Value["Color"]; found && len(v) > 0 {
				color = strings.TrimSpace(v[0])
			}
			var val string
			if v, found := data.Value["Value"]; found && len(v) > 0 {
				val = strings.TrimSpace(v[0])
			}
			if val == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Invalid value"})
			}
			var availability string
			if v, found := data.Value["Availability"]; found && len(v) > 0 {
				availability = strings.TrimSpace(v[0])
			}
			var sort int
			if v, found := data.Value["Sort"]; found && len(v) > 0 {
				sort, _ = strconv.Atoi(v[0])
			}
			value.Title = title
			value.Description = description
			value.Color = color
			value.Value = val
			value.Availability = availability
			//value.Sending = sending
			value.Sort = sort
			//
			if v, found := data.Value["Thumbnail"]; found && len(v) > 0 && v[0] == "" {
				// To delete existing
				if value.Thumbnail != "" {
					if err = os.Remove(path.Join(dir, value.Thumbnail)); err != nil {
						logger.Errorf("%v", err)
					}
					value.Thumbnail = ""
				}
			}else if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
				p := path.Join(dir, "storage", "values")
				if _, err := os.Stat(p); err != nil {
					if err = os.MkdirAll(p, 0755); err != nil {
						logger.Errorf("%v", err)
					}
				}
				filename := fmt.Sprintf("%d-%s-thumbnail%s", id, regexp.MustCompile(`(?i)[^-a-z0-9]+`).ReplaceAllString(value.Title, "-"), path.Ext(v[0].Filename))
				if p := path.Join(p, filename); len(p) > 0 {
					if in, err := v[0].Open(); err == nil {
						var mod time.Time
						if fi, err := os.Stat(p); err == nil {
							mod = fi.ModTime()
						}
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
								if mod.IsZero() {
									mod = fi.ModTime()
								}
								filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], mod.Unix(), filepath.Ext(filename))
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
			//
			if err := models.UpdateValue(common.Database, value); err == nil {
				return c.JSON(ValueView{ID: value.ID, Title: value.Title, Thumbnail: value.Thumbnail, Value: value.Value, Sort: value.Sort})
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
// DelValue godoc
// @Summary Delete value
// @Accept json
// @Produce json
// @Param id path int true "Value ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/values/{id} [delete]
// @Tags value
func delValueHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if value, err := models.GetValue(common.Database, id); err == nil {
		if value.Thumbnail != "" {
			if err = models.DeleteCacheValueByValueId(common.Database, value.ID); err != nil {
				logger.Warningf("%v", err)
			}
			if err = common.STORAGE.DeleteImage(path.Join("images", value.Thumbnail), common.Config.Resize.Thumbnail.Size); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}
		p := path.Join(dir, "storage", value.Thumbnail)
		if _, err := os.Stat(p); err == nil {
			if err = os.Remove(p); err != nil {
				logger.Errorf("%v", err.Error())
			}
		}
		if err = models.DeleteValue(common.Database, value); err == nil {
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