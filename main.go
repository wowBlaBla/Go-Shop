package main

import (
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/cmd"
	"fmt"
	"github.com/google/logger"
	"io/ioutil"
	"time"
)

func main() {
	logger.Init(fmt.Sprintf("[INF] [APP] %v v%v %v", common.APPLICATION, common.VERSION, common.COMPILED), true, false, ioutil.Discard)
	common.Started = time.Now()
	cmd.Execute()
}
