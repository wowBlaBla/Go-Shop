package common

import (
	"bytes"
	"encoding/json"
	"github.com/yonnic/goshop/config"
	"gorm.io/gorm"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

const (
	DEFAULT_PASSWORD = "goshoppass"
	PRODUCTS_NAME    = "products"
)

var (
	APPLICATION = "GoShop"
	VERSION = "1.0.0"
	COMPILED = "20201119103805"
	//
	Started          time.Time
	Config           *config.Config
	Database *gorm.DB
	//
	SALT = "goshop"
)

//

type CategoryFile struct {
	ID           uint
	Date         time.Time
	Title        string
	Thumbnail    string
	Path         string
	BasePriceMin float64
	BasePriceMax float64
	Properties   []*Property
	Options      []*Option
	Type         string
	//
	Content string
}

type Property struct {
	Name string
	Title string
	Values []*Value
}

type Option struct {
	ID uint
	Name string
	Title string
	Values []*Value
}

type Value struct {
	ID uint
	Thumbnail string
	Title string
	Value string
}

func (p *CategoryFile) MarshalJSON() ([]byte, error) {
	if bts, err := json.MarshalIndent(&struct {
		ID uint
		Date time.Time
		Title string
		Description string
		Thumbnail string
		Path string
		BasePriceMin float64
		BasePriceMax float64
		Options []*Option
		Type string
	}{
		ID: p.ID,
		Date: p.Date,
		Title: p.Title,
		Thumbnail: p.Thumbnail,
		Path: p.Path,
		BasePriceMin: p.BasePriceMin,
		BasePriceMax: p.BasePriceMax,
		Options: p.Options,
		Type: p.Type,
	}, "", "   "); err == nil {
		bts = append(bts, "\n\n"...)
		bts = append(bts, p.Content...)
		return bts, nil
	}else{
		return []byte{}, err
	}
}

func (p *CategoryFile) UnmarshalJSON(data []byte) error {
	type Alias CategoryFile
	v := &struct {
		*Alias
	}{
		Alias: (*Alias)(p),
	}
	n := bytes.Index(data, []byte("\n\n"))
	if n > -1 {
		if err := json.Unmarshal(data[:n], &v); err != nil {
			return err
		}
		v.Content = strings.TrimSpace(string(data[n:]))
	}else{
		return json.Unmarshal(data, &v)
	}
	return nil
}

func ReadCategoryFile(p string) (*CategoryFile, error) {
	//logger.Infof("TEST001 ReadCategoryFile: %v", p)
	if _, err := os.Stat(p); err != nil {
		return nil, err
	}
	bts, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}
	//logger.Infof("TEST001 Bts: %+v", string(bts))
	categoryFile := &CategoryFile{}
	if err = categoryFile.UnmarshalJSON(bts); err != nil {
		return nil, err
	}
	//logger.Infof("TEST001 categoryFile.Options: %+v", categoryFile.Options)
	return categoryFile, nil
}

func WriteCategoryFile(p string, categoryFile *CategoryFile) error {
	//logger.Infof("TEST001 WriteCategoryFile: %v", p)
	bts, err := categoryFile.MarshalJSON()
	//logger.Infof("TEST001 Bts: %+v", string(bts))
	if err != nil {
		return err
	}
	return ioutil.WriteFile(p, bts, 644)
}