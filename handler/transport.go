package handler

import (
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/google/logger"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/models"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type TransportsView []TransportView

type TransportView struct{
	ID uint
	Enabled bool
	Name string
	Title string
	Thumbnail string
	Weight float64
	Volume float64
	Order string
	Item string
	Kg float64
	M3 float64
	Free float64 `json:",omitempty"`
	Services string `json:",omitempty"`
}

// @security BasicAuth
// GetTransports godoc
// @Summary Get transports
// @Accept json
// @Produce json
// @Success 200 {object} TransportsView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/transports [get]
// @Tags transport
func getTransportsHandler(c *fiber.Ctx) error {
	if transports, err := models.GetTransports(common.Database); err == nil {
		sort.Slice(transports, func(i, j int) bool {
			return transports[i].Weight < transports[j].Weight
		})
		var view TransportsView
		if bts, err := json.MarshalIndent(transports, "", "   "); err == nil {
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

type NewTransport struct {
	Enabled bool
	Name string
	Title string
	Weight float64
	Volume float64
	Order string
	Item string
	Kg float64
	M3 float64
}

// @security BasicAuth
// CreateTransport godoc
// @Summary Create transport
// @Accept json
// @Produce json
// @Param transport body NewTransport true "body"
// @Success 200 {object} TransportView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/transports [post]
// @Tags transport
func postTransportHandler(c *fiber.Ctx) error {
	var view TransportView
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			var enabled bool
			if v, found := data.Value["Enabled"]; found && len(v) > 0 {
				enabled, err = strconv.ParseBool(v[0])
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var name string
			if v, found := data.Value["Name"]; found && len(v) > 0 {
				name = strings.TrimSpace(v[0])
			}
			var title string
			if v, found := data.Value["Title"]; found && len(v) > 0 {
				title = strings.TrimSpace(v[0])
			}
			var weight float64
			if v, found := data.Value["Weight"]; found && len(v) > 0 {
				weight, err = strconv.ParseFloat(v[0],10)
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var volume float64
			if v, found := data.Value["Volume"]; found && len(v) > 0 {
				volume, err = strconv.ParseFloat(v[0],10)
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var order string
			if v, found := data.Value["Order"]; found && len(v) > 0 {
				order = strings.TrimSpace(v[0])
			}
			var item string
			if v, found := data.Value["Item"]; found && len(v) > 0 {
				item = strings.TrimSpace(v[0])
			}
			var kg float64
			if v, found := data.Value["Kg"]; found && len(v) > 0 {
				kg, err = strconv.ParseFloat(v[0],10)
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var m3 float64
			if v, found := data.Value["M3"]; found && len(v) > 0 {
				m3, err = strconv.ParseFloat(v[0],10)
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var free float64
			if v, found := data.Value["Free"]; found && len(v) > 0 {
				free, err = strconv.ParseFloat(v[0],10)
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var services string
			if v, found := data.Value["Services"]; found && len(v) > 0 {
				services = strings.TrimSpace(v[0])
			}
			transport := &models.Transport {
				Enabled: enabled,
				Name:    name,
				Title:   title,
				Weight:  weight,
				Volume:  volume,
				Order:   order,
				Item:    item,
				Kg:      kg,
				M3:      m3,
				Free: free,
				Services: services,
			}
			if id, err := models.CreateTransport(common.Database, transport); err == nil {
				if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
					p := path.Join(dir, "storage", "values")
					if _, err := os.Stat(p); err != nil {
						if err = os.MkdirAll(p, 0755); err != nil {
							logger.Errorf("%v", err)
						}
					}
					filename := fmt.Sprintf("%d-%s-thumbnail%s", id, regexp.MustCompile(`(?i)[^-a-z0-9]+`).ReplaceAllString(transport.Name, "-"), path.Ext(v[0].Filename))
					if p := path.Join(p, filename); len(p) > 0 {
						if in, err := v[0].Open(); err == nil {
							out, err := os.OpenFile(p, os.O_WRONLY | os.O_CREATE, 0644)
							if err != nil {
								c.Status(http.StatusInternalServerError)
								return c.JSON(HTTPError{err.Error()})
							}
							defer out.Close()
							if _, err := io.Copy(out, in); err != nil {
								c.Status(http.StatusInternalServerError)
								return c.JSON(HTTPError{err.Error()})
							}
							transport.Thumbnail = "/" + path.Join("transports", filename)
							if err = models.UpdateTransport(common.Database, transport); err != nil {
								c.Status(http.StatusInternalServerError)
								return c.JSON(HTTPError{err.Error()})
							}
						}
					}
				}
			}else{
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			if bts, err := json.Marshal(transport); err == nil {
				if err = json.Unmarshal(bts, &view); err != nil {
					c.Status(http.StatusInternalServerError)
					return c.JSON(HTTPError{err.Error()})
				}
			}
			return c.JSON(view)
		} else {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	return c.JSON(view)
}

type TransportsListResponse struct {
	Data []TransportsListItem
	Filtered int64
	Total int64
}

type TransportsListItem struct {
	ID uint
	Enabled bool
	Name string
	Title string
	Weight float64
	Volume float64
	Order string
	Item string
	Kg float64
	M3 float64
}

// @security BasicAuth
// SearchTransports godoc
// @Summary Search transports
// @Accept json
// @Produce json
// @Param request body ListRequest true "body"
// @Success 200 {object} TransportsListResponse
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/transports/list [post]
// @Tags transport
func postTransportsListHandler(c *fiber.Ctx) error {
	var response TransportsListResponse
	var request ListRequest
	if err := c.BodyParser(&request); err != nil {
		return err
	}
	if len(request.Sort) == 0 {
		request.Sort["ID"] = "asc"
	}
	if request.Length == 0 {
		request.Length = 10
	}
	// Filter
	var keys1 []string
	var values1 []interface{}
	if len(request.Filter) > 0 {
		for key, value := range request.Filter {
			if key != "" && len(strings.TrimSpace(value)) > 0 {
				switch key {
				case "Order":
					keys1 = append(keys1, fmt.Sprintf("transports.`%v` like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				default:
					keys1 = append(keys1, fmt.Sprintf("transports.%v like ?", key))
					values1 = append(values1, "%" + strings.TrimSpace(value) + "%")
				}
			}
		}
	}
	//logger.Infof("keys1: %+v, values1: %+v", keys1, values1)
	//
	// Sort
	var order string
	if len(request.Sort) > 0 {
		var orders []string
		for key, value := range request.Sort {
			if key != "" && value != "" {
				switch key {
				case "Order":
					orders = append(orders, fmt.Sprintf("transports.`%v` %v", key, value))
				default:
					orders = append(orders, fmt.Sprintf("transports.%v %v", key, value))
				}
			}
		}
		order = strings.Join(orders, ", ")
	}
	//logger.Infof("order: %+v", order)
	//
	rows, err := common.Database.Debug().Model(&models.Transport{}).Select("transports.ID, transports.Enabled, transports.Name, transports.Title, transports.Weight, transports.Volume, transports.`Order`, transports.Item, transports.Kg, transports.M3").Where(strings.Join(keys1, " and "), values1...).Order(order).Limit(request.Length).Offset(request.Start).Rows()
	if err == nil {
		if err == nil {
			for rows.Next() {
				var item TransportsListItem
				if err = common.Database.ScanRows(rows, &item); err == nil {
					response.Data = append(response.Data, item)
				} else {
					logger.Errorf("%v", err)
				}
			}
		}else{
			logger.Errorf("%v", err)
		}
		rows.Close()
	}
	rows, err = common.Database.Debug().Model(&models.Transport{}).Select("transports.ID, transports.Enabled, transports.Name, transports.Title, transports.Weight, transports.Volume, transports.`Order`, transports.Item, transports.Kg, transports.M3").Where(strings.Join(keys1, " and "), values1...).Rows()
	if err == nil {
		for rows.Next() {
			response.Filtered ++
		}
		rows.Close()
	}
	if len(keys1) > 0 {
		common.Database.Debug().Model(&models.Transport{}).Count(&response.Total)
	}else{
		response.Total = response.Filtered
	}
	c.Status(http.StatusOK)
	return c.JSON(response)
}

// @security BasicAuth
// GetTransport godoc
// @Summary Get transport
// @Accept json
// @Produce json
// @Param id path int true "Shipping ID"
// @Success 200 {object} TransportView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/transports/{id} [get]
// @Tags transport
func getTransportHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if transport, err := models.GetTransport(common.Database, id); err == nil {
		var view TransportView
		if bts, err := json.MarshalIndent(transport, "", "   "); err == nil {
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
// UpdateTransport godoc
// @Summary Update transport
// @Accept json
// @Produce json
// @Param transport body TransportView true "body"
// @Param id path int true "Shipping ID"
// @Success 200 {object} TagView
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/transports/{id} [put]
// @Tags transport
func putTransportHandler(c *fiber.Ctx) error {
	var transport *models.Transport
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
		var err error
		if transport, err = models.GetTransport(common.Database, id); err != nil {
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(fiber.Map{"ERROR": "ID is not defined"})
	}
	if contentType := string(c.Request().Header.ContentType()); contentType != "" {
		if strings.HasPrefix(contentType, fiber.MIMEMultipartForm) {
			data, err := c.Request().MultipartForm()
			if err != nil {
				c.Status(http.StatusInternalServerError)
				return c.JSON(HTTPError{err.Error()})
			}
			var enabled bool
			if v, found := data.Value["Enabled"]; found && len(v) > 0 {
				enabled, err = strconv.ParseBool(v[0])
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var title string
			if v, found := data.Value["Title"]; found && len(v) > 0 {
				title = strings.TrimSpace(v[0])
			}
			var weight float64
			if v, found := data.Value["Weight"]; found && len(v) > 0 {
				weight, err = strconv.ParseFloat(v[0],10)
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var volume float64
			if v, found := data.Value["Volume"]; found && len(v) > 0 {
				volume, err = strconv.ParseFloat(v[0],10)
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var order string
			if v, found := data.Value["Order"]; found && len(v) > 0 {
				order = strings.TrimSpace(v[0])
			}
			var item string
			if v, found := data.Value["Item"]; found && len(v) > 0 {
				item = strings.TrimSpace(v[0])
			}
			var kg float64
			if v, found := data.Value["Kg"]; found && len(v) > 0 {
				kg, err = strconv.ParseFloat(v[0],10)
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var m3 float64
			if v, found := data.Value["M3"]; found && len(v) > 0 {
				m3, err = strconv.ParseFloat(v[0],10)
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var free float64
			if v, found := data.Value["Free"]; found && len(v) > 0 {
				free, err = strconv.ParseFloat(v[0],10)
				if err != nil {
					logger.Infof("%+v", err)
				}
			}
			var services string
			if v, found := data.Value["Services"]; found && len(v) > 0 {
				services = strings.TrimSpace(v[0])
			}
			transport.Enabled = enabled
			transport.Title = title
			transport.Weight = weight
			transport.Volume = volume
			transport.Order = order
			transport.Item = item
			transport.Kg = kg
			transport.M3 = m3
			transport.Free = free
			transport.Services = services
			if v, found := data.Value["Thumbnail"]; found && len(v) > 0 && v[0] == "" {
				// To delete existing
				if transport.Thumbnail != "" {
					if err = os.Remove(path.Join(dir, transport.Thumbnail)); err != nil {
						logger.Errorf("%v", err)
					}
					transport.Thumbnail = ""
				}
			}else if v, found := data.File["Thumbnail"]; found && len(v) > 0 {
				p := path.Join(dir, "storage", "transports")
				if _, err := os.Stat(p); err != nil {
					if err = os.MkdirAll(p, 0755); err != nil {
						logger.Errorf("%v", err)
					}
				}
				filename := fmt.Sprintf("%d-%s-thumbnail%s", id, regexp.MustCompile(`(?i)[^-a-z0-9]+`).ReplaceAllString(transport.Name, "-"), path.Ext(v[0].Filename))
				if p := path.Join(p, filename); len(p) > 0 {
					if in, err := v[0].Open(); err == nil {
						out, err := os.OpenFile(p, os.O_WRONLY | os.O_CREATE, 0644)
						if err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
						defer out.Close()
						if _, err := io.Copy(out, in); err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
						transport.Thumbnail = "/" + path.Join("transports", filename)
						if err = models.UpdateTransport(common.Database, transport); err != nil {
							c.Status(http.StatusInternalServerError)
							return c.JSON(HTTPError{err.Error()})
						}
					}
				}
			}
			//
			if err := models.UpdateTransport(common.Database, transport); err == nil {
				var view TransportView
				if bts, err := json.Marshal(transport); err == nil {
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
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{"Unsupported Content-Type"})
		}
	}
	c.Status(http.StatusInternalServerError)
	return c.JSON(HTTPError{"Something went wrong"})
}

// @security BasicAuth
// DelTransport godoc
// @Summary Delete transport
// @Accept json
// @Produce json
// @Param id path int true "Shipping ID"
// @Success 200 {object} HTTPMessage
// @Failure 404 {object} HTTPError
// @Failure 500 {object} HTTPError
// @Router /api/v1/transports/{id} [delete]
// @Tags transport
func delTransportHandler(c *fiber.Ctx) error {
	var id int
	if v := c.Params("id"); v != "" {
		id, _ = strconv.Atoi(v)
	}
	if transport, err := models.GetTransport(common.Database, id); err == nil {
		if err = models.DeleteTransport(common.Database, transport); err == nil {
			return c.JSON(HTTPMessage{MESSAGE: "OK"})
		}else{
			c.Status(http.StatusInternalServerError)
			return c.JSON(HTTPError{err.Error()})
		}
	}else{
		c.Status(http.StatusInternalServerError)
		return c.JSON(HTTPError{err.Error()})
	}
}