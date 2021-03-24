package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/google/logger"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/models"
	"html/template"
	"math"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

type CheckoutRequest struct {
	Items []NewItem
	Comment string
	BillingProfileId uint
	ShippingProfileId uint
	//
	PaymentId uint
	//
	TransportId uint
	TransportServices []TransportServiceView
	//
	PaymentMethod string
	Coupons []string
}

type NewOrder struct {
	Created time.Time
	Items []NewItem
	Comment string
	ProfileId uint
	TransportId uint
	Coupons []string
}

type NewItem struct {
	UUID string
	CategoryId uint
	Quantity int
}

/**/

type OrderShortView struct{
	Billing *BillingOrderView `json:",omitempty"`
	Items []ItemShortView `json:",omitempty"`
	Quantity int `json:",omitempty"`
	Sum float64 `json:",omitempty"`
	Discount float64 `json:",omitempty"`
	Delivery float64 `json:",omitempty"`
	Discount2 float64 `json:",omitempty"`
	VAT float64
	Total float64 `json:",omitempty"`
	Volume float64 `json:",omitempty"`
	Weight float64 `json:",omitempty"`
	Comment string `json:",omitempty"`
	//
	Deliveries []DeliveryView          `json:",omitempty"`
	Payments []PaymentView `json:",omitempty"`
	//
	Shipping *ShippingOrderView `json:",omitempty"`
	//
	//PaymentMethods *PaymentMethodsView `json:",omitempty"`
}

type BillingOrderView struct {
	ID uint
	Name string
	Title string
	Profile BillingProfileOrderView `json:",omitempty"`
	Method string `json:",omitempty"`
}

type BillingProfileOrderView struct {
	ID uint
	Email string `json:",omitempty"`
	Name     string `json:",omitempty"`
	Lastname string `json:",omitempty"`
	Company  string `json:",omitempty"`
	Phone    string `json:",omitempty"`
	Address  string `json:",omitempty"`
	Zip      string `json:",omitempty"`
	City     string `json:",omitempty"`
	Region   string `json:",omitempty"`
	Country  string `json:",omitempty"`
}

type ShippingOrderView struct{
	ID        uint
	Name      string
	Title     string
	Thumbnail string  `json:",omitempty"`
	Services  []TransportServiceView `json:",omitempty"`
	Profile ShippingProfileOrderView `json:",omitempty"`
	Coupons   []*CouponOrderView         `json:",omitempty"`
	Volume float64  `json:",omitempty"`
	Weight float64  `json:",omitempty"`
	Value float64
	Discount float64 `json:",omitempty"`
	Total float64
}

type ShippingProfileOrderView struct {
	ID uint
	Email string `json:",omitempty"`
	Name     string `json:",omitempty"`
	Lastname string `json:",omitempty"`
	Company  string `json:",omitempty"`
	Phone    string `json:",omitempty"`
	Address  string `json:",omitempty"`
	Zip      string `json:",omitempty"`
	City     string `json:",omitempty"`
	Region   string `json:",omitempty"`
	Country  string `json:",omitempty"`
}

type CouponOrderView struct {
	ID uint
	Code string
	Title string
	Type string // order, item, shipment
	Amount string
	Value float64
}

type PaymentView struct {
	ID uint
	Name string
	Title string
	Methods []string `json:",omitempty"`
	Details string `json:",omitempty"`
}

type DeliveryView struct {
	ID uint
	Title string
	Thumbnail string  `json:",omitempty"`
	ByVolume float64  `json:",omitempty"`
	ByWeight float64  `json:",omitempty"`
	Services []TransportServiceView `json:",omitempty"`
	Value float64
}

type TransportServiceView struct {
	Name string
	Title string
	Description string `json:",omitempty"`
	Price float64
	Selected bool
}

type ItemShortView struct {
	Uuid string                    `json:",omitempty"`
	Title string                   `json:",omitempty"`
	Path string                    `json:",omitempty"`
	Thumbnail string               `json:",omitempty"`
	Variation VariationShortView    `json:",omitempty"`
	Properties []PropertyShortView `json:",omitempty"`
	Coupons []*CouponOrderView     `json:",omitempty"`
	Price float64                  `json:",omitempty"`
	Discount float64                  `json:",omitempty"`
	Quantity int                   `json:",omitempty"`
	VAT        float64
	Total      float64             `json:",omitempty"`
	Volume float64 `json:",omitempty"`
	Weight float64 `json:",omitempty"`
}

type VariationShortView struct {
	Title string `json:",omitempty"`
	Thumbnail string `json:",omitempty"`
}

type PropertyShortView struct {
	Title string `json:",omitempty"`
	Thumbnail string `json:",omitempty"`
	Value string `json:",omitempty"`
	Price float64 `json:",omitempty"`
}

// GetOrders godoc
// @Summary Get checkout information
// @Accept json
// @Produce json
// @Param category body CheckoutRequest true "body"
// @Success 200 {object} OrderShortView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/checkout [post]
// @Tags frontend
func postCheckoutHandler(c *fiber.Ctx) error {
	var request CheckoutRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	_, view, err := Checkout(request)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	return c.JSON(view)
}

// CreateOrder godoc
// @Summary Post account order
// @Accept json
// @Produce json
// @Param cart body CheckoutRequest true "body"
// @Success 200 {object} OrderView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders [post]
// @Tags account
// @Tags frontend
func postAccountOrdersHandler(c *fiber.Ctx) error {
	var request CheckoutRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	order, view, err := Checkout(request)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	if v := c.Locals("user"); v != nil {
		if user, ok := v.(*models.User); ok {
			order.UserId = user.ID
		}
	}
	if _, err := models.CreateOrder(common.Database, order); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	var orderView struct {
		*OrderShortView
		ID uint
		CreatedAt time.Time
		PaymentMethod string `json:",omitempty"`
		Status string
	}
	orderView.ID = order.ID
	orderView.CreatedAt = order.CreatedAt
	orderView.PaymentMethod = view.Billing.Method
	orderView.Status = order.Status
	orderView.OrderShortView = view
	c.Status(http.StatusOK)
	return c.JSON(orderView)
}

func Checkout(request CheckoutRequest) (*models.Order, *OrderShortView, error){
	order := &models.Order{Status: models.ORDER_STATUS_NEW}
	now := time.Now()
	vat := common.Config.Payment.VAT
	tax := 1.0
	var err error
	// Billing ShillingProfile
	var billingProfile  *models.BillingProfile
	if request.BillingProfileId > 0 {
		order.BillingProfileId = request.BillingProfileId
		billingProfile, err = models.GetBillingProfile(common.Database, request.BillingProfileId)
	}
	// Shipping ShillingProfile
	var shippingProfile *models.ShippingProfile
	if request.ShippingProfileId > 0 {
		order.ShippingProfileId = request.ShippingProfileId
		shippingProfile, err = models.GetShippingProfile(common.Database, request.ShippingProfileId)
		if err == nil {
			if !strings.EqualFold(shippingProfile.Country, common.Config.Payment.Country) {
				var allowed bool
				switch shippingProfile.Country {
				/*case "AT":
					allowed = shippingProfile.ITN != ""*/
				default:
					allowed = true
				}
				if allowed {
					tax -= common.Config.Payment.VAT / 100.0
					vat = 0
				}
			}
		} else {
			return nil, nil, err
		}
	}
	// Coupons
	var coupons []*models.Coupon
	for _, code := range request.Coupons {
		allowed := true
		for _, coupon := range coupons {
			if coupon.Code == code {
				allowed = false
				break
			}
		}
		if allowed {
			if coupon, err := models.GetCouponByCode(common.Database, code); err == nil {
				if coupon.Enabled && coupon.Start.Before(now) && coupon.End.After(now) {
					coupons = append(coupons, coupon)
				}
			}
		}
	}
	//
	var itemsShortView []ItemShortView
	for _, rItem := range request.Items {
		var arr []int
		if err := json.Unmarshal([]byte(rItem.UUID), &arr); err == nil && len(arr) >= 2{
			//
			productId := arr[0]
			var product *models.Product
			if product, err = models.GetProduct(common.Database, productId); err != nil {
				return nil, nil, err
			}
			variationId := arr[1]

			//var variation *models.Variation
			var vId uint
			var title string
			var basePrice, salePrice, weight float64
			var start, end time.Time
			//var dimensions string
			var width, height, depth float64
			if variationId == 0 {
				title = "default"
				basePrice = product.BasePrice
				salePrice = product.SalePrice
				start = product.Start
				end = product.End
				//dimensions = product.Dimensions
				width = product.Width
				height = product.Height
				depth = product.Depth
				weight = product.Weight
			} else {
				if variation, err := models.GetVariation(common.Database, variationId); err == nil {
					if product.ID != variation.ProductId {
						err = fmt.Errorf("Products and Variation mismatch")
						return nil, nil, err
					}
					vId = variation.ID
					title = variation.Title
					basePrice = variation.BasePrice
					salePrice = variation.SalePrice
					start = variation.Start
					end = variation.End
					width = variation.Width
					height = variation.Height
					depth = variation.Depth
					weight = variation.Weight
				} else {
					return nil, nil, err
				}
			}
			categoryId := rItem.CategoryId
			item := &models.Item{
				Uuid:     rItem.UUID,
				CategoryId: rItem.CategoryId,
				Title:    product.Title,
				BasePrice: basePrice,
				Quantity: rItem.Quantity,
			}
			if salePrice > 0 && start.Before(now) && end.After(now) {
				item.Price = salePrice * tax
				item.SalePrice = salePrice * tax
			}else{
				item.Price = basePrice * tax
			}
			item.VAT = vat
			item.Volume = width * height * depth / 1000000.0
			item.Weight = weight
			//
			if breadcrumbs := models.GetBreadcrumbs(common.Database, categoryId); len(breadcrumbs) > 0 {
				var chunks []string
				for _, crumb := range breadcrumbs {
					chunks = append(chunks, crumb.Name)
				}
				item.Path = "/" + path.Join(append(chunks, product.Name)...)
			}

			if cache, err := models.GetCacheProductByProductId(common.Database, product.ID); err == nil {
				item.Thumbnail = cache.Thumbnail
			}else{
				logger.Warningf("%v", err.Error())
			}
			if cache, err := models.GetCacheVariationByVariationId(common.Database, vId); err == nil {
				if item.Thumbnail == "" {
					item.Thumbnail = cache.Thumbnail
				}
			} else {
				logger.Warningf("%v", err.Error())
			}

			var propertiesShortView []PropertyShortView
			var couponsOrderView []*CouponOrderView
			if len(arr) > 2 {
				//
				for _, id := range arr[2:] {
					if price, err := models.GetPrice(common.Database, id); err == nil {
						propertyShortView := PropertyShortView{}
						propertyShortView.Title = price.Property.Title
						//
						if cache, err := models.GetCacheValueByValueId(common.Database, price.Value.ID); err == nil {
							propertyShortView.Thumbnail = cache.Thumbnail
						}
						//
						propertyShortView.Value = price.Value.Value
						if price.Price > 0 {
							propertyShortView.Price = price.Price * tax
						}
						item.Price += price.Price * tax
						propertiesShortView = append(propertiesShortView, propertyShortView)
					} else {
						return nil, nil, err
					}
				}
			}
			// Coupons
			for _, coupon := range coupons {
				var allowed bool
				if coupon.Type == "item" {
					logger.Infof("Coupon: %+v", coupon)
					if coupon.ApplyTo == "all" {
						allowed = true
					} else if coupon.ApplyTo == "categories" {
						allowed = len(coupon.Categories) == 0
						for _, category := range coupon.Categories{
							if category.ID == item.CategoryId {
								allowed = true
								break
							}
						}
					} else if coupon.ApplyTo == "products" {
						allowed = len(coupon.Products) == 0
						for _, product := range coupon.Products{
							if int(product.ID) == productId {
								allowed = true
								break
							}
						}
					} else {
						allowed = false
					}
				}else if coupon.Type == "order" {
					allowed = true
				}
				if allowed {
					//
					var value float64
					if res := rePercent.FindAllStringSubmatch(coupon.Amount, 1); len(res) > 0 && len(res[0]) > 1 {
						// percent
						if p, err := strconv.ParseFloat(res[0][1], 10); err == nil {
							value = item.Price * p / 100.0
							item.Discount = value
						}
					}else{
						if n, err := strconv.ParseFloat(coupon.Amount, 10); err == nil {
							value = n
							item.Discount = value
						}
					}
					if item.Discount > item.Price {
						value = item.Price
						item.Discount = value
					}
					item.Discount = math.Round(item.Discount * 100) / 100
					//
					couponsOrderView = append(couponsOrderView, &CouponOrderView{
						ID:     coupon.ID,
						Code:   coupon.Code,
						Title:  coupon.Title,
						Type: coupon.Type,
						Amount: coupon.Amount,
						Value: value,
					})
				}
			}
			// /Coupons
			item.Total = (item.Price - item.Discount) * float64(item.Quantity)
			// [Item Description]
			var itemShortView ItemShortView
			if bts, err := json.Marshal(item); err == nil {
				if err = json.Unmarshal(bts, &itemShortView); err != nil {
					logger.Warningf("%v", err.Error())
				}
			}else{
				logger.Warningf("%v", err.Error())
			}
			itemShortView.Variation = VariationShortView{
				Title: title,
			}
			itemShortView.Properties = propertiesShortView
			itemShortView.Coupons = couponsOrderView
			if bts, err := json.Marshal(itemShortView); err == nil {
				item.Description = string(bts)
			}
			// [/Item Description]
			itemsShortView = append(itemsShortView, itemShortView)
			order.Items = append(order.Items, item)
			order.Quantity += item.Quantity
			order.Volume += item.Volume
			order.Weight += item.Weight
			order.Sum += item.Total
			order.Discount += item.Discount
		}
	}
	// Transports
	var deliveriesShortView []DeliveryView
	var shippingView *ShippingOrderView
	if transports, err := models.GetTransports(common.Database); err == nil {
		for _, transport := range transports {
			// All available transports OR selected
			if transport.Enabled && (order.Volume >= transport.Volume || order.Weight >= transport.Weight) && (transport.ID == request.TransportId || request.TransportId == 0) {
				//
				var orderFixed, orderPercent, itemFixed, itemPercent, kg, m3 float64
				var orderIsPercent, itemIsPercent bool
				//
				var tariff *models.Tariff
				if shippingProfile != nil {
					// 2 Get Zone by Country, Country and Zip
					var zoneId uint
					if zone, err := models.GetZoneByCountry(common.Database, shippingProfile.Country); err == nil {
						zoneId = zone.ID
					}
					for i := 0; i <= len(shippingProfile.Zip); i++ {
						n := len(shippingProfile.Zip) - i
						zip := shippingProfile.Zip[0:n]
						for j := n; j < len(shippingProfile.Zip); j++ {
							zip += "X"
						}
						zone, err := models.GetZoneByCountryAndZIP(common.Database, shippingProfile.Country, zip)
						if err == nil {
							zoneId = zone.ID
							break
						}
					}
					// 3 Get Tariff by Shipping and Zone
					tariff, _ = models.GetTariffByTransportIdAndZoneId(common.Database, transport.ID, zoneId)
				}
				//
				if tariff == nil {
					if res := rePercent.FindAllStringSubmatch(transport.Order, 1); len(res) > 0 && len(res[0]) > 1 {
						if v, err := strconv.ParseFloat(res[0][1], 10); err == nil {
							orderPercent = v
							orderIsPercent = true
						}
					}else{
						if v, err := strconv.ParseFloat(transport.Order, 10); err == nil {
							orderFixed = v
						}
					}
				}else{
					if res := rePercent.FindAllStringSubmatch(tariff.Order, 1); len(res) > 0 && len(res[0]) > 1 {
						if v, err := strconv.ParseFloat(res[0][1], 10); err == nil {
							orderPercent = v
							orderIsPercent = true
						}
					}else{
						if v, err := strconv.ParseFloat(tariff.Order, 10); err == nil {
							orderFixed = v
						}
					}
				}
				// Item
				if tariff == nil {
					if res := rePercent.FindAllStringSubmatch(transport.Item, 1); len(res) > 0 && len(res[0]) > 1 {
						if v, err := strconv.ParseFloat(res[0][1], 10); err == nil {
							itemPercent = v
							itemIsPercent = true
						}
					}else{
						if v, err := strconv.ParseFloat(transport.Item, 10); err == nil {
							itemFixed = v
						}
					}
				}else{
					if res := rePercent.FindAllStringSubmatch(tariff.Item, 1); len(res) > 0 && len(res[0]) > 1 {
						if v, err := strconv.ParseFloat(res[0][1], 10); err == nil {
							orderPercent = v
							itemIsPercent = true
						}
					}else{
						if v, err := strconv.ParseFloat(tariff.Item, 10); err == nil {
							orderFixed = v
						}
					}
				}
				// Kg
				if tariff == nil {
					kg = transport.Kg
				}else{
					kg = tariff.Kg
				}
				// M3
				if tariff == nil {
					m3 = transport.M3
				}else{
					m3 = tariff.M3
				}
				//
				// [Delivery]
				var delivery float64
				for _, item := range order.Items {
					if itemIsPercent {
						delivery += (item.Price * itemPercent / 100.0) * float64(item.Quantity)
					}else{
						delivery += itemFixed * tax * float64(item.Quantity)
					}
				}
				// Delivery: fixed
				if orderIsPercent {
					delivery += order.Sum * orderPercent / 100.0
				}else{
					delivery += orderFixed * tax
				}
				// Delivery: dynamic
				byVolume := delivery + order.Volume * m3
				byWeight := delivery + order.Weight * kg
				var value float64
				if byVolume > byWeight {
					value = byVolume
				} else {
					value = byWeight
				}
				//
				if transport.Free > 0 && order.Sum > transport.Free {
					value = 0
				}
				//
				var services []TransportServiceView
				if transport.Services != "" {
					if err = json.Unmarshal([]byte(transport.Services), &services); err != nil {
						logger.Warningf("%+v", err)
					}
				}
				//
				if request.TransportId > 0 {
					shippingView = &ShippingOrderView{
						ID: transport.ID,
						Name: transport.Name,
						Title: transport.Title,
						Thumbnail: transport.Thumbnail,
					}
					if cache, err := models.GetCacheTransportByTransportId(common.Database, transport.ID); err == nil {
						shippingView.Thumbnail = cache.Thumbnail
					}
					if bts, err := json.Marshal(shippingProfile); err == nil {
						if err = json.Unmarshal(bts, &shippingView.Profile); err != nil {
							logger.Warningf("%+v", err)
						}
					}
					//
					if shippingProfile != nil {
						order.ShippingProfileEmail = shippingProfile.Email
						order.ShippingProfileName = shippingProfile.Name
						order.ShippingProfileLastname = shippingProfile.Lastname
						order.ShippingProfileCompany = shippingProfile.Company
						order.ShippingProfilePhone = shippingProfile.Phone
						order.ShippingProfileAddress = shippingProfile.Address
						order.ShippingProfileZip = shippingProfile.Zip
						order.ShippingProfileCity = shippingProfile.City
						order.ShippingProfileRegion = shippingProfile.Region
						order.ShippingProfileCountry = shippingProfile.Country
						order.ShippingProfileTransport = transport.Title
					}
					//
					var selectedServices []string
					for _, service := range services {
						for j := 0; j < len(request.TransportServices); j++ {
							if request.TransportServices[j].Name == service.Name {
								service.Selected = true
								break
							}
						}
						if service.Selected {
							selectedServices = append(selectedServices, service.Title)
							shippingView.Services = append(shippingView.Services, service)
							value += service.Price
						}
					}
					order.ShippingProfileServices = strings.Join(selectedServices, ", ")
					//
					order.TransportId = request.TransportId
					order.Delivery = math.Round(value * 100) / 100
				}else{
					deliveriesShortView = append(deliveriesShortView, DeliveryView{
						ID:        transport.ID,
						Title:     transport.Title,
						Thumbnail: transport.Thumbnail,
						ByVolume: math.Round(byVolume * 100) / 100,
						ByWeight: math.Round(byWeight * 100) / 100,
						Services: services,
						Value: math.Round(value * 100) / 100,
					})
				}
			}
		}
	}
	sort.Slice(deliveriesShortView[:], func(i, j int) bool {
		return deliveriesShortView[i].Value < deliveriesShortView[j].Value
	})
	// [/Delivery]
	// [Payments]
	var billingView *BillingOrderView
	var paymentsShortView []PaymentView
	if true {
		var payments []PaymentView
		if common.Config.Payment.Stripe.Enabled {
			payments = append(payments, PaymentView{
				ID: 1,
				Name: "stripe",
				Title: "Stripe",
			})
		}
		if common.Config.Payment.Mollie.Enabled {
			payments = append(payments, PaymentView{
				ID: 2,
				Name: "mollie",
				Title: "Mollie",
				Methods: reCSV.Split(common.Config.Payment.Mollie.Methods, -1),
			})
		}
		if common.Config.Payment.AdvancePayment.Enabled {
			var details string
			if tmpl, err := template.New("details").Parse(common.Config.Payment.AdvancePayment.Details); err == nil {
				var tpl bytes.Buffer
				vars := map[string]interface{}{	}
				if err := tmpl.Execute(&tpl, vars); err == nil {
					details = tpl.String()
				}else{
					logger.Errorf("%v", err)
				}
			}else{
				logger.Errorf("%v", err)
			}
			payments = append(payments, PaymentView{
				ID: 3,
				Name: "advance-payment",
				Title: "Advance Payment",
				Details: details,
			})
		}
		if common.Config.Payment.OnDelivery.Enabled {
			payments = append(payments, PaymentView{
				ID: 4,
				Name: "on-delivery",
				Title: "On Delivery",
			})
		}
		for _, payment := range payments {
			//if payment.ID == request.PaymentId || request.PaymentId == 0 {
			if (payment.ID == request.PaymentId || strings.Index(request.PaymentMethod, payment.Name) >= 0) || (request.PaymentId == 0 && request.PaymentMethod == "") {
				// if request.PaymentId > 0 {
				if request.PaymentId > 0 || strings.Index(request.PaymentMethod, payment.Name) >= 0 {
					billingView = &BillingOrderView{
						ID:    payment.ID,
						Name:  payment.Name,
						Title: payment.Title,
					}
					if billingProfile != nil {
						order.BillingProfileEmail = billingProfile.Email
						order.BillingProfileName = billingProfile.Name
						order.BillingProfileLastname = billingProfile.Lastname
						order.BillingProfileCompany = billingProfile.Company
						order.BillingProfilePhone = billingProfile.Phone
						order.BillingProfileAddress = billingProfile.Address
						order.BillingProfileZip = billingProfile.Zip
						order.BillingProfileCity = billingProfile.City
						order.BillingProfileRegion = billingProfile.Region
						order.BillingProfileCountry = billingProfile.Country
						order.BillingProfilePayment = payment.Title
					}
					if request.PaymentMethod != "" {
						//billingView.Method = request.PaymentMethod
						method := strings.Replace(request.PaymentMethod, "mollie:", "", 1)
						billingView.Method = method
						order.BillingProfileMethod = method
					}
					if bts, err := json.Marshal(billingProfile); err == nil {
						if err = json.Unmarshal(bts, &billingView.Profile); err != nil {
							logger.Warningf("%+v", err)
						}
					}
				} else {
					paymentsShortView = append(paymentsShortView, payment)
				}
			}
		}
	}
	sort.Slice(paymentsShortView[:], func(i, j int) bool {
		return paymentsShortView[i].ID < paymentsShortView[j].ID
	})
	// [/Payments]
	// [PaymentMethods]
	var paymentMethodsView *PaymentMethodsView
	if request.PaymentMethod == "" {
		paymentMethodsView = &PaymentMethodsView{}
		if common.Config.Payment.Stripe.Enabled {
			paymentMethodsView.Stripe.Enabled = true
			paymentMethodsView.Default = "stripe"
		}
		if common.Config.Payment.Mollie.Enabled {
			paymentMethodsView.Mollie.Enabled = true
			paymentMethodsView.Mollie.Methods = reCSV.Split(common.Config.Payment.Mollie.Methods, -1)
			paymentMethodsView.Default = "mollie"
		}
		if common.Config.Payment.AdvancePayment.Enabled {
			paymentMethodsView.AdvancePayment.Enabled = true
			if tmpl, err := template.New("details").Parse(common.Config.Payment.AdvancePayment.Details); err == nil {
				var tpl bytes.Buffer
				vars := map[string]interface{}{
					/* Something should be here */
				}
				if err := tmpl.Execute(&tpl, vars); err == nil {
					paymentMethodsView.AdvancePayment.Details = tpl.String()
				}else{
					logger.Errorf("%v", err)
				}
			}else{
				logger.Errorf("%v", err)
			}
		}
		if common.Config.Payment.OnDelivery.Enabled {
			paymentMethodsView.OnDelivery.Enabled = true
		}
	}else{
		order.PaymentMethod = request.PaymentMethod
	}
	// [/PaymentMethod]

	// *****************************************************************************************************************
	// Coupons: input order.Delivery
	if shippingView != nil {
		shippingView.Volume = order.Volume
		shippingView.Weight = order.Weight
		shippingView.Value = order.Delivery
		for _, coupon := range coupons {
			if coupon.Type == "shipment" {
				var value float64
				if res := rePercent.FindAllStringSubmatch(coupon.Amount, 1); len(res) > 0 && len(res[0]) > 1 {
					// percent
					if p, err := strconv.ParseFloat(res[0][1], 10); err == nil {
						value = order.Delivery * p / 100.0
						order.Discount2 = value
					}
				} else {
					if n, err := strconv.ParseFloat(coupon.Amount, 10); err == nil {
						value = n
						order.Discount2 = value
					}
				}
				if order.Discount2 > order.Delivery {
					order.Discount2 = order.Delivery
					value = order.Delivery
				}
				shippingView.Discount += value
				order.Discount2 = math.Round(order.Discount2*100) / 100
				//
				shippingView.Coupons = append(shippingView.Coupons, &CouponOrderView{
					ID:     coupon.ID,
					Code:   coupon.Code,
					Title:  coupon.Title,
					Type:   coupon.Type,
					Amount: coupon.Amount,
					Value:  value,
				})
			}
		}
		shippingView.Total = shippingView.Value - shippingView.Discount
	}
	//
	order.VAT = vat
	order.Total = (order.Sum - order.Discount) + (order.Delivery - order.Discount2)
	var view *OrderShortView
	if bts, err := json.Marshal(order); err == nil {
		if err = json.Unmarshal(bts, &view); err == nil {
			view.Items = itemsShortView
			view.Deliveries = deliveriesShortView
			//
			view.Billing = billingView
			view.Shipping = shippingView
			//
			//view.PaymentMethods = paymentMethodsView
			view.Payments = paymentsShortView
		} else {
			logger.Warningf("%v", err.Error())
		}
	}else{
		logger.Warningf("%v", err.Error())
	}
	// [Order Description]
	if bts, err := json.Marshal(view); err == nil {
		order.Description = string(bts)
	}
	// [/Order Description]
	return order, view, nil
}