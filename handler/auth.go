package handler

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/google/logger"
	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/models"
	"html/template"
	"io"
	"io/ioutil"
	"log"
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

func AuthInit(router *mux.Router, prefix string) {
	// Database
	common.Database.AutoMigrate(&models.Token{})
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
	// Routes
	router.HandleFunc(prefix + "/login", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t1 := time.Now()
		defer func() {
			logger.Infof("%s %s %s ~ %.3f ms", r.RemoteAddr, r.Method, r.URL, float64(time.Since(t1).Nanoseconds())/1000000)
		}()
		if r.Method == http.MethodGet {
			var action string
			if v := r.Referer(); v != "" {
				action = "?ref=" + base64.URLEncoding.EncodeToString([]byte(v))
			}
			var csrf string
			if ciphertext, err := encrypt([]byte(HASH32), []byte(time.Now().Format(time.RFC3339))); err == nil {
				csrf = base64.StdEncoding.EncodeToString(ciphertext)
			}
			var err error
			t := template.New("login.tpl")
			if t, err = t.ParseFiles(path.Join(dir, TEMPLATES_FOLDER, "login.tpl")); err == nil {
				if err = t.Execute(w, struct {
					Action string
					CSRF string
				}{
					Action: action,
					CSRF: csrf,
				}); err == nil {
					return
				}
			}
			w.Header().Add("Content-Type", "text/html")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("%v", err)))
		}else if r.Method == http.MethodPost {
			var err error
			contentType := r.Header.Get("Content-Type")
			if strings.EqualFold(strings.Split(contentType, ";")[0], "application/x-www-form-urlencoded") {
				if err = r.ParseForm(); err == nil {
					var login = r.PostFormValue("login")
					var password = r.PostFormValue("password")
					var remember = r.PostFormValue("remember") // (empty) or "true"
					var csrf = r.PostFormValue("csrf")
					logger.Infof("%v %v %v %v", login, password, remember, csrf)
					var valid bool
					var ciphertext []byte
					if ciphertext, err = base64.StdEncoding.DecodeString(csrf); err == nil {
						var timestamp []byte
						if timestamp, err = decrypt([]byte(HASH32), ciphertext); err == nil {
							var t1 time.Time
							if t1, err = time.Parse(time.RFC3339, string(timestamp)); err == nil {
								t2 := time.Now()
								if t2.Sub(t1).Seconds() < 60 * 10 {
									valid = true
								}
							}
						}
					}
					if valid {
						var user *models.User
						if user, err = models.GetUserByLoginAndPassword(common.Database, login, models.MakeUserPassword(password)); err == nil {
							setSession(user.Login, user.Password, "", w)
							if ref := r.URL.Query().Get("ref"); ref != "" {
								w.Header().Set("Location", ref)
								w.WriteHeader(http.StatusFound)
							}else{
								w.Header().Set("Location", "/")
								w.WriteHeader(http.StatusFound)
							}
							w.Write([]byte{})
							return
						}
					}
				}
			}else if strings.EqualFold(strings.Split(contentType, ";")[0], "application/json") {
				var bts []byte
				var err error
				if bts, err = ioutil.ReadAll(r.Body); err == nil {
					var form struct {
						Login string
						Password string
					}
					if err = json.Unmarshal(bts, &form); err == nil {
						var user *models.User
						if user, err = models.GetUserByLoginAndPassword(common.Database, form.Login, models.MakeUserPassword(form.Password)); err == nil {
							if v := r.Header.Get("Accept"); strings.EqualFold(v, "application/jwt") {
								// jwt
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
									var response struct {
										Message string `json:"MESSAGE"`
										Token string
										Expiration time.Time
									}
									response.Message = "OK"
									response.Token = str
									response.Expiration = expiration
									logger.Infof("response: %+v", response)
									returnStructAsJson(w, http.StatusOK, response)
									return
								}else{
									returnMapAsJson(w, http.StatusInternalServerError, map[string]interface{}{"ERROR": err.Error()})
									return
								}
							} else {
								setSession(user.Login, user.Password, "", w)
								returnMapAsJson(w, http.StatusOK, map[string]interface{}{"MESSAGE": "OK"})
								return
							}
						}
					}
				}
			}
			w.Header().Add("Content-Type", "text/html")
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("%v", err)))
		}
	}))
	router.HandleFunc(prefix + "/logout", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t1 := time.Now()
		defer func() {
			logger.Infof("%s %s %s ~ %.3f ms", r.RemoteAddr, r.Method, r.URL, float64(time.Since(t1).Nanoseconds())/1000000)
		}()
		if r.Method == http.MethodGet {
			var err error
			if auth := r.Header.Get("Authorization"); auth != "" {
				if strings.Index(strings.ToLower(auth), "bearer ") == 0 {
					const prefix= "Bearer "
					claims := &JWTClaims{}
					logger.Infof("auth[len(prefix):]: %+v", auth[len(prefix):])
					if token, err := jwt.ParseWithClaims(auth[len(prefix):], claims, func(token *jwt.Token) (interface{}, error) {
						return JWTSecret, nil
					}); err == nil {
						if token.Valid {
							if time.Now().Before(time.Unix(claims.ExpiresAt, 0)) {
								if _, err = models.GetUserByLoginAndPassword(common.Database, claims.Login, claims.Password); err == nil {
									if _, err = models.GetTokenByTokenAndStatus(common.Database, auth[len(prefix):], models.TOKEN_STATUS_REVOKED); err != nil {
										logger.Infof("AuthenticateWrap JWT OK")
										if _, err = models.CreateToken(common.Database, &models.Token{Token: auth[len(prefix):], ExpiresAt: claims.ExpiresAt, Status: models.TOKEN_STATUS_REVOKED}); err == nil {
											if err = models.DeleteTokenByExpiration(common.Database); err != nil {
												logger.Errorf("%v", err)
											}
										}else{
											logger.Errorf("%v", err)
										}
									} else {
										logger.Errorf("Token %v revoked", auth[len(prefix):])
									}
								} else {
									logger.Errorf("%v", err)
								}
							}else{
								logger.Errorf("Expired token")
							}
						}else{
							logger.Errorf("Invalid token")
						}
					}else{
						logger.Errorf("%v", err)
					}
				}else{
					returnMapAsJson(w, http.StatusInternalServerError, map[string]interface{}{"ERROR": "Unsupported Authorization type"})
					return
				}
				if err == nil {
					returnMapAsJson(w, http.StatusOK, map[string]interface{}{"MESSAGE": "OK"})
				} else {
					returnMapAsJson(w, http.StatusInternalServerError, map[string]interface{}{"ERROR": err})
				}
				return
			}else{
				clearSession(w)
				if ref := r.URL.Query().Get("ref"); ref != "" {
					w.Header().Set("Location", ref)
					w.WriteHeader(http.StatusFound)
				}else{
					w.Header().Set("Location", "/")
					w.WriteHeader(http.StatusFound)
				}
				w.Write([]byte{})
				return
			}
		}
	}))
	router.HandleFunc(prefix + "/refresh", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t1 := time.Now()
		defer func() {
			log.Printf("[INF] [API] %s %s %s ~ %.3f ms", r.RemoteAddr, r.Method, r.URL, float64(time.Since(t1).Nanoseconds())/1000000)
		}()
		if r.Method == http.MethodGet {
			if auth := r.Header.Get("Authorization"); auth != "" {
				if strings.Index(strings.ToLower(auth), "bearer ") == 0 {
					const prefix= "Bearer "
					claims := &JWTClaims{}
					logger.Infof("auth[len(prefix):]: %+v", auth[len(prefix):])
					if token, err := jwt.ParseWithClaims(auth[len(prefix):], claims, func(token *jwt.Token) (interface{}, error) {
						return JWTSecret, nil
					}); err == nil {
						if token.Valid {
							if time.Now().Before(time.Unix(claims.ExpiresAt, 0)) {
								logger.Infof("")
								if time.Unix(claims.ExpiresAt, 0).Sub(time.Now()) <= JWTRefreshDuration {
									if user, err := models.GetUserByLoginAndPassword(common.Database, claims.Login, claims.Password); err == nil {
										if _, err := models.GetTokenByTokenAndStatus(common.Database, auth[len(prefix):], models.TOKEN_STATUS_REVOKED); err != nil {
											logger.Infof("AuthenticateWrap JWT OK")
											if _, err = models.CreateToken(common.Database, &models.Token{Token: auth[len(prefix):], ExpiresAt: claims.ExpiresAt, Status: models.TOKEN_STATUS_REVOKED}); err == nil {
												if err = models.DeleteTokenByExpiration(common.Database); err != nil {
													logger.Errorf("%v", err)
												}
												expiration := time.Now().Add(JWTLoginDuration)
												claims := &JWTClaims{
													Login: user.Login,
													Password: user.Password,
													StandardClaims: jwt.StandardClaims{
														ExpiresAt: expiration.Unix(),
													},
												}
												logger.Infof("claims: %+v", claims)
												token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
												logger.Infof("token: %+v", token)
												if str, err := token.SignedString(JWTSecret); err == nil {
													logger.Infof("str: %+v", str)
													var response struct {
														Message string `json:"MESSAGE"`
														Token string
														Expiration time.Time
													}
													response.Message = "OK"
													response.Token = str
													response.Expiration = expiration
													logger.Infof("response: %+v", response)
													returnStructAsJson(w, http.StatusOK, response)
													return
												}
											}
											returnMapAsJson(w, http.StatusInternalServerError, map[string]interface{}{"ERROR": err})
											return
										} else {
											logger.Errorf("Token %v revoked", auth[len(prefix):])
										}
									} else {
										logger.Errorf("%v", err)
									}
								}else{
									returnMapAsJson(w, http.StatusInternalServerError, map[string]interface{}{"ERROR": "Too early to do refresh, try again later"})
									return
								}
							}else{
								logger.Errorf("Expired token")
							}
						}else{
							logger.Errorf("Invalid token")
						}
					}else{
						logger.Errorf("%v", err)
					}
				}else{
					returnMapAsJson(w, http.StatusInternalServerError, map[string]interface{}{"ERROR": "Unsupported Authorization type"})
					return
				}
			}else{
				returnMapAsJson(w, http.StatusInternalServerError, map[string]interface{}{"ERROR": "Authorization header missed"})
				return
			}
		}else{
			returnMapAsJson(w, http.StatusInternalServerError, map[string]interface{}{"ERROR": "Method is not implemented"})
			return
		}
	}))
	// Templates
	if p1 := path.Join(dir, TEMPLATES_FOLDER); len(p1) > 0 {
		if _, err := os.Stat(p1); err != nil {
			if err = os.MkdirAll(p1, 0755); err != nil {
				logger.Errorf("%v", err)
			}
		}
		if p2 := path.Join(p1, "login.tpl"); len(p2) > 0 {
			if _, err := os.Stat(p2); err != nil {
				if err = ioutil.WriteFile(p2, []byte(`<!DOCTYPE html>
<html>
    <head>
        <title>Login</title>
    </head>
    <body>
        <form action="{{ .Action }}" method="POST">
			<input type="text" name="login" placeholder="Login" autocomplete="off"><br />
			<input type="password" name="password" placeholder="Password"><br />
			<input type="hidden" name="csrf" value="{{ .CSRF }}" />
			<input type="checkbox" name="remember" value="true"> Remember Me</input><br />
			<button type="submit">Login</button>
		</form>
    </body>
</html>`), 0644); err != nil {

				}
			}
		}
	}
}

type AuthenticatedHandlerFunc func(http.ResponseWriter, *AuthenticatedRequest)

type AuthenticatedRequest struct {
	http.Request
	User *models.User
	Login string
	Password string
	Origin string
}

func AuthenticateOptionalWrap(wrapped http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if user, origin, err := getAuthenticatedUser(r); err == nil {
			r = r.WithContext(context.WithValue(r.Context(), "user", user))
			if origin != "" {
				r = r.WithContext(context.WithValue(r.Context(), "origin", origin))
			}
		}
		wrapped(w, r)
	}
}

func AuthenticateWrap(wrapped AuthenticatedHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if user, origin, err := getAuthenticatedUser(r); err == nil {
			wrapped(w, &AuthenticatedRequest{Request: *r, User: user, Origin: origin})
			return
		}
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("403 Access Deny"))
	}
}

func getAuthenticatedUser(r *http.Request) (*models.User, string, error) {
	if auth := r.Header.Get("Authorization"); auth != "" {
		if strings.Index(strings.ToLower(auth), "basic ") == 0 {
			const prefix = "Basic "
			if c, err := base64.StdEncoding.DecodeString(auth[len(prefix):]); err == nil {
				cs := string(c)
				if s := strings.IndexByte(cs, ':'); s > -1 {
					var login = cs[:s]
					var password = cs[s+1:]
					if user, err := models.GetUserByLoginAndPassword(common.Database, login, models.MakeUserPassword(password)); err == nil {
						if user.Enable {
							logger.Infof("AuthenticateWrap Basic OK")
							return user, "", nil
						}else{
							logger.Errorf("User %v is not enabled", user.Login)
						}
					}else{
						logger.Errorf("%v", err)
					}
				}
			}
		}else if strings.Index(strings.ToLower(auth), "bearer ") == 0 {
			const prefix= "Bearer "
			claims := &JWTClaims{}
			if token, err := jwt.ParseWithClaims(auth[len(prefix):], claims, func(token *jwt.Token) (interface{}, error) {
				return JWTSecret, nil
			}); err == nil {
				if token.Valid {
					if time.Now().Before(time.Unix(claims.ExpiresAt, 0)) {
						if user, err := models.GetUserByLoginAndPassword(common.Database, claims.Login, claims.Password); err == nil {
							if _, err := models.GetTokenByTokenAndStatus(common.Database, auth[len(prefix):], models.TOKEN_STATUS_REVOKED); err != nil {
								logger.Infof("AuthenticateWrap JWT OK")
								return user, claims.Origin, nil
							} else {
								logger.Errorf("Token %v revoked", auth[len(prefix):])
							}
						} else {
							logger.Errorf("%v", err)
						}
					}else{
						logger.Errorf("Expired token")
					}
				}else{
					logger.Errorf("Invalid token")
				}
			}else{
				logger.Errorf("%v", err)
			}
		}
	}else if cookie, err := r.Cookie(COOKIE_NAME); err == nil {
		cookieValue := make(map[string]string)
		if err = cookieHandler.Decode(COOKIE_NAME, cookie.Value, &cookieValue); err == nil {
			var login = cookieValue["login"]
			var password = cookieValue["password"]
			var origin = cookieValue["origin"]
			if user, err := models.GetUserByLoginAndPassword(common.Database, login, password); err == nil {
				logger.Infof("AuthenticateWrap Cookie OK")
				return user, origin, nil
			}else{
				logger.Errorf("%v", err)
			}
		}
	}
	return nil, "", fmt.Errorf("not authenticated")
}

var cookieHandler = func() *securecookie.SecureCookie {
	if value := os.Getenv("HASH16"); value != "" {
		HASH16 = value
	}
	if value := os.Getenv("HASH32"); value != "" {
		HASH32 = value
	}
	return securecookie.New([]byte(HASH16),[]byte(HASH32))
} ()

func getAuthentication(request *http.Request) (login, password, origin string) {
	if auth := request.Header.Get("Authorization"); auth != "" {
		if strings.Index(auth, "Basic ") == 0 {
			const prefix = "Basic "
			if strings.Index(auth, prefix) == 0 {
				if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
					return
				}
				c, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
				if err != nil {
					return
				}
				cs := string(c)
				if s := strings.IndexByte(cs, ':'); s > -1 {
					login = cs[:s]
					password = cs[s+1:]
				}
			}
		}else if strings.Index(auth, "Bearer ") == 0 {
			const prefix= "Bearer "
			if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
				return
			}
			claims := &JWTClaims{}
			token, err := jwt.ParseWithClaims(auth[len(prefix):], claims, func(token *jwt.Token) (interface{}, error) {
				return JWTSecret, nil
			})
			if err != nil {
				if err == jwt.ErrSignatureInvalid {
					return
				}
				return
			}
			if !token.Valid {
				return
			}
			if _, err := models.GetTokenByTokenAndStatus(common.Database, auth[len(prefix):], models.TOKEN_STATUS_REVOKED); err == nil {
				return
			}
			login = claims.Login
			origin = claims.Origin
			//bearer = true
		}
	}else if cookie, err := request.Cookie(COOKIE_NAME); err == nil {
		cookieValue := make(map[string]string)
		if err = cookieHandler.Decode(COOKIE_NAME, cookie.Value, &cookieValue); err == nil {
			login = cookieValue["login"]
			password = cookieValue["password"]
			origin = cookieValue["origin"]
		}
	}
	return
}

func setSession(login string, password string, origin string, response http.ResponseWriter) *http.Cookie {
	value := map[string]string{
		"login": login,
		"password": password,
		"origin": origin,
	}
	if encoded, err := cookieHandler.Encode(COOKIE_NAME, value); err == nil {
		cookie := &http.Cookie{
			Name:  COOKIE_NAME,
			Value: encoded,
			Path:  "/",
			MaxAge: 86400 * 365,
			Expires: time.Now().AddDate(1, 0, 0),
		}
		http.SetCookie(response, cookie)
		return cookie
	}
	return nil
}

func clearSession(response http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:   COOKIE_NAME,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	}
	http.SetCookie(response, cookie)
}

type JWTClaims struct {
	Login string `json:"login"`
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