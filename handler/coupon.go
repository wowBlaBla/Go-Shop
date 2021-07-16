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

type CouponsFullView []CouponFullView

type CouponFullView struct {
	CouponShortView
	Discounts []DiscountView `json:",omitempty"`
}

// @security BasicAuth
// GetCoupons godoc
// @Summary Get coupons
// @Accept json
// @Produce json
// @Success 200 {object} CouponsFullView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/coupons [get]
// @Tags coupon
func getCouponsHandler(c *fiber.Ctx) error {
	if options, err := models.GetCouponsFull(common.Database); err == nil {
		var view CouponsFullView
		if bts, err := json.MarshalIndent(options, "", "   "); err == nil {
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

type CouponsShortView []CouponShortView

type CouponShortView struct {
	ID uint
	Enabled bool
	Title string `json:",omitempty"`
	Description string `json:",omitempty"`
}

type NewCoupon struct {
	Enabled bool
	Title string
	Code string
	Type string
	Limit int
	Description string
}

// @security BasicAuth
// CreateCoupon godoc
// @Summary Create coupon
// @Accept json
// @Produce json
// @Param option body NewCoupon true "body"
// @Success 200 {object} CouponView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/coupons [post]
// @Tags option
func postCouponHandler(c *fiber.Ctx) error {
	var view CouponView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewCoupon
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			request.Title = strings.TrimSpace(request.Title)
			if request.Title == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": "Title is not defined"})
			}
			request.Code = strings.TrimSpace(request.Code)
			if request.Code == "" {
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": "Code is not defined"})
			}
			if len(request.Description) > 256 {
				request.Description = request.Description[0:255]
			}
			if coupons, err := models.GetCouponsByTitle(common.Database, request.Title); err == nil && len(coupons) > 0 {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Coupon exists"})
			}
			if coupons, err := models.GetCouponsByCode(common.Database, request.Code); err == nil && len(coupons) > 0 {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Coupon exists"})
			}
			now := time.Now()
			year, month, day := now.Date()
			midnight := time.Date(year, month, day, 0, 0, 0, 0, now.Location())
			coupon := &models.Coupon {
				Enabled: request.Enabled,
				Title: request.Title,
				Code: request.Code,
				Description: request.Description,
				Type: request.Type,
				Limit: request.Limit,
				Start: midnight,
				End: midnight.AddDate(1, 0, 0).Add(-1 * time.Second),
			}
			if _, err := models.CreateCoupon(common.Database, coupon); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if bts, err := json.Marshal(coupon); err == nil {
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

type CouponsListResponse struct {
	Data []CouponListItem
	Filtered int64
	Total int64
}

type CouponListItem struct {
	ID uint
	Enabled bool
	Title string
	Code string
	Description string
	Type string
	Amount string
	Minimum float64
	ApplyTo string
	Discounts int
}

// @security BasicAuth
// SearchCoupons godoc
// @Summary Search coupons
// @Accept json
// @Produce json
// @Param request body ListRequest true "body"
// @Success 200 {object} CouponsListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/coupons/list [post]
// @Tags coupon
func postCouponsListHandler(c *fiber.Ctx) error {
	var response CouponsListResponse
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
					keys1 = append(keys1, fmt.Sprintf("coupons.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				default:
					orders = append(orders, fmt.Sprintf("coupons.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//
	rows, err := common.Database.Debug().Model(&models.Coupon{}).Select("coupons.ID, coupons.Enabled, coupons.Title, coupons.Code, coupons.Type, coupons.Amount, coupons.Minimum, coupons.Apply_To as ApplyTo, coupons.Description, count(`discounts`.ID) as Discounts").Joins("left join `discounts` on `discounts`.coupon_id = coupons.id").Group("coupons.id").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item CouponListItem
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
	rows, err = common.Database.Debug().Model(&models.Coupon{}).Select("coupons.ID, coupons.Enabled, coupons.Title, coupons.Code, coupons.Type, coupons.Amount, coupons.Minimum, coupons.Apply_To as ApplyTo, coupons.Description, count(`discounts`.ID) as Discounts").Joins("left join `discounts` on `discounts`.coupon_id = coupons.id").Group("coupons.id").Where(strings.Join(keys1, " and "), values1...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	if len(keys1) > 0 {
		common.Database.Debug().Model(&models.Coupon{}).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

type CouponView struct {
	ID uint
	Enabled bool
	Title string `json:",omitempty"`
	Code string `json:",omitempty"`
	Description string `json:",omitempty"`
	Type string
	Start time.Time
	End time.Time
	Amount string
	Minimum float64
	Count int                           `json:",omitempty"`
	Limit int                           `json:",omitempty"`
	ApplyTo string                      `json:",omitempty"`
	Categories []models.CatalogItemView `json:",omitempty"`
	Products []ProductShortView         `json:",omitempty"`
}

type DiscountsView []DiscountView

type DiscountView struct {
	ID uint
	/*Title string `json:",omitempty"`
	Description string `json:",omitempty"`
	Thumbnail string `json:",omitempty"`
	Value string `json:",omitempty"`
	Availability string `json:",omitempty"`
	Sending string `json:",omitempty"`*/
}

// @security BasicAuth
// GetCoupon godoc
// @Summary Get coupon
// @Accept json
// @Produce json
// @Param id path int true "Coupon ID"
// @Success 200 {object} CouponView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/coupons/{id} [get]
// @Tags option
func getCouponHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if coupon, err := models.GetCoupon(common.Database, id); err == nil {
		var view CouponView
		if bts, err := json.MarshalIndent(coupon, "", "   "); err == nil {
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

type CouponRequest struct {
	CouponView
	Categories string
	Products string
}

// @security BasicAuth
// UpdateCoupon godoc
// @Summary Update coupon
// @Accept json
// @Produce json
// @Param option body CouponShortView true "body"
// @Param id path int true "Coupon ID"
// @Success 200 {object} CouponShortView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/coupons/{id} [put]
// @Tags coupon
func putCouponHandler(c *fiber.Ctx) error {
	var request CouponRequest
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
	var coupon *models.Coupon
	var err error
	if coupon, err = models.GetCoupon(common.Database, int(id)); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	coupon.Enabled = request.Enabled
	request.Title = strings.TrimSpace(request.Title)
	if request.Title == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "Title is not defined"})
	}
	if len(request.Description) > 256 {
		request.Description = request.Description[0:255]
	}
	coupon.Title = request.Title
	coupon.Description = request.Description
	coupon.Start = request.Start
	coupon.End = request.End
	coupon.Type = request.Type
	coupon.Amount = request.Amount
	coupon.Minimum = request.Minimum
	coupon.Limit = request.Limit
	coupon.Count = request.Count
	coupon.ApplyTo = request.ApplyTo
	if err = models.DeleteAllCategoriesFromCoupon(common.Database, coupon); err != nil {
		logger.Warningf("%+v", err)
	}
	for _, v := range strings.Split(request.Categories, ",") {
		if id, err := strconv.Atoi(v); err == nil {
			if category, err := models.GetCategory(common.Database, id); err == nil {
				if err = models.AddCategoryToCoupon(common.Database, coupon, category); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}
		}
	}
	if err = models.DeleteAllProductsFromCoupon(common.Database, coupon); err != nil {
		logger.Warningf("%+v", err)
	}
	for _, v := range strings.Split(request.Products, ",") {
		if id, err := strconv.Atoi(v); err == nil {
			if product, err := models.GetProduct(common.Database, id); err == nil {
				if err = models.AddProductToCoupon(common.Database, coupon, product); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}
		}
	}
	if err := models.UpdateCoupon(common.Database, coupon); err == nil {
		var view CouponView
		if bts, err := json.MarshalIndent(coupon, "", "   "); err == nil {
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
// DelCoupon godoc
// @Summary Delete coupon
// @Accept json
// @Produce json
// @Param id path int true "Coupon ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/coupons/{id} [delete]
// @Tags coupon
func delCouponHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if coupon, err := models.GetCoupon(common.Database, id); err == nil {
		for _, discount := range coupon.Discounts {
			if err = models.DeleteDiscount(common.Database, discount); err != nil {
				logger.Errorf("%v", err)
			}
		}
		if err = models.DeleteCoupon(common.Database, coupon); err == nil {
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