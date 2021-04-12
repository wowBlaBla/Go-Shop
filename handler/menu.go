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

type MenusView []MenuView

type MenuView struct{
	ID uint
	Enabled bool
	Name string
	Title string
	Description string
	Location string
}

// @security BasicAuth
// GetMenus godoc
// @Summary Get menus
// @Accept json
// @Produce json
// @Success 200 {object} MenusView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/menus [get]
// @Tags menu
func getMenusHandler(c *fiber.Ctx) error {
	if menus, err := models.GetMenus(common.Database); err == nil {
		var view MenusView
		if bts, err := json.MarshalIndent(menus, "", "   "); err == nil {
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

type NewMenu struct {
	Enabled bool
	Name string
	Title string
	Description string
	Location string
}

// @security BasicAuth
// CreateMenu godoc
// @Summary Create menu
// @Accept json
// @Produce json
// @Param menu body NewMenu true "body"
// @Success 200 {object} MenuView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/menus [post]
// @Tags menu
func postMenuHandler(c *fiber.Ctx) error {
	var view MenuView

	var request NewMenu
	if err := c.BodyParser(&request); err != nil {
		return err
	}

	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Name is empty"})
	}

	request.Title = strings.TrimSpace(request.Title)
	if request.Title == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Title is empty"})
	}

	request.Description = strings.TrimSpace(request.Description)

	request.Location = strings.TrimSpace(request.Location)

	menu := &models.Menu {
		Enabled: request.Enabled,
		Name:    request.Name,
		Title:   request.Title,
		Description: request.Description,
		Location: request.Location,
	}
	if _, err := models.CreateMenu(common.Database, menu); err == nil {
		if bts, err := json.Marshal(menu); err == nil {
			if err = json.Unmarshal(bts, &view); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}
		return c.JSON(view)
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

type MenusListResponse struct {
	Data []MenusListItem
	Filtered int64
	Total int64
}

type MenusListItem struct {
	ID uint
	Enabled bool
	Name string
	Title string
	Location string
}

// @security BasicAuth
// SearchMenus godoc
// @Summary Search menus
// @Accept json
// @Produce json
// @Param request body ListRequest true "body"
// @Success 200 {object} MenusListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/menus/list [post]
// @Tags menu
func postMenusListHandler(c *fiber.Ctx) error {
	var response MenusListResponse
	var request ListRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	if len(request.Sort) == 0 {
		request.Sort["ID"] = "asc"
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
					keys1 = append(keys1, fmt.Sprintf("menus.%v like ?", key))
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
				default:
					orders = append(orders, fmt.Sprintf("menus.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//logger.Infof("order: %+v", order)
	//
	rows, err := common.Database.Debug().Model(&models.Menu{}).Select("menus.ID, menus.Enabled, menus.Name, menus.Title, menus.Location").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item MenusListItem
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
	rows, err = common.Database.Debug().Model(&models.Menu{}).Select("menus.ID, menus.Enabled, menus.Name, menus.Title, menus.Location").Where(strings.Join(keys1, " and "), values1...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	if len(keys1) > 0 {
		common.Database.Debug().Model(&models.Menu{}).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

// @security BasicAuth
// GetMenu godoc
// @Summary Get menu
// @Accept json
// @Produce json
// @Param id path int true "Shipping ID"
// @Success 200 {object} MenuView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/menus/{id} [get]
// @Tags menu
func getMenuHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if menu, err := models.GetMenu(common.Database, id); err == nil {
		var view MenuView
		if bts, err := json.MarshalIndent(menu, "", "   "); err == nil {
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
// UpdateMenu godoc
// @Summary Update menu
// @Accept json
// @Produce json
// @Param menu body MenuView true "body"
// @Param id path int true "Shipping ID"
// @Success 200 {object} TagView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/menus/{id} [put]
// @Tags menu
func putMenuHandler(c *fiber.Ctx) error {
	var menu *models.Menu
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
		var err error
		if menu, err = models.GetMenu(common.Database, id); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "ID is not defined"})
	}

	var request NewMenu
	if err := c.BodyParser(&request); err != nil {
		return err
	}

	request.Title = strings.TrimSpace(request.Title)
	if request.Title == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Title is empty"})
	}

	request.Description = strings.TrimSpace(request.Description)

	request.Location = strings.TrimSpace(request.Location)

	menu.Enabled = request.Enabled
	menu.Title = request.Title
	menu.Description = request.Description
	menu.Location = request.Location

	if err := models.UpdateMenu(common.Database, menu); err == nil {
		var view MenuView
		if bts, err := json.Marshal(menu); err == nil {
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
// DelMenu godoc
// @Summary Delete menu
// @Accept json
// @Produce json
// @Param id path int true "Shipping ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/menus/{id} [delete]
// @Tags menu
func delMenuHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if menu, err := models.GetMenu(common.Database, id); err == nil {
		if err = models.DeleteMenu(common.Database, menu); err == nil {
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