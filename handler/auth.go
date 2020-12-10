package handler

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html"
	"github.com/google/logger"
	"github.com/gorilla/securecookie"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/models"
	"io"
	"io/ioutil"
	math_rand "math/rand"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	TEMPLATES_FOLDER = "templates"
	LOGIN_TEMPLATE = `<!DOCTYPE html>
<html>
    <head>
        <title>Login</title>
    </head>
    <body>
        <form action="{{ .Action }}" method="POST">
            {{ if .Error }}
                <b>Error: {{ .Error }}</b><br/>
            {{ end }}
			<input type="text" name="login" placeholder="Login" autocomplete="off"><br />
			<input type="password" name="password" placeholder="Password" autocomplete="off"><br />
            {{ if .Remember }}
			<input type="checkbox" name="remember" value="true"> Remember Me</input><br />
            {{ end }}
			<button type="submit">Login</button>
		</form>
    </body>
</html>`
	REGISTER_TEMPLATE = `<!DOCTYPE html>
<html>
    <head>
        <title>Register</title>
    </head>
    <body>
        <form action="{{ .Action }}" method="POST">
            {{ if .Error }}
                <b>Error: {{ .Error }}</b><br/>
            {{ end }}
			<input type="email" name="email" placeholder="Email" autocomplete="off"><br />
			<input type="password" name="password" placeholder="Password" autocomplete="off"><br />
			<input type="password" name="password2" placeholder="Password Again" autocomplete="off"><br />
			<button type="submit">Register</button>
		</form>
    </body>
</html>`
)

var (
	dir, _ = filepath.Abs(filepath.Dir(os.Args[0]))

	authMultipleConfig AuthMultipleConfig

	DEFAULT_USER = "admin"
	DEFAULT_PASSWORD = "goshoppass"

	COOKIE_NAME = "goshop"
	HASH16 = "#goshop16#######"
	HASH32 = "#goshop32#######################"
	cookieHandler *securecookie.SecureCookie
)

type AuthMultipleConfig struct {
	FiberConfig                  fiber.Config
	AuthRedirect                 bool // send redirect 302 instead of 401
	CookieDuration               time.Duration
	EmailConfirmationRequired    bool
	Log bool
	PasswordMinLength            int
	PasswordSpecialChartRequired bool
	SameSite                     string
	UseForm                      bool // use html form to login
}

func CreateFiberAppWithAuthMultiple(config AuthMultipleConfig, middleware ...interface{}) (*fiber.App, func (c *fiber.Ctx) error) {
	if config.PasswordMinLength == 0 {
		config.PasswordMinLength = 6
	}
	if config.SameSite == "" {
		config.SameSite = "None"
	}
	authMultipleConfig = config
	// Create default user
	common.Database.AutoMigrate(&models.User{})
	if users, err := models.GetUsers(common.Database); err != nil || len(users) == 0 {
		user := models.User{
			Enabled:        true,
			Login:          DEFAULT_USER,
			Email:          DEFAULT_USER + "@goshop",
			EmailConfirmed: true,
			Password:       models.MakeUserPassword(DEFAULT_PASSWORD),
			Role: models.ROLE_ROOT,
		}
		models.CreateUser(common.Database, &user)
	}
	if config.UseForm {
		// Check pages
		if p := path.Join(dir, TEMPLATES_FOLDER); len(p) > 0 {
			if _, err := os.Stat(p); err != nil {
				os.MkdirAll(p, 0755)
			}
			if p := path.Join(p, "login.html"); len(p) > 0 {
				if _, err := os.Stat(p); err != nil {
					if err = ioutil.WriteFile(p, []byte(LOGIN_TEMPLATE), 0755); err != nil {
						logger.Errorf("%v", err)
					}
				}
			}
			if p := path.Join(p, "register.html"); len(p) > 0 {
				if _, err := os.Stat(p); err != nil {
					if err = ioutil.WriteFile(p, []byte(REGISTER_TEMPLATE), 0755); err != nil {
						logger.Errorf("%v", err)
					}
				}
			}
		}
	}
	//
	cookieHandler = func() *securecookie.SecureCookie {
		if value := os.Getenv("HASH16"); value != "" {
			HASH16 = value
		}
		if value := os.Getenv("HASH32"); value != "" {
			HASH32 = value
		}
		return securecookie.New([]byte(HASH16),[]byte(HASH32))
	} ()
	//
	if config.UseForm {
		engine := html.New(path.Join(dir, TEMPLATES_FOLDER), ".html")
		// You can add engine function here
		/*engine.AddFunc("greet", func(name string) string {
			return "Hello, " + name + "!"
		})*/
		config.FiberConfig.Views = engine
	}
	app := fiber.New(config.FiberConfig)
	app.Use(middleware...)
	// Login
	if config.UseForm {
		app.Get("/login", func(c *fiber.Ctx) error {
			var action string
			if referer := c.Request().Header.Referer(); len(referer) > 0 {
				action = "?ref=" + base64.URLEncoding.EncodeToString(referer)
			}
			return c.Render("login", fiber.Map{
				"Action":   action,
				"Remember": true,
			})
		})
	}
	app.Post("/api/v1/login", postLoginHandler)
	app.Post("/login", postLoginHandler)
	// Register
	if config.UseForm {
		app.Get("/register", func(c *fiber.Ctx) error {
			var action string
			if referer := c.Request().Header.Referer(); len(referer) > 0 {
				action = "?ref=" + base64.URLEncoding.EncodeToString(referer)
			}
			return c.Render("register", fiber.Map{
				"Action":   action,
			})
		})
	}
	postRegisterHandler := func (c *fiber.Ctx) error {
		var request struct {
			Email string
			Password string
			Password2 string
		}
		if err := c.BodyParser(&request); err != nil {
			return err
		}
		req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(c.Request().Header.Header())))
		if err != nil {
			logger.Errorf("%+v", err)
		}
		request.Email = strings.TrimSpace(request.Email)
		request.Password = strings.TrimSpace(request.Password)
		request.Password2 = strings.TrimSpace(request.Password2)
		var e error
		if request.Email == "" || !regexp.MustCompile("[^@]+@[^@]+").MatchString(request.Email) {
			e = fmt.Errorf("Invalid email")
		}
		if e == nil && len(request.Password) < config.PasswordMinLength {
			e = fmt.Errorf("Password should contains at least %d chars", config.PasswordMinLength)
		}
		if e == nil && !regexp.MustCompile("(?i)[0-9]+").MatchString(request.Password) {
			e = fmt.Errorf("Password should contains digits")
		}
		if e == nil && !regexp.MustCompile("(?i)[a-z]+").MatchString(request.Password) {
			e = fmt.Errorf("Password should contains abc chars")
		}
		if e == nil && config.PasswordSpecialChartRequired && !regexp.MustCompile(`[-_\+=\!@#\$%\^\&\*\(\)\[\]\{\}<>:;'"~]+`).MatchString(request.Password) {
			e = fmt.Errorf("Password should contains special chars")
		}
		if e == nil && request.Password != request.Password2 {
			e = fmt.Errorf("Passwords mismatch")
		}
		if _, err := models.GetUserByEmail(common.Database, request.Email); err == nil {
			e = fmt.Errorf("Email already is use")
		}
		if e != nil {
			if v := req.Header.Get("Content-Type"); v != "" {
				for _, chunk := range strings.Split(v, ";") {
					if strings.EqualFold(chunk, "application/json") {
						c.Status(http.StatusInternalServerError)
						return c.JSON(fiber.Map{"ERROR": e.Error()})
					}
				}
			}
			return c.Render("login", fiber.Map{
				"Error":    e.Error(),
			})
		}
		var login string
		if res := regexp.MustCompile(`^([^@]+)@`).FindAllStringSubmatch(request.Email, 1); len(res) > 0 && len(res[0]) > 1 {
			s := math_rand.NewSource(time.Now().UnixNano())
			r := math_rand.New(s)
			login = fmt.Sprintf("%v-%d", res[0][1], r.Intn(8999) + 1000)
		}
		user := models.User{
			Enabled:  true,
			Login:    login,
			Email:    request.Email,
			Password: models.MakeUserPassword(request.Password),
			Role: models.ROLE_USER,
		}
		if !config.EmailConfirmationRequired {
			user.EmailConfirmed = true
		}
		if _, err := models.CreateUser(common.Database, &user); err == nil {
			value := map[string]string{
				"email": user.Email,
				"login": user.Login,
				"password": request.Password,
			}
			if encoded, err := cookieHandler.Encode(COOKIE_NAME, value); err == nil {
				expires := time.Time{}
				if config.CookieDuration > 0 {
					expires = time.Now().Add(config.CookieDuration)
				}
				cookie := &fiber.Cookie{
					Name:  COOKIE_NAME,
					Value: encoded,
					Path:  "/",
					Expires: expires,
					SameSite: config.SameSite,
				}
				c.Cookie(cookie)
				if v := req.Header.Get("Content-Type"); v != "" {
					for _, chunk := range strings.Split(v, ";") {
						if strings.EqualFold(chunk, "application/json") {
							return c.JSON(fiber.Map{"MESSAGE": "OK"})
						}
					}
				}
				c.Redirect("/", http.StatusFound)
			}else{
				c.Status(http.StatusInternalServerError)
				c.JSON(fiber.Map{"ERROR": err.Error()})
			}
		}else{
			c.Status(http.StatusInternalServerError)
			c.JSON(fiber.Map{"ERROR": err.Error()})
		}
		return nil
	}
	app.Post("/api/v1/register", postRegisterHandler)
	app.Post("/register", postRegisterHandler)
	// Refresh
	app.Get("/refresh", func (c *fiber.Ctx) error {
		if req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(c.Request().Header.Header()))); err == nil {
			if authorization := req.Header.Get("Authorization"); authorization != "" {
				if strings.Index(strings.ToLower(authorization), "bearer ") == 0 {
					const prefix = "Bearer "
					claims := &JWTClaims{}
					logger.Infof("authorization[len(prefix):]: %+v", authorization[len(prefix):])
					if token, err := jwt.ParseWithClaims(authorization[len(prefix):], claims, func(token *jwt.Token) (interface{}, error) {
						return JWTSecret, nil
					}); err == nil {
						if token.Valid {
							if time.Now().Before(time.Unix(claims.ExpiresAt, 0)) {
								if user, err := models.GetUserByLoginAndPassword(common.Database, claims.Login, claims.Password); err == nil {
									expiration := time.Now().Add(JWTLoginDuration)
									claims := &JWTClaims{
										Login: user.Login,
										Password: user.Password,
										StandardClaims: jwt.StandardClaims{
											ExpiresAt: expiration.Unix(),
										},
									}
									token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
									if str, err := token.SignedString(JWTSecret); err == nil {
										c.JSON(fiber.Map{
											"MESSAGE": "OK",
											"Token": str,
											"Expiration": expiration,
										})
									}else{
										c.JSON(fiber.Map{"ERROR": err.Error()})
									}
								} else {
									logger.Errorf("%v", err)
									c.JSON(fiber.Map{"ERROR": err.Error()})
									c.Status(http.StatusInternalServerError)
								}
							} else {
								err = fmt.Errorf("expired token")
								logger.Errorf("%v", err)
								c.JSON(fiber.Map{"ERROR": err.Error()})
								c.Status(http.StatusInternalServerError)
							}
						} else {
							err = fmt.Errorf("invalid token")
							logger.Errorf("%v", err)
							c.JSON(fiber.Map{"ERROR": err.Error()})
							c.Status(http.StatusInternalServerError)
						}
					} else {
						logger.Errorf("%v", err)
						c.JSON(fiber.Map{"ERROR": err.Error()})
						c.Status(http.StatusInternalServerError)
					}
				} else {
					err = fmt.Errorf("unsupported Authorization type")
					logger.Errorf("%v", err)
					c.JSON(fiber.Map{"ERROR": err.Error()})
					c.Status(http.StatusInternalServerError)
				}
			}else{
				err = fmt.Errorf("authorization header missed")
				logger.Errorf("%v", err)
				c.JSON(fiber.Map{"ERROR": err.Error()})
				c.Status(http.StatusInternalServerError)
			}
		}else{
			logger.Errorf("%v", err)
			c.JSON(fiber.Map{"ERROR": err.Error()})
			c.Status(http.StatusInternalServerError)
		}
		return nil
	})
	app.Get("/api/v1/logout", getLogoutHandler)
	app.Get("/logout", getLogoutHandler)
	return app, func (c *fiber.Ctx) error {
		var auth bool
		req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(c.Request().Header.Header())))
		if err != nil {
			return err
		}
		if authorization := req.Header.Get("Authorization"); authorization != "" {
			if strings.Index(strings.ToLower(authorization), "basic ") == 0 {
				const prefix = "Basic "
				if bts, err := base64.StdEncoding.DecodeString(authorization[len(prefix):]); err == nil {
					cs := string(bts)
					if s := strings.IndexByte(cs, ':'); s > -1 {
						var emailOrLogin = cs[:s]
						var password = cs[s+1:]
						if user, err := models.GetUserByEmailOrLoginAndPassword(common.Database, emailOrLogin, models.MakeUserPassword(password)); err == nil {
							if user.Enabled {
								if config.Log {
									logger.Infof("Auth Basic #%v %v %v OK", user.ID, user.Login, user.Email)
								}
								c.Locals("authorization", "basic")
								c.Locals("user", user)
								auth = true
							}else{
								logger.Errorf("User %v is not enabled", user.Login)
							}
						}else{
							logger.Errorf("%v", err)
						}
					}
				}
			}else if strings.Index(strings.ToLower(authorization), "bearer ") == 0 {
				const prefix= "Bearer "
				claims := &JWTClaims{}
				logger.Infof("authorization[len(prefix):]: %+v", authorization[len(prefix):])
				if token, err := jwt.ParseWithClaims(authorization[len(prefix):], claims, func(token *jwt.Token) (interface{}, error) {
					return JWTSecret, nil
				}); err == nil {
					if token.Valid {
						if time.Now().Before(time.Unix(claims.ExpiresAt, 0)) {
							if user, err := models.GetUserByLoginAndPassword(common.Database, claims.Login, claims.Password); err == nil {
								if config.Log {
									logger.Infof("Auth Bearer #%v %v %v up to %v OK", user.ID, user.Login, user.Email, time.Unix(claims.ExpiresAt, 0).Format(time.RFC3339))
								}
								c.Locals("authorization", "jwt")
								c.Locals("expiration", claims.ExpiresAt)
								c.Locals("user", user)
								auth = true
							} else {
								logger.Errorf("%v", err)
								c.JSON(fiber.Map{"ERROR": err.Error()})
								c.Status(http.StatusInternalServerError)
							}
						}else{
							err = fmt.Errorf("expired token")
							logger.Errorf("%v", err)
							c.JSON(fiber.Map{"ERROR": err.Error()})
							c.Status(http.StatusInternalServerError)
						}
					}else{
						err = fmt.Errorf("invalid token")
						logger.Errorf("%v", err)
						c.JSON(fiber.Map{"ERROR": err.Error()})
						c.Status(http.StatusInternalServerError)
					}
				}else{
					logger.Errorf("%v", err)
					c.JSON(fiber.Map{"ERROR": err.Error()})
					c.Status(http.StatusInternalServerError)
				}
			}else{
				err = fmt.Errorf("unsupported Authorization type")
				logger.Errorf("%v", err)
				c.JSON(fiber.Map{"ERROR": err.Error()})
				c.Status(http.StatusInternalServerError)
			}
		}
		if !auth {
			if value := c.Cookies(COOKIE_NAME); len(value) > 0 {
				m := make(map[string]string)
				if err := cookieHandler.Decode(COOKIE_NAME, value, &m); err == nil {
					var login = m["login"]
					var password = m["password"]
					if user, err := models.GetUserByLoginAndPassword(common.Database, login, models.MakeUserPassword(password)); err == nil {
						if config.Log {
							logger.Infof("Auth Cookie #%v %v %v OK", user.ID, user.Login, user.Email)
						}
						c.Locals("authorization", "cookie")
						c.Locals("user", user)
						auth = true
					} else {
						logger.Errorf("%v", err)
					}
				}
			}
		}
		if auth {
			return c.Next()
		}
		if config.Log {
			logger.Infof("Auth fail")
		}
		if config.AuthRedirect {
			return c.Redirect("/login", http.StatusFound)
		}
		c.Status(http.StatusForbidden)
		if v := req.Header.Get("Accept"); v != "" {
			for _, chunk := range strings.Split(v, ";") {
				for _, value := range strings.Split(chunk, ",") {
					if strings.Contains(strings.ToLower(value), "application/json") {
						return c.JSON(fiber.Map{"ERROR": "Unauthenticated"})
					}
				}
			}
		}
		if v := req.Header.Get("Content-Type"); v != "" {
			for _, chunk := range strings.Split(v, ";") {
				if strings.EqualFold(chunk, "application/json") {
					return c.JSON(fiber.Map{"ERROR": "Unauthenticated"})
				}
			}
		}
		c.SendString("Unauthenticated")
		return nil
	}
}

type LoginRequest struct {
	Email string
	Login string
	Password string
	Remember bool
}

// Login godoc
// @Summary login
// @Accept json
// @Produce json
// @Param form body LoginRequest true "body"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/login [post]
func postLoginHandler(c *fiber.Ctx) error {
	var request LoginRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(c.Request().Header.Header())))
	if err != nil {
		logger.Errorf("%+v", err)
	}
	var emailOrLogin string
	if request.Email != "" {
		emailOrLogin = request.Email
	}
	if request.Login != "" {
		emailOrLogin = request.Login
	}
	var user *models.User
	if user, err = models.GetUserByEmailOrLoginAndPassword(common.Database, emailOrLogin, models.MakeUserPassword(request.Password)); err == nil {
		if v := req.Header.Get("Accept"); strings.EqualFold(v, "application/jwt") {
			expiration := time.Now().Add(JWTLoginDuration)
			claims := &JWTClaims{
				Login: user.Login,
				Password: user.Password,
				StandardClaims: jwt.StandardClaims{
					ExpiresAt: expiration.Unix(),
				},
			}
			token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
			if str, err := token.SignedString(JWTSecret); err == nil {
				c.JSON(fiber.Map{
					"MESSAGE": "OK",
					"Token": str,
					"Expiration": expiration,
				})
			}else{
				c.Status(http.StatusInternalServerError)
				c.JSON(fiber.Map{"ERROR": err.Error()})
			}
		} else {
			value := map[string]string{
				"email": user.Email,
				"login": user.Login,
				"password": request.Password,
			}
			if encoded, err := cookieHandler.Encode(COOKIE_NAME, value); err == nil {
				// Remember?
				expires := time.Time{}
				if authMultipleConfig.CookieDuration > 0 {
					expires = time.Now().Add(authMultipleConfig.CookieDuration)
				}
				if request.Remember {
					expires = time.Now().AddDate(1, 0, 0)
				}
				cookie := &fiber.Cookie{
					Name:  COOKIE_NAME,
					Value: encoded,
					Path:  "/",
					Expires: expires,
					SameSite: authMultipleConfig.SameSite,
				}
				c.Cookie(cookie)
				if v := req.Header.Get("Content-Type"); v != "" {
					for _, chunk := range strings.Split(v, ";") {
						if strings.EqualFold(chunk, "application/json") {
							c.Status(http.StatusOK)
							return c.JSON(fiber.Map{"MESSAGE": "OK"})
						}
					}
				}
				return c.Redirect("/", http.StatusFound)
			}else{
				c.Status(http.StatusInternalServerError)
				c.JSON(fiber.Map{"ERROR": err.Error()})
			}
		}
	}else{
		if v := req.Header.Get("Accept"); strings.EqualFold(v, "application/jwt") {
			// TODO:
		} else if v := req.Header.Get("Content-Type"); v != "" {
			for _, chunk := range strings.Split(v, ";") {
				if strings.EqualFold(chunk, "application/json") {
					c.Status(http.StatusForbidden)
					return c.JSON(fiber.Map{"ERROR": err.Error()})
				}
			}
		} else {
			return c.Render("login", fiber.Map{
				"Error":    err.Error(),
				"Remember": true,
			})
		}
	}
	return nil
}

// @security BasicAuth
// Logout godoc
// @Summary logout
// @Description get string
// @Accept json
// @Produce json
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/logout [get]
func getLogoutHandler (c *fiber.Ctx) error {
	//c.ClearCookie(COOKIE_NAME)
	cookie := &fiber.Cookie{
		Name:  COOKIE_NAME,
		Value: "",
		Path:  "/",
		Expires: time.Now().AddDate(1, 0, 0),
		SameSite: authMultipleConfig.SameSite,
	}
	c.Cookie(cookie)
	if authMultipleConfig.AuthRedirect {
		c.Redirect("/", http.StatusFound)
		return nil
	}
	c.Status(http.StatusOK)
	return c.JSON(fiber.Map{"MESSAGE": "OK"})
}

type JWTClaims struct {
	Login string `json:"loginHandler"`
	Password string `json:"password"`
	Origin string `json:"origin,omitempty"`
	jwt.StandardClaims
}

var JWTSecret = []byte(HASH16)
var JWTLoginDuration = time.Duration(1) * time.Hour
var JWTRefreshDuration = time.Duration(5) * time.Minute

func encrypt(key, text []byte) ([]byte, error) {
	// IMPORTANT: Key should be 32 bytes length, if different make md5sum of key to have exactly 32 bytes!
	if len(key) != 32 {
		key = []byte(fmt.Sprintf("%x", md5.Sum(key)))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	b := base64.StdEncoding.EncodeToString(text)
	ciphertext := make([]byte, aes.BlockSize+len(b))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	cfb := cipher.NewCFBEncrypter(block, iv)
	cfb.XORKeyStream(ciphertext[aes.BlockSize:], []byte(b))
	return ciphertext, nil
}

func decrypt(key, text []byte) ([]byte, error) {
	// IMPORTANT: Key should be 32 bytes length, if different make md5sum of key to have exactly 32 bytes!
	if len(key) != 32 {
		key = []byte(fmt.Sprintf("%x", md5.Sum(key)))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(text) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}
	iv := text[:aes.BlockSize]
	text = text[aes.BlockSize:]
	cfb := cipher.NewCFBDecrypter(block, iv)
	cfb.XORKeyStream(text, text)
	data, err := base64.StdEncoding.DecodeString(string(text))
	if err != nil {
		return nil, err
	}
	return data, nil
}