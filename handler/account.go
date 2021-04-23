package handler

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/gofiber/fiber/v2"
	"github.com/google/logger"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/models"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type AccountView struct {
	Admin bool
	BillingProfiles []ProfileView `json:",omitempty"`
	BillingProfileId uint `json:",omitempty"`
	ShippingProfiles []ProfileView `json:",omitempty"`
	ShippingProfileId uint `json:",omitempty"`
	Wishes []WishView
	UserView
}

// @security BasicAuth
// @Summary Get account
// @Description get account
// @Accept json
// @Produce json
// @Success 200 {object} AccountView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account [get]
// @Tags account
// @Tags frontend
func getAccountHandler(c *fiber.Ctx) error {
	if v := c.Locals("user"); v != nil {
		if user, ok := v.(*models.User); ok {
			var err error
			if user, err = models.GetUserFull(common.Database, user.ID); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			var view AccountView
			if bts, err := json.Marshal(user); err == nil {
				if err = json.Unmarshal(bts, &view); err == nil {
					view.Admin = user.Role < models.ROLE_USER
					if len(user.BillingProfiles) > 0 {
						view.BillingProfileId = user.BillingProfiles[len(user.BillingProfiles) - 1].ID
					}
					if len(user.ShippingProfiles) > 0 {
						view.ShippingProfileId = user.ShippingProfiles[len(user.ShippingProfiles) - 1].ID
					}
					if wishes, err := models.GetWishesByUserId(common.Database, user.ID); err == nil {
						var views []WishWrapperView
						if bts, err := json.Marshal(wishes); err == nil {
							if err = json.Unmarshal(bts, &views); err == nil {
								view.Wishes = make([]WishView, len(views))
								for i := 0; i < len(views); i++ {
									view.Wishes[i].WishWrapperView = views[i]
									if err = json.Unmarshal([]byte(wishes[i].Description), &view.Wishes[i].WishItemView); err == nil {
										view.Wishes[i].Description = ""
									}else{
										c.Status(http.StatusInternalServerError)
										return c.JSON(HTTPError{err.Error()})
									}
								}
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
	}
	return c.JSON(HTTPError{"Something went wrong"})
}

type NewAccount struct {
	Email string
	Password string
	Csrf string
	Name string
	Lastname string
	//
	BillingProfile NewProfile
	ShippingProfile NewProfile
	//
	AllowReceiveEmails bool
}

type Account2View struct {
	AccountView
	Token string `json:",omitempty"`
	Expiration *time.Time `json:",omitempty"`
}

// CreateAccout godoc
// @Summary Create account
// @Accept json
// @Produce json
// @Param profile body NewAccount true "body"
// @Success 200 {object} Account2View
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account [post]
// @Tags account
// @Tags frontend
func postAccountHandler(c *fiber.Ctx) error {
	var request NewAccount
	if err := c.BodyParser(&request); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	logger.Infof("Profile1: %+v", request)
	//
	if request.Email == "" {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Email is empty"})
	}
	email := request.Email
	//
	if _, err := models.GetUserByEmail(common.Database, email); err == nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{"Account already exists"})
	}
	//
	var login string
	if res := regexp.MustCompile(`^([^@]+)@`).FindAllStringSubmatch(email, 1); len(res) > 0 && len(res[0]) > 1 {
		login = fmt.Sprintf("%v-%d", res[0][1], rand.New(rand.NewSource(time.Now().UnixNano())).Intn(8999) + 1000)
	}
	var password = NewPassword(12)
	if request.Password != "" {
		password = request.Password
	}
	request.Name = strings.TrimSpace(request.Name)
	if len(request.Name) > 32 {
		request.Name = request.Name[0:32]
	}
	request.Lastname = strings.TrimSpace(request.Lastname)
	if len(request.Lastname) > 32 {
		request.Lastname = request.Lastname[0:32]
	}
	logger.Infof("Create new user %v %v by email %v", login, password, email)
	user := &models.User{
		Enabled: true,
		Email: email,
		EmailConfirmed: true,
		Login: login,
		Password: models.MakeUserPassword(password),
		Name: request.Name,
		Lastname: request.Lastname,
		Role: models.ROLE_USER,
		Notification: true,
		AllowReceiveEmails: request.AllowReceiveEmails,
	}
	// Billing profile
	if err := sanitizeProfile(&request.BillingProfile); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	// Shipping profile
	if request.ShippingProfile.Name != "" {
		if err := sanitizeProfile(&request.ShippingProfile); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		if bts, err := json.Marshal(request.BillingProfile); err == nil {
			if err = json.Unmarshal(bts, &request.ShippingProfile); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}
	//
	id, err := models.CreateUser(common.Database, user)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	user.ID = id
	//
	template, err := models.GetEmailTemplateByType(common.Database, common.NOTIFICATION_TYPE_CREATE_ACCOUNT)
	if err == nil {
		if user, err := models.GetUser(common.Database, int(user.ID)); err == nil {
			if user.EmailConfirmed {
				logger.Infof("Send email to user: %+v", user.Email)
				vars := &common.NotificationTemplateVariables{
					Url: common.Config.Url,
					Email: user.Email,
					Password: password,
				}
				if err := common.NOTIFICATION.SendEmail(mail.NewEmail(common.Config.Notification.Email.Name, common.Config.Notification.Email.Email), mail.NewEmail(user.Login, user.Email), template.Topic, template.Message, vars); err != nil {
					logger.Warningf("%+v", err)
				}
			}else{
				logger.Warningf("User's %v email %v is not confirmed", user.Login, user.Email)
			}
		} else {
			logger.Warningf("%+v", err)
		}
	}
	// Billing ShillingProfile
	billingProfile := &models.BillingProfile{
		Name:     request.BillingProfile.Name,
		Lastname: request.BillingProfile.Lastname,
		Email: request.BillingProfile.Email,
		Company:  request.BillingProfile.Company,
		Phone:    request.BillingProfile.Phone,
		Address:  request.BillingProfile.Address,
		Zip:      request.BillingProfile.Zip,
		City:     request.BillingProfile.City,
		Region:   request.BillingProfile.Region,
		Country:  request.BillingProfile.Country,
		UserId:   user.ID,
	}
	billingProfileId, err := models.CreateBillingProfile(common.Database, billingProfile)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	user.BillingProfiles = []*models.BillingProfile{billingProfile}
	// Shipping ShillingProfile
	shippingProfile := &models.ShippingProfile{
		Name:     request.ShippingProfile.Name,
		Lastname: request.ShippingProfile.Lastname,
		Email: request.ShippingProfile.Email,
		Company:  request.ShippingProfile.Company,
		Phone:    request.ShippingProfile.Phone,
		Address:  request.ShippingProfile.Address,
		Zip:      request.ShippingProfile.Zip,
		City:     request.ShippingProfile.City,
		Region:   request.ShippingProfile.Region,
		Country:  request.ShippingProfile.Country,
		UserId:   user.ID,
	}
	shippingProfileId, err := models.CreateShippingProfile(common.Database, shippingProfile)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	user.ShippingProfiles = []*models.ShippingProfile{shippingProfile}
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(c.Request().Header.Header())))
	if err != nil {
		logger.Errorf("%+v", err)
		return c.JSON(HTTPError{err.Error()})
	}
	/* *** */
	// How to create profile?
	/*var profileId uint
	if request.ShillingProfile.Name != "" {
		logger.Infof("Profile2: %+v", request.ShillingProfile)
		// create profile from shipping data
		var name = strings.TrimSpace(request.ShillingProfile.Name)
		if name == "" {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Name is empty"})
		}
		var lastname = strings.TrimSpace(request.ShillingProfile.Lastname)
		if lastname == "" {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Lastname is empty"})
		}
		var company = strings.TrimSpace(request.ShillingProfile.Company)
		var phone = strings.TrimSpace(request.ShillingProfile.Phone)
		if len(phone) > 64 {
			phone = ""
		}
		var address = strings.TrimSpace(request.ShillingProfile.Address)
		if address == "" {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Address is empty"})
		}
		var zip = strings.TrimSpace(request.ShillingProfile.Zip)
		if zip == "" {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Zip is empty"})
		}
		var city = strings.TrimSpace(request.ShillingProfile.City)
		if city == "" {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"City is empty"})
		}
		var region = strings.TrimSpace(request.ShillingProfile.Region)
		if region == "" {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Region is empty"})
		}
		var country = strings.TrimSpace(request.ShillingProfile.Country)
		if country == "" {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Country is empty"})
		}
		var itn = strings.TrimSpace(request.ShillingProfile.ITN)
		if len(itn) > 32 {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"ITN is incorrect"})
		}
		profile := &models.ShillingProfile{
			Name:     name,
			Lastname: lastname,
			Company:  company,
			Phone:    phone,
			Address:  address,
			Zip:      zip,
			City:     city,
			Region:   region,
			Country:  country,
			ITN:      itn,
			UserId:   user.ID,
		}
		if profileId, err = models.CreateProfile(common.Database, profile); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
		user.Profiles = []*models.ShillingProfile{profile}
	}else{
		// create profile from billing
		profile := &models.ShillingProfile{
			Name:     name,
			Lastname: lastname,
			Company:  company,
			Phone:    phone,
			Address:  address,
			Zip:      zip,
			City:     city,
			Region:   region,
			Country:  country,
			ITN:      itn,
			UserId:   user.ID,
			Billing:  true,
		}
		if profileId, err = models.CreateProfile(common.Database, profile); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
		user.Profiles = []*models.ShillingProfile{profile}
	}*/
	var view AccountView
	if bts, err := json.Marshal(user); err == nil {
		if err = json.Unmarshal(bts, &view); err == nil {
			view.Admin = user.Role < models.ROLE_USER
			view.BillingProfileId = billingProfileId
			view.ShippingProfileId = shippingProfileId
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	if v := req.Header.Get("Accept"); strings.EqualFold(v, "application/jwt") {
		expiration := time.Now().AddDate(1, 0, 0)
		claims := &JWTClaims{
			Login: user.Login,
			Password: user.Password,
			StandardClaims: jwt.StandardClaims{
				ExpiresAt: expiration.Unix(),
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		if str, err := token.SignedString(JWTSecret); err == nil {
			c.Status(http.StatusOK)
			return c.JSON(Account2View{
				AccountView: view,
				Token: str,
				Expiration: &expiration,
			})
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		credentials := map[string]string{
			"email": email,
			"login": login,
			"password": password,
		}
		if encoded, err := cookieHandler.Encode(COOKIE_NAME, credentials); err == nil {
			cookie := &fiber.Cookie{
				Name:  COOKIE_NAME,
				Value: encoded,
				Path:  "/",
				Expires: time.Now().AddDate(1, 0, 0),
				SameSite: authMultipleConfig.SameSite,
			}
			c.Cookie(cookie)
		}
		c.Status(http.StatusOK)
		return c.JSON(view)
	}
}

type User2View struct {
	OldPassword string
	NewPassword string
	NewPassword2 string
	Name string
	Lastname string
}

// @security BasicAuth
// UpdateAccount godoc
// @Summary Update account
// @Accept json
// @Produce json
// @Param account body AccountView true "body"
// @Success 200 {object} User2View
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account [put]
// @Tags frontend
// @Tags account
func putAccountHandler(c *fiber.Ctx) error {
	if v := c.Locals("user"); v != nil {
		var user *models.User
		var ok bool
		if user, ok = v.(*models.User); ok {
			//
			var request User2View
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			request.OldPassword = strings.TrimSpace(request.OldPassword)
			request.NewPassword = strings.TrimSpace(request.NewPassword)
			request.NewPassword2 = strings.TrimSpace(request.NewPassword2)
			request.Name = strings.TrimSpace(request.Name)
			if len(request.Name) > 32 {
				request.Name = request.Name[0:32]
			}
			request.Lastname = strings.TrimSpace(request.Lastname)
			if len(request.Lastname) > 32 {
				request.Lastname = request.Lastname[0:32]
			}
			if request.NewPassword != "" {
				// !!! IMPORTANT !!!
				/*if models.MakeUserPassword(request.OldPassword) != user.Password {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{"Incorrect password"})
				}*/
				if len(request.NewPassword) < 6 {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{"Too short password"})
				}
				if len(request.NewPassword) > 32 {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{"Too long password"})
				}
				if request.NewPassword != request.NewPassword2 {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{"Passwords mismatch"})
				}
				user.Password = models.MakeUserPassword(request.NewPassword2)
			}
			//
			user.Name = request.Name
			user.Lastname = request.Lastname
			if err := models.UpdateUser(common.Database, user); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			//
			var err error
			if user, err = models.GetUserFull(common.Database, user.ID); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			var view struct {
				AccountView
				Token string
				Expiration time.Time
			}
			expiration := time.Now().Add(JWTLoginDuration)
			claims := &JWTClaims{
				Login: user.Login,
				Password: user.Password,
				StandardClaims: jwt.StandardClaims{
					ExpiresAt: expiration.Unix(),
				},
			}
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
			str, err := token.SignedString(JWTSecret)
			if err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(fiber.Map{"ERROR": err.Error()})
			}
			view.Token = str
			view.Expiration = expiration
			if bts, err := json.Marshal(user); err == nil {
				if err = json.Unmarshal(bts, &view); err == nil {
					view.Admin = user.Role < models.ROLE_USER
					if len(user.BillingProfiles) > 0 {
						view.BillingProfileId = user.BillingProfiles[len(user.BillingProfiles) - 1].ID
					}
					if len(user.ShippingProfiles) > 0 {
						view.ShippingProfileId = user.ShippingProfiles[len(user.ShippingProfiles) - 1].ID
					}
					if wishes, err := models.GetWishesByUserId(common.Database, user.ID); err == nil {
						var views []WishWrapperView
						if bts, err := json.Marshal(wishes); err == nil {
							if err = json.Unmarshal(bts, &views); err == nil {
								view.Wishes = make([]WishView, len(views))
								for i := 0; i < len(views); i++ {
									view.Wishes[i].WishWrapperView = views[i]
									if err = json.Unmarshal([]byte(wishes[i].Description), &view.Wishes[i].WishItemView); err == nil {
										view.Wishes[i].Description = ""
									}else{
										c.Status(http.StatusInternalServerError)
										return c.JSON(HTTPError{err.Error()})
									}
								}
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
			return c.JSON(HTTPError{"User not found"})
		}
	}
	c.Status(http.StatusInternalServerError)
	return c.JSON(HTTPError{"Something went wrong"})
}

type NewProfile struct {
	Email string
	Name string
	Lastname string
	Company string
	Phone string
	Address string
	Zip string
	City string
	Region string
	Country string
	ITN string
}

// @security BasicAuth
// @Summary Get account billing profiles
// @Description get account billing profiles
// @Accept json
// @Produce json
// @Success 200 {object} []ProfileView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/billing_profiles [get]
// @Tags account
// @Tags frontend
func getAccountBillingProfilesHandler(c *fiber.Ctx) error {
	if v := c.Locals("user"); v != nil {
		if user, ok := v.(*models.User); ok {
			if profiles, err := models.GetBillingProfilesByUser(common.Database, user.ID); err == nil {
				var views []ProfileView
				if bts, err := json.Marshal(profiles); err == nil {
					if err = json.Unmarshal(bts, &views); err == nil {
						return c.JSON(views)
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
	}
	return c.JSON(HTTPError{"Something went wrong"})
}

// @security BasicAuth
// CreateBillingProfile godoc
// @Summary Create billing profile in existing account
// @Accept json
// @Produce json
// @Param profile body NewProfile true "body"
// @Success 200 {object} ProfileView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/billing_profiles [post]
// @Tags profile
// @Tags frontend
func postAccountBillingProfileHandler(c *fiber.Ctx) error {
	var view ProfileView
	//
	var request NewProfile
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	//
	var userId uint
	if v := c.Locals("user"); v != nil {
		var user *models.User
		var ok bool
		if user, ok = v.(*models.User); ok {
			userId = user.ID
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"User not found"})
		}
	}
	//
	if err := sanitizeProfile(&request); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	profile := &models.BillingProfile{
		Name:     request.Name,
		Lastname: request.Lastname,
		Email: request.Email,
		Company:  request.Company,
		Phone:    request.Phone,
		Address:  request.Address,
		Zip:      request.Zip,
		City:     request.City,
		Region:   request.Region,
		Country:  request.Country,
		UserId:   userId,
	}
	//
	if _, err := models.CreateBillingProfile(common.Database, profile); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	if bts, err := json.Marshal(profile); err == nil {
		if err = json.Unmarshal(bts, &view); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}
	//
	return c.JSON(view)
}

// GetBillingProfile godoc
// @Summary Get billing profile
// @Accept json
// @Produce json
// @Param id path int true "BillingProfile ID"
// @Success 200 {object} OrderView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/billing_profiles/{id} [get]
// @Tags account
// @Tags frontend
func getAccountBillingProfileHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var userId uint
	if v := c.Locals("user"); v != nil {
		if user, ok := v.(*models.User); ok {
			userId = user.ID
		}
	}
	if profile, err := models.GetBillingProfile(common.Database, uint(id)); err == nil {
		if profile.UserId != userId {
			c.Status(http.StatusForbidden)
			return c.JSON(fiber.Map{"ERROR": "You are not allowed to do that"})
		}
		var view BillingProfileOrderView
		if bts, err := json.MarshalIndent(profile, "", "   "); err == nil {
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
// UpdateBillingProfile godoc
// @Summary Update billing profile
// @Accept json
// @Produce json
// @Param id path int true "Profile ID"
// @Param profile body NewProfile true "body"
// @Success 200 {object} ProfileView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/billing_profiles/{id} [put]
// @Tags profile
// @Tags frontend
func putAccountBillingProfileHandler(c *fiber.Ctx) error {
	var view ProfileView
	//
	var request NewProfile
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	//
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	profile, err := models.GetBillingProfile(common.Database, uint(id))
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	if v := c.Locals("user"); v != nil {
		var user *models.User
		var ok bool
		if user, ok = v.(*models.User); ok {
			if profile.UserId != user.ID {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Access violation"})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"User not found"})
		}
	}
	//
	if err := sanitizeProfile(&request); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	profile.Name = request.Name
	profile.Lastname = request.Lastname
	profile.Email = request.Email
	profile.Company = request.Company
	profile.Phone = request.Phone
	profile.Address = request.Address
	profile.Zip = request.Zip
	profile.City = request.City
	profile.Region = request.Region
	profile.Country = request.Country
	//
	if err := models.UpdateBillingProfile(common.Database, profile); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	if bts, err := json.Marshal(profile); err == nil {
		if err = json.Unmarshal(bts, &view); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}
	//
	return c.JSON(view)
}

// @security BasicAuth
// DelBillingProfile godoc
// @Summary Delete billing profile
// @Accept json
// @Produce json
// @Param id path int true "Billing Profile ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/billing_profiles/{id} [delete]
// @Tags account
// @Tags frontend
func delAccountBillingProfileHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	profile, err := models.GetBillingProfile(common.Database, uint(id))
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	if v := c.Locals("user"); v != nil {
		var user *models.User
		var ok bool
		if user, ok = v.(*models.User); ok {
			if profile.UserId != user.ID {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Access violation"})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"User not found"})
		}
	}
	//
	if err = models.DeleteBillingProfile(common.Database, profile); err == nil {
		return c.JSON(HTTPMessage{"OK"})
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

// @security BasicAuth
// @Summary Get account shipping profiles
// @Description get account shipping profiles
// @Accept json
// @Produce json
// @Success 200 {object} []ProfileView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/shipping_profiles [get]
// @Tags account
// @Tags frontend
func getAccountShippingProfilesHandler(c *fiber.Ctx) error {
	if v := c.Locals("user"); v != nil {
		if user, ok := v.(*models.User); ok {
			if profiles, err := models.GetShippingProfilesByUser(common.Database, user.ID); err == nil {
				var views []ProfileView
				if bts, err := json.Marshal(profiles); err == nil {
					if err = json.Unmarshal(bts, &views); err == nil {
						return c.JSON(views)
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
	}
	return c.JSON(HTTPError{"Something went wrong"})
}

// @security BasicAuth
// CreateShippingProfile godoc
// @Summary Create shipping profile in existing account
// @Accept json
// @Produce json
// @Param profile body NewProfile true "body"
// @Success 200 {object} ProfileView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/billing_profiles [post]
// @Tags profile
// @Tags frontend
func postAccountShippingProfileHandler(c *fiber.Ctx) error {
	var view ProfileView
	//
	var request NewProfile
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	//
	var userId uint
	if v := c.Locals("user"); v != nil {
		var user *models.User
		var ok bool
		if user, ok = v.(*models.User); ok {
			userId = user.ID
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"User not found"})
		}
	}
	//
	if err := sanitizeProfile(&request); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	profile := &models.ShippingProfile{
		Name:     request.Name,
		Lastname: request.Lastname,
		Email: request.Email,
		Company:  request.Company,
		Phone:    request.Phone,
		Address:  request.Address,
		Zip:      request.Zip,
		City:     request.City,
		Region:   request.Region,
		Country:  request.Country,
		UserId:   userId,
	}
	//
	if _, err := models.CreateShippingProfile(common.Database, profile); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	if bts, err := json.Marshal(profile); err == nil {
		if err = json.Unmarshal(bts, &view); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}
	//
	return c.JSON(view)
}

// GetShippingProfile godoc
// @Summary Get shipping profile
// @Accept json
// @Produce json
// @Param id path int true "ShippingProfile ID"
// @Success 200 {object} OrderView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/shipping_profiles/{id} [get]
// @Tags account
// @Tags frontend
func getAccountShippingProfileHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var userId uint
	if v := c.Locals("user"); v != nil {
		if user, ok := v.(*models.User); ok {
			userId = user.ID
		}
	}
	if profile, err := models.GetShippingProfile(common.Database, uint(id)); err == nil {
		if profile.UserId != userId {
			c.Status(http.StatusForbidden)
			return c.JSON(fiber.Map{"ERROR": "You are not allowed to do that"})
		}
		var view ShippingProfileOrderView
		if bts, err := json.MarshalIndent(profile, "", "   "); err == nil {
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
// UpdateShippingProfile godoc
// @Summary Update shipping profile
// @Accept json
// @Produce json
// @Param id path int true "Profile ID"
// @Param profile body NewProfile true "body"
// @Success 200 {object} ProfileView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/shipping_profiles/{id} [put]
// @Tags profile
// @Tags frontend
func putAccountShippingProfileHandler(c *fiber.Ctx) error {
	var view ProfileView
	//
	var request NewProfile
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	//
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	profile, err := models.GetShippingProfile(common.Database, uint(id))
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	if v := c.Locals("user"); v != nil {
		var user *models.User
		var ok bool
		if user, ok = v.(*models.User); ok {
			if profile.UserId != user.ID {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Access violation"})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"User not found"})
		}
	}
	//
	if err := sanitizeProfile(&request); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	profile.Name = request.Name
	profile.Lastname = request.Lastname
	profile.Email = request.Email
	profile.Company = request.Company
	profile.Phone = request.Phone
	profile.Address = request.Address
	profile.Zip = request.Zip
	profile.City = request.City
	profile.Region = request.Region
	profile.Country = request.Country
	//
	if err := models.UpdateShippingProfile(common.Database, profile); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	if bts, err := json.Marshal(profile); err == nil {
		if err = json.Unmarshal(bts, &view); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}
	//
	return c.JSON(view)
}

// @security BasicAuth
// DelShippingProfile godoc
// @Summary Delete shipping profile
// @Accept json
// @Produce json
// @Param id path int true "Shipping Profile ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/shipping_profiles/{id} [delete]
// @Tags account
// @Tags frontend
func delAccountShippingProfileHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	profile, err := models.GetShippingProfile(common.Database, uint(id))
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	//
	if v := c.Locals("user"); v != nil {
		var user *models.User
		var ok bool
		if user, ok = v.(*models.User); ok {
			if profile.UserId != user.ID {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Access violation"})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"User not found"})
		}
	}
	//
	if err = models.DeleteShippingProfile(common.Database, profile); err == nil {
		return c.JSON(HTTPMessage{"OK"})
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

type OrdersView []*OrderView

type OrderView struct {
	ID uint
	CreatedAt time.Time
	Items []*ItemView
	Status string
	Sum float64
	Delivery float64
	Total float64
	Comment string `json:",omitempty"`
}

type ItemView struct{
	ID uint
	Uuid string
	Title string
	Description string `json:",omitempty"`
	Path string
	Thumbnail string
	Variation VariationShortView `json:",omitempty"`
	Properties []PropertyShortView `json:",omitempty"`
	Comment string `json:",omitempty"`
	Price float64
	Quantity int
	Total float64
}

// GetOrders godoc
// @Summary Get account orders
// @Accept json
// @Produce json
// @Success 200 {object} OrdersView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders [get]
// @Tags account
// @Tags frontend
func getAccountOrdersHandler(c *fiber.Ctx) error {
	var userId uint
	if v := c.Locals("user"); v != nil {
		if user, ok := v.(*models.User); ok {
			userId = user.ID
		}
	}
	if orders, err := models.GetOrdersByUserId(common.Database, userId); err == nil {
		var views []*OrderView
		if bts, err := json.Marshal(orders); err == nil {
			if err = json.Unmarshal(bts, &views); err == nil {
				for i := 0; i < len(views); i++ {
					for j := 0; j < len(views[i].Items); j++ {
						var itemView ItemShortView
						if err = json.Unmarshal([]byte(views[i].Items[j].Description), &itemView); err == nil {
							views[i].Items[j].Description = ""
							views[i].Items[j].Properties = itemView.Properties
							views[i].Items[j].Variation = itemView.Variation
						}else{
							logger.Warningf("%+v", err)
						}
					}
				}
				return c.JSON(views)
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

// GetOrder godoc
// @Summary Get order
// @Accept json
// @Produce json
// @Param id path int true "Order ID"
// @Success 200 {object} OrderView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders/{id} [get]
// @Tags account
// @Tags frontend
func getAccountOrderHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	var userId uint
	if v := c.Locals("user"); v != nil {
		if user, ok := v.(*models.User); ok {
			userId = user.ID
		}
	}
	if order, err := models.GetOrder(common.Database, id); err == nil {
		if order.UserId != userId {
			c.Status(http.StatusForbidden)
			return c.JSON(fiber.Map{"ERROR": "You are not allowed to do that"})
		}
		var view OrderView
		if bts, err := json.MarshalIndent(order, "", "   "); err == nil {
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
// UpdateOrder godoc
// @Summary Update order by user
// @Accept json
// @Produce json
// @Param user body ExistingOrder true "body"
// @Param id path int true "Order ID"
// @Success 200 {object} UserView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account/orders/{id} [put]
// @Tags order
func putAccountOrderHandler(c *fiber.Ctx) error {
	var request ExistingOrder
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
	var order *models.Order
	var err error
	if order, err = models.GetOrder(common.Database, int(id)); err != nil {
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
	if (order.Status == models.ORDER_STATUS_NEW || order.Status == models.ORDER_STATUS_WAITING_FROM_PAYMENT) && request.Status == models.ORDER_STATUS_CANCELED {
		order.Status = models.ORDER_STATUS_CANCELED
	}
	if err := models.UpdateOrder(common.Database, order); err == nil {
		return c.JSON(HTTPMessage{"OK"})
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}

func sanitizeProfile (p *NewProfile) error {
	p.Email = strings.TrimSpace(p.Email)
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return fmt.Errorf("name is empty")
	}
	p.Lastname = strings.TrimSpace(p.Lastname)
	if p.Lastname == "" {
		return fmt.Errorf("lastname is empty")
	}
	p.Company = strings.TrimSpace(p.Company)
	p.Phone = strings.TrimSpace(p.Phone)
	if len(p.Phone) > 32 {
		p.Phone = ""
	}
	p.Address = strings.TrimSpace(p.Address)
	if p.Address == "" {
		return fmt.Errorf("address is empty")
	}
	p.Zip = strings.TrimSpace(p.Zip)
	if p.Zip == "" {
		return fmt.Errorf("zip is empty")
	}
	p.City = strings.TrimSpace(p.City)
	if p.City == "" {
		return fmt.Errorf("city is empty")
	}
	p.Region = strings.TrimSpace(p.Region)
	/*if p.Region == "" {
		return fmt.Errorf("region is empty")
	}*/
	p.Country = strings.TrimSpace(p.Country)
	if p.Country == "" {
		return fmt.Errorf("country is empty")
	}
	p.ITN = strings.TrimSpace(p.ITN)
	if len(p.ITN) > 32 {
		return fmt.Errorf("itn is incorrect")
	}
	return nil
}