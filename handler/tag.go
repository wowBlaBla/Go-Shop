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
)

// Tags

type TagsView []TagView

type TagView struct{
	ID uint
	Enabled bool
	Name string
	Title string
	Thumbnail string `json:",omitempty"`
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
	data, err := c.Request().MultipartForm()
	if err != nil {
		return err
	}
	var enabled bool
	if v, found := data.Value["Enabled"]; found && len(v) > 0 {
		enabled, _ = strconv.ParseBool(v[0])
	}
	var title string
	if v, found := data.Value["Title"]; found && len(v) > 0 {
		title = strings.TrimSpace(v[0])
	}
	if title == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Title is not defined"})
	}
	var name string
	if v, found := data.Value["Name"]; found && len(v) > 0 {
		name = strings.TrimSpace(v[0])
	}
	if name == "" {
		name = reNotAbc.ReplaceAllString(strings.ToLower(name), "-")
	}
	var description string
	if v, found := data.Value["Description"]; found && len(v) > 0 {
		description = strings.TrimSpace(v[0])
	}
	if len(description) > 256 {
		description = description[0:255]
	}
	if tags, err := models.GetTagsByName(common.Database, name); err == nil && len(tags) > 0 {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Tag exists"})
	}
	tag := &models.Tag{Enabled: enabled, Title: title, Name: name, Description: description}
	if id, err := models.CreateTag(common.Database, tag); err == nil {
		if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
			p := path.Join(dir, "storage", "tags")
			if _, err := os.Stat(p); err != nil {
				if err = os.MkdirAll(p, 0755); err != nil {
					logger.Errorf("%v", err)
				}
			}
			filename := fmt.Sprintf("%d-%s-thumbnail%s", id, regexp.MustCompile(`(?i)[^-a-z0-9]+`).ReplaceAllString(tag.Title, "-"), path.Ext(v[0].Filename))
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
					tag.Thumbnail = "/" + path.Join("tags", filename)
					if err = models.UpdateTag(common.Database, tag); err != nil {
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
					//
					if p1 := path.Join(dir, "storage", "tags", filename); len(p1) > 0 {
						if fi, err := os.Stat(p1); err == nil {
							filename := filepath.Base(p1)
							filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
							logger.Infof("Copy %v => %v %v bytes", p1, path.Join("images", "tags", filename), fi.Size())
							var paths string
							if thumbnails, err := common.STORAGE.PutImage(p1, path.Join("images", "tags", filename), common.Config.Resize.Thumbnail.Size); err == nil {
								paths = strings.Join(thumbnails, ",")
							} else {
								logger.Warningf("%v", err)
							}
							// Cache
							if _, err = models.CreateCacheTag(common.Database, &models.CacheTag{
								TagID:   tag.ID,
								Title:     tag.Title,
								Name:     tag.Name,
								Thumbnail: paths,
							}); err != nil {
								logger.Warningf("%v", err)
							}
						}
					}
				}
			}
		}
		if bts, err := json.Marshal(tag); err == nil {
			if err = json.Unmarshal(bts, &view); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
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
	Thumbnail string
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
	rows, err := common.Database.Debug().Model(&models.Tag{}).Select("tags.ID, tags.Enabled, tags.Hidden, tags.Name, tags.Title, cache_tags.Thumbnail as Thumbnail, tags.Description").Joins("left join cache_tags on cache_tags.tag_id = tags.ID").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
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
	rows, err = common.Database.Debug().Model(&models.Tag{}).Select("tags.ID, tags.Enabled, tags.Hidden, tags.Name, tags.Title, cache_tags.Thumbnail as Thumbnail, tags.Description").Joins("left join cache_tags on cache_tags.tag_id = tags.ID").Where(strings.Join(keys1, " and "), values1...).Rows()
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
	var tag *models.Tag
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
		var err error
		if tag, err = models.GetTag(common.Database, id); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Tag ID is not defined"})
	}
	var view TagView
	if bts, err := json.Marshal(tag); err == nil {
		if err = json.Unmarshal(bts, &view); err == nil {
			if cache, err := models.GetCacheTagByTagId(common.Database, tag.ID); err == nil {
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
	var view TagView
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
	data, err := c.Request().MultipartForm()
	if err != nil {
		return err
	}
	var enabled bool
	if v, found := data.Value["Enabled"]; found && len(v) > 0 {
		enabled, _ = strconv.ParseBool(v[0])
	}
	var title string
	if v, found := data.Value["Title"]; found && len(v) > 0 {
		title = strings.TrimSpace(v[0])
	}
	if title == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Title is not defined"})
	}
	var name string
	if v, found := data.Value["Name"]; found && len(v) > 0 {
		name = strings.TrimSpace(v[0])
	}
	if name == "" {
		name = reNotAbc.ReplaceAllString(strings.ToLower(name), "-")
	}
	var description string
	if v, found := data.Value["Description"]; found && len(v) > 0 {
		description = strings.TrimSpace(v[0])
	}
	if len(description) > 256 {
		description = description[0:255]
	}
	tag.Enabled = enabled
	tag.Title = title
	tag.Name = name
	tag.Description = description
	if v, found := data.Value["Thumbnail"]; found && len(v) > 0 && v[0] == "" {
		// To delete existing
		if tag.Thumbnail != "" {
			if err = os.Remove(path.Join(dir, tag.Thumbnail)); err != nil {
				logger.Errorf("%v", err)
			}
			tag.Thumbnail = ""
		}
	}else if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
		p := path.Join(dir, "storage", "tags")
		if _, err := os.Stat(p); err != nil {
			if err = os.MkdirAll(p, 0755); err != nil {
				logger.Errorf("%v", err)
			}
		}
		filename := fmt.Sprintf("%d-%s-thumbnail%s", id, regexp.MustCompile(`(?i)[^-a-z0-9]+`).ReplaceAllString(tag.Title, "-"), path.Ext(v[0].Filename))
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
				tag.Thumbnail = "/" + path.Join("tags", filename)
				if err = models.UpdateTag(common.Database, tag); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
				//
				if p1 := path.Join(dir, "storage", "tags", filename); len(p1) > 0 {
					if fi, err := os.Stat(p1); err == nil {
						filename := filepath.Base(p1)
						filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
						logger.Infof("Copy %v => %v %v bytes", p1, path.Join("images", "tags", filename), fi.Size())
						var paths string
						if thumbnails, err := common.STORAGE.PutImage(p1, path.Join("images", "tags", filename), common.Config.Resize.Thumbnail.Size); err == nil {
							paths = strings.Join(thumbnails, ",")
						} else {
							logger.Warningf("%v", err)
						}
						// Cache
						if err = models.DeleteCacheTagByTagId(common.Database, tag.ID); err != nil {
							logger.Warningf("%v", err)
						}
						if _, err = models.CreateCacheTag(common.Database, &models.CacheTag{
							TagID:   tag.ID,
							Title:     tag.Title,
							Name:     tag.Name,
							Thumbnail: paths,
						}); err != nil {
							logger.Warningf("%v", err)
						}
					}
				}
			}
		}
	}
	//
	if err := models.UpdateTag(common.Database, tag); err == nil {
		if bts, err := json.Marshal(tag); err == nil {
			if err = json.Unmarshal(bts, &view); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	return c.JSON(view)
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
		if tag.Thumbnail != "" {
			//common.STORAGE.DeleteImage()
		}
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