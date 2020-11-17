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
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	reLeadingDigit = regexp.MustCompile(`^[0-9]+-`)
)

var renderCmd = &cobra.Command{
	Use:   "render",
	Short: "Render data to hugo compatible data structures",
	Long:  `Render data to hugo compatible data structures`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Infof("Render module")
		output := path.Join(dir, "hugo", "content")
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
		logger.Infof("Configure Hugo Theme scripts")
		if p := path.Join(dir, "hugo", "themes", common.Config.Hugo.Theme, "layouts", "partials", "scripts.html"); len(p) > 0 {
			if _, err = os.Stat(p); err == nil {
				if bts, err := ioutil.ReadFile(p); err == nil {
					content := string(bts)
					content = strings.ReplaceAll(content, "%API_URL%", common.Config.Base)
					if err = ioutil.WriteFile(p, []byte(content), 0755); err != nil {
						logger.Warningf("%v", err.Error())
					}
				}
			}else{
				logger.Warningf("File %v not found!", p)
			}
		}
		//
		t1 := time.Now()
		if p := path.Join(dir, "hugo", "assets", "images", "variations"); p != "" {
			if _, err = os.Stat(p); err != nil {
				if err = os.MkdirAll(p, 0755); err != nil {
					logger.Warningf("%v", err)
				}
			}
		}
		if p := path.Join(dir, "hugo", "assets", "images", "values"); p != "" {
			if _, err = os.Stat(p); err != nil {
				if err = os.MkdirAll(p, 0755); err != nil {
					logger.Warningf("%v", err)
				}
			}
		}
		// Cache
		common.Database.Unscoped().Where("ID > ?", 0).Delete(&models.CacheProduct{})
		common.Database.Unscoped().Where("ID > ?", 0).Delete(&models.CacheImage{})
		// Categories
		if categories, err := models.GetCategories(common.Database); err == nil {
			// Clear existing "products" folder
			if err := os.RemoveAll(path.Join(output, "products")); err != nil {
				logger.Infof("%v", err)
			}
			logger.Infof("Categories found: %v", len(categories))
			for i, category := range categories {
				logger.Infof("Category %d: %+v", i, category)
				breadcrumbs := &[]*models.Category{}
				var f3 func(connector *gorm.DB, id uint)
				f3 = func(connector *gorm.DB, id uint) {
					if id != 0 {
						if category, err := models.GetCategory(common.Database, int(id)); err == nil {
							//*names = append([]string{category.Name}, *names...)
							if category.Thumbnail == "" {
								if len(*breadcrumbs) > 0 {
									category.Thumbnail = (*breadcrumbs)[0].Thumbnail
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
					if _, err := os.Stat(p1); err != nil {
						if err = os.MkdirAll(p1, 0755); err == nil {
							logger.Infof("Create directory %v", p1)
						} else {
							logger.Errorf("%v", err)
							os.Exit(2)
						}
					}
					var arr = []string{}
					for _, category := range *breadcrumbs {
						arr = append(arr, category.Name)
						if p2 := path.Join(append(append([]string{output}, arr...), "_index.md")...); len(p2) > 0 {
							if _, err := os.Stat(p2); err != nil {
								if bts, err := json.MarshalIndent(&CategoryView{
									ID: category.ID,
									Date:      category.UpdatedAt,
									Title:     category.Title,
									Thumbnail: category.Thumbnail,
									Path:      "/" + path.Join(arr...),
									Type:      "categories",
								}, "", "   "); err == nil {
									// Copy image
									if category.Thumbnail != "" {
										if p1 := path.Join(dir, category.Thumbnail); len(p1) > 0 {
											if fi, err := os.Stat(p1); err == nil {
												p2 := path.Join(path.Dir(p2), fmt.Sprintf("thumbnail-%d%v", fi.ModTime().Unix(), path.Ext(p1)))
												logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
												if err = cp(p1, p2); err != nil {
													logger.Warningf("%v", err)
												}
											}
										}
									}
									//
									if err = ioutil.WriteFile(p2, bts, 0644); err == nil {
										logger.Infof("Write %v: %v bytes", p2, len(bts))
									} else {
										logger.Errorf("%v", err)
										os.Exit(4)
									}
								}
							}
						}
					}
				}
			}
		}
		// Products
		if products, err := models.GetProducts(common.Database); err == nil {
			//
			logger.Infof("Products found: %v", len(products))
			for i, product := range products {
				logger.Infof("[%d] Product ID: %+v Name: %v Title: %v", i, product.ID, product.Name, product.Title)
				product, _ = models.GetProductFull(common.Database, int(product.ID))
				if categories, err := models.GetCategoriesOfProduct(common.Database, product); err == nil {
					var canonical string
					for i, category := range categories {
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
						if i == 0 {
							canonical = fmt.Sprintf("/%s/", path.Join(strings.Join(names, "/"), product.Name))
						}
						if p1 := path.Join(append([]string{output}, names...)...); len(p1) > 0 {
							if _, err := os.Stat(p1); err != nil {
								if err = os.MkdirAll(p1, 0755); err == nil {
									logger.Infof("Create directory %v", p1)
								} else {
									logger.Errorf("%v", err)
									os.Exit(2)
								}
							}
							//
							view := &PageView{
								ID: product.ID,
								Date: time.Now(),
								Title: product.Title,
								//Tags: []string{"Floor Light"},
								//BasePrice: "₹ 87,341.00",
								Type: "products",
								CategoryId: category.ID,
							}
							if i > 0 {
								view.Canonical = canonical
							}
							//
							var arr = []string{}
							for _, category := range *breadcrumbs {
								logger.Infof("TEST001 ID: %v, Name: %v", category.ID, category.Name)
								arr = append(arr, category.Name)
								if p2 := path.Join(append(append([]string{output}, arr...), "_index.md")...); len(p2) > 0 {
									logger.Infof("TEST001 p2: %v", p2)
									// Update category file
									if categoryFile, err := common.ReadCategoryFile(p2); err == nil {
										logger.Infof("TEST001 categoryFile.Options: %+v", categoryFile.Options)
										min := categoryFile.BasePriceMin
										max := categoryFile.BasePriceMax
										for _, variation := range product.Variations {
											// Min Max
											if min > variation.BasePrice || min == 0 {
												min = variation.BasePrice
											}
											if max < variation.BasePrice {
												max = variation.BasePrice
											}
											//
											for _, property := range variation.Properties {
												var found bool
												for i, opt := range categoryFile.Options {
													if opt.ID == property.Option.ID {
														logger.Infof("TEST001 property ID: %v, Name: %v FOUND", property.ID, property.Name)
														for _, price := range property.Prices {
															var found bool
															for _, value := range opt.Values {
																if value.ID == price.Value.ID {
																	found = true
																	break
																}
															}
															if !found {
																categoryFile.Options[i].Values = append(categoryFile.Options[i].Values, &common.Value{
																	ID: price.Value.ID,
																	Thumbnail: price.Value.Thumbnail,
																	Title:     price.Value.Title,
																	Value:     price.Value.Value,
																})
															}
														}
														found = true
													}
												}
												if !found {
													logger.Infof("TEST001 property ID: %v, Name: %v NOT FOUND (%v)", property.ID, property.Name, len(categoryFile.Options))
													opt := &common.Option{
														ID: property.Option.ID,
														Name: property.Option.Name,
														Title: property.Option.Title,
													}
													for _, price := range property.Prices {
														if price.Enabled {
															opt.Values = append(opt.Values, &common.Value{
																ID: price.Value.ID,
																Thumbnail: price.Value.Thumbnail,
																Title:     price.Value.Title,
																Value:     price.Value.Value,
															})
														}
													}
													logger.Infof("TEST001 property ID: %v, Name: %v ADD %+v", property.ID, property.Name, opt)
													categoryFile.Options = append(categoryFile.Options, opt)
												}
											}
										}
										categoryFile.BasePriceMin = min
										categoryFile.BasePriceMax = max
										//
										if err = common.WriteCategoryFile(p2, categoryFile); err != nil {
											logger.Warningf("%v", err)
										}
									}
									//
								}
								view.Categories = append(view.Categories, category.Title)
							}

							if len(product.Images) > 0 {
								for _, image := range product.Images {
									if image.Url != "" {
										view.Images = append(view.Images, image.Url)
									}else if image.Path != "" {
										view.Images = append(view.Images, image.Path)
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
							if p := path.Join(p1, product.Name); p != "" {
								if _, err := os.Stat(p); err != nil {
									if err = os.MkdirAll(p, 0755); err != nil {
										logger.Warningf("%v", err)
									}
								}
							}
							var basePriceMin float64
							if len(product.Variations) > 0 {
								view.BasePrice = fmt.Sprintf("%.2f", product.Variations[0].BasePrice)
								for i, variation := range product.Variations {
									variationView := VariationView{
										Id:          variation.ID,
										Name:        variation.Name,
										Title:       variation.Title,
										//Thumbnail:   variation.Thumbnail,
										Description: variation.Description,
										BasePrice:   variation.BasePrice,
										Selected:    i == 0,
									}
									if basePriceMin > variation.BasePrice || basePriceMin == 0 {
										basePriceMin = variation.BasePrice
									}
									// Thumbnail
									if variation.Thumbnail != "" {
										if p1 := path.Join(dir, variation.Thumbnail); len(p1) > 0 {
											if fi, err := os.Stat(p1); err == nil {
												filename := filepath.Base(p1)
												filename = fmt.Sprintf("%v-%d%v", filename[:len(filename) - len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
												p2 := path.Join(dir, "hugo", "assets", "images", "variations", filename)
												if _, err = os.Stat(p2); err != nil {
													logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
													if err = cp(p1, p2); err != nil {
														logger.Warningf("%v", err)
													}
												}
												variationView.Thumbnail = "/" + path.Join("images", "variations", filename)
											}
										}
									}
									for _, property := range variation.Properties {
										propertyView := PropertyView{
											Id: property.ID,
											Type: property.Type,
											Name:        property.Name,
											Title:       property.Title,
										}
										for h, price := range property.Prices {
											valueView := ValueView{
												Id:        price.Value.ID,
												Enabled:   price.Enabled,
												Title:     price.Value.Title,
												//Thumbnail: price.Value.Thumbnail,
												Value:     price.Value.Value,
												Price:PriceView{
													Id:    price.ID,
													Price:     price.Price,
												},
												Selected: h == 0,
											}
											// Thumbnail
											if price.Value.Thumbnail != "" {
												if p1 := path.Join(dir, price.Value.Thumbnail); len(p1) > 0 {
													if fi, err := os.Stat(p1); err == nil {
														filename := filepath.Base(p1)
														filename = fmt.Sprintf("%v-%d%v", filename[:len(filename) - len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
														p2 := path.Join(dir, "hugo", "assets", "images", "values", filename)
														if _, err = os.Stat(p2); err != nil {
															logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
															if err = cp(p1, p2); err != nil {
																logger.Warningf("%v", err)
															}
														}
														valueView.Thumbnail = "/" + path.Join("images", "values", filename)
													}
												}
											}
											propertyView.Values = append(propertyView.Values, valueView)
										}
										variationView.Properties = append(variationView.Properties, propertyView)
									}
									productView.Variations = append(productView.Variations, variationView)
								}
							}
							productView.Path = "/" + path.Join(append(names, product.Name)...) + "/"
							view.Product = productView
							for _, tag := range product.Tags {
								if tag.Enabled {
									view.Tags = append(view.Tags, tag.Name)
								}
							}
							if bts, err := json.MarshalIndent(&view, "", "   "); err == nil {
								file := path.Join(p1, product.Name, fmt.Sprintf("%v.md", "index"))
								if _, err := os.Stat(path.Dir(file)); err != nil {
									if err = os.MkdirAll(path.Dir(file), 0755); err != nil {
										logger.Error("%v", err)
										return
									}
								}
								// Copy thumbnail
								var thumbnail string
								if product.Thumbnail != "" {
									p0 := path.Join(p1, product.Name)
									if p1 := path.Join(dir, product.Thumbnail); len(p1) > 0 {
										if fi, err := os.Stat(p1); err == nil {
											filename := fmt.Sprintf("thumbnail-%d%v", fi.ModTime().Unix(), path.Ext(p1))
											p2 := path.Join(p0, filename)
											logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
											if err = cp(p1, p2); err == nil {
												thumbnail = fmt.Sprintf("/%s/%s", strings.Join(append(names, product.Name), "/"), filename)
											} else {
												logger.Warningf("%v", err)
											}
										}
									}
								}
								// Copy images
								if len(product.Images) > 0 {
									for i, image := range product.Images {
										if image.Path != "" {
											p0 := path.Join(p1, product.Name)
											if _, err = os.Stat(p0); err != nil {
												if err = os.MkdirAll(p0, 0755); err != nil {
													logger.Warningf("%v", err)
												}
											}
											if p1 := path.Join(dir, image.Path); len(p1) > 0 {
												if fi, err := os.Stat(p1); err == nil {
													p2 := path.Join(p0, fmt.Sprintf("image-%d-%d%v", i + 1, fi.ModTime().Unix(), path.Ext(p1)))
													logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
													if err = cp(p1, p2); err != nil {
														logger.Warningf("%v", err)
													}
												}
											}
										}
									}
								}
								//
								bts = []byte(fmt.Sprintf(`%s

%s`, string(bts), product.Description))
								if err = ioutil.WriteFile(file, bts, 0755); err == nil {
									logger.Infof("Write %v %v bytes", file, len(bts))
								} else {
									logger.Error("%v", err)
								}
								// Cache
								if _, err = models.CreateCacheProduct(common.Database, &models.CacheProduct{
									ProductID:   product.ID,
									Path:        fmt.Sprintf("/%s/", strings.Join(names, "/")),
									Name:        product.Name,
									Title:       product.Title,
									Description: product.Description,
									Thumbnail:   thumbnail,
									CategoryID:  category.ID,
									BasePrice:   basePriceMin,
								}); err != nil {
									logger.Warningf("%v", err)
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
	ID uint
	Date time.Time
	Title string
	Thumbnail string
	Path string
	Type string
}

type PageView struct {
	ID uint
	Type       string
	Title      string
	Date       time.Time
	Tags       []string
	Canonical  string
	Categories []string
	CategoryId  uint
	Images     []string
	Thumbnail  string
	BasePrice  string
	Product    ProductView
}

type ProductView struct {
	Id         uint `json:"Id"`
	CategoryId uint
	Name       string
	Title      string
	Thumbnail  string
	Path       string
	Variations []VariationView
}

type VariationView struct {
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
	Type string
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

func cp(src, dst string) error {
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