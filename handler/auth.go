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
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)


const (
	TEMPLATES_FOLDER = "templates"
)

var (
	dir, _ = filepath.Abs(filepath.Dir(os.Args[0]))

	DEFAULT_USER = "admin"
	DEFAULT_PASSWORD = "goshoppass"

	COOKIE_NAME = "goshop"
	HASH16 = "#goshop16#######"
	HASH32 = "#goshop32#######################"
)

func CreateFiberAppWithAuthMultiple(config *fiber.Config) (*fiber.App, func (c *fiber.Ctx) error) {
	// Create default user
	common.Database.AutoMigrate(&models.User{})
	if users, err := models.GetUsers(common.Database); err != nil || len(users) == 0 {
		user := models.User{
			Enable: true,
			Login: DEFAULT_USER,
			Email: DEFAULT_USER + "@goshop",
			Password: models.MakeUserPassword(DEFAULT_PASSWORD),
		}
		models.CreateUser(common.Database, &user)
	}
	// Check pages
	if p := path.Join(dir, TEMPLATES_FOLDER); len(p) > 0 {
		if _, err := os.Stat(p); err != nil {
			os.MkdirAll(p, 0755)
		}
		if p := path.Join(p, "login.html"); len(p) > 0 {
			if _, err := os.Stat(p); err != nil {
				if err = ioutil.WriteFile(p, []byte(`<!DOCTYPE html>
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
			<input type="password" name="password" placeholder="Password"><br />
            {{ if .Remember }}
			<input type="checkbox" name="remember" value="true"> Remember Me</input><br />
            {{ end }}
			<button type="submit">Login</button>
		</form>
    </body>
</html>`), 0755); err != nil {
					logger.Errorf("%v", err)
				}
			}
		}
	}
	//
	cookieHandler := func() *securecookie.SecureCookie {
		if value := os.Getenv("HASH16"); value != "" {
			HASH16 = value
		}
		if value := os.Getenv("HASH32"); value != "" {
			HASH32 = value
		}
		return securecookie.New([]byte(HASH16),[]byte(HASH32))
	} ()
	//
	engine := html.New(path.Join(dir, TEMPLATES_FOLDER), ".html")
	// Example of function to embed
	engine.AddFunc("greet", func(name string) string {
		return "Hello, " + name + "!"
	})
	//
	app := fiber.New(fiber.Config{
		Views: engine,
	})
	// TODO: Create required handlers to make login, logout and token refresh
	//
	app.Get("/login", func (c *fiber.Ctx) error {
		var action string
		if referer := c.Request().Header.Referer(); len(referer) > 0 {
			action = "?ref=" + base64.URLEncoding.EncodeToString(referer)
		}
		return c.Render("login", fiber.Map{
			"Action": action,
			"Remember": true,
		})
	})
	app.Post("/login", func (c *fiber.Ctx) error {
		var request struct {
			Login string
			Password string
			Remember bool
		}
		if err := c.BodyParser(&request); err != nil {
			return err
		}
		logger.Infof("request: %+v", request)
		req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(c.Request().Header.Header())))
		if err != nil {
			logger.Errorf("%+v", err)
		}
		var user *models.User
		if user, err = models.GetUserByLoginAndPassword(common.Database, request.Login, models.MakeUserPassword(request.Password)); err == nil {
			logger.Infof("User: %+v", user)
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
					"login": request.Login,
					"password": request.Password,
				}
				if encoded, err := cookieHandler.Encode(COOKIE_NAME, value); err == nil {
					// Remember?
					expires := time.Time{}
					if request.Remember {
						expires = time.Now().AddDate(1, 0, 0)
					}
					cookie := &fiber.Cookie{
						Name:  COOKIE_NAME,
						Value: encoded,
						Path:  "/",
						Expires: expires,
					}
					c.Cookie(cookie)
					c.Redirect("/", http.StatusFound)
				}else{
					c.Status(http.StatusInternalServerError)
					c.JSON(fiber.Map{"ERROR": err.Error()})
				}
			}
		}else{
			// TODO: Here
			if v := req.Header.Get("Accept"); strings.EqualFold(v, "application/jwt") {

			} else {
				return c.Render("login", fiber.Map{
					"Error":    err.Error(),
					"Remember": true,
				})
			}
		}
		return nil
	})
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
	app.Get("/logout", func (c *fiber.Ctx) error {
		c.ClearCookie(COOKIE_NAME)
		c.Redirect("/", http.StatusFound)
		return nil
	})
	return app, func (c *fiber.Ctx) error {
		logger.Infof("Here we are")
		var auth bool
		if !auth {
			if req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(c.Request().Header.Header()))); err == nil {
				if authorization := req.Header.Get("Authorization"); authorization != "" {
					if strings.Index(strings.ToLower(authorization), "basic ") == 0 {
						const prefix = "Basic "
						if bts, err := base64.StdEncoding.DecodeString(authorization[len(prefix):]); err == nil {
							cs := string(bts)
							if s := strings.IndexByte(cs, ':'); s > -1 {
								var login = cs[:s]
								var password = cs[s+1:]
								if user, err := models.GetUserByLoginAndPassword(common.Database, login, models.MakeUserPassword(password)); err == nil {
									if user.Enable {
										logger.Infof("AuthenticateWrap Basic OK")
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
			}
		}
		if !auth {
			if value := c.Cookies(COOKIE_NAME); len(value) > 0 {
				m := make(map[string]string)
				if err := cookieHandler.Decode(COOKIE_NAME, value, &m); err == nil {
					var login = m["login"]
					var password = m["password"]
					if user, err := models.GetUserByLoginAndPassword(common.Database, login, models.MakeUserPassword(password)); err == nil {
						logger.Infof("AuthenticateWrap Cookie OK")
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
		return c.Redirect("/login", http.StatusFound)
	}
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