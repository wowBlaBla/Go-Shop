package handler

import (
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/google/logger"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/common/mollie"
	"github.com/yonnic/goshop/models"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type MollieOrderView struct {
	Id string
	ProfileId string `json:",omitempty"`
	Checkout string `json:",omitempty"`
}

type MollieSubmitRequest struct {
	Language string `json:"language"`
	Method string
}

// PostMollieOrder godoc
// @Summary Post mollie order
// @Accept json
// @Produce json
// @Param id path int true "Order ID"
// @Query method query string true "for example: postMessage"
// @Param form body MollieSubmitRequest true "body"
// @Success 200 {object} MollieOrderView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders/{id}/mollie/submit [post]
// @Tags account
// @Tags frontend
func postAccountOrderMollieSubmitHandler(c *fiber.Ctx) error {
	logger.Infof("postAccountOrderMollieSubmitHandler")
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var user *models.User
	if v := c.Locals("user"); v != nil {
		var ok bool
		if user, ok = v.(*models.User); !ok {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"User not found"})
		}
	}
	if order, err := models.GetOrder(common.Database, id); err == nil {
		if order.UserId != user.ID {
			c.Status(http.StatusForbidden)
			return c.JSON(fiber.Map{"ERROR": "You are not allowed to do that"})
		}
		// PAYLOAD
		//logger.Infof("order: %+v", order)

		var base string
		var redirectUrl string
		if v := c.Request().Header.Referer(); len(v) > 0 {
			if u, err := url.Parse(string(v)); err == nil {
				u.Path = ""
				u.RawQuery = ""
				base = u.String()
				u.Path = fmt.Sprintf("/api/v1/account/orders/%v/mollie/success", order.ID)
				values := url.Values{}
				if v := c.Query("method", ""); v != "" {
					values.Set("method", v)
					u.RawQuery = values.Encode()
				}
				redirectUrl = u.String()
			}
		}

		var request MollieSubmitRequest
		if err := c.BodyParser(&request); err != nil {
			return err
		}
		//
		o := &mollie.Order{
			OrderNumber: fmt.Sprintf("%d", order.ID),
		}
		//
		// Lines
		var orderShortView OrderShortView
		if err := json.Unmarshal([]byte(order.Description), &orderShortView); err == nil {
			re := regexp.MustCompile(`^\[(.*)\]$`)
			for _, item := range orderShortView.Items {
				sku := strings.Replace(re.ReplaceAllString(item.Uuid, ""), ",", ".", -1)
				meta := map[string]string{"variation": item.Variation.Title}
				line := mollie.Line{
					Type:           "physical",
					Category:       "gift",
					Sku:            sku,
					Name:           item.Title,
					ProductUrl:     base + item.Path,
					Metadata:       meta,
					Quantity:       item.Quantity,
					UnitPrice:      mollie.NewAmount(strings.ToUpper(common.Config.Currency), item.Price),
					DiscountAmount: mollie.NewAmount(strings.ToUpper(common.Config.Currency), item.Discount * float64(item.Quantity)),
					TotalAmount:    mollie.NewAmount(strings.ToUpper(common.Config.Currency), item.Total),
					VatAmount:      mollie.NewAmount(strings.ToUpper(common.Config.Currency), item.Total * (common.Config.Payment.VAT / 100.0) / ((100.0 + common.Config.Payment.VAT) / 100.0)),
					VatRate:        fmt.Sprintf("%.2f", common.Config.Payment.VAT),
				}
				/*total := item.Total
				if item.Discount > 0 {
					total = item.Total
					line.DiscountAmount = mollie.NewAmount(strings.ToUpper(common.Config.Currency), item.Discount * float64(item.Quantity))
				}else if order.Discount > 0 {
					discount := 1 - (order.Discount / order.Sum)
					total = (item.Price * discount) * float64(item.Quantity)
					line.DiscountAmount = mollie.NewAmount(strings.ToUpper(common.Config.Currency), item.Price * (1 - discount) * float64(item.Quantity))
				}else{
					line.DiscountAmount = mollie.NewAmount(strings.ToUpper(common.Config.Currency), 0)
				}
				line.TotalAmount = mollie.NewAmount(strings.ToUpper(common.Config.Currency), total)
				line.VatAmount = mollie.NewAmount(strings.ToUpper(common.Config.Currency), total * (common.Config.Payment.ITN / 100.0) / ((100.0 + common.Config.Payment.ITN) / 100.0))*/
				if item.Thumbnail != "" {
					line.ImageUrl = base + strings.Split(item.Thumbnail, ",")[0]
				}
				o.Lines = append(o.Lines, line)
			}
		}
		if order.Delivery > 0 {
			o.Lines = append(o.Lines, mollie.Line{
				Type: "shipping_fee",
				Name: "Shipping Fee",
				Quantity:       1,
				VatRate:        fmt.Sprintf("%.2f", common.Config.Payment.VAT),
				UnitPrice:      mollie.NewAmount(strings.ToUpper(common.Config.Currency), order.Delivery),
				TotalAmount:    mollie.NewAmount(strings.ToUpper(common.Config.Currency), order.Delivery - order.Discount2),
				DiscountAmount: mollie.NewAmount(strings.ToUpper(common.Config.Currency), order.Discount2),
				VatAmount:      mollie.NewAmount(strings.ToUpper(common.Config.Currency), (order.Delivery - order.Discount2) * (common.Config.Payment.VAT / 100.0) / ((100.0 + common.Config.Payment.VAT) / 100.0)),
			})
		}
		//
		o.Amount = mollie.NewAmount(strings.ToUpper(common.Config.Currency), order.Total)
		o.RedirectUrl = redirectUrl
		bts, _ := json.Marshal(o)
		logger.Infof("Bts: %+v", string(bts))
		// Address
		address := mollie.Address{
			Email: user.Email,
		}
		if profile, err := models.GetBillingProfile(common.Database, order.BillingProfileId); err == nil {
			address.GivenName = profile.Name
			address.FamilyName = profile.Lastname
			address.OrganizationName = profile.Company
			address.StreetAndNumber = profile.Address
			address.Phone = profile.Phone
			address.City = profile.City
			address.PostalCode = profile.Zip
			address.Country = profile.Country
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
		o.BillingAddress = address
		o.ShippingAddress = address
		o.Locale = "en_US"
		// Locale
		if request.Language != "" {
			if request.Language == "de" {
				o.Locale = "de_DE"
			}
		}
		// Method
		if request.Method != "" {
			o.Method = request.Method
		}
		if bts, err := json.Marshal(o); err == nil {
			logger.Infof("Bts: %+v", string(bts))
		}

		if o, links, err := common.MOLLIE.CreateOrder(o); err == nil {
			logger.Infof("o: %+v", o)
			transaction := &models.Transaction{Amount: order.Total, Status: models.TRANSACTION_STATUS_NEW, Order: order}
			transactionPayment := models.TransactionPayment{Mollie: &models.TransactionPaymentMollie{Id: o.Id}}
			if bts, err := json.Marshal(transactionPayment); err == nil {
				transaction.Payment = string(bts)
			}
			if _, err = models.CreateTransaction(common.Database, transaction); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}

			var response MollieOrderView

			response.Id = o.Id
			if link, found := links["checkout"]; found {
				response.Checkout = link.Href
			}

			c.Status(http.StatusOK)
			return c.JSON(response)
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	} else {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// GetMolliePaymentSuccess godoc
// @Summary Get mollie payment success
// @Accept json
// @Produce html
// @Param id path int true "Order ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders/{id}/mollie/success [get]
// @Tags account
// @Tags frontend
func getAccountOrderMollieSuccessHandler(c *fiber.Ctx) error {
	c.Response().Header.Set("Content-Type", "text/html")
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if order, err := models.GetOrderFull(common.Database, id); err == nil {
		if transactions, err := models.GetTransactionsByOrderId(common.Database, id); err == nil {
			for _, transaction := range transactions {
				var payment models.TransactionPayment
				if err = json.Unmarshal([]byte(transaction.Payment), &payment); err == nil {
					if payment.Mollie != nil && payment.Mollie.Id != "" {
						if o, err := common.MOLLIE.GetOrder(payment.Mollie.Id); err == nil {
							if o.Status == "paid" {
								transaction.Status = models.TRANSACTION_STATUS_COMPLETE
								if err = models.UpdateTransaction(common.Database, transaction); err != nil {
									c.Status(http.StatusInternalServerError)
									return c.JSON(HTTPError{err.Error()})
								}
								order.Status = models.ORDER_STATUS_PAID
								if err = models.UpdateOrder(common.Database, order); err != nil {
									c.Status(http.StatusInternalServerError)
									return c.JSON(HTTPError{err.Error()})
								}
								// Notifications
								if common.Config.Notification.Enabled {
									if common.Config.Notification.Email.Enabled {
										// to admin
										//logger.Infof("Time to send admin email")
										if users, err := models.GetUsersByRoleLessOrEqualsAndNotification(common.Database, models.ROLE_ADMIN, true); err == nil {
											//logger.Infof("users found: %v", len(users))
											template, err := models.GetEmailTemplateByType(common.Database, common.NOTIFICATION_TYPE_ADMIN_ORDER_PAID)
											if err == nil {
												//logger.Infof("Template: %+v", template)
												for _, user := range users {
													logger.Infof("Send email admin user: %+v", user.Email)
													if err = SendOrderPaidEmail(mail.NewEmail(user.Login, user.Email), int(order.ID), template); err != nil {
														logger.Errorf("%+v", err)
													}
												}
											}else{
												logger.Warningf("%+v", err)
											}
										}else{
											logger.Warningf("%+v", err)
										}
										// to user
										//logger.Infof("Time to send user email")
										template, err := models.GetEmailTemplateByType(common.Database, common.NOTIFICATION_TYPE_USER_ORDER_PAID)
										if err == nil {
											//logger.Infof("Template: %+v", template)
											if user, err := models.GetUser(common.Database, int(order.UserId)); err == nil {
												if user.EmailConfirmed {
													logger.Infof("Send email to user: %+v", user.Email)
													if err = SendOrderPaidEmail(mail.NewEmail(user.Login, user.Email), int(order.ID), template); err != nil {
														logger.Errorf("%+v", err)
													}
												}else{
													logger.Warningf("User's %v email %v is not confirmed", user.Login, user.Email)
												}
											} else {
												logger.Warningf("%+v", err)
											}
										}
									}
								}
								return sendString(c, http.StatusOK, map[string]interface{}{"MESSAGE": "OK", "Status": o.Status})
							}else{
								return sendString(c, http.StatusInternalServerError, map[string]interface{}{"ERROR": "Unknown status: " + o.Status, "Status": o.Status})
							}
						}else{
							logger.Errorf("%+v", err)
							return sendString(c, http.StatusInternalServerError, map[string]interface{}{"ERROR": err.Error()})
						}
					}
				}else{
					logger.Errorf("%+v", err)
					return sendString(c, http.StatusInternalServerError, map[string]interface{}{"ERROR": err.Error()})
				}
			}
		}else{
			logger.Errorf("%+v", err)
			return sendString(c, http.StatusInternalServerError, map[string]interface{}{"ERROR": err.Error()})
		}
	}else{
		logger.Errorf("%+v", err)
		return sendString(c, http.StatusInternalServerError, map[string]interface{}{"ERROR": err.Error()})
	}
	err := fmt.Errorf("something wrong")
	logger.Errorf("%+v", err)
	return sendString(c, http.StatusInternalServerError, map[string]interface{}{"ERROR": err.Error()})
}

func sendString(c *fiber.Ctx, status int, raw map[string]interface{}) error {
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTML)
	if bts, err := json.Marshal(raw); err == nil {
		c.Status(status)
		return c.SendString("<script>window.opener.postMessage(JSON.stringify(" + string(bts) + "),'*');</script>")
	}else{
		c.Status(http.StatusInternalServerError)
		return c.SendString("<b>ERROR:</b> " + err.Error())
	}
}