package handler

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/google/logger"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/models"
	"net/http"
	"strconv"
	"strings"
)

type CommentsView []*CommentView

type CommentView struct {
	Id int
	Title string
	Body string
	Max int
}

// @security BasicAuth
// GetComments godoc
// @Summary Get comments
// @Accept json
// @Produce json
// @Param pid query int true "ProductId"
// @Success 200 {object} CommentsView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/comments [get]
// @Tags comment
func getCommentsHandler(c *fiber.Ctx) error {
	var pid int
	if v := c.Query("pid"); v != "" {
		pid, _ = strconv.Atoi(v)
	}
	var product *models.Product
	var err error
	if product, err = models.GetProduct(common.Database, pid); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	var offset int
	if v := c.Query("offset"); v != "" {
		offset, _ = strconv.Atoi(v)
	}
	var limit int
	if v := c.Query("limit"); v != "" {
		limit, _ = strconv.Atoi(v)
	}
	if limit == 0 {
		limit = 20
	}
	//
	if comments, err := models.GetCommentsByProductWithOffsetLimit(common.Database, product.ID, offset, limit); err == nil {
		var view CommentsView
		if bts, err := json.Marshal(comments); err == nil {
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
// GetComment godoc
// @Summary Get comment
// @Accept json
// @Produce json
// @Param id path int true "Comment ID"
// @Success 200 {object} CommentView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/comments/{id} [get]
// @Tags comment
func getCommentHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if comment, err := models.GetComment(common.Database, id); err == nil {
		var view CommentView
		if bts, err := json.Marshal(comment); err == nil {
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


type NewComment struct {
	Title string
	Body string
	Max int
}

// @security BasicAuth
// CreateComment godoc
// @Summary Create comment
// @Accept json
// @Produce json
// @Param iid query int true "Item Id"
// @Param option body NewComment true "body"
// @Success 200 {object} CommentView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/comments [post]
// @Tags comment
// @Tags product
func postCommentHandler(c *fiber.Ctx) error {
	var iid int
	if v := c.Query("iid"); v != "" {
		iid, _ = strconv.Atoi(v)
	}
	var item *models.Item
	var err error
	if item, err = models.GetItem(common.Database, iid); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	if item.CommentId != 0 {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Comment already exists"})
	}
	//
	var arr []int
	if err = json.Unmarshal([]byte(item.Uuid), &arr); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	if len(arr) < 1 {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Invalid Uuid"})
	}
	//
	var product *models.Product
	if product, err = models.GetProduct(common.Database, arr[0]); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	var user *models.User
	if v := c.Locals("user"); v != nil {
		var ok bool
		if user, ok = v.(*models.User); !ok {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Undefined user"})
		}
	}
	//
	var view CommentView
	var request NewComment
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	request.Title = strings.TrimSpace(request.Title)
	if request.Title == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Title required"})
	}else if len(request.Title) > 1024 {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Title is too big"})
	}
	if request.Body == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Body required"})
	}else if len(request.Body) > 4096 {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Body is too big"})
	}
	if request.Max < 0 || request.Max > 5 {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Max should be [0,5]"})
	}
	comment := &models.Comment{
		Uuid: item.Uuid,
		Title: request.Title,
		Body: request.Body,
		Max: request.Max,
		Product: product,
		User: user,
	}
	var id uint
	if id, err = models.CreateComment(common.Database, comment); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	item.CommentId = id
	if err = models.UpdateItem(common.Database, item); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	if bts, err := json.Marshal(comment); err == nil {
		if err = json.Unmarshal(bts, &view); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}
	return c.JSON(view)
}

// @security BasicAuth
// UpdateComment godoc
// @Summary Update comment
// @Accept json
// @Produce json
// @Param id path int true "Comment ID"
// @Param option body NewComment true "body"
// @Success 200 {object} CommentView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/comments/{id} [put]
// @Tags comment
func putCommentHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var comment *models.Comment
	var err error
	if comment, err = models.GetComment(common.Database, id); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	var user *models.User
	if v := c.Locals("user"); v != nil {
		var ok bool
		if user, ok = v.(*models.User); !ok {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Undefined user"})
		}
	}
	//
	if comment.UserId != user.ID {
		c.Status(http.StatusForbidden)
		return c.JSON(HTTPError{"You are not allowed to use that"})
	}
	//
	var view CommentView
	var request NewComment
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	request.Title = strings.TrimSpace(request.Title)
	if request.Title == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Title required"})
	}else if len(request.Title) > 1024 {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Title is too big"})
	}
	if request.Body == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Body required"})
	}else if len(request.Body) > 4096 {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Body is too big"})
	}
	if request.Max < 0 || request.Max > 5 {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Max should be [0,5]"})
	}
	comment.Title = request.Title
	comment.Body = request.Body
	comment.Max = request.Max
	if err := models.UpdateComment(common.Database, comment); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	if bts, err := json.Marshal(comment); err == nil {
		if err = json.Unmarshal(bts, &view); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}
	return c.JSON(view)
}

// @security BasicAuth
// DelComment godoc
// @Summary Delete comment
// @Accept json
// @Produce json
// @Param id path int true "Comment ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/comments/{id} [delete]
// @Tags comment
func delCommentHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var comment *models.Comment
	var err error
	if comment, err = models.GetComment(common.Database, id); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	if item, err := models.GetItemByComment(common.Database, id); err == nil {
		item.CommentId = 0
		if err = models.UpdateItem(common.Database, item); err != nil {
			logger.Warning("%+v", err)
		}
	}
	if err := models.DeleteComment(common.Database, comment); err == nil {
		return c.JSON(HTTPMessage{MESSAGE: "OK"})
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}