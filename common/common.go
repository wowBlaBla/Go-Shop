package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/google/logger"
	"github.com/nfnt/resize"
	"github.com/yonnic/goshop/config"
	"github.com/yonnic/goshop/storage"
	"gorm.io/gorm"
	"image"
	"image/jpeg"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	DEFAULT_TITLE = "Demo Shop"
	DEFAULT_THEME = "default"
	DEFAULT_PASSWORD = "goshoppass"
	DEFAULT_PAGINATE = 32
	PRODUCTS_NAME    = "products"
	SECRET = "goshop"
)

var (
	APPLICATION = "GoShop"
	VERSION = "1.0.0"
	COMPILED = "20210517112022"
	STORAGE storage.Storage
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
	Url        string `json:",omitempty"`
	Aliases    []string `json:",omitempty"`
	Thumbnail    string
	Path         string
	Properties   []*PropertyCF
	Options      []*OptionCF
	Price        MiniMaxCF
	Dimensions   DimensionsCF
	Volume       MiniMaxCF
	Weight       MiniMaxCF
	Widgets      []WidgetCF
	Type         string
	Count int
	//
	Content string
}

type MiniMaxCF struct {
	Min float64
	Max float64
}

type DimensionsCF struct {
	Width MiniMaxCF
	Height MiniMaxCF
	Depth MiniMaxCF
}

type PropertyCF struct {
	Name string
	Title string
	Values []*ValueCF
}

type WidgetCF struct {
	Name    string
	Title   string
	Content string
	Location string
	ApplyTo string
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
		Url string
		Aliases    []string `json:",omitempty"`
		Description string `json:",omitempty"`
		Thumbnail string
		Path string
		Price MiniMaxCF
		Options []*OptionCF
		Dimensions DimensionsCF
		Volume MiniMaxCF
		Weight MiniMaxCF
		Widgets []WidgetCF
		Type string
		Count int
	}{
		ID: p.ID,
		Date: p.Date,
		Title: p.Title,
		Url: p.Url,
		Aliases: p.Aliases,
		Thumbnail: p.Thumbnail,
		Path: p.Path,
		Options: p.Options,
		Price: p.Price,
		Dimensions: p.Dimensions,
		Volume: p.Volume,
		Weight: p.Weight,
		Widgets: p.Widgets,
		Type: p.Type,
		Count: p.Count,
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
	ID         uint
	Type       string
	Title      string
	Url        string `json:",omitempty"`
	Aliases    []string `json:",omitempty"`
	Date       time.Time
	//Tags       []string
	Tags []TagPF `json:",omitempty"`
	Canonical  string `json:",omitempty"`
	Categories []string
	CategoryId uint
	Thumbnail  string
	BasePrice  string
	SalePrice  string `json:",omitempty"`
	Start      *time.Time `json:",omitempty"`
	End        *time.Time `json:",omitempty"`
	Product    ProductPF
	Related    []string `json:",omitempty"`
	Widgets    []WidgetCF `json:",omitempty"`
	Sku string `json:",omitempty"`
	Max float64 `json:",omitempty"`
	Votes int `json:",omitempty"`
	Comments []CommentPF `json:",omitempty"`
	Sort int
	//
	Content string
}

type TagPF struct {
	Id         uint
	Name       string
	Title      string
	Thumbnail  string `json:",omitempty"`
}

type ProductPF struct {
	Id         uint `json:"Id"`
	CategoryId uint
	Path       string
	Name       string
	Title      string
	Description string `json:",omitempty"`
	Thumbnail  string `json:",omitempty"`
	Files     []FilePF
	Images     []string
	Parameters []ParameterPF
	CustomParameters []CustomParameterPF
	Pattern string `json:",omitempty"`
	Dimensions string `json:",omitempty"`
	Volume float64 `json:",omitempty"`
	Weight float64 `json:",omitempty"`
	Availability string `json:",omitempty"`
	Time string `json:",omitempty"`
	Properties []PropertyPF
	Variations []VariationPF
}

type CustomParameterPF struct {
	Key string
	Value string
}

type FilePF struct {
	Id uint
	Type string
	Name string
	Path string
	Size int64
}

type ParameterPF struct {
	Id uint
	Name string
	Title string
	Value ValuePPF
	CustomValue string `json:",omitempty"`
}

type ValuePPF struct {
	Id uint
	Title string
	Thumbnail string `json:",omitempty"`
	Value string
	Availability string `json:",omitempty"`
	//Sending string `json:",omitempty"`
}

type VariationPF struct {
	Id uint
	Name string
	Title string
	Thumbnail string `json:",omitempty"`
	Description string `json:",omitempty"`
	Images     []string `json:",omitempty"`
	Files     []FilePF `json:",omitempty"`
	BasePrice float64
	SalePrice  float64  `json:",omitempty"`
	Start *time.Time    `json:",omitempty"`
	End *time.Time      `json:",omitempty"`
	Prices []PricePF     `json:",omitempty"`
	Pattern string      `json:",omitempty"`
	Dimensions string   `json:",omitempty"`
	Width float64       `json:",omitempty"`
	Height float64      `json:",omitempty"`
	Depth float64       `json:",omitempty"`
	Volume float64 `json:",omitempty"`
	Weight float64      `json:",omitempty"`
	Availability string `json:",omitempty"`
	//Sending string `json:",omitempty"`
	Time string `json:",omitempty"`
	Properties []PropertyPF `json:",omitempty"`
	Sku string `json:",omitempty"`
	Selected bool
}

type PricePF struct {
	Ids []uint
	Price float64
	Availability string `json:",omitempty"`
	Sku string `json:",omitempty"`
}

type PropertyPF struct {
	Id uint
	Type string
	Size string `json:",omitempty"`
	Name string
	Title string
	Description string `json:",omitempty"`
	Values []ValuePF
}

type ValuePF struct {
	Id uint
	Enabled bool
	Title string
	Thumbnail string `json:",omitempty"`
	Description string `json:",omitempty"`
	Value string
	Availability string `json:",omitempty"`
	//Sending string `json:",omitempty"`
	Price    RatePF
	Selected bool
}

type RatePF struct {
	Id uint
	Price float64
	Availability string `json:",omitempty"`
	Sku string `json:",omitempty"`
}

type CommentPF struct {
	Id uint
	Uuid string
	Title string
	Body string
	Max int
	Images []string `json:",omitempty"`
	Author string `json:",omitempty"`
}

func (p *ProductFile) MarshalJSON() ([]byte, error) {
	if bts, err := json.MarshalIndent(&struct {
		ID uint
		Type       string
		Title      string
		Url        string `json:",omitempty"`
		Aliases    []string `json:",omitempty"`
		Date       time.Time
		Tags       []TagPF
		Canonical  string
		Categories []string
		CategoryId  uint
		Thumbnail  string
		BasePrice  string
		SalePrice  string `json:",omitempty"`
		Start       *time.Time `json:",omitempty"`
		End       *time.Time `json:",omitempty"`
		Product    ProductPF
		Related []string `json:",omitempty"`
		Widgets []WidgetCF `json:",omitempty"`
		Sku string
		Max float64 `json:",omitempty"`
		Votes int `json:",omitempty"`
		Comments []CommentPF `json:",omitempty"`
		Sort int
	}{
		ID: p.ID,
		Type: p.Type,
		Title: p.Title,
		Url: p.Url,
		Aliases: p.Aliases,
		Date: p.Date,
		Tags: p.Tags,
		Canonical: p.Canonical,
		Categories: p.Categories,
		CategoryId: p.CategoryId,
		Thumbnail: p.Thumbnail,
		BasePrice: p.BasePrice,
		SalePrice: p.SalePrice,
		Start: p.Start,
		End: p.End,
		Product: p.Product,
		Related: p.Related,
		Widgets: p.Widgets,
		Sku: p.Sku,
		Max: p.Max,
		Votes: p.Votes,
		Comments: p.Comments,
		Sort: p.Sort,
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

/* Tag */
type TagFile struct {
	ID        uint
	Name      string
	Title     string
	Thumbnail string
	Type         string
	//
	Content string
}

func (o *TagFile) MarshalJSON() ([]byte, error) {
	if bts, err := json.MarshalIndent(&struct {
		ID        uint
		Name      string
		Title     string
		Thumbnail string
		Type         string
	}{
		ID:         o.ID,
		Name:       o.Name,
		Title:      o.Title,
		Thumbnail:  o.Thumbnail,
		Type: o.Type,
	}, "", "   "); err == nil {
		bts = append(bts, "\n\n"...)
		bts = append(bts, o.Content...)
		return bts, nil
	}else{
		return []byte{}, err
	}
}

func (o *TagFile) UnmarshalJSON(data []byte) error {
	type Alias TagFile
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

func ReadTagFile(p string) (*TagFile, error) {
	if _, err := os.Stat(p); err != nil {
		return nil, err
	}
	bts, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}
	tagFile := &TagFile{}
	if err = tagFile.UnmarshalJSON(bts); err != nil {
		return nil, err
	}
	return tagFile, nil
}

func WriteTagFile(p string, tagFile *TagFile) error {
	bts, err := tagFile.MarshalJSON()
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
	Description  string `json:",omitempty"`
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
		Description  string `json:",omitempty"`
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
	fi1, err := os.Stat(src)
	if err != nil {
		return err
	}
	if fi2, err := os.Stat(dst); err == nil {
		if fi1.ModTime().Equal(fi2.ModTime()) && fi1.Size() == fi2.Size() {
			//logger.Infof("ModTime and Size are the same, skip")
			return nil
		}
	}
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
	err = out.Close()
	if err != nil {
		return err
	}
	return os.Chtimes(dst, fi1.ModTime(), fi1.ModTime())
}

type Image struct {
	Filename string
	Size string
}

func ImageResize(src, sizes string) ([]Image, error) {
	var images []Image
	fi1, err := os.Stat(src)
	if err != nil {
		return images, err
	}
	var img image.Image
	if p := path.Join(path.Dir(src), "resize"); len(p) > 0 {
		if _, err := os.Stat(p); err != nil {
			if err = os.MkdirAll(p, 0755); err != nil {
				logger.Warningf("%v", err)
			}
		}
	}
	for _, size := range strings.Split(sizes, ",") {
		pair := strings.Split(size, "x")
		var width int
		if width, err = strconv.Atoi(pair[0]); err != nil {
			return images, err
		}
		var height int
		if height, err = strconv.Atoi(pair[1]); err != nil {
			return images, err
		}
		filename := path.Base(src)
		filename = filename[:len(filename) - len(filepath.Ext(filename))]
		filename = fmt.Sprintf("%s_%dx%d%s", filename, width, height, filepath.Ext(src))
		if fi2, err := os.Stat(path.Join(path.Dir(src), "resize", filename)); err != nil || !fi1.ModTime().Equal(fi2.ModTime()) {
			if img == nil {
				file, err := os.Open(src)
				if err != nil {
					return images, err
				}
				img, err = jpeg.Decode(file)
				if err != nil {
					return images, err
				}
				file.Close()
			}
			m := resize.Resize(uint(width), uint(height), img, resize.Lanczos3)
			out, err := os.Create(path.Join(path.Dir(src), "resize", filename))
			if err != nil {
				return images, err
			}
			if err = jpeg.Encode(out, m, &jpeg.Options{Quality: Config.Resize.Quality}); err != nil {
				return images, err
			}
			out.Close()
			if err = os.Chtimes(path.Join(path.Dir(src), "resize", filename), fi1.ModTime(), fi1.ModTime()); err != nil {
				logger.Warningf("%+v", err)
			}
		}
		images = append(images, Image{Filename: filename, Size: fmt.Sprintf("%dw", width)})
	}
	return images, nil
}

type MenuView2 struct {
	Name string
	Title string
	Location string
	Children []interface{}
	//Children []MenuX
}

type MenuView3 struct {
	Name string
	Data MenuView4
	Children []MenuView3
}

type MenuView4 struct {
	Type string
	Id uint
	Name string
	Title string
	Path string
	Url string
	Anchor string
}

type MenuItemView struct {
	ID uint `json:",omitempty"`
	Name string `json:",omitempty"`
	Type  string
	Url   string `json:",omitempty"`
	Path string `json:",omitempty"`
	Title string `json:",omitempty"`
	Thumbnail string `json:",omitempty"`
	Children []interface{} `json:",omitempty"`
}