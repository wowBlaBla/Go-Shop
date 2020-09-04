package handler

import (
	"encoding/json"
	"fmt"
	"github.com/google/logger"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/yonnic/goshop/models"
	"os"

	_ "github.com/volatiletech/authboss/v3/auth"
	_ "github.com/volatiletech/authboss/v3/logout"
	_ "github.com/volatiletech/authboss/v3/recover"
	_ "github.com/volatiletech/authboss/v3/register"
	"github.com/yonnic/goshop/common"
	"net/http"
	"time"
)

func GetRouter() http.Handler {
	r := mux.NewRouter()
	//
	r.HandleFunc("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t1 := time.Now()
		defer func() {
			logger.Infof("%s %s %s ~ %.3f ms", r.RemoteAddr, r.Method, r.URL, float64(time.Since(t1).Nanoseconds())/1000000)
		}()
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)

		w.Write([]byte(``))
	}))
	apiV1 := r.PathPrefix("/api/v1").Subrouter()
	apiV1.HandleFunc("/info", AuthenticateWrap(func(w http.ResponseWriter, r *AuthenticatedRequest) {
		t1 := time.Now()
		defer func() {
			logger.Infof("%s %s %s ~ %.3f ms", r.RemoteAddr, r.Method, r.URL, float64(time.Since(t1).Nanoseconds()) / 1000000)
		}()
		var result struct {
			Application string
			Started time.Time
			User *models.User
		}
		result.Application = fmt.Sprintf("%v v%v %v", common.APPLICATION, common.VERSION, common.COMPILED)
		result.Started = common.Started
		result.User = r.User
		returnStructAsJson(w, http.StatusOK, result)
		return
	}))

	AuthInit(r, "")

	return handlers.CORS(
		handlers.AllowedOrigins([]string{os.Getenv("ORIGIN_ALLOWED")}),
		handlers.AllowedHeaders([]string{"X-Requested-With"}),
		handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS"}))(r)
}

/**/

func returnJson(w http.ResponseWriter, code int, body []byte) {
	w.Header().Add("Content-Category", "application/json")
	w.WriteHeader(code)
	w.Write(body)
}

func returnStringAsJson(w http.ResponseWriter, code int, body string) {
	returnJson(w, code, []byte(body))
}

func returnMapAsJson(w http.ResponseWriter, code int, map_ map[string]interface{}) {
	var bts []byte
	var err error
	if bts, err = json.Marshal(map_); err != nil {
		returnStringAsJson(w, http.StatusInternalServerError, err.Error())
		return
	}
	returnJson(w, code, bts)
}

func returnStructAsJson(w http.ResponseWriter, code int, v interface{}) {
	var bts []byte
	var err error
	if bts, err = json.Marshal(v); err != nil {
		returnStringAsJson(w, http.StatusInternalServerError, err.Error())
		return
	}
	returnJson(w, code, bts)
}