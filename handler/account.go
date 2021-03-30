package handler

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/gofiber/fiber/v2"
	"github.com/google/logger"
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
	Csrf string
	//
	BillingProfile NewProfile
	ShippingProfile NewProfile
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
	password := NewPassword(12)
	logger.Infof("Create new user %v %v by email %v", login, password, email)
	user := &models.User{
		Enabled: true,
		Email: email,
		EmailConfirmed: true,
		Login: login,
		Password: models.MakeUserPassword(password),
		Role: models.ROLE_USER,
		Notification: true,
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
}

// @security BasicAuth
// UpdateAccount godoc
// @Summary update account
// @Accept json
// @Produce json
// @Param account body AccountView true "body"
// @Success 200 {object} User2View
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/account [put]
// @Tags order
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
			//
			if models.MakeUserPassword(request.OldPassword) != user.Password {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{"Incorrect password"})
			}
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
			if err := models.UpdateUser(common.Database, user); err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			c.Status(http.StatusOK)
			return c.JSON(HTTPMessage{"OK"})
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
// @Router /api/v1/account/billing_profile/{id} [delete]
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
// @Router /api/v1/account/shipping_profile/{id} [delete]
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