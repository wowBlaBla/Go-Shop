package main

import (
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/cmd"
	"fmt"
	"github.com/google/logger"
	"io/ioutil"
	"time"
)

// @title GoShop API
// @version 1.0
// @description GoShop full featured api
// @termsOfService http://swagger.io/terms/
// @contact.name API Support
// @contact.email fiber@swagger.io
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @host localhost:18092
// @BasePath /
// @securityDefinitions.basic BasicAuth
func main() {
	logger.Init(fmt.Sprintf("[INF] [APP] %v v%v %v", common.APPLICATION, common.VERSION, common.COMPILED), true, false, ioutil.Discard)
	common.Started = time.Now()
	cmd.Execute()
}
