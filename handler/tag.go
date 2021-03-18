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

// Tags

type TagsView []TagView

type TagView struct{
	ID uint
	Enabled bool
	Name string
	Title string
	Description string
	Hidden bool
}

// @security BasicAuth
// GetTags godoc
// @Summary Get tags
// @Accept json
// @Produce json
// @Success 200 {object} TagsView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/tags [get]
// @Tags tag
func getTagsHandler(c *fiber.Ctx) error {
	if tags, err := models.GetTags(common.Database); err == nil {
		var view TagsView
		if bts, err := json.MarshalIndent(tags, "", "   "); err == nil {
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


type NewTag struct {
	Enabled bool
	Hidden bool
	Name string
	Title string
	Description string
}

// @security BasicAuth
// CreateTag godoc
// @Summary Create tag
// @Accept json
// @Produce json
// @Param option body NewTag true "body"
// @Success 200 {object} TagView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/tags [post]
// @Tags tag
func postTagHandler(c *fiber.Ctx) error {
	var view TagView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewTag
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
			if tags, err := models.GetTagsByName(common.Database, request.Name); err == nil && len(tags) > 0 {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Option exists"})
			}
			tag := &models.Tag {
				Enabled: request.Enabled,
				Hidden: request.Hidden,
				Name: request.Name,
				Title: request.Title,
				Description: request.Description,
			}
			if _, err := models.CreateTag(common.Database, tag); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if bts, err := json.Marshal(tag); err == nil {
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

type TagsListResponse struct {
	Data []TagsListItem
	Filtered int64
	Total int64
}

type TagsListItem struct {
	ID uint
	Enabled bool
	Hidden bool
	Name string
	Title string
	Description string
}

// @security BasicAuth
// SearchTags godoc
// @Summary Search tags
// @Accept json
// @Produce json
// @Param request body ListRequest true "body"
// @Success 200 {object} TagsListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/tags/list [post]
// @Tags tag
func postTagsListHandler(c *fiber.Ctx) error {
	var response TagsListResponse
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
					keys1 = append(keys1, fmt.Sprintf("tags.%v like ?", key))
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
					orders = append(orders, fmt.Sprintf("tags.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//logger.Infof("order: %+v", order)
	//
	rows, err := common.Database.Debug().Model(&models.Tag{}).Select("tags.ID, tags.Enabled, tags.Hidden, tags.Name, tags.Title, tags.Description").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item TagsListItem
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
	rows, err = common.Database.Debug().Model(&models.Tag{}).Select("tags.ID, tags.Enabled, tags.Hidden, tags.Name, tags.Title, tags.Description").Where(strings.Join(keys1, " and "), values1...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	if len(keys1) > 0 {
		common.Database.Debug().Model(&models.Tag{}).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

// @security BasicAuth
// GetTag godoc
// @Summary Get tag
// @Accept json
// @Produce json
// @Param id path int true "Tag ID"
// @Success 200 {object} TagView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/tags/{id} [get]
// @Tags tag
func getTagHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if tag, err := models.GetTag(common.Database, id); err == nil {
		var view TagView
		if bts, err := json.MarshalIndent(tag, "", "   "); err == nil {
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
// UpdateTag godoc
// @Summary update tag
// @Accept json
// @Produce json
// @Param tag body TagView true "body"
// @Param id path int true "Tag ID"
// @Success 200 {object} TagView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/tags/{id} [put]
// @Tags tag
func putTagHandler(c *fiber.Ctx) error {
	var request TagView
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
	var tag *models.Tag
	var err error
	if tag, err = models.GetTag(common.Database, int(id)); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	request.Title = strings.TrimSpace(request.Title)
	if request.Title == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Title is not defined"})
	}
	if len(request.Description) > 256 {
		request.Description = request.Description[0:255]
	}
	tag.Enabled = request.Enabled
	tag.Title = request.Title
	tag.Description = request.Description
	tag.Hidden = request.Hidden
	if err := models.UpdateTag(common.Database, tag); err == nil {
		return c.JSON(TagView{ID: tag.ID, Name: tag.Name, Title: tag.Title, Description: tag.Description})
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// @security BasicAuth
// DelTag godoc
// @Summary Delete tag
// @Accept json
// @Produce json
// @Param id path int true "Tag ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/tags/{id} [delete]
// @Tags tag
func delTagHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if tag, err := models.GetTag(common.Database, id); err == nil {
		if err = models.DeleteTag(common.Database, tag); err == nil {
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