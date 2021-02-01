package common

import (
	"bytes"
	"encoding/json"
	"github.com/yonnic/goshop/config"
	"gorm.io/gorm"
	"io"
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
	COMPILED = "20210129181247"
	//
	Started          time.Time
	Config           *config.Config
	Database *gorm.DB
	//
	STRIPE *Stripe
	MOLLIE *Mollie
	//
	NOTIFICATION *Notification
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
	Properties   []*PropertyCF
	Options      []*OptionCF
	Type         string
	//
	Content string
}

type PropertyCF struct {
	Name string
	Title string
	Values []*ValueCF
}

type OptionCF struct {
	ID uint
	Type string
	Name string
	Title string
	Values []*ValueCF
}

type ValueCF struct {
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
		Options []*OptionCF
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
	if _, err := os.Stat(p); err != nil {
		return nil, err
	}
	bts, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}
	categoryFile := &CategoryFile{}
	if err = categoryFile.UnmarshalJSON(bts); err != nil {
		return nil, err
	}
	return categoryFile, nil
}

func WriteCategoryFile(p string, categoryFile *CategoryFile) error {
	bts, err := categoryFile.MarshalJSON()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(p, bts, 0644)
}

/**/

type ProductFile struct {
	ID uint
	Type       string
	Title      string
	Date       time.Time
	Tags       []string
	Canonical  string
	Categories []string
	CategoryId  uint
	Thumbnail  string
	BasePrice  string
	Product    ProductPF
	//
	Content string
}

type ProductPF struct {
	Id         uint `json:"Id"`
	CategoryId uint
	Name       string
	Title      string
	Thumbnail  string `json:",omitempty"`
	Images     []string
	Parameters []ParameterPF
	Path       string
	Variations []VariationPF
}


type ParameterPF struct {
	Id uint
	Name string
	Title string
	Value ValuePPF
	CustomValue string
}

type ValuePPF struct {
	Id uint
	Title string
	Thumbnail string `json:",omitempty"`
	Value string
}

type VariationPF struct {
	Id uint
	Name string
	Title string
	Thumbnail string `json:",omitempty"`
	Description string
	BasePrice float64
	Properties []PropertyPF
	Selected bool
}

type PropertyPF struct {
	Id uint
	Type string
	Name string
	Title string
	Description string
	Values []ValuePF
}

type ValuePF struct {
	Id uint
	Enabled bool
	Title string
	Thumbnail string `json:",omitempty"`
	Value string
	Price PricePF
	Selected bool
}

type PricePF struct {
	Id uint
	Price float64
}

func (p *ProductFile) MarshalJSON() ([]byte, error) {
	if bts, err := json.MarshalIndent(&struct {
		ID uint
		Type       string
		Title      string
		Date       time.Time
		Tags       []string
		Canonical  string
		Categories []string
		CategoryId  uint
		Thumbnail  string
		BasePrice  string
		Product    ProductPF
	}{
		ID: p.ID,
		Type: p.Type,
		Title: p.Title,
		Date: p.Date,
		Tags: p.Tags,
		Canonical: p.Canonical,
		Categories: p.Categories,
		CategoryId: p.CategoryId,
		Thumbnail: p.Thumbnail,
		BasePrice: p.BasePrice,
		Product: p.Product,
	}, "", "   "); err == nil {
		bts = append(bts, "\n\n"...)
		bts = append(bts, p.Content...)
		return bts, nil
	}else{
		return []byte{}, err
	}
}

func (p *ProductFile) UnmarshalJSON(data []byte) error {
	type Alias ProductFile
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

func ReadProductFile(p string) (*ProductFile, error) {
	if _, err := os.Stat(p); err != nil {
		return nil, err
	}
	bts, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}
	productFile := &ProductFile{}
	if err = productFile.UnmarshalJSON(bts); err != nil {
		return nil, err
	}
	return productFile, nil
}

func WriteProductFile(p string, productFile *ProductFile) error {
	bts, err := productFile.MarshalJSON()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(p, bts, 0644)
}

/* Option */
type OptionFile struct {
	ID           uint
	Date         time.Time
	Title        string
	Type         string
	//
	Content string
}

func (o *OptionFile) MarshalJSON() ([]byte, error) {
	if bts, err := json.MarshalIndent(&struct {
		ID uint
		Type       string
		Title      string
		Date       time.Time
	}{
		ID:         o.ID,
		Type:       o.Type,
		Title:      o.Title,
		Date:       o.Date,
	}, "", "   "); err == nil {
		bts = append(bts, "\n\n"...)
		bts = append(bts, o.Content...)
		return bts, nil
	}else{
		return []byte{}, err
	}
}

func (o *OptionFile) UnmarshalJSON(data []byte) error {
	type Alias OptionFile
	v := &struct {
		*Alias
	}{
		Alias: (*Alias)(o),
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

func ReadOptionFile(p string) (*OptionFile, error) {
	if _, err := os.Stat(p); err != nil {
		return nil, err
	}
	bts, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}
	optionFile := &OptionFile{}
	if err = optionFile.UnmarshalJSON(bts); err != nil {
		return nil, err
	}
	return optionFile, nil
}

func WriteOptionFile(p string, optionFile *OptionFile) error {
	bts, err := optionFile.MarshalJSON()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(p, bts, 0644)
}

/* Value */
type ValueFile struct {
	ID           uint
	Date         time.Time
	Title        string
	Description  string
	Type         string
	Thumbnail    string `json:",omitempty"`
	Value        string
	//
	Content string
}

func (v *ValueFile) MarshalJSON() ([]byte, error) {
	if bts, err := json.MarshalIndent(&struct {
		ID uint
		Type       string
		Title      string
		Description  string
		Date       time.Time
		Thumbnail    string
		Value      string
	}{
		ID:    v.ID,
		Type:  v.Type,
		Title: v.Title,
		Description: v.Description,
		Date:  v.Date,
		Thumbnail: v.Thumbnail,
		Value: v.Value,
	}, "", "   "); err == nil {
		bts = append(bts, "\n\n"...)
		bts = append(bts, v.Content...)
		return bts, nil
	}else{
		return []byte{}, err
	}
}

func (v *ValueFile) UnmarshalJSON(data []byte) error {
	type Alias ValueFile
	vv := &struct {
		*Alias
	}{
		Alias: (*Alias)(v),
	}
	n := bytes.Index(data, []byte("\n\n"))
	if n > -1 {
		if err := json.Unmarshal(data[:n], &vv); err != nil {
			return err
		}
		vv.Content = strings.TrimSpace(string(data[n:]))
	}else{
		return json.Unmarshal(data, &vv)
	}
	return nil
}

func ReadValueFile(p string) (*ValueFile, error) {
	if _, err := os.Stat(p); err != nil {
		return nil, err
	}
	bts, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}
	valueFile := &ValueFile{}
	if err = valueFile.UnmarshalJSON(bts); err != nil {
		return nil, err
	}
	return valueFile, nil
}

func WriteValueFile(p string, valueFile *ValueFile) error {
	bts, err := valueFile.MarshalJSON()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(p, bts, 0644)
}
/**/

func Copy(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}