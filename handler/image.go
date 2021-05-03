package handler

import (
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/google/logger"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/models"
	"image"
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

// Images

type NewImage struct {
	Name string
	Image string
}

// @security BasicAuth
// CreateImage godoc
// @Summary Create image
// @Accept multipart/form-data
// @Produce json
// @Param pid query int false "Products id"
// @Param image body NewImage true "body"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/images [post]
// @Tags image
func postImageHandler(c *fiber.Ctx) error {
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
			if v, found := data.File["Image"]; found && len(v) > 0 {
				for _, vv := range v {
					if name == "" {
						name = strings.TrimSuffix(vv.Filename, filepath.Ext(vv.Filename))
					}
					img := &models.Image{Name: name, Size: vv.Size}
					if id, err := models.CreateImage(common.Database, img); err == nil {
						p := path.Join(dir, "storage", "images")
						if _, err := os.Stat(p); err != nil {
							if err = os.MkdirAll(p, 0755); err != nil {
								logger.Errorf("%v", err)
							}
						}
						filename := fmt.Sprintf("%d-%s%s", id, img.Name, path.Ext(vv.Filename))
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
								img.Url = common.Config.Base + "/" + path.Join("images", filename)
								img.Path = "/" + path.Join("images", filename)
								if reader, err := os.Open(p); err == nil {
									defer reader.Close()
									if config, _, err := image.DecodeConfig(reader); err == nil {
										img.Height = config.Height
										img.Width = config.Width
									} else {
										logger.Errorf("%v", err.Error())
									}
								}
								if err = models.UpdateImage(common.Database, img); err != nil {
									c.Status(http.StatusInternalServerError)
									return c.JSON(HTTPError{err.Error()})
								}
								if v := c.Query("pid"); len(v) > 0 {
									if id, err := strconv.Atoi(v); err == nil {
										if product, err := models.GetProductFull(common.Database, id); err == nil {
											if err = models.AddImageToProduct(common.Database, product, img); err != nil {
												logger.Errorf("%v", err.Error())
											}
											// Images processing
											if img.Path != "" {
												if p1 := path.Join(dir, "storage", img.Path); len(p1) > 0 {
													if fi, err := os.Stat(p1); err == nil {
														name := product.Name
														if len(name) > 32 {
															name = name[:32]
														}
														filename := fmt.Sprintf("%d-%s-%d%v", img.ID, name, fi.ModTime().Unix(), path.Ext(p1))
														logger.Infof("Copy %v => %v %v bytes", p1, path.Join("images", "products", filename), fi.Size())
														if thumbnails, err := common.STORAGE.PutImage(p1, path.Join("images", "products", filename), common.Config.Resize.Image.Size); err == nil {
															// Cache
															if _, err = models.CreateCacheImage(common.Database, &models.CacheImage{
																ImageId:   img.ID,
																Name: img.Name,
																Thumbnail: strings.Join(thumbnails, ","),
															}); err != nil {
																logger.Warningf("%v", err)
															}
														} else {
															logger.Warningf("%v", err)
														}
													}
												}
											}
											//
											if product.Image == nil {
												product.ImageId = img.ID
												if err = models.UpdateProduct(common.Database, product); err != nil {
													logger.Warningf("%+v", err)
												}
											}
										}else{
											logger.Errorf("%v", err.Error())
										}
									}else{
										logger.Errorf("%v", err.Error())
									}
								}else if v := c.Query("vid"); len(v) > 0 {
									if id, err := strconv.Atoi(v); err == nil {
										if variation, err := models.GetVariation(common.Database, id); err == nil {
											if err = models.AddImageToVariation(common.Database, variation, img); err != nil {
												logger.Errorf("%v", err.Error())
											}
											var product *models.Product
											if product, err = models.GetProduct(common.Database, int(variation.ProductId)); err != nil {
												logger.Errorf("%v", err.Error())
											}
											// Images processing
											if len(variation.Images) > 0 {
												for _, image := range variation.Images {
													if image.Path != "" {
														if p1 := path.Join(dir, "storage", image.Path); len(p1) > 0 {
															if fi, err := os.Stat(p1); err == nil {
																name := variation.Name
																if product != nil {
																	name = product.Name + "-" + name
																}
																if len(name) > 32 {
																	name = name[:32]
																}
																filename := fmt.Sprintf("%d-%s-%d%v", image.ID, name, fi.ModTime().Unix(), path.Ext(p1))
																p2 := path.Join(dir, "hugo", "static", "images", "variations", filename)
																logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
																if _, err := os.Stat(path.Dir(p2)); err != nil {
																	if err = os.MkdirAll(path.Dir(p2), 0755); err != nil {
																		logger.Warningf("%v", err)
																	}
																}
															}
														}
													}
												}
											}
										}else{
											logger.Errorf("%v", err.Error())
										}
									}else{
										logger.Errorf("%v", err.Error())
									}
								}
								c.Status(http.StatusOK)
								return c.JSON(img)
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

type ImagesListResponse struct {
	Data []ImagesListItem
	Filtered int64
	Total int64
}

type ImagesListItem struct {
	ID uint
	Created time.Time
	Path string
	Thumbnail string
	Name string
	Height int
	Width int
	Size int
	Updated time.Time
}

// @security BasicAuth
// SearchImages godoc
// @Summary Search images
// @Accept json
// @Produce json
// @Param request body ListRequest true "body"
// @Success 200 {object} ImagesListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/images/list [post]
// @Tags image
func postImagesListHandler(c *fiber.Ctx) error {
	var response ImagesListResponse
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
				case "Height":
				case "Weight":
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
					keys1 = append(keys1, fmt.Sprintf("images.%v like ?", key))
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
				case "Values":
					orders = append(orders, fmt.Sprintf("%v %v", key, value))
				default:
					orders = append(orders, fmt.Sprintf("images.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//logger.Infof("order: %+v", order)
	if v := c.Query("product_id"); v != "" {
		//id, _ = strconv.Atoi(v)
		keys1 = append(keys1, "products_images.product_id = ?")
		values1 = append(values1, v)
		rows, err := common.Database.Debug().Model(&models.Image{}).Select("images.ID, images.Created_At as Created, images.Name, images.Path, cache_images.Thumbnail as Thumbnail, images.Height, images.Width, images.Size, images.Updated_At as Updated").Joins("left join cache_products on images.id = cache_products.image_id").Joins("left join products_images on products_images.image_id = images.id").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
		if err == nil {
			if err == nil {
				for rows.Next() {
					var item ImagesListItem
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
		rows, err = common.Database.Debug().Model(&models.Image{}).Select("images.ID, images.Created_At as Created, images.Name, images.Path, images.Height, images.Width, images.Size, images.Updated_At as Updated").Joins("left join products_images on products_images.image_id = images.id").Where(strings.Join(keys1, " and "), values1...).Rows()
		if err == nil {
			for rows.Next() {
				response.Filtered ++
			}
			rows.Close()
		}
		if len(keys1) > 0 || len(keys2) > 0 {
			common.Database.Debug().Model(&models.Image{}).Select("images.ID").Joins("left join products_images on products_images.image_id = images.id").Where("products_images.product_id = ?", v).Count(&response.Total)
		}else{
			response.Total = response.Filtered
		}
	}else{
		rows, err := common.Database.Debug().Model(&models.Image{}).Select("images.ID, images.Created_At as Created, images.Name, images.Path, cache_images.Thumbnail as Thumbnail, images.Height, images.Width, images.Size, images.Updated_At as Updated").Joins("left join cache_images on images.id = cache_images.image_id").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
		if err == nil {
			if err == nil {
				for rows.Next() {
					var item ImagesListItem
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
		rows, err = common.Database.Debug().Model(&models.Image{}).Select("images.ID, images.Created_At as Created, images.Name, images.Path, images.Height, images.Width, images.Size, images.Updated_At as Updated").Where(strings.Join(keys1, " and "), values1...).Rows()
		if err == nil {
			for rows.Next() {
				response.Filtered ++
			}
			rows.Close()
		}
		if len(keys1) > 0 || len(keys2) > 0 {
			common.Database.Debug().Model(&models.Image{}).Count(&response.Total)
		}else{
			response.Total = response.Filtered
		}
	}
	//

	c.Status(http.StatusOK)
	return c.JSON(response)
}

type ImageView struct {
	ID uint
	CreatedAt time.Time `json:",omitempty"`
	Name string `json:",omitempty"`
	Thumbnail string `json:",omitempty"`
	Path string `json:",omitempty"`
	Url string `json:",omitempty"`
	Height int `json:",omitempty"`
	Width int `json:",omitempty"`
	Size int `json:",omitempty"`
	Updated time.Time `json:",omitempty"`
}

// @security BasicAuth
// GetOption godoc
// @Summary Get image
// @Accept json
// @Produce json
// @Param id path int true "Image ID"
// @Success 200 {object} ImageView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/images/{id} [get]
// @Tags image
func getImageHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if option, err := models.GetImage(common.Database, id); err == nil {
		var view ImageView
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

type ExistingImage struct {

}

// @security BasicAuth
// UpdateImage godoc
// @Summary Update image
// @Accept json
// @Produce json
// @Param image body ExistingImage true "body"
// @Param id path int true "Image ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/images/{id} [put]
// @Tags image
func putImageHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var img *models.Image
	var err error
	if img, err = models.GetImage(common.Database, id); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	if img.Path == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Image does not exists, please create new"})
	}
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				return err
			}
			if v, found := data.File["Image"]; found && len(v) > 0 {
				p := path.Dir(path.Join(dir, "storage", img.Path))
				if _, err := os.Stat(p); err != nil {
					if err = os.MkdirAll(p, 0755); err != nil {
						logger.Errorf("%v", err)
					}
				}
				if err = os.Remove(img.Path); err != nil {
					logger.Errorf("%v", err)
				}
				if img.Name == "" {
					img.Name = strings.TrimSuffix(v[0].Filename, filepath.Ext(v[0].Filename))
				}
				img.Size = v[0].Size
				//
				filename := fmt.Sprintf("%d-%s-%d%s", id, regexp.MustCompile(`(?i)[^-a-z0-9]+`).ReplaceAllString(img.Name, "-"), time.Now().Unix(), path.Ext(v[0].Filename))
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
						img.Path = path.Join(path.Dir(img.Path), filename)
						img.Url = common.Config.Base + path.Join(path.Dir(strings.Replace(img.Path, "/hugo/", "/", 1)), filename)
						//
						if reader, err := os.Open(p); err == nil {
							defer reader.Close()
							if config, _, err := image.DecodeConfig(reader); err == nil {
								img.Height = config.Height
								img.Width = config.Width
							} else {
								logger.Errorf("%v", err.Error())
							}
						}
					}
				}
			}
			if err = models.UpdateImage(common.Database, img); err != nil {
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
// DelImage godoc
// @Summary Delete image
// @Accept json
// @Produce json
// @Param id path int true "Image ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/images/{id} [delete]
// @Tags image
func delImageHandler(c *fiber.Ctx) error {
	var oid int
	if v := c.Params("id"); v != "" {
		oid, _ = strconv.Atoi(v)
		if image, err := models.GetImage(common.Database, oid); err == nil {
			if err = os.Remove(path.Join(dir, "storage", image.Path)); err != nil {
				logger.Errorf("%v", err.Error())
			}
			name := fmt.Sprintf("%d-", image.ID)
			if err = filepath.Walk(path.Join(dir, "hugo", "static", "images", "products"), func(p string, fi os.FileInfo, err error) error {
				if err == nil && !fi.IsDir() {
					if strings.Index(fi.Name(), name) == 0 {
						if err = os.Remove(p); err != nil {
							logger.Warningf("%+v", err)
						}
					}
				}
				return nil
			}); err != nil {
				logger.Warningf("%+v", err)
			}
			if err = models.DeleteImage(common.Database, image); err == nil {
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
		return c.JSON(fiber.Map{"ERROR": "Image ID is not defined"})
	}
}
