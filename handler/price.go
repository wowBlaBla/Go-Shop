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
	"sort"
	"strconv"
	"strings"
	"time"
)

type NewPrice struct {
	Enabled bool
	ID uint
	ProductId uint
	VariationId uint
	//RateIds string
	Rates []*models.Rate // ID matter
	Price float64
	Availability string
	Sending string
	Sku string
	Stock uint
}

type PriceView struct {
	ID uint
	Enabled bool
	ProductId uint `json:",omitempty"`
	VariationId uint `json:",omitempty"`
	//RateIds string
	Rates []*RateView
	Thumbnail string `json:",omitempty"`
	Price float64
	Prices []*PriceView
	Availability string
	Sending string
	Sku string
	Stock uint
}

// @security BasicAuth
// CreatePrice godoc
// @Summary Create prices
// @Accept json
// @Produce json
// @Param product_id query int true "Product id"
// @Param variation_id query int true "Variation id"
// @Param price body NewPrice true "body"
// @Success 200 {object} PriceView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/prices [post]
// @Tags price
func postPriceHandler(c *fiber.Ctx) error {
	var view PriceView
	var request NewPrice
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	//
	price := &models.Price{
		Enabled: request.Enabled,
		Price: request.Price,
		Availability: request.Availability,
		Sending: request.Sending,
		Sku: request.Sku,
		Stock: request.Stock,
	}
	if request.VariationId > 0 {
		price.VariationId = request.VariationId
	}else if request.ProductId > 0 {
		price.ProductId = request.ProductId
	}
	for _, rate := range request.Rates {
		if rate.ID > 0 {
			price.Rates = append(price.Rates, rate)
		}
	}
	//
	if _, err := models.CreatePrice(common.Database, price); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	if bts, err := json.Marshal(price); err == nil {
		if err = json.Unmarshal(bts, &view); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}
	return c.JSON(view)
}

type NewPrices []NewPrice

// @security BasicAuth
// CreatePrices godoc
// @Summary Create prices
// @Accept json
// @Produce json
// @Param price body NewPrices true "body"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/prices/all [post]
// @Tags price
func postPriceAllHandler(c *fiber.Ctx) error {
	var requests []NewPrice
	if err := c.BodyParser(&requests); err != nil {
		return err
	}
	//
	for _, request := range requests {
		price := &models.Price{
			Enabled: request.Enabled,
			Price: request.Price,
			Availability: request.Availability,
			Sending: request.Sending,
			Sku: request.Sku,
			Stock: request.Stock,
		}
		if request.VariationId > 0 {
			price.VariationId = request.VariationId
		}else if request.ProductId > 0 {
			price.ProductId = request.ProductId
		}
		for _, rate := range request.Rates {
			if rate.ID > 0 {
				price.Rates = append(price.Rates, rate)
			}
		}
		//
		var err error
		if _, err = models.CreatePrice(common.Database, price); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}
	return c.JSON(HTTPMessage{"OK"})
}

type ExistingPrice struct {
	ID uint
	Price float64
	Availability string
	Sending string
	Sku string
	Stock uint
}

type ExistingPrices []ExistingPrice

// @security BasicAuth
// UpdatePrices godoc
// @Summary Update prices
// @Accept json
// @Produce json
// @Param price body NewPrices true "body"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/prices/all [put]
// @Tags price
func putPriceAllHandler(c *fiber.Ctx) error {
	var requests []ExistingPrice
	if err := c.BodyParser(&requests); err != nil {
		return err
	}
	//
	for _, request := range requests {
		if price, err := models.GetPrice(common.Database, int(request.ID)); err == nil {
			price.Price = request.Price
			price.Availability = request.Availability
			price.Sending = request.Sending
			price.Sku = request.Sku
			price.Stock = request.Stock
			//
			var err error
			if err = models.UpdatePrice(common.Database, price); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			logger.Warningf("%+v", err)
		}
	}
	return c.JSON(HTTPMessage{"OK"})
}

type PricesView []*PriceView

// @security BasicAuth
// GetPrices godoc
// @Summary Get prices
// @Accept json
// @Produce json
// @Param product_id path int true "Product ID"
// @Param variation_id path int true "Variation ID"
// @Success 200 {object} PriceView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/prices [get]
// @Tags price
func getPricesHandler(c *fiber.Ctx) error {
	var pid int
	if v := c.Query("product_id"); v != "" {
		pid, _ = strconv.Atoi(v)
	}
	var vid int
	if v := c.Query("variation_id"); v != "" {
		vid, _ = strconv.Atoi(v)
	}
	var prices []*models.Price
	var err error
	if pid > 0 {
		if prices, err = models.GetPricesByProductId(common.Database, uint(pid)); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else if vid > 0 {
		if prices, err = models.GetPricesByVariationId(common.Database, uint(vid)); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Either product_id or variation_id required"})
	}
	var view PricesView
	if bts, err := json.MarshalIndent(prices, "", "   "); err == nil {
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
}

// @security BasicAuth
// GetPrice godoc
// @Summary Get price
// @Accept json
// @Produce json
// @Param id path int true "Price ID"
// @Success 200 {object} PriceView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/prices/{id} [get]
// @Tags price
func getPriceHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if price, err := models.GetPrice(common.Database, id); err == nil {
		var view PriceView
		if bts, err := json.MarshalIndent(price, "", "   "); err == nil {
			if err = json.Unmarshal(bts, &view); err == nil {
				if cache, err := models.GetCachePriceByPriceId(common.Database, price.ID); err == nil {
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
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// @security BasicAuth
// UpdatePrice godoc
// @Summary Update price
// @Accept json
// @Produce json
// @Param id path int true "Price ID"
// @Param request body NewPrice true "body"
// @Success 200 {object} PriceView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/prices/{id} [put]
// @Tags price
func putPriceHandler(c *fiber.Ctx) error {
	var view PriceView
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var price *models.Price
	var err error
	if price, err = models.GetPrice(common.Database, id); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewPrice
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			price.Price = request.Price
			price.Availability = request.Availability
			price.Sending = request.Sending
			price.Sku = request.Sku
			price.Stock = request.Stock
			if err = models.UpdatePrice(common.Database, price); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if bts, err := json.Marshal(price); err == nil {
				if err = json.Unmarshal(bts, &view); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}
		} else if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if v, found := data.Value["Price"]; found && len(v) > 0 {
				if vv, _ := strconv.ParseFloat(v[0], 10); err == nil {
					price.Price = vv
				}
			}
			if v, found := data.Value["Availability"]; found && len(v) > 0 {
				price.Availability = v[0]
			}
			if v, found := data.Value["Sending"]; found && len(v) > 0 {
				price.Sending = v[0]
			}
			if v, found := data.Value["Sku"]; found && len(v) > 0 {
				price.Sku = v[0]
			}
			if v, found := data.Value["Stock"]; found && len(v) > 0 {
				if vv, _ := strconv.Atoi(v[0]); err == nil {
					price.Stock = uint(vv)
				}
			}
			var ids []int
			price.Rates = []*models.Rate{}
			if v, found := data.Value["Rates"]; found && len(v) > 0 {
				for _, v := range strings.Split(strings.TrimSpace(v[0]), ","){
					if vv, err := strconv.Atoi(v); err == nil {
						ids = append(ids, vv)
						if rate, err := models.GetRate(common.Database, vv); err == nil {
							price.Rates = append(price.Rates, rate)
						}
					}
				}
			}
			if bts, err := json.Marshal(price); err == nil {
				logger.Infof("Price: %+v", string(bts))
			}
			sort.Ints(ids)
			var ids2 []string
			for _, id := range ids {
				ids2 = append(ids2, strconv.Itoa(id))
			}
			if err = models.UpdatePrice(common.Database, price); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if v, found := data.Value["Thumbnail"]; found && len(v) > 0 && v[0] == "" {
				// To delete existing
				if price.Thumbnail != "" {
					if err = os.Remove(path.Join(dir, price.Thumbnail)); err != nil {
						logger.Errorf("%v", err)
					}
					price.Thumbnail = ""
				}
			}else if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
				p := path.Join(dir, "storage", "prices")
				if _, err := os.Stat(p); err != nil {
					if err = os.MkdirAll(p, 0755); err != nil {
						logger.Errorf("%v", err)
					}
				}
				filename := fmt.Sprintf("%d-%s-thumbnail%s", id, strings.Join(ids2, "-"), path.Ext(v[0].Filename))
				if p := path.Join(p, filename); len(p) > 0 {
					if in, err := v[0].Open(); err == nil {
						var mod time.Time
						if fi, err := os.Stat(p); err == nil {
							mod = fi.ModTime()
						}
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
						price.Thumbnail = "/" + path.Join("prices", filename)
						if err = models.UpdatePrice(common.Database, price); err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
						//
						if p1 := path.Join(dir, "storage", "prices", filename); len(p1) > 0 {
							if fi, err := os.Stat(p1); err == nil {
								filename := filepath.Base(p1)
								if mod.IsZero() {
									mod = fi.ModTime()
								}
								filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], mod.Unix(), filepath.Ext(filename))
								logger.Infof("Copy %v => %v %v bytes", p1, path.Join("images", "prices", filename), fi.Size())
								var paths string
								if thumbnails, err := common.STORAGE.PutImage(p1, path.Join("images", "prices", filename), common.Config.Resize.Thumbnail.Size); err == nil {
									paths = strings.Join(thumbnails, ",")
								} else {
									logger.Warningf("%v", err)
								}
								// Cache
								if err = models.DeleteCachePriceByPriceId(common.Database, price.ID); err != nil {
									logger.Warningf("%v", err)
								}
								if _, err = models.CreateCachePrice(common.Database, &models.CachePrice{
									PriceID: price.ID,
									Thumbnail: paths,
								}); err != nil {
									logger.Warningf("%v", err)
								}
							}
						}
					}
				}
			}

			if bts, err := json.Marshal(price); err == nil {
				if err = json.Unmarshal(bts, &view); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}
		}
	}
	//

	return c.JSON(view)
}

func patchPriceHandler(c *fiber.Ctx) error {
	var view PriceView
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var price *models.Price
	var err error
	if price, err = models.GetPrice(common.Database, id); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}

	if action := c.Query("action", ""); action != "" {
		switch action {
		case "setThumbnail":
			data, err := c.Request().MultipartForm()
			if err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			var ids []int
			for _, rate := range price.Rates {
				ids = append(ids, int(rate.ID))
			}
			sort.Ints(ids)
			var ids2 []string
			for _, id := range ids {
				ids2 = append(ids2, strconv.Itoa(id))
			}
			var thumbnail string
			if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
				p := path.Join(dir, "storage", "prices")
				if _, err := os.Stat(p); err != nil {
					if err = os.MkdirAll(p, 0755); err != nil {
						logger.Errorf("%v", err)
					}
				}
				filename := fmt.Sprintf("%d-%s-thumbnail%s", id, strings.Join(ids2, "-"), path.Ext(v[0].Filename))
				if p := path.Join(p, filename); len(p) > 0 {
					if in, err := v[0].Open(); err == nil {
						var mod time.Time
						if fi, err := os.Stat(p); err == nil {
							mod = fi.ModTime()
						}
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
						price.Thumbnail = "/" + path.Join("prices", filename)
						if err = models.UpdatePrice(common.Database, price); err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
						//
						if p1 := path.Join(dir, "storage", "prices", filename); len(p1) > 0 {
							if fi, err := os.Stat(p1); err == nil {
								filename := filepath.Base(p1)
								if mod.IsZero() {
									mod = fi.ModTime()
								}
								filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], mod.Unix(), filepath.Ext(filename))
								logger.Infof("Copy %v => %v %v bytes", p1, path.Join("images", "prices", filename), fi.Size())
								if thumbnails, err := common.STORAGE.PutImage(p1, path.Join("images", "prices", filename), common.Config.Resize.Thumbnail.Size); err == nil {
									thumbnail = strings.Join(thumbnails, ",")
								} else {
									logger.Warningf("%v", err)
								}
								// Cache
								if err = models.DeleteCachePriceByPriceId(common.Database, price.ID); err != nil {
									logger.Warningf("%v", err)
								}
								if _, err = models.CreateCachePrice(common.Database, &models.CachePrice{
									PriceID: price.ID,
									Thumbnail: thumbnail,
								}); err != nil {
									logger.Warningf("%v", err)
								}
							}
						}
					}
				}
			}

			if bts, err := json.Marshal(price); err == nil {
				if err = json.Unmarshal(bts, &view); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}

			if thumbnail != "" {
				arr := strings.Split(thumbnail, ",")
				if len(arr) > 1 {
					view.Thumbnail = strings.Split(arr[1], " ")[0]
				}else{
					view.Thumbnail = strings.Split(arr[0], " ")[0]
				}
			}

		default:
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unknown action"})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"No action defined"})
	}

	return c.JSON(view)
}

// @security BasicAuth
// DelPrice godoc
// @Summary Delete price
// @Accept json
// @Produce json
// @Param id path int true "Price ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/prices/{id} [delete]
// @Tags price
func deletePriceHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if price, err := models.GetPrice(common.Database, id); err == nil {
		if err = models.DeletePrice(common.Database, price); err != nil {
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