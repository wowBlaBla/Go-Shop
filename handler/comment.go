package handler

import (
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/google/logger"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/models"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type CommentsView []*CommentView

type CommentView struct {
	Id int
	Enabled bool
	CreatedAt time.Time
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
	Enabled bool
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
// @Router /api/v1/account/comments [post]
// @Tags comment
// @Tags product
func postAccountCommentHandler(c *fiber.Ctx) error {
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
	var title, body string
	var max int
	var data *multipart.Form
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewComment
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			title = strings.TrimSpace(request.Title)
			body = strings.TrimSpace(request.Title)
			max = request.Max
		}else if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err = c.Request().MultipartForm()
			if err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if v, found := data.Value["Title"]; found && len(v) > 0 {
				title = strings.TrimSpace(v[0])
			}
			if v, found := data.Value["Body"]; found && len(v) > 0 {
				body = strings.TrimSpace(v[0])
			}
			if v, found := data.Value["Max"]; found && len(v) > 0 {
				if vv, err := strconv.Atoi(strings.TrimSpace(v[0])); err == nil {
					max = vv
				}
			}
		}
	}

	if title == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Title required"})
	}else if len(title) > 1024 {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Title is too big"})
	}

	if body == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Body required"})
	}else if len(body) > 4096 {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Body is too big"})
	}

	if max < 0 || max > 5 {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Max should be [0,5]"})
	}

	comment := &models.Comment{
		Uuid: item.Uuid,
		Title: title,
		Body: body,
		Max: max,
		Product: product,
		User: user,
	}
	var id uint
	if id, err = models.CreateComment(common.Database, comment); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	if data != nil {
		var images []string
		var images2 []string
		if v, found := data.File["Images"]; found && len(v) > 0 {
			for i, vv := range v {
				p := path.Join(dir, "storage", "comments")
				if _, err := os.Stat(p); err != nil {
					if err = os.MkdirAll(p, 0755); err != nil {
						logger.Errorf("%v", err)
					}
				}
				title := regexp.MustCompile(`(?i)[^-a-z0-9]+`).ReplaceAllString(comment.Title, "-")
				if len(title) > 24 {
					title = title[:24]
				}
				filename := fmt.Sprintf("%d-%s-%d-%s", id, title, i, path.Ext(v[0].Filename))
				if p := path.Join(p, filename); len(p) > 0 {
					if in, err := vv.Open(); err == nil {
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
						images = append(images, "/" + path.Join("comments", filename))
						//
						if p1 := path.Join(dir, "storage", "values", filename); len(p1) > 0 {
							if fi, err := os.Stat(p1); err == nil {
								filename := filepath.Base(p1)
								filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
								logger.Infof("Copy %v => %v %v bytes", p1, path.Join("images", "values", filename), fi.Size())
								if thumbnails, err := common.STORAGE.PutImage(p1, path.Join("images", "values", filename), common.Config.Resize.Thumbnail.Size); err == nil {
									images2 = append(images2, strings.Join(thumbnails, ","))
								} else {
									logger.Warningf("%v", err)
								}

							}
						}
					}
				}
			}
		}
		comment.Images = strings.Join(images, ",")
		if err = models.UpdateComment(common.Database, comment); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
		// Cache
		if _, err = models.CreateCacheComment(common.Database, &models.CacheComment{
			CommentID:   comment.ID,
			Title:     comment.Title,
			Body:     comment.Body,
			Max:     comment.Max,
			Images: strings.Join(images2, ";"),
		}); err != nil {
			logger.Warningf("%v", err)
		}
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

type CommentsListResponse struct {
	Data []CommentsListItem
	Filtered int64
	Total int64
}

type CommentsListItem struct {
	ID uint
	Enabled bool
	CreatedAt time.Time
	Title string
	Max float64
	ProductId uint
	ProductTitle string
	ProductThumbnail string
	UserId uint
	UserName string
	UserLastname string
	Author string
}

// @security BasicAuth
// SearchComments godoc
// @Summary Search comments
// @Accept json
// @Produce json
// @Param request body ListRequest true "body"
// @Success 200 {object} CommentsListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/comments/list [post]
// @Tags comment
func postCommentsListHandler(c *fiber.Ctx) error {
	var response CommentsListResponse
	var request ListRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	if len(request.Sort) == 0 {
		request.Sort = make(map[string]string)
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
				case "ProductTitle":
					keys1 = append(keys1, fmt.Sprintf("products.%v like ?", "Title"))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				case "UserName":
					keys1 = append(keys1, fmt.Sprintf("users.%v like ?", "Name"))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				case "UserLastname":
					keys1 = append(keys1, fmt.Sprintf("users.%v like ?", "Lastname"))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				case "Author":
					keys1 = append(keys1, fmt.Sprintf("(users.%v like ? or users.%v like ?)", "Name", "Lastname"))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				case "Max":
					v := strings.TrimSpace(value)
					if strings.Index(v, ">=") == 0 {
						if vv, err := strconv.Atoi(v[2:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("comments.%v >= ?", key))
							values1 = append(values1, vv)
						}
					} else if strings.Index(v, "<=") == 0 {
						if vv, err := strconv.Atoi(v[2:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("comments.%v <= ?", key))
							values1 = append(values1, vv)
						}
					} else if strings.Index(v, "!=") == 0 || strings.Index(v, "<>") == 0 {
						if vv, err := strconv.Atoi(v[2:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("comments.%v <> ?", key))
							values1 = append(values1, vv)
						}
					} else if strings.Index(v, ">") == 0 {
						if vv, err := strconv.Atoi(v[1:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("comments.%v > ?", key))
							values1 = append(values1, vv)
						}
					} else if strings.Index(v, "<") == 0 {
						if vv, err := strconv.Atoi(v[1:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("comments.%v < ?", key))
							values1 = append(values1, vv)
						}
					} else {
						if vv, err := strconv.Atoi(v); err == nil {
							keys1 = append(keys1, fmt.Sprintf("comments.%v = ?", key))
							values1 = append(values1, vv)
						}
					}
				default:
					keys1 = append(keys1, fmt.Sprintf("comments.%v like ?", key))
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
					orders = append(orders, fmt.Sprintf("comments.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//logger.Infof("order: %+v", order)
	//
	response.Data = []CommentsListItem{}
	rows, err := common.Database.Debug().Model(&models.Comment{}).Select("comments.ID, comments.created_at as CreatedAt, comments.Enabled, comments.Title, comments.Max, comments.product_id as ProductId, products.Title as ProductTitle, cache_products.Thumbnail as ProductThumbnail, users.Name as UserName, users.Lastname as UserLastname, comments.user_id as UserId").Joins("left join products on comments.product_id = products.id").Joins("left join cache_products on comments.product_id = cache_products.product_id").Joins("left join users on comments.user_id = users.id").Where(strings.Join(keys1, " and "), values1...).Group("comments.id").Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item CommentsListItem
				if err = common.Database.ScanRows(rows, &item); err == nil {
					item.Author = fmt.Sprintf("%s %s", item.UserName, item.UserLastname)
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
	rows, err = common.Database.Debug().Model(&models.Comment{}).Select("comments.ID, comments.Enabled, comments.Title, comments.Max, comments.product_id as ProductId, comments.user_id as UserId").Joins("left join products on comments.product_id = products.id").Joins("left join cache_products on comments.product_id = cache_products.product_id").Joins("left join users on comments.user_id = users.id").Where(strings.Join(keys1, " and "), values1...).Group("comments.id").Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	if len(keys1) > 0 {
		common.Database.Debug().Model(&models.Comment{}).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

type CommentPatchRequest struct {
	Enabled string
}

// @security BasicAuth
// PatchComment godoc
// @Summary patch comment
// @Accept json
// @Produce json
// @Param option body CommentPatchRequest true "body"
// @Param id path int true "Comment ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/comments/{id} [patch]
// @Tags comment
func patchCommentHandler(c *fiber.Ctx) error {
	var request CommentPatchRequest
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
	var comment *models.Comment
	var err error
	if comment, err = models.GetComment(common.Database, int(id)); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	if request.Enabled == "true" {
		comment.Enabled = true
	}else if request.Enabled == "false" {
		comment.Enabled = false
	}
	if err = models.UpdateComment(common.Database, comment); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	return c.JSON(HTTPMessage{"OK"})
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
	comment.Enabled = request.Enabled
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
			logger.Warningf("%+v", err)
		}
	}
	if err := models.DeleteComment(common.Database, comment); err == nil {
		return c.JSON(HTTPMessage{MESSAGE: "OK"})
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}