package handler

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/models"
	"net/http"
	"strconv"
	"strings"
)

// Parameters
type ParameterView struct {
	ID uint
	Type string // select / radio
	Name string
	Title string
	OptionId uint `json:",omitempty"`
	Option struct {
		ID uint
		Name string
		Title string
		Description string `json:",omitempty"`
		Weight int
	}
	ValueId uint
	Value struct {
		ID uint
		Title string
		Thumbnail string `json:",omitempty"`
	}
	CustomValue string `json:",omitempty"`
	Filtering bool `json:",omitempty"`
}

type NewParameter struct {
	Name string
	Title string
	OptionId uint
	ValueId uint
	CustomValue string
	Filtering bool
}

// @security BasicAuth
// CreateParameter godoc
// @Summary Create parameter
// @Accept json
// @Produce json
// @Param product_id query int true "Products id"
// @Param property body NewParameter true "body"
// @Success 200 {object} ParameterView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/parameters [post]
// @Tags parameter
func postParameterHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Query("product_id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var err error
	var product *models.Product
	if product, err = models.GetProduct(common.Database, id); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	var view PropertyView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewParameter
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			//
			if properties, err := models.GetParametersByProductAndName(common.Database, id, request.Name); err == nil {
				if len(properties) > 0 {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{"Parameter already defined, edit existing"})
				}
			}
			//
			parameter := &models.Parameter{
				Name:        request.Name,
				Title:       request.Title,
				OptionId:    request.OptionId,
				ValueId:     request.ValueId,
				CustomValue: request.CustomValue,
				Filtering:   request.Filtering,
				ProductId: product.ID,
			}
			//
			if _, err := models.CreateParameter(common.Database, parameter); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if bts, err := json.Marshal(parameter); err == nil {
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
// GetParameter godoc
// @Summary Get parameter
// @Accept json
// @Produce json
// @Param id path int true "Parameter ID"
// @Success 200 {object} ParameterView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/parameters/{id} [get]
// @Tags parameter
func getParameterHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if parameter, err := models.GetParameter(common.Database, id); err == nil {
		var view ParameterView
		if bts, err := json.MarshalIndent(parameter, "", "   "); err == nil {
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
// UpdateParameter godoc
// @Summary Update parameter
// @Accept json
// @Produce json
// @Param id path int true "Parameter ID"
// @Param category body NewParameter true "body"
// @Success 200 {object} ParameterView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/parameter/{id} [put]
// @Tags parameter
func putParameterHandler(c *fiber.Ctx) error {
	var view ParameterView
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var parameter *models.Parameter
	var err error
	if parameter, err = models.GetParameter(common.Database, id); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewParameter
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			parameter.Title = request.Title
			if parameter.ValueId != request.ValueId {
				if value, err := models.GetValue(common.Database, int(request.ValueId)); err == nil {
					parameter.Value = value
				}
			}
			parameter.CustomValue = request.CustomValue
			parameter.Filtering = request.Filtering
			if err = models.UpdateParameter(common.Database, parameter); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if bts, err := json.Marshal(parameter); err == nil {
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
// DelParameter godoc
// @Summary Delete parameter
// @Accept json
// @Produce json
// @Param id query int true "Parameter id"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/parameters/{id} [delete]
// @Tags parameter
func deleteParameterHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if parameter, err := models.GetParameter(common.Database, id); err == nil {
		if err = models.DeleteParameter(common.Database, parameter); err != nil {
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