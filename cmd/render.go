package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/google/logger"
	"github.com/spf13/cobra"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/models"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"io/ioutil"
	"os"
	"path"
	"time"
)

var renderCmd = &cobra.Command{
	Use:   "render",
	Short: "Render data to hugo compatible data structures",
	Long:  `Render data to hugo compatible data structures`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Infof("Render module")
		output := path.Join(dir, "content")
		if flagOutput := cmd.Flag("products").Value.String(); flagOutput != "" {
			output = flagOutput
		}
		logger.Infof("output: %v", output)
		var err error
		// Database
		var dialer gorm.Dialector
		if common.Config.Database.Dialer == "mysql" {
			dialer = mysql.Open(common.Config.Database.Uri)
		}else {
			var uri = path.Join(dir, os.Getenv("DATABASE_FOLDER"), "database.sqlite")
			if common.Config.Database.Uri != "" {
				uri = common.Config.Database.Uri
			}
			dialer = sqlite.Open(uri)
		}
		common.Database, err = gorm.Open(dialer, &gorm.Config{})
		if err != nil {
			logger.Errorf("%v", err)
			os.Exit(1)
		}
		common.Database.DB()
		//
		/*var f1 func (connector *gorm.DB, root *CategoryView) *CategoryView
		f1 = func (connector *gorm.DB, root *CategoryView) *CategoryView {
			root.Path = append(root.Path, root.Name)
			for _, category := range models.GetChildrenOfCategoryById(connector, root.ID) {
				child := f1(connector, &CategoryView{ID: category.ID, Name: category.Name, Title: category.Title, Path: root.Path})
				root.Children = append(root.Children, child)
				if p := path.Join(append([]string{output}, child.Path...)...); len(p) > 0 {
					if _, err := os.Stat(p); err != nil {
						if err = os.MkdirAll(p, 0755); err != nil {
							logger.Errorf("%v", err)
							os.Exit(2)
						}
					}
				}
			}
			return root
		}
		f1(common.Database, &CategoryView{Name: "", Title: "Root"})*/
		//
		if products, err := models.GetProducts(common.Database); err == nil {
			logger.Infof("Products found: %v", len(products))
			for i, product := range products {
				logger.Infof("%d: %+v", i, product)
				product, _ = models.GetProductFull(common.Database, int(product.ID))
				if categories, err := models.GetCategoriesOfProduct(common.Database, product); err == nil {
					for _, category := range categories {
						breadcrumbs := &[]*models.Category{}
						var f3 func(connector *gorm.DB, id uint)
						f3 = func(connector *gorm.DB, id uint) {
							if id != 0 {
								if category, err := models.GetCategory(common.Database, int(id)); err == nil {
									//*names = append([]string{category.Name}, *names...)
									if category.Thumbnail == "" {
										if len(*breadcrumbs) > 0 {
											category.Thumbnail = (*breadcrumbs)[0].Thumbnail
										} else if product.Thumbnail != "" {
											category.Thumbnail = product.Thumbnail
										}
									}
									*breadcrumbs = append([]*models.Category{category}, *breadcrumbs...)
									f3(connector, category.ParentId)
								}
							}
						}
						f3(common.Database, category.ID)
						*breadcrumbs = append([]*models.Category{{Name: "products", Title: "Products", Model: gorm.Model{UpdatedAt: time.Now()}}}, *breadcrumbs...)
						var names []string
						for _, crumb := range *breadcrumbs {
							names = append(names, crumb.Name)
						}
						if p1 := path.Join(append([]string{output}, names...)...); len(p1) > 0 {
							logger.Infof("Directory %v", p1)
							if _, err := os.Stat(p1); err != nil {
								if err = os.MkdirAll(p1, 0755); err != nil {
									logger.Errorf("%v", err)
									os.Exit(2)
								}
							}
							//
							view := &PageView{
								Date: time.Now(),
								Title: product.Title,
								//Tags: []string{"Floor Light"},
								//BasePrice: "₹ 87,341.00",
								Type: "products",
								CategoryId: category.ID,
							}
							var arr = []string{}
							for _, category := range *breadcrumbs {
								arr = append(arr, category.Name)
								if p2 := path.Join(append(append([]string{output}, arr...), "_index.md")...); len(p2) > 0 {
									logger.Infof("File %v", p2)
									if _, err := os.Stat(p2); err != nil {
										if bts, err := json.MarshalIndent(&CategoryView{
											Date:      category.UpdatedAt,
											Title:     category.Title,
											Thumbnail: category.Thumbnail,
											Path:      "/" + path.Join(arr...),
											Type:      "categories",
										}, "", "   "); err == nil {
											if err = ioutil.WriteFile(p2, bts, 0644); err != nil {
												logger.Errorf("%v", err)
												os.Exit(4)
											}
										}
									}
								}
								view.Categories = append(view.Categories, category.Title)
							}

							if len(product.Images) > 0 {
								for _, image := range product.Images {
									if image.Path != "" {
										view.Images = append(view.Images, image.Path)
									}else if image.Url != "" {
										view.Images = append(view.Images, image.Url)
									}
								}
							}
							productView := ProductView{
								Id: product.ID,
								CategoryId: category.ID,
								Name: product.Name,
								Title: product.Title,
							}
							if product.Thumbnail != "" {
								view.Thumbnail = product.Thumbnail
								productView.Thumbnail = product.Thumbnail
							}
							if len(product.Offers) > 0 {
								view.BasePrice = fmt.Sprintf("$%.2f", product.Offers[0].BasePrice)
								for i, offer := range product.Offers {
									offerView := OfferView{
										Id: offer.ID,
										Name:        offer.Name,
										Title:       offer.Title,
										Thumbnail: offer.Thumbnail,
										Description: offer.Description,
										BasePrice:   offer.BasePrice,
										Selected: i == 0,
									}
									for _, property := range offer.Properties {
										propertyView := PropertyView{
											Id: property.ID,
											Name:        property.Name,
											Title:       property.Title,
										}
										for h, price := range property.Prices {
											valueView := ValueView{
												Id:        price.Value.ID,
												Enabled:   price.Enabled,
												Title:     price.Value.Title,
												Thumbnail: price.Value.Thumbnail,
												Value:     price.Value.Value,
												Price:PriceView{
													Id:    price.ID,
													Price:     price.Price,
												},
												Selected: h == 0,
											}
											propertyView.Values = append(propertyView.Values, valueView)
										}
										offerView.Properties = append(offerView.Properties, propertyView)
									}
									productView.Offers = append(productView.Offers, offerView)
								}
							}
							productView.Path = "/" + path.Join(append(names, product.Name)...) + "/"
							view.Product = productView
							if bts, err := json.MarshalIndent(&view, "", "   "); err == nil {
								file := path.Join(p1, product.Name, fmt.Sprintf("%v.md", "index"))
								if _, err := os.Stat(path.Dir(file)); err != nil {
									if err = os.MkdirAll(path.Dir(file), 0755); err != nil {
										logger.Error("%v", err)
										return
									}
								}
								bts = []byte(fmt.Sprintf(`%s

%s`, string(bts), product.Description))
								if err = ioutil.WriteFile(file, bts, 0755); err == nil {
									logger.Infof("File %v wrote %v bytes", file, len(bts))
								} else {
									logger.Error("%v", err)
								}
							}else{
								logger.Error("%v", err)
							}
						}
					}
				}
			}
		}else{
			logger.Error("%v", err)
			return
		}
		//
		t1 := time.Now()

		/*result := struct {
			Title string
			Date time.Time
			Tags []string
			Categories []string
			Images []string
			Thumbnail string
			BasePrice string
			ComparePrice *models.Price
			InStock bool
		}{
			Title: "Duke2",
			Date: time.Now(),
			Tags: []string{"Floor Light"},
			Categories: []string{"Floor Light"},
			Images: []string{"img/duke/1.jpg", "img/duke/2.jpg", "img/duke/3.jpg"},
			Thumbnail: "img/duke/thumbnail.jpg",
			BasePrice: "₹ 87,341.00",
			ComparePrice: nil,
			InStock: true,
		}
		// JSON
		if bts, err := json.MarshalIndent(&result, "", "   "); err == nil {
			file := path.Join(output, fmt.Sprintf("%d-%s.md", 1, "hello"))
			if _, err := os.Stat(path.Dir(file)); err != nil {
				if err = os.MkdirAll(path.Dir(file), 0755); err != nil {
					logger.Error("%v", err)
					return
				}
			}
			bts = []byte(fmt.Sprintf(`%s

Some content here`, string(bts)))
			if err = ioutil.WriteFile(file, bts, 0755); err == nil {
				logger.Infof("File %v wrote %v", file, len(bts))
			} else {
				logger.Error("%v", err)
			}
		}else{
			logger.Error("%v", err)
		}*/
		// YAML
		/*if bts, err := yaml.Marshal(&result); err == nil {
			file := path.Join(output, "first.md")
			if _, err := os.Stat(path.Dir(file)); err != nil {
				if err = os.MkdirAll(path.Dir(file), 0755); err != nil {
					logger.Error("%v", err)
					return
				}
			}
			bts = []byte(fmt.Sprintf(`---
%s
---
Some content here`, string(bts)))
			if err = ioutil.WriteFile(file, bts, 0755); err == nil {
				logger.Infof("File %v wrote %v", file, len(bts))
			} else {
				logger.Error("%v", err)
			}
		}else{
			logger.Error("%v", err)
		}*/
		logger.Infof("Rendered ~ %.3f ms", float64(time.Since(t1).Nanoseconds())/1000000)
	},
}

func init() {
	RootCmd.AddCommand(renderCmd)
	renderCmd.Flags().StringP("products", "p", "products", "products output folder")
}

/**/

/*type CategoriesView []CategoryView

type CategoryView struct {
	ID uint
	Path []string
	Name string
	Title string
	Children []*CategoryView `json:",omitempty"`
	ParentId uint
}*/

/**/

/*
date: %s
draft: false
title: %s
thumbnail: %s
path: %s
type: categories
*/

type CategoryView struct {
	Date time.Time
	Title string
	Thumbnail string
	Path string
	Type string
}

type PageView struct {
	Type       string
	Title      string
	Date       time.Time
	Tags       []string
	Categories []string
	CategoryId  uint
	Images     []string
	Thumbnail  string
	BasePrice  string
	Product    ProductView
}

type ProductView struct {
	Id uint `json:"Id"`
	CategoryId uint
	Name string
	Title string
	Thumbnail string
	Path string
	Offers []OfferView
}

type OfferView struct {
	Id uint
	Name string
	Title string
	Thumbnail string
	Description string
	BasePrice float64
	Properties []PropertyView
	Selected bool
}

type PropertyView struct {
	Id uint
	Name string
	Title string
	Description string
	Values []ValueView
}

type ValueView struct {
	Id uint
	Enabled bool
	Title string
	Thumbnail string
	Value string
	Price PriceView
	Selected bool
}

type PriceView struct {
	Id uint
	Price float64
}