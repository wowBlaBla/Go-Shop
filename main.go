package main

import (
	"fmt"
	"github.com/google/logger"
	"github.com/yonnic/goshop/cmd"
	"github.com/yonnic/goshop/common"
	"io/ioutil"
	"time"
)

// @title GoShop API
// @version 1.0
// @description GoShop full featured api, see documentation https://docs.google.com/document/d/1VlkAYTqZG9oGvZxnSNDpcuafU-msOb2Qm2FevMJkA7w/edit
// @termsOfService http://swagger.io/terms/
// @contact.name API Support
// @contact.email fiber@swagger.io
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @BasePath /
// @securityDefinitions.basic BasicAuth
func main() {
	logger.Init(fmt.Sprintf("[INF] [APP] %v v%v %v", common.APPLICATION, common.VERSION, common.COMPILED), true, false, ioutil.Discard)
	/*if session := common.GetS3Session(); session != nil {
		src := "/home/brain/Pictures/accessories.jpg"
		common.PostS3File(session, src, "images/" + path.Base(src))
	}else{
		logger.Warningf("session is nil")
	}*/

	common.Started = time.Now()
	cmd.Execute()
}
