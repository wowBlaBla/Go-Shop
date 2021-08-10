package handler

import (
	"bytes"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/google/logger"
	"github.com/yonnic/goshop/common"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"
	"time"
)

const (
	PROCESS_STATUS_IDLE = iota
	PROCESS_STATUS_STARTED
	PROCESS_STATUS_FINISHED
)

var (
	PREPARE *Process
	RENDER *Process
	PUBLISH *Process
)

func NewProcess(arguments []string, interval int) *Process {
	p := &Process{Arguments: arguments, Interval: 3000}
	if interval > 0 {
		p.Interval = interval
	}
	return p
}

type Process struct {
	Arguments []string
	Interval int
	Cmd *exec.Cmd
	Return int
	Status int
	Buff *bytes.Buffer
	Pid int
}

func (p *Process) Start() error {
	logger.Infof("Start process: %v %+v", p.Arguments[0], p.Arguments[1:])
	p.Cmd = exec.Command(p.Arguments[0], p.Arguments[1:]...)
	p.Buff = &bytes.Buffer{}
	p.Cmd.Stderr = p.Buff
	p.Cmd.Stdout = p.Buff
	if err := p.Cmd.Start(); err != nil {
		return err
	}
	p.Status = PROCESS_STATUS_STARTED
	p.Pid = p.Cmd.Process.Pid
	logger.Infof("PID: %+v", p.Pid)
	go func() {
		logger.Infof("[%d] Waiting for finish", p.Pid)
		if err := p.Cmd.Wait(); err != nil {
			if err2, ok := err.(*exec.ExitError); ok {
				if status, ok := err2.Sys().(syscall.WaitStatus); ok {
					p.Return = status.ExitStatus()
					logger.Infof("[%d] Fail: code %+v", p.Pid, p.Return)
				}
			} else {
				logger.Infof("[%d] Fail: %+v", p.Pid, err)
			}
		}else{
			logger.Infof("[%d] Finished", p.Pid)
		}

		p.Status = PROCESS_STATUS_FINISHED
		go func() {
			time.Sleep(time.Duration(2 * p.Interval) * time.Millisecond)
			logger.Infof("[%d] Switch to idle", p.Pid)
			p.Status = PROCESS_STATUS_IDLE
		}()
	}()
	return nil
}

type NewCommand struct {
	Interval int
}

type CommandView struct {
	Output string
	Error string `json:"ERROR,omitempty"`
	Status string
	Return int `json:",omitempty"`
}

// @security BasicAuth
// MakePrepare godoc
// @Summary Make prepare
// @Accept json
// @Produce json
// @Param request body NewCommand true "body"
// @Success 200 {object} CommandView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/prepare [post]
func postPrepareHandler(c *fiber.Ctx) error {
	var view CommandView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewCommand
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			//
			var err error
			if PREPARE == nil {
				PREPARE = NewProcess([]string{os.Args[0], "render", "-p", path.Join(dir, "hugo", "content")}, request.Interval)
			}
			process := PREPARE
			if process.Status == PROCESS_STATUS_IDLE {
				if err = process.Start(); err != nil {
					view.Output = process.Buff.String()
					view.Status = "ERROR"
					c.Status(http.StatusInternalServerError)
					return c.JSON(view)
				}
				view.Output = process.Buff.String()
				view.Status = "Started"
				return c.JSON(view)
			}else if process.Status == PROCESS_STATUS_STARTED {
				view.Output = process.Buff.String()
				view.Status = "Executing"
				return c.JSON(view)
			}else if process.Status == PROCESS_STATUS_FINISHED {
				view.Output = process.Buff.String()
				if process.Return == 0 {
					if _, err := os.Stat(path.Join(dir, HAS_CHANGES)); err == nil {
						if err := os.Remove(path.Join(dir, HAS_CHANGES)); err != nil {
							logger.Errorf("%v", err)
						}
					}
				}
				view.Return = process.Return
				view.Status = "Finished"
				return c.JSON(view)
			}else{
				view.Output = "Unknown state"
				view.Status = "Finished"
				c.Status(http.StatusInternalServerError)
				return c.JSON(view)
			}
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	return c.JSON(view)
}

// @security BasicAuth
// MakeRender godoc
// @Summary Make render
// @Accept json
// @Produce json
// @Param request body NewCommand true "body"
// @Success 200 {object} CommandView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/render [post]
func postRenderHandler(c *fiber.Ctx) error {
	var view CommandView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewCommand
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			//
			bin := strings.Split(common.Config.Hugo.Bin, " ")
			//
			var arguments []string
			if len(bin) > 1 {
				for _, x := range bin[1:]{
					x = strings.Replace(x, "%DIR%", dir, -1)
					arguments = append(arguments, x)
				}
			}
			arguments = append(arguments, "--cleanDestinationDir")
			if common.Config.Hugo.Minify {
				arguments = append(arguments, "--minify")
			}
			if len(bin) == 1 {
				arguments = append(arguments, []string{"-s", path.Join(dir, "hugo")}...)
			}
			var err error
			if RENDER == nil {
				RENDER = NewProcess(append([]string{bin[0]}, arguments...), request.Interval)
			}
			process := RENDER
			if process.Status == PROCESS_STATUS_IDLE {
				if err = process.Start(); err != nil {
					view.Output = process.Buff.String()
					view.Status = "ERROR"
					c.Status(http.StatusInternalServerError)
					return c.JSON(view)
				}
				view.Output = process.Buff.String()
				view.Status = "Started"
				return c.JSON(view)
			}else if process.Status == PROCESS_STATUS_STARTED {
				view.Output = process.Buff.String()
				view.Status = "Executing"
				return c.JSON(view)
			}else if process.Status == PROCESS_STATUS_FINISHED {
				view.Output = process.Buff.String()
				view.Return = process.Return
				view.Status = "Finished"
				return c.JSON(view)
			}else{
				view.Output = "Unknown state"
				view.Status = "Finished"
				c.Status(http.StatusInternalServerError)
				return c.JSON(view)
			}
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	return c.JSON(view)
}

type NewPublish struct {

}

type PublishView struct {
	Output string
	Status string
}

// @security BasicAuth
// MakePublish godoc
// @Summary Make publish
// @Accept json
// @Produce json
// @Param request body NewCommand true "body"
// @Success 200 {object} CommandView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/publish [post]
func postPublishHandler(c *fiber.Ctx) error {
	var view CommandView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEApplicationJSON) {
			var request NewCommand
			if err := c.BodyParser(&request); err != nil {
				return err
			}
			//
			if !common.Config.Publisher.Enabled{
				err := fmt.Errorf("wrangler disabled")
				logger.Errorf("%v", err.Error())
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			//
			bin := strings.Split(common.Config.Publisher.Bin, " ")
			//
			var arguments []string
			if len(bin) > 1 {
				for _, x := range bin[1:]{
					x = strings.Replace(x, "%DIR%", dir, -1)
					arguments = append(arguments, x)
				}
				if common.Config.Publisher.ApiToken == "" {
					err := fmt.Errorf("api_token is not specified")
					logger.Errorf("%v", err.Error())
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
				arguments = append(arguments, common.Config.Publisher.ApiToken)
			}
			//
			var err error
			if PUBLISH == nil {
				PUBLISH = NewProcess(append([]string{bin[0]}, arguments...), request.Interval)
			}
			process := PUBLISH
			if process.Status == PROCESS_STATUS_IDLE {
				if err = process.Start(); err != nil {
					view.Output = process.Buff.String()
					view.Status = "ERROR"
					c.Status(http.StatusInternalServerError)
					return c.JSON(view)
				}
				view.Output = process.Buff.String()
				view.Status = "Started"
				return c.JSON(view)
			}else if process.Status == PROCESS_STATUS_STARTED {
				view.Output = process.Buff.String()
				view.Status = "Executing"
				return c.JSON(view)
			}else if process.Status == PROCESS_STATUS_FINISHED {
				view.Output = process.Buff.String()
				view.Return = process.Return
				view.Status = "Finished"
				return c.JSON(view)
			}else{
				view.Output = "Unknown state"
				view.Status = "Finished"
				c.Status(http.StatusInternalServerError)
				return c.JSON(view)
			}
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	return c.JSON(view)
}