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

type OptionsShortView []OptionShortView

type OptionShortView struct {
	ID uint
	Name string `json:",omitempty"`
	Title string `json:",omitempty"`
	Description string `json:",omitempty"`
	ValueId uint
	Standard bool
	Sort int `json:",omitempty"`
}

type NewOption struct {
	Name string
	Title string
	Description string
	ValueId uint
	Standard bool
	Sort int
}

// @security BasicAuth
// CreateOption godoc
// @Summary Create option
// @Accept json
// @Produce json
// @Param option body NewOption true "body"
// @Success 200 {object} OptionView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/options [post]
// @Tags option
func postOptionHandler(c *fiber.Ctx) error {
	var view OptionView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewOption
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			request.Title = strings.TrimSpace(request.Title)
			if request.Title == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": "Title is not defined"})
			}
			if request.Name == "" {
				request.Name = strings.TrimSpace(request.Name)
				request.Name = reNotAbc.ReplaceAllString(strings.ToLower(request.Title), "-")
			}
			if len(request.Description) > 256 {
				request.Description = request.Description[0:255]
			}
			if options, err := models.GetOptionsByName(common.Database, request.Name); err == nil && len(options) > 0 {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Option exists"})
			}
			option := &models.Option {
				Name: request.Name,
				Title: request.Title,
				Description: request.Description,
				Standard: request.Standard,
				ValueId: request.ValueId,
				Sort: request.Sort,
			}
			if id, err := models.CreateOption(common.Database, option); err == nil {
				option.Sort = int(id)
				if err = models.UpdateOption(common.Database, option); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			} else {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if bts, err := json.Marshal(option); err == nil {
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

type OptionsListResponse struct {
	Data []OptionsListItem
	Filtered int64
	Total int64
}

type OptionsListItem struct {
	ID uint
	Name string
	Title string
	Description string
	ValueValue string
	ValuesValues string
	Standard bool
	Sort int
}

// @security BasicAuth
// SearchOptions godoc
// @Summary Search options
// @Accept json
// @Produce json
// @Param request body ListRequest true "body"
// @Success 200 {object} OptionsListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/options/list [post]
// @Tags option
func postOptionsListHandler(c *fiber.Ctx) error {
	var response OptionsListResponse
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
	var keys2 []string
	var values2 []interface{}
	if len(request.Filter) > 0 {
		for key, value := range request.Filter {
			if key != "" && len(strings.TrimSpace(value)) > 0 {
				switch key {
				case "Values":
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
					keys1 = append(keys1, fmt.Sprintf("options.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
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
				case "Values":
					orders = append(orders, fmt.Sprintf("%v %v", key, value))
				default:
					orders = append(orders, fmt.Sprintf("options.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//logger.Infof("order: %+v", order)
	//
	rows, err := common.Database.Debug().Model(&models.Option{}).Select("options.ID, options.Name, options.Title, options.Description, options.Standard, options.Sort, group_concat(`values`.Value, ', ') as ValuesValues").Joins("left join `values` on `values`.option_id = options.id").Group("options.id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item OptionsListItem
				if err = common.Database.ScanRows(rows, &item); err == nil {
					item.ValuesValues = strings.TrimRight(item.ValuesValues, ", ")
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
	rows, err = common.Database.Debug().Model(&models.Option{}).Select("options.ID, options.Name, options.Title, options.Description, options.Standard, options.Sort, group_concat(`values`.Value, ', ') as ValuesValues").Joins("left join `values` on `values`.option_id = options.id").Group("options.id").Where(strings.Join(keys1, " and "), values1...).Having(strings.Join(keys2, " and "), values2...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	if len(keys1) > 0 || len(keys2) > 0 {
		common.Database.Debug().Model(&models.Option{}).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

type OptionsFullView []OptionFullView

type OptionFullView struct {
	OptionShortView
	Values []ValueView `json:",omitempty"`
}

// @security BasicAuth
// GetOptions godoc
// @Summary Get options
// @Accept json
// @Produce json
// @Success 200 {object} OptionsFullView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/options [get]
// @Tags option
func getOptionsHandler(c *fiber.Ctx) error {
	if options, err := models.GetOptionsFull(common.Database); err == nil {
		var view OptionsFullView
		if bts, err := json.MarshalIndent(options, "", "   "); err == nil {
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

type OptionView struct {
	ID uint
	Name string `json:",omitempty"`
	Title string `json:",omitempty"`
	Description string `json:",omitempty"`
	Value ValueView `json:",omitempty"`
	Values []ValueView
	Standard bool `json:",omitempty"`
	Sort int
}

// @security BasicAuth
// GetOption godoc
// @Summary Get option
// @Accept json
// @Produce json
// @Param id path int true "Option ID"
// @Success 200 {object} OptionView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/options/{id} [get]
// @Tags option
func getOptionHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if option, err := models.GetOption(common.Database, id); err == nil {
		var view OptionView
		if bts, err := json.MarshalIndent(option, "", "   "); err == nil {
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

type OptionPatchRequest struct {
	Sort int
}

// @security BasicAuth
// PatchOption godoc
// @Summary patch option
// @Accept json
// @Produce json
// @Param option body OptionPatchRequest true "body"
// @Param id path int true "Option ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/options/{id} [put]
// @Tags option
func patchOptionHandler(c *fiber.Ctx) error {
	var request OptionPatchRequest
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
	var option *models.Option
	var err error
	if option, err = models.GetOption(common.Database, int(id)); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	option.Sort = request.Sort
	if err := models.UpdateOption(common.Database, option); err == nil {
		return c.JSON(HTTPMessage{"OK"})
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// @security BasicAuth
// UpdateOption godoc
// @Summary Update option
// @Accept json
// @Produce json
// @Param option body OptionShortView true "body"
// @Param id path int true "Option ID"
// @Success 200 {object} OptionShortView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/options/{id} [put]
// @Tags option
func putOptionHandler(c *fiber.Ctx) error {
	var request OptionShortView
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
	var option *models.Option
	var err error
	if option, err = models.GetOption(common.Database, int(id)); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	request.Title = strings.TrimSpace(request.Title)
	if request.Title == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Title is not defined"})
	}
	if request.Name == "" {
		request.Name = strings.TrimSpace(request.Name)
		request.Name = reNotAbc.ReplaceAllString(strings.ToLower(request.Title), "-")
	}
	if len(request.Description) > 256 {
		request.Description = request.Description[0:255]
	}
	option.Name = request.Name
	option.Title = request.Title
	option.Description = request.Description
	option.ValueId = request.ValueId
	option.Standard = request.Standard
	option.Sort = request.Sort
	if err := models.UpdateOption(common.Database, option); err == nil {
		return c.JSON(OptionShortView{ID: option.ID, Name: option.Name, Title: option.Title, Description: option.Description, Sort: option.Sort})
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// @security BasicAuth
// DelOption godoc
// @Summary Delete option
// @Accept json
// @Produce json
// @Param id path int true "Option ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/options/{id} [delete]
// @Tags option
func delOptionHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if option, err := models.GetOption(common.Database, id); err == nil {
		for _, value := range option.Values {
			if err = models.DeleteValue(common.Database, value); err != nil {
				logger.Errorf("%v", err)
			}
		}
		if err = models.DeleteOption(common.Database, option); err == nil {
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