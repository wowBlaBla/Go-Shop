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
	"time"
)

// @security BasicAuth
// GetTransports godoc
// @Summary Get transports
// @Accept json
// @Produce json
// @Success 200 {object} MessagesView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/messages [get]
// @Tags message
func getMessagesHandler(c *fiber.Ctx) error {
	if messages, err := models.GetMessages(common.Database); err == nil {
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

type NewMessage struct {
	Title string
	Body string
}

// @security BasicAuth
// CreateMessage godoc
// @Summary Create message
// @Accept json
// @Produce json
// @Param option body NewMessage true "body"
// @Success 200 {object} MessageView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/messages [post]
// @Tags message
func postMessageHandler(c *fiber.Ctx) error {
	var view MessageView
	var request NewMessage
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	request.Title = strings.TrimSpace(request.Title)
	if request.Title == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Title is not defined"})
	}
	if len(request.Body) > 4096 {
		request.Body = request.Body[0:4096]
	}
	message := &models.Message {
		Title: request.Title,
		Body: request.Body,
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

type MessagesListResponse struct {
	Data []MessageListItem
	Filtered int64
	Total int64
}

type MessageListItem struct {
	ID uint
	Created time.Time
	FormTitle string
	Title string
	Body string
	Length int
}

// @security BasicAuth
// SearchMessages godoc
// @Summary Search messages
// @Accept json
// @Produce json
// @Param form_id query int true "Form ID"
// @Param request body ListRequest true "body"
// @Success 200 {object} MessagesListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/messages/list [post]
// @Tags message
func postMessagesListHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Query("form_id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var response MessagesListResponse
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
					keys1 = append(keys1, fmt.Sprintf("messages.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	if id > 0 {
		keys1 = append(keys1, "form_id = ?")
		values1 = append(values1, id)
	}
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				default:
					orders = append(orders, fmt.Sprintf("messages.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//
	rows, err := common.Database.Debug().Model(&models.Message{}).Select("messages.ID, messages.Created_At as Created, forms.Title as FormTitle, messages.Title, messages.Body, length(messages.Body) as length").Joins("left join forms on messages.form_id = forms.id").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item MessageListItem
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
	rows, err = common.Database.Debug().Model(&models.Message{}).Select("messages.ID, messages.Created_At as Created, forms.Title as FormTitle, messages.Title, messages.Body, length(messages.Body) as length").Joins("left join forms on messages.form_id = forms.id").Where(strings.Join(keys1, " and "), values1...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	if len(keys1) > 0 {
		common.Database.Debug().Model(&models.Message{}).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

type MessagesView []MessageView

type MessageView struct {
	ID uint
	Title string `json:",omitempty"`
	Body string `json:",omitempty"`
}

// @security BasicAuth
// GetMessage godoc
// @Summary Get message
// @Accept json
// @Produce json
// @Param id path int true "Message ID"
// @Success 200 {object} MessageView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/messages/{id} [get]
// @Tags message
func getMessageHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if message, err := models.GetMessage(common.Database, id); err == nil {
		var view MessageView
		if bts, err := json.MarshalIndent(message, "", "   "); err == nil {
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
// UpdateMessage godoc
// @Summary Update message
// @Accept json
// @Produce json
// @Param message body MessageView true "body"
// @Param id path int true "Coupon ID"
// @Success 200 {object} MessageView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/messages/{id} [put]
// @Tags message
func putMessageHandler(c *fiber.Ctx) error {
	var request MessageView
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
	var message *models.Message
	var err error
	if message, err = models.GetMessage(common.Database, int(id)); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	request.Title = strings.TrimSpace(request.Title)
	if request.Title == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Title is not defined"})
	}
	if len(request.Body) > 4096 {
		request.Body = request.Body[0:4096]
	}
	message.Title = request.Title
	message.Body = request.Body
	if err := models.UpdateMessage(common.Database, message); err == nil {
		var view MessageView
		if bts, err := json.MarshalIndent(message, "", "   "); err == nil {
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
// DelMessage godoc
// @Summary Delete message
// @Accept json
// @Produce json
// @Param id path int true "Message ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/messages/{id} [delete]
// @Tags message
func delMessageHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if message, err := models.GetMessage(common.Database, id); err == nil {
		if err = models.DeleteMessage(common.Database, message); err == nil {
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