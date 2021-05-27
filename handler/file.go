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

// Files

type NewFile struct {
	Name string
	File string
}

// @security BasicAuth
// CreateFile godoc
// @Summary Create file
// @Accept multipart/form-data
// @Produce json
// @Param category body NewFile true "body"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/files [post]
// @Tags file
func postFileHandler(c *fiber.Ctx) error {
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				return err
			}
			var name string
			if v, found := data.Value["Name"]; found && len(v) > 0 {
				name = strings.TrimSpace(v[0])
			}
			if v, found := data.File["File"]; found && len(v) > 0 {
				for _, vv := range v {
					if name == "" {
						name = strings.TrimSuffix(vv.Filename, filepath.Ext(vv.Filename))
					}
					file := &models.File{Name: name, Size: vv.Size}
					if id, err := models.CreateFile(common.Database, file); err == nil {
						p := path.Join(dir, "storage", "files")
						if _, err := os.Stat(p); err != nil {
							if err = os.MkdirAll(p, 0755); err != nil {
								logger.Errorf("%v", err)
							}
						}
						filename := fmt.Sprintf("%d-%s%s", id, file.Name, path.Ext(vv.Filename))
						if p := path.Join(p, filename); len(p) > 0 {
							if in, err := vv.Open(); err == nil {
								out, err := os.OpenFile(p, os.O_WRONLY | os.O_CREATE, 0644)
								if err != nil {
									c.Status(http.StatusInternalServerError)
									return c.JSON(HTTPError{err.Error()})
								}
								if _, err := io.Copy(out, in); err != nil {
									c.Status(http.StatusInternalServerError)
									return c.JSON(HTTPError{err.Error()})
								}
								out.Close()
								file.Url = common.Config.Base + "/" + path.Join("files", filename)
								file.Path = "/" + path.Join("files", filename)
								if reader, err := os.Open(p); err == nil {
									defer reader.Close()
									buff := make([]byte, 512)
									if _, err := reader.Read(buff); err == nil {
										file.Type = http.DetectContentType(buff)
									}else{
										logger.Warningf("%v", err)
									}
								}
								if p1 := path.Join(dir, "storage", file.Path); len(p1) > 0 {
									if fi, err := os.Stat(p1); err == nil {
										filename := fmt.Sprintf("%d-%s-%d%v", file.ID, file.Name, fi.ModTime().Unix(), path.Ext(p1))
										location := path.Join("files", filename)
										logger.Infof("Copy %v => %v %v bytes", p1, location, fi.Size())
										if url, err := common.STORAGE.PutFile(p1, path.Join("files", filename)); err == nil {
											// Cache
											if _, err = models.CreateCacheFile(common.Database, &models.CacheFile{
												FileId:   file.ID,
												Name: file.Name,
												File: url,
											}); err != nil {
												logger.Warningf("%v", err)
											}
										} else {
											logger.Warningf("%v", err)
										}
									}
								}
								if err = models.UpdateFile(common.Database, file); err != nil {
									c.Status(http.StatusInternalServerError)
									return c.JSON(HTTPError{err.Error()})
								}
								if v := c.Query("pid"); len(v) > 0 {
									if id, err := strconv.Atoi(v); err == nil {
										if product, err := models.GetProduct(common.Database, id); err == nil {
											if err = models.AddFileToProduct(common.Database, product, file); err != nil {
												logger.Errorf("%v", err.Error())
											}
										}else{
											logger.Errorf("%v", err.Error())
										}
									}else{
										logger.Errorf("%v", err.Error())
									}
								} else if v := c.Query("vid"); len(v) > 0 {
									if id, err := strconv.Atoi(v); err == nil {
										if variation, err := models.GetVariation(common.Database, id); err == nil {
											if err = models.AddFileToVariation(common.Database, variation, file); err != nil {
												logger.Errorf("%v", err.Error())
											}
										}else{
											logger.Errorf("%v", err.Error())
										}
									}else{
										logger.Errorf("%v", err.Error())
									}
								}
								c.Status(http.StatusOK)
								return c.JSON(file)
							}else{
								c.Status(http.StatusInternalServerError)
								return c.JSON(HTTPError{err.Error()})
							}
						}
					}else{
						c.Status(http.StatusInternalServerError)
						return c.JSON(HTTPError{err.Error()})
					}
				}
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Image missed"})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	return c.JSON(HTTPMessage{"OK"})
}

type FilesListResponse struct {
	Data []FilesListItem
	Filtered int64
	Total int64
}

type FilesListItem struct {
	ID uint
	Created time.Time
	Type string
	Path string
	Url string
	File string `json:",omitempty"`
	Name string
	Size int
	Updated time.Time
}

// @security BasicAuth
// SearchFiles godoc
// @Summary Search files
// @Accept json
// @Produce json
// @Param request body ListRequest true "body"
// @Success 200 {object} FilesListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/files/list [post]
// @Tags file
func postFilesListHandler(c *fiber.Ctx) error {
	var response FilesListResponse
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
	var keys2 []string
	if len(request.Filter) > 0 {
		for key, value := range request.Filter {
			if key != "" && len(strings.TrimSpace(value)) > 0 {
				switch key {
				case "Created":
					keys1 = append(keys1, fmt.Sprintf("files.%v = ?", "Created_At"))
					values1 = append(values1, value)
				case "Size":
					v := strings.TrimSpace(value)
					if strings.Index(v, ">=") == 0 {
						if vv, err := strconv.Atoi(v[2:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("%v >= ?", key))
							values1 = append(values1, vv)
						}
					} else if strings.Index(v, "<=") == 0 {
						if vv, err := strconv.Atoi(v[2:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("%v <= ?", key))
							values1 = append(values1, vv)
						}
					} else if strings.Index(v, "!=") == 0 || strings.Index(v, "<>") == 0 {
						if vv, err := strconv.Atoi(v[2:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("%v <> ?", key))
							values1 = append(values1, vv)
						}
					} else if strings.Index(v, ">") == 0 {
						if vv, err := strconv.Atoi(v[1:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("%v > ?", key))
							values1 = append(values1, vv)
						}
					} else if strings.Index(v, "<") == 0 {
						if vv, err := strconv.Atoi(v[1:]); err == nil {
							keys1 = append(keys1, fmt.Sprintf("%v < ?", key))
							values1 = append(values1, vv)
						}
					} else {
						if vv, err := strconv.Atoi(v); err == nil {
							keys1 = append(keys1, fmt.Sprintf("%v = ?", key))
							values1 = append(values1, vv)
						}
					}
				default:
					keys1 = append(keys1, fmt.Sprintf("files.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	//
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				case "Created":
					orders = append(orders, fmt.Sprintf("files.%v %v", "Created_At", value))
				case "Values":
					orders = append(orders, fmt.Sprintf("%v %v", key, value))
				default:
					orders = append(orders, fmt.Sprintf("files.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//logger.Infof("order: %+v", order)
	if v := c.Query("product_id"); v != "" {
		//id, _ = strconv.Atoi(v)
		keys1 = append(keys1, "products_files.product_id = ?")
		values1 = append(values1, v)
		rows, err := common.Database.Debug().Model(&models.File{}).Select("files.ID, files.Created_At as Created, files.Type, files.Name, files.Path, files.Url, cache_files.File as File, files.Size, files.Updated_At as Updated").Joins("left join cache_files on cache_files.file_id = files.id").Joins("left join products_files on products_files.file_id = files.id").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
		if err == nil {
			if err == nil {
				for rows.Next() {
					var item FilesListItem
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
		rows, err = common.Database.Debug().Model(&models.File{}).Select("files.ID, files.Created_At as Created, files.Type, files.Name, files.Path, files.Url, cache_files.File as File, files.Size, files.Updated_At as Updated").Joins("left join cache_files on cache_files.file_id = files.id").Joins("left join products_files on products_files.file_id = files.id").Where(strings.Join(keys1, " and "), values1...).Rows()
		if err == nil {
			for rows.Next() {
				response.Filtered ++
			}
			rows.Close()
		}
		if len(keys1) > 0 || len(keys2) > 0 {
			common.Database.Debug().Model(&models.File{}).Select("files.ID").Joins("left join products_files on products_files.file_id = files.id").Where("products_files.product_id = ?", v).Count(&response.Total)
		}else{
			response.Total = response.Filtered
		}
	}else{
		rows, err := common.Database.Debug().Model(&models.File{}).Select("files.ID, files.Created_At as Created, files.Type, files.Name, files.Path, files.Url, cache_files.File as File, files.Size, files.Updated_At as Updated").Joins("left join cache_files on cache_files.file_id = files.id").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
		if err == nil {
			if err == nil {
				for rows.Next() {
					var item FilesListItem
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
		rows, err = common.Database.Debug().Model(&models.File{}).Select("files.ID, files.Created_At as Created, files.Type, files.Name, files.Path, files.Url, cache_files.File as File, files.Size, files.Updated_At as Updated").Joins("left join cache_files on cache_files.file_id = files.id").Where(strings.Join(keys1, " and "), values1...).Rows()
		if err == nil {
			for rows.Next() {
				response.Filtered ++
			}
			rows.Close()
		}
		if len(keys1) > 0 || len(keys2) > 0 {
			common.Database.Debug().Model(&models.File{}).Count(&response.Total)
		}else{
			response.Total = response.Filtered
		}
	}
	//
	c.Status(http.StatusOK)
	return c.JSON(response)
}

type File2View struct {
	ID uint
	CreatedAt time.Time `json:",omitempty"`
	Type string `json:",omitempty"`
	Name string `json:",omitempty"`
	Path string `json:",omitempty"`
	Url string `json:",omitempty"`
	File string `json:",omitempty"`
	Size int `json:",omitempty"`
	Updated time.Time `json:",omitempty"`
}

// @security BasicAuth
// GetFile godoc
// @Summary Get file
// @Accept json
// @Produce json
// @Param id path int true "File ID"
// @Success 200 {object} File2View
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/files/{id} [get]
// @Tags file
func getFileHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if file, err := models.GetFile(common.Database, id); err == nil {
		var view File2View
		if bts, err := json.MarshalIndent(file, "", "   "); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				if cache, err := models.GetCacheFileByFileId(common.Database, file.ID); err == nil {
					view.File = cache.File
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
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

type ExistingFile struct {

}

// @security BasicAuth
// UpdateFile godoc
// @Summary Update file
// @Accept json
// @Produce json
// @Param file body ExistingFile true "body"
// @Param id path int true "File ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/files/{id} [put]
// @Tags file
func putFileHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var file *models.File
	var err error
	if file, err = models.GetFile(common.Database, id); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	if file.Path == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"File does not exists, please create new"})
	}
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				return err
			}
			if v := data.Value["Name"]; len(v) > 0 {
				file.Name = strings.TrimSpace(v[0])
			}
			if v, found := data.File["File"]; found && len(v) > 0 {
				p := path.Dir(path.Join(dir, "storage", file.Path))
				if _, err := os.Stat(p); err != nil {
					if err = os.MkdirAll(p, 0755); err != nil {
						logger.Errorf("%v", err)
					}
				}
				if file.Name == "" {
					file.Name = strings.TrimSuffix(v[0].Filename, filepath.Ext(v[0].Filename))
				}
				file.Size = v[0].Size
				//
				filename := fmt.Sprintf("%d-%s%s", id, regexp.MustCompile(`(?i)[^-a-z0-9]+`).ReplaceAllString(file.Name, "-"), path.Ext(v[0].Filename))
				if p := path.Join(p, filename); len(p) > 0 {
					if in, err := v[0].Open(); err == nil {
						out, err := os.OpenFile(p, os.O_WRONLY | os.O_CREATE, 0644)
						if err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
						if _, err := io.Copy(out, in); err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
						out.Close()
						file.Path = path.Join(path.Dir(file.Path), filename)
						file.Url = common.Config.Base + path.Join(path.Dir(strings.Replace(file.Path, "/hugo/", "/", 1)), filename)
						//
						if reader, err := os.Open(p); err == nil {
							defer reader.Close()
							buff := make([]byte, 512)
							if _, err := reader.Read(buff); err == nil {
								file.Type = http.DetectContentType(buff)
							}
						}
						//
						if p1 := path.Join(dir, "storage", file.Path); len(p1) > 0 {
							if fi, err := os.Stat(p1); err == nil {
								filename := fmt.Sprintf("%d-%s-%d%v", file.ID, file.Name, fi.ModTime().Unix(), path.Ext(p1))
								location := path.Join("files", filename)
								logger.Infof("Copy %v => %v %v bytes", p1, location, fi.Size())
								if url, err := common.STORAGE.PutFile(p1, path.Join("files", filename)); err == nil {
									file.Url = url
									// Cache
									if err = models.DeleteCacheFileByFileId(common.Database, file.ID); err != nil {
										logger.Warningf("%v", err)
									}
									if _, err = models.CreateCacheFile(common.Database, &models.CacheFile{
										FileId:   file.ID,
										Name: file.Name,
										File: url,
									}); err != nil {
										logger.Warningf("%v", err)
									}
								} else {
									logger.Warningf("%v", err)
								}
							}
						}
					}
				}
			}
			if err = models.UpdateFile(common.Database, file); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			//
			return c.JSON(HTTPMessage{"OK"})
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	c.Status(http.StatusInternalServerError)
	return c.JSON(HTTPError{"Something went wrong"})
}

// @security BasicAuth
// DelFile godoc
// @Summary Delete file
// @Accept json
// @Produce json
// @Param id path int true "File ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/files/{id} [delete]
// @Tags file
func delFileHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
		if file, err := models.GetFile(common.Database, id); err == nil {
			if err = os.Remove(path.Join(dir, file.Path)); err != nil {
				logger.Errorf("%v", err.Error())
			}
			if err = models.DeleteFile(common.Database, file); err == nil {
				return c.JSON(HTTPMessage{MESSAGE: "OK"})
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "File ID is not defined"})
	}
}