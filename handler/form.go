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

type NewForm struct {
	Enabled bool
	Name string
	Title string
	Description string
}

// @security BasicAuth
// CreateForm godoc
// @Summary Create form
// @Accept json
// @Produce json
// @Param option body NewForm true "body"
// @Success 200 {object} FormView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/forms [post]
// @Tags form
func postFormHandler(c *fiber.Ctx) error {
	var view FormView
	var request NewForm
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Name is not defined"})
	}
	request.Title = strings.TrimSpace(request.Title)
	if request.Title == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Title is not defined"})
	}
	if len(request.Description) > 256 {
		request.Description = request.Description[0:255]
	}
	form := &models.Form {
		Enabled: request.Enabled,
		Name: request.Name,
		Title: request.Title,
		Description: request.Description,
	}
	if _, err := models.CreateForm(common.Database, form); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	if bts, err := json.Marshal(form); err == nil {
		if err = json.Unmarshal(bts, &view); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}
	return c.JSON(view)
}

type FormsListResponse struct {
	Data []FormListItem
	Filtered int64
	Total int64
}

type FormListItem struct {
	ID uint
	Enabled bool
	Name string
	Title string
}

// @security BasicAuth
// SearchForms godoc
// @Summary Search forms
// @Accept json
// @Produce json
// @Param request body ListRequest true "body"
// @Success 200 {object} FormsListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/forms/list [post]
// @Tags form
func postFormsListHandler(c *fiber.Ctx) error {
	var response FormsListResponse
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
				default:
					keys1 = append(keys1, fmt.Sprintf("forms.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				default:
					orders = append(orders, fmt.Sprintf("forms.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//
	rows, err := common.Database.Debug().Model(&models.Form{}).Select("forms.ID, forms.Enabled, forms.Name, forms.Title, forms.Description").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item FormListItem
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
	rows, err = common.Database.Debug().Model(&models.Form{}).Select("forms.ID, forms.Enabled, forms.Name, forms.Title, forms.Description").Where(strings.Join(keys1, " and "), values1...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	if len(keys1) > 0 {
		common.Database.Debug().Model(&models.Coupon{}).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

type FormView struct {
	ID uint
	Enabled bool
	Name string `json:",omitempty"`
	Title string `json:",omitempty"`
	Description string `json:",omitempty"`
}

// @security BasicAuth
// GetForm godoc
// @Summary Get form
// @Accept json
// @Produce json
// @Param id path int true "Form ID"
// @Success 200 {object} FormView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/forms/{id} [get]
// @Tags form
func getFormHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if form, err := models.GetForm(common.Database, id); err == nil {
		var view FormView
		if bts, err := json.MarshalIndent(form, "", "   "); err == nil {
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
// UpdateForm godoc
// @Summary Update form
// @Accept json
// @Produce json
// @Param form body FormView true "body"
// @Param id path int true "Coupon ID"
// @Success 200 {object} FormView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/forms/{id} [put]
// @Tags form
func putFormHandler(c *fiber.Ctx) error {
	var request FormView
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
	var form *models.Form
	var err error
	if form, err = models.GetForm(common.Database, int(id)); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	form.Enabled = request.Enabled
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Name is not defined"})
	}
	request.Title = strings.TrimSpace(request.Title)
	if request.Title == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Title is not defined"})
	}
	if len(request.Description) > 256 {
		request.Description = request.Description[0:255]
	}
	form.Title = request.Title
	form.Description = request.Description
	if err := models.UpdateForm(common.Database, form); err == nil {
		var view FormView
		if bts, err := json.MarshalIndent(form, "", "   "); err == nil {
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
// DelForm godoc
// @Summary Delete form
// @Accept json
// @Produce json
// @Param id path int true "Form ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/forms/{id} [delete]
// @Tags form
func delFormHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if form, err := models.GetForm(common.Database, id); err == nil {
		if err = models.DeleteForm(common.Database, form); err == nil {
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

/**/

// @security BasicAuth
// Get Message By Form godoc
// @Summary Get messages
// @Accept json
// @Produce json
// @Param id path int true "Form ID"
// @Success 200 {object} MessagesView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/forms/{id}/messages [get]
// @Tags message
func getFormMessagesHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if messages, err := models.GetMessagesByFormId(common.Database, id); err == nil {
		var view MessagesView
		if bts, err := json.MarshalIndent(messages, "", "   "); err == nil {
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

type NewFormMessage struct {
	Title string
	Body interface{}
	Module string
}

// @security BasicAuth
// CreateMessage godoc
// @Summary Create message
// @Accept json
// @Produce json
// @Param id path int true "Form ID"
// @Param option body NewMessage true "body"
// @Success 200 {object} MessageView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/forms/{id}/messages [post]
// @Tags message
func postFormMessageHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	//var form *models.Form
	var err error
	if _, err = models.GetForm(common.Database, id); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	var view MessageView
	var request NewFormMessage
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	request.Title = strings.TrimSpace(request.Title)
	if request.Title == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Title is not defined"})
	}
	var body string
	if request.Module == "Samples" {
		if bts, err := json.Marshal(request.Body); err == nil {
			body = string(bts)
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}
	message := &models.Message {
		Title: request.Title,
		Body: body,
		FormId: uint(id),
	}
	if _, err := models.CreateMessage(common.Database, message); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	if bts, err := json.Marshal(message); err == nil {
		if err = json.Unmarshal(bts, &view); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}
	return c.JSON(view)
}