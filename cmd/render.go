package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/google/logger"
	"github.com/spf13/cobra"
	cmap "github.com/streamrail/concurrent-map"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/config"
	"github.com/yonnic/goshop/models"
	"github.com/yonnic/goshop/storage"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"io/ioutil"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	VALUES = cmap.New()
	reKV = regexp.MustCompile(`^([^\:]+):\s*(.*)$`)
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
		remove := false
		if flagRemove := cmd.Flag("remove").Value.String(); flagRemove == "true" {
			remove = true
		}
		logger.Infof("remove: %v", remove)
		now := time.Now()
		var err error
		// Database
		var dialer gorm.Dialector
		if common.Config.Database.Dialer == "mysql" {
			dialer = mysql.Open(common.Config.Database.Uri)
		} else if common.Config.Database.Dialer == "postgres" {
			dialer = postgres.Open(common.Config.Database.Uri)
		} else {
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
		if _, err := common.Database.DB(); err != nil {
			logger.Fatalf("%v", err)
		}
		//
		/*logger.Infof("Configure Hugo Theme index")
		if p := path.Join(dir, "hugo", "themes", common.Config.Hugo.Theme, "layouts", "partials", "scripts.html"); len(p) > 0 {
			if _, err = os.Stat(p); err == nil {
				if bts, err := ioutil.ReadFile(p); err == nil {
					content := string(bts)
					content = strings.ReplaceAll(content, "%API_URL%", common.Config.Base)
					if common.Config.Payment.Enabled {
						if common.Config.Payment.Mollie.Enabled {
							content = strings.ReplaceAll(content, "%MOLLIE_PROFILE_ID%", common.Config.Payment.Mollie.ProfileID)
						}
						if common.Config.Payment.Stripe.Enabled {
							content = strings.ReplaceAll(content, "%STRIPE_PUBLISHED_KEY%", common.Config.Payment.Stripe.PublishedKey)
						}
					}
					if err = ioutil.WriteFile(p, []byte(content), 0755); err != nil {
						logger.Warningf("%v", err.Error())
					}
				}
			}else{
				logger.Warningf("File %v not found!", p)
			}
		}*/
		//
		t1 := time.Now()
		/*if p := path.Join(dir, "hugo", "assets", "images", "variations"); p != "" {
			if _, err = os.Stat(p); err != nil {
				if err = os.MkdirAll(p, 0755); err != nil {
					logger.Warningf("%v", err)
				}
			}
		}
		if p := path.Join(dir, "hugo", "static", "images", "values"); p != "" {
			if _, err = os.Stat(p); err != nil {
				if err = os.MkdirAll(p, 0755); err != nil {
					logger.Warningf("%v", err)
				}
			}
		}*/
		// Languages
		languages := []config.Language{
			{
				Enabled: true,
				Name: "English",
				Code: "", // default
			},
		}
		if common.Config.I18n.Enabled {
			logger.Infof("I18n enabled")
			for _, language := range common.Config.I18n.Languages {
				if language.Enabled {
					logger.Infof("Add language: %+v", language)
					language.Suffix = "." + language.Code
					languages = append(languages, language)
				}
			}
		}
		// Cache
		common.Database.Unscoped().Where("ID > ?", 0).Delete(&models.CacheCategory{})
		common.Database.Unscoped().Where("ID > ?", 0).Delete(&models.CacheProduct{})
		common.Database.Unscoped().Where("ID > ?", 0).Delete(&models.CacheImage{})
		common.Database.Unscoped().Where("ID > ?", 0).Delete(&models.CacheVariation{})
		common.Database.Unscoped().Where("ID > ?", 0).Delete(&models.CacheValue{})
		common.Database.Unscoped().Where("ID > ?", 0).Delete(&models.CacheTransport{})
		//
		common.STORAGE, err = storage.NewLocalStorage(path.Join(dir, "hugo", "static"), common.Config.Resize.Quality)
		if err != nil {
			logger.Warningf("%+v", err)
		}
		if common.Config.Storage.Enabled {
			if common.Config.Storage.S3.Enabled {
				if common.STORAGE, err = storage.NewAWSS3Storage(common.Config.Storage.S3.AccessKeyID,common.Config.Storage.S3.SecretAccessKey, common.Config.Storage.S3.Region, common.Config.Storage.S3.Bucket, common.Config.Storage.S3.Prefix, path.Join(dir, "temp", "s3"), common.Config.Resize.Quality, common.Config.Storage.S3.Rewrite); err != nil {
					logger.Warningf("%+v", err)
				}
			}
		}
		// Files
		/*if files, err := models.GetFiles(common.Database); err == nil {
			logger.Infof("Files found: %v", len(files))
			if _, err = os.Stat(path.Join(dir, "hugo", "static", "files")); err != nil {
				if err = os.MkdirAll(path.Join(dir, "hugo", "static", "files"), 0755); err != nil {
					logger.Warningf("%v", err)
				}
			}
			for _, file := range files {
				if err = common.Copy(path.Join(dir, file.Path), path.Join(dir, "hugo", "static", "files", path.Base(file.Path))); err != nil {
					logger.Warningf("%v", err)
				}
			}
		}*/
		// Widgets
		var allWidgets []common.WidgetCF
		if widgets, err := models.GetWidgetsByApplyTo(common.Database, "all"); err == nil {
			for _, widget := range widgets {
				if widget.Enabled {
					allWidgets = append(allWidgets, common.WidgetCF{
						Name:     widget.Name,
						Title:    widget.Title,
						Content:  widget.Content,
						Location: widget.Location,
						ApplyTo:  widget.ApplyTo,
					})
				}
			}
		} else {
			logger.Warningf("%+v", err)
		}
		// Catalog
		if tree, err := models.GetCategoriesView(common.Database, 0, 999, false, true); err == nil {
			if bts, err := json.MarshalIndent(tree, " ", "   "); err == nil {
				p := path.Join(dir, "hugo", "data")
				if _, err = os.Stat(p); err != nil {
					if err = os.MkdirAll(p, 0755); err != nil {
						logger.Warningf("%+v", err)
					}
				}
				if err = ioutil.WriteFile(path.Join(p, "catalog.json"), bts, 0755); err != nil {
					logger.Warningf("%+v", err)
				}
			}
		}else{
			logger.Warningf("%+v", err)
		}
		if categories, err := models.GetCategories(common.Database); err == nil {
			// Clear existing "products" folder
			if common.Config.Products != "" {
				if err := os.RemoveAll(path.Join(output, strings.ToLower(common.Config.Products))); err != nil {
					logger.Infof("%v", err)
				}
			}
			//
			if p2 := path.Join(output, strings.ToLower(common.Config.Products)); len(p2) > 0 {
				if _, err := os.Stat(p2); err != nil {
					if err = os.MkdirAll(p2, 0755); err != nil {
						logger.Warningf("%+v", err)
					}
				}
				categoryFile := &common.CategoryFile{
					ID:    0,
					Date:  time.Now(),
					Title: common.Config.Products,
					Path:    "/" + strings.ToLower(common.Config.Products),
					Type:    "categories",
				}
				if tree, err := models.GetCategoriesView(common.Database, 0, 999, true, true); err == nil {
					categoryFile.Count = tree.Count
				}else{
					logger.Warningf("%+v", err)
				}
				if err = common.WriteCategoryFile(path.Join(p2, "_index.html"), categoryFile); err != nil {
					logger.Warningf("%v", err)
				}
			}
			logger.Infof("Categories found: %v", len(categories))
			// Widgets
			var allCategoriesWidgets []common.WidgetCF
			if widgets, err := models.GetWidgetsByApplyTo(common.Database, "all-categories"); err == nil {
				for _, widget := range widgets {
					if widget.Enabled {
						allCategoriesWidgets = append(allCategoriesWidgets, common.WidgetCF{
							Name:     widget.Name,
							Title:    widget.Title,
							Content:  widget.Content,
							Location: widget.Location,
							ApplyTo:  widget.ApplyTo,
						})
					}
				}
			} else {
				logger.Warningf("%+v", err)
			}
			//
			for i, category := range categories {
				logger.Infof("Category %d: %v %v", i, category.Name, category.Title)
				breadcrumbs := &[]*models.Category{}
				var f3 func(connector *gorm.DB, id uint)
				f3 = func(connector *gorm.DB, id uint) {
					if id != 0 {
						if category, err := models.GetCategory(common.Database, int(id)); err == nil {
							//*names = append([]string{category.Country}, *names...)
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
				if common.Config.Products != "" {
					*breadcrumbs = append([]*models.Category{{Name: strings.ToLower(common.Config.Products), Title: common.Config.Products, Model: gorm.Model{UpdatedAt: time.Now()}}}, *breadcrumbs...)
				}
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
					//
					var thumbnails []string
					if category.Thumbnail != "" {
						//p0 := path.Join(p1, product.Country)
						if p1 := path.Join(dir, "storage", category.Thumbnail); len(p1) > 0 {
							if fi, err := os.Stat(p1); err == nil {
								name := category.Name
								if len(name) > 32 {
									name = name[:32]
								}
								filename := fmt.Sprintf("%d-%s-%d%v", category.ID, name, fi.ModTime().Unix(), path.Ext(p1))
								logger.Infof("Copy %v => %v %v bytes", p1, path.Join("images", "categories", filename), fi.Size())
								if thumbnails, err = common.STORAGE.PutImage(p1, path.Join("images", "categories", filename), common.Config.Resize.Thumbnail.Size); err != nil {
									logger.Warningf("%v", err)
								}
							}
						}
					}
					//var arr = []string{}
					//for _, category := range *breadcrumbs {
						//arr = append(arr, category.Name)
						//
						for _, language := range languages {
							if p2 := path.Join(append(append([]string{output}, names...), fmt.Sprintf("_index%s.html", language.Suffix))...); len(p2) > 0 {
								if _, err := os.Stat(p2); err != nil {
									categoryFile := &common.CategoryFile{
										ID:    category.ID,
										Date:  category.UpdatedAt,
										Title: category.Title,
										//Thumbnail: category.Thumbnail,
										Path:    "/" + path.Join(names...),
										Type:    "categories",
										Content: category.Content,
									}
									if common.Config.FlatUrl {
										if len(names) == 1 && names[0] == strings.ToLower(common.Config.Products) {
											categoryFile.Url = "/" + strings.ToLower(common.Config.Products)
										}else{
											categoryFile.Url = "/" + path.Join(names[1:]...) + "/"
											categoryFile.Aliases = append(categoryFile.Aliases,"/" + path.Join(names...) + "/")
										}
									}
									//
									categoryFile.Thumbnail = strings.Join(thumbnails, ",")
									//
									categoryFile.Widgets = append(allWidgets, allCategoriesWidgets...)
									if widgets, err := models.GetWidgetsByCategory(common.Database, category.ID); err == nil {
										for _, widget := range widgets {
											if widget.Enabled {
												categoryFile.Widgets = append(categoryFile.Widgets, common.WidgetCF{
													Name:     widget.Name,
													Title:    widget.Title,
													Content:  widget.Content,
													Location: widget.Location,
													ApplyTo:  widget.ApplyTo,
												})
											}
										}
									}
									//
									if tree, err := models.GetCategoriesView(common.Database, int(category.ID), 999, true, true); err == nil {
										categoryFile.Count = tree.Count
									}else{
										logger.Warningf("%+v", err)
									}
									//
									if err = common.WriteCategoryFile(p2, categoryFile); err != nil {
										logger.Warningf("%v", err)
									}
								}
							}
						}
					//}
					// Cache
					if _, err = models.CreateCacheCategory(common.Database, &models.CacheCategory{
						CategoryID:   category.ID,
						Path:        fmt.Sprintf("/%s/", strings.Join(names[:len(names) - 1], "/")),
						Name:        category.Name,
						Title:       category.Title,
						Thumbnail:   strings.Join(thumbnails, ","),
						Link: fmt.Sprintf("/%s/%s", strings.Join(names[:len(names) - 1], "/"), category.Name),
					}); err != nil {
						logger.Warningf("%v", err)
					}
				}
			}
		}
		// Products
		if products, err := models.GetProducts(common.Database); err == nil {
			// Widgets
			var allProductsWidgets []common.WidgetCF
			if widgets, err := models.GetWidgetsByApplyTo(common.Database, "all-products"); err == nil {
				for _, widget := range widgets {
					if widget.Enabled {
						allProductsWidgets = append(allProductsWidgets, common.WidgetCF{
							Name:     widget.Name,
							Title:    widget.Title,
							Content:  widget.Content,
							Location: widget.Location,
							ApplyTo:  widget.ApplyTo,
						})
					}
				}
			} else {
				logger.Warningf("%+v", err)
			}
			//
			logger.Infof("Products found: %v", len(products))
			for i, product := range products {
				if product.Enabled {
					logger.Infof("[%d] Products ID: %+v Name: %v Title: %v", i, product.ID, product.Name, product.Title)
					product, _ = models.GetProductFull(common.Database, int(product.ID))
					if categories, err := models.GetCategoriesOfProduct(common.Database, product); err == nil {
						var canonical string
						for i, category := range categories {
							breadcrumbs := &[]*models.Category{}
							var f3 func(connector *gorm.DB, id uint)
							f3 = func(connector *gorm.DB, id uint) {
								if id != 0 {
									if category, err := models.GetCategory(common.Database, int(id)); err == nil {
										//*names = append([]string{category.Country}, *names...)
										if category.Thumbnail == "" {
											if len(*breadcrumbs) > 0 {
												category.Thumbnail = (*breadcrumbs)[0].Thumbnail
											} else if product.Image != nil {
												category.Thumbnail = product.Image.Url
											}
										}
										*breadcrumbs = append([]*models.Category{category}, *breadcrumbs...)
										f3(connector, category.ParentId)
									}
								}
							}
							f3(common.Database, category.ID)
							if common.Config.Products != "" {
								*breadcrumbs = append([]*models.Category{{Name: strings.ToLower(common.Config.Products), Title: common.Config.Products, Model: gorm.Model{UpdatedAt: time.Now()}}}, *breadcrumbs...)
							}
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
								productFile := &common.ProductFile{
									ID:         product.ID,
									Date:       time.Now(),
									Title:      product.Title,
									Type:       "products",
									CategoryId: category.ID,
								}
								if i > 0 {
									productFile.Canonical = canonical
								}
								// Related
								rows, err := common.Database.Debug().Table("products_relations").Select("ProductIdL, ProductIdR").Where("ProductIdL = ? or ProductIdR = ?", product.ID, product.ID).Rows()
								if err == nil {
									var ids []uint
									for rows.Next() {
										var p struct {
											ProductIdL uint
											ProductIdR uint
										}
										if err = common.Database.ScanRows(rows, &p); err == nil {
											if p.ProductIdL == product.ID {
												var found bool
												for _, id1 := range ids {
													if id1 == p.ProductIdR {
														found = true
														break
													}
												}
												if !found {
													ids = append(ids, p.ProductIdR)
												}
											} else {
												var found bool
												for _, id1 := range ids {
													if id1 == p.ProductIdL {
														found = true
														break
													}
												}
												if !found {
													ids = append(ids, p.ProductIdL)
												}
											}
										} else {
											logger.Errorf("%v", err)
										}
									}
									rows.Close()
									for _, id := range ids {
										if id < product.ID {
											productFile.Related = append(productFile.Related, fmt.Sprintf("%d-%d", id, product.ID))
										}else{
											productFile.Related = append(productFile.Related, fmt.Sprintf("%d-%d", product.ID, id))
										}
									}
								}
								//
								var arr = []string{}
								for _, category := range *breadcrumbs {
									arr = append(arr, category.Name)
									for _, language := range languages {
										if p2 := path.Join(append(append([]string{output}, arr...), fmt.Sprintf("_index%s.html", language.Suffix))...); len(p2) > 0 {
											// Update category file
											if categoryFile, err := common.ReadCategoryFile(p2); err == nil {
												variations := append([]*models.Variation{{
													BasePrice: product.BasePrice,
													Dimensions: product.Dimensions,
													Width: product.Width,
													Height: product.Height,
													Depth: product.Depth,
													Volume: product.Volume,
													Weight: product.Weight,
													Properties: product.Properties,
												}}, product.Variations...)
												for _, variation := range variations {
													// Rate
													if categoryFile.Price.Max == 0 {
														categoryFile.Price.Max = variation.BasePrice
														if categoryFile.Price.Min == 0 {
															categoryFile.Price.Min = categoryFile.Price.Max
														}
													}
													if categoryFile.Price.Min > variation.BasePrice {
														categoryFile.Price.Min = variation.BasePrice
													}
													if categoryFile.Price.Max < variation.BasePrice {
														categoryFile.Price.Max = variation.BasePrice
													}
													// Width
													if categoryFile.Dimensions.Width.Max == 0 {
														categoryFile.Dimensions.Width.Max = variation.Width
														if categoryFile.Dimensions.Width.Min == 0 {
															categoryFile.Dimensions.Width.Min = categoryFile.Dimensions.Width.Max
														}
													}
													if categoryFile.Dimensions.Width.Min > variation.Width {
														categoryFile.Dimensions.Width.Min = variation.Width
													}
													if categoryFile.Dimensions.Width.Max < variation.Width {
														categoryFile.Dimensions.Width.Max = variation.Width
													}
													// Height
													if categoryFile.Dimensions.Height.Max == 0 {
														categoryFile.Dimensions.Height.Max = variation.Height
														if categoryFile.Dimensions.Height.Min == 0 {
															categoryFile.Dimensions.Height.Min = categoryFile.Dimensions.Height.Max
														}
													}
													if categoryFile.Dimensions.Height.Min > variation.Height {
														categoryFile.Dimensions.Height.Min = variation.Height
													}
													if categoryFile.Dimensions.Height.Max < variation.Height {
														categoryFile.Dimensions.Height.Max = variation.Height
													}
													// Depth
													if categoryFile.Dimensions.Depth.Max == 0 {
														categoryFile.Dimensions.Depth.Max = variation.Depth
														if categoryFile.Dimensions.Depth.Min == 0 {
															categoryFile.Dimensions.Depth.Min = categoryFile.Dimensions.Depth.Max
														}
													}
													if categoryFile.Dimensions.Depth.Min > variation.Depth {
														categoryFile.Dimensions.Depth.Min = variation.Depth
													}
													if categoryFile.Dimensions.Depth.Max < variation.Depth {
														categoryFile.Dimensions.Depth.Max = variation.Depth
													}
													// Volume
													if categoryFile.Volume.Max == 0 {
														categoryFile.Volume.Max = variation.Volume
														if categoryFile.Volume.Min == 0 {
															categoryFile.Volume.Min = categoryFile.Volume.Max
														}
													}
													if categoryFile.Volume.Min > variation.Volume {
														categoryFile.Volume.Min = variation.Volume
													}
													if categoryFile.Volume.Max < variation.Volume {
														categoryFile.Volume.Max = variation.Volume
													}
													// Weight
													if categoryFile.Weight.Max == 0 {
														categoryFile.Weight.Max = variation.Weight
														if categoryFile.Weight.Min == 0 {
															categoryFile.Weight.Min = categoryFile.Weight.Max
														}
													}
													if categoryFile.Weight.Min > variation.Weight {
														categoryFile.Weight.Min = variation.Weight
													}
													if categoryFile.Weight.Max < variation.Weight {
														categoryFile.Weight.Max = variation.Weight
													}
													// Products parameters
													for _, parameter := range product.Parameters {
														if parameter.ID > 0 && parameter.Filtering && parameter.Option != nil && (parameter.Value != nil || parameter.CustomValue != "") {
															var found bool
															for _, opt := range categoryFile.Options {
																if opt.ID == parameter.Option.ID {
																	var found2 bool
																	for _, value := range opt.Values {
																		if value.ID == parameter.Value.ID {
																			found2 = true
																			break
																		}
																	}
																	if !found2 {
																		//
																		var thumbnail string
																		if parameter.Value.Thumbnail != "" {
																			if p1 := path.Join(dir, "storage", parameter.Value.Thumbnail); len(p1) > 0 {
																				if fi, err := os.Stat(p1); err == nil {
																					filename := filepath.Base(p1)
																					filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
																					logger.Infof("Copy %v => %v %v bytes", p1, path.Join("images", "values", filename), fi.Size())
																					if thumbnails, err := common.STORAGE.PutImage(p1, path.Join("images", "values", filename), common.Config.Resize.Thumbnail.Size); err == nil {
																						thumbnail = strings.Join(thumbnails, ",")
																					} else {
																						logger.Warningf("%v", err)
																					}
																				}
																			}
																		}
																		//
																		categoryFile.Options[i].Values = append(categoryFile.Options[i].Values, &common.ValueCF{
																			ID:        parameter.Value.ID,
																			Thumbnail: thumbnail,
																			Title:     parameter.Value.Title,
																			Value:     parameter.Value.Value,
																		})
																		//
																	}
																	found = true
																	break
																}
															}
															if !found {
																opt := &common.OptionCF{
																	ID:    parameter.Option.ID,
																	Type:  "Products",
																	Name:  parameter.Option.Name,
																	Title: parameter.Option.Title,
																}
																var thumbnail string
																if parameter.Value.Thumbnail != "" {
																	if p1 := path.Join(dir, "storage", parameter.Value.Thumbnail); len(p1) > 0 {
																		if fi, err := os.Stat(p1); err == nil {
																			filename := filepath.Base(p1)
																			filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
																			logger.Infof("Copy %v => %v %v bytes", p1, path.Join("images", "values", filename), fi.Size())
																			if thumbnails, err := common.STORAGE.PutImage(p1, path.Join("images", "values", filename), common.Config.Resize.Thumbnail.Size); err == nil {
																				thumbnail = strings.Join(thumbnails, ",")
																			} else {
																				logger.Warningf("%v", err)
																			}
																		}
																	}
																}
																//
																opt.Values = append(opt.Values, &common.ValueCF{
																	ID:        parameter.Value.ID,
																	Thumbnail: thumbnail,
																	Title:     parameter.Value.Title,
																	Value:     parameter.Value.Value,
																})
																//
																categoryFile.Options = append(categoryFile.Options, opt)
															}
														}
													}
													// Properties
													for _, property := range product.Properties {
														if property.Filtering {
															var found bool
															for i, opt := range categoryFile.Options {
																if opt.ID == property.Option.ID {
																	//logger.Infof("TEST001 property ID: %v, Country: %v FOUND", property.ID, property.Country)
																	for _, price := range property.Rates {
																		var found bool
																		for _, value := range opt.Values {
																			if value.ID == price.Value.ID {
																				found = true
																				break
																			}
																		}
																		if !found {
																			//
																			var thumbnail string
																			if price.Value.Thumbnail != "" {
																				if p1 := path.Join(dir, "storage", price.Value.Thumbnail); len(p1) > 0 {
																					if fi, err := os.Stat(p1); err == nil {
																						filename := filepath.Base(p1)
																						filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
																						logger.Infof("Copy %v => %v %v bytes", p1, path.Join("images", "values", filename), fi.Size())
																						if thumbnails, err := common.STORAGE.PutImage(p1, path.Join("images", "values", filename), common.Config.Resize.Thumbnail.Size); err == nil {
																							thumbnail = strings.Join(thumbnails, ",")
																						} else {
																							logger.Warningf("%v", err)
																						}
																					}
																				}
																			}
																			//
																			categoryFile.Options[i].Values = append(categoryFile.Options[i].Values, &common.ValueCF{
																				ID:        price.Value.ID,
																				Thumbnail: thumbnail,
																				Title:     price.Value.Title,
																				Value:     price.Value.Value,
																			})
																			//
																		}
																	}
																	found = true
																}
															}
															if !found {
																//logger.Infof("TEST001 property ID: %v, Country: %v NOT FOUND (%v)", property.ID, property.Country, len(categoryFile.Options))
																opt := &common.OptionCF{
																	ID:    property.Option.ID,
																	Type:  "Variation",
																	Name:  property.Option.Name,
																	Title: property.Option.Title,
																}
																for _, price := range property.Rates {
																	if price.Enabled {
																		//
																		var thumbnail string
																		if price.Value.Thumbnail != "" {
																			if p1 := path.Join(dir, "storage", price.Value.Thumbnail); len(p1) > 0 {
																				if fi, err := os.Stat(p1); err == nil {
																					filename := filepath.Base(p1)
																					filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
																					logger.Infof("Copy %v => %v %v bytes", p1, path.Join("images", "values", filename), fi.Size())
																					if thumbnails, err := common.STORAGE.PutImage(p1, path.Join("images", "values", filename), common.Config.Resize.Thumbnail.Size); err == nil {
																						thumbnail = strings.Join(thumbnails, ",")
																					} else {
																						logger.Warningf("%v", err)
																					}
																				}
																			}
																		}
																		//
																		opt.Values = append(opt.Values, &common.ValueCF{
																			ID:        price.Value.ID,
																			Thumbnail: thumbnail,
																			Title:     price.Value.Title,
																			Value:     price.Value.Value,
																		})
																	}
																}
																//logger.Infof("TEST001 property ID: %v, Country: %v ADD %+v", property.ID, property.Country, opt)
																categoryFile.Options = append(categoryFile.Options, opt)
															}
														}
													}
													// Variation properties
													for _, property := range variation.Properties {
														if property.Filtering {
															var found bool
															for i, opt := range categoryFile.Options {
																if opt.ID == property.Option.ID {
																	//logger.Infof("TEST001 property ID: %v, Country: %v FOUND", property.ID, property.Country)
																	for _, price := range property.Rates {
																		var found bool
																		for _, value := range opt.Values {
																			if value.ID == price.Value.ID {
																				found = true
																				break
																			}
																		}
																		if !found {
																			//
																			var thumbnail string
																			if price.Value.Thumbnail != "" {
																				if p1 := path.Join(dir, "storage", price.Value.Thumbnail); len(p1) > 0 {
																					if fi, err := os.Stat(p1); err == nil {
																						filename := filepath.Base(p1)
																						filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
																						logger.Infof("Copy %v => %v %v bytes", p1, path.Join("images", "values", filename), fi.Size())
																						if thumbnails, err := common.STORAGE.PutImage(p1, path.Join("images", "values", filename), common.Config.Resize.Thumbnail.Size); err == nil {
																							thumbnail = strings.Join(thumbnails, ",")
																						} else {
																							logger.Warningf("%v", err)
																						}
																					}
																				}
																			}
																			//
																			categoryFile.Options[i].Values = append(categoryFile.Options[i].Values, &common.ValueCF{
																				ID:        price.Value.ID,
																				Thumbnail: thumbnail,
																				Title:     price.Value.Title,
																				Value:     price.Value.Value,
																			})
																			//
																		}
																	}
																	found = true
																}
															}
															if !found {
																//logger.Infof("TEST001 property ID: %v, Country: %v NOT FOUND (%v)", property.ID, property.Country, len(categoryFile.Options))
																opt := &common.OptionCF{
																	ID:    property.Option.ID,
																	Type:  "Variation",
																	Name:  property.Option.Name,
																	Title: property.Option.Title,
																}
																for _, price := range property.Rates {
																	if price.Enabled {
																		//
																		var thumbnail string
																		if price.Value.Thumbnail != "" {
																			if p1 := path.Join(dir, "storage", price.Value.Thumbnail); len(p1) > 0 {
																				if fi, err := os.Stat(p1); err == nil {
																					filename := filepath.Base(p1)
																					filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
																					logger.Infof("Copy %v => %v %v bytes", p1, path.Join("images", "values", filename), fi.Size())
																					if thumbnails, err := common.STORAGE.PutImage(p1, path.Join("images", "values", filename), common.Config.Resize.Thumbnail.Size); err == nil {
																						thumbnail = strings.Join(thumbnails, ",")
																					} else {
																						logger.Warningf("%v", err)
																					}
																				}
																			}
																		}
																		//
																		opt.Values = append(opt.Values, &common.ValueCF{
																			ID:        price.Value.ID,
																			Thumbnail: thumbnail,
																			Title:     price.Value.Title,
																			Value:     price.Value.Value,
																		})
																	}
																}
																//logger.Infof("TEST001 property ID: %v, Country: %v ADD %+v", property.ID, property.Country, opt)
																categoryFile.Options = append(categoryFile.Options, opt)
															}
														}
													}
												}
												// Sort to put Products options above Variation options
												sort.Slice(categoryFile.Options, func(i, j int) bool {
													if categoryFile.Options[i].Type == categoryFile.Options[j].Type {
														return categoryFile.Options[i].Title < categoryFile.Options[j].Title
													} else {
														return categoryFile.Options[i].Type < categoryFile.Options[j].Type
													}
												})
												if err = common.WriteCategoryFile(p2, categoryFile); err != nil {
													logger.Warningf("%v", err)
												}
											}
											//
										}
									}
									productFile.Categories = append(productFile.Categories, category.Title)
								}
								productView := common.ProductPF{
									Id:         product.ID,
									CategoryId: category.ID,
									Name:       product.Name,
									Title:      product.Title,
								}
								// Process thumbnail
								//var thumbnails []string
								if product.Image != nil {
									if p1 := path.Join(dir, "storage", product.Image.Path); len(p1) > 0 {
										if fi, err := os.Stat(p1); err == nil {
											name := product.Name + "-thumbnail"
											if len(name) > 32 {
												name = name[:32]
											}
											filename := fmt.Sprintf("%d-%s-%d%v", product.ID, name, fi.ModTime().Unix(), path.Ext(p1))
											//p2 := path.Join(p0, filename)
											logger.Infof("Copy %v => %v %v bytes", p1, path.Join("images", "products", filename), fi.Size())
											if thumbnails, err := common.STORAGE.PutImage(p1, path.Join("images", "products", filename), common.Config.Resize.Image.Size); err == nil {
												productFile.Thumbnail = strings.Join(thumbnails, ",")
												productView.Thumbnail = strings.Join(thumbnails, ",")
											} else {
												logger.Warningf("%v", err)
											}
										}
									}
								}
								// Copy images
								var images []string
								if len(product.Images) > 0 {
									for i, image := range product.Images {
										if image.Path != "" {
											if p1 := path.Join(dir, "storage", image.Path); len(p1) > 0 {
												if fi, err := os.Stat(p1); err == nil {
													name := product.Name + "-" + image.Name
													if len(name) > 32 {
														name = name[:32]
													}
													filename := fmt.Sprintf("%d-%s-%d%v", image.ID, name, fi.ModTime().Unix(), path.Ext(p1))
													logger.Infof("Copy %v => %v %v bytes", p1, path.Join("images", "products", filename), fi.Size())
													if images2, err := common.STORAGE.PutImage(p1, path.Join("images", "products", filename), common.Config.Resize.Image.Size); err == nil {
														// Generate thumbnail
														if i == 0 || product.ImageId == image.ID {
															productFile.Thumbnail = strings.Join(images2, ",")
															productView.Thumbnail = strings.Join(images2, ",")
														}
														//
														images = append(images, strings.Join(images2, ","))
														//
														// Cache
														if _, err = models.CreateCacheImage(common.Database, &models.CacheImage{
															ImageId:   image.ID,
															Name: image.Name,
															Thumbnail: strings.Join(images2, ","),
														}); err != nil {
															logger.Warningf("%v", err)
														}
													} else {
														logger.Warningf("%v", err)
													}

												}
											}
										}
									}
									productView.Images = images
								}
								// Copy files
								var files []common.FilePF
								if len(product.Files) > 0 {
									for _, file := range product.Files {
										if file.Path != "" {
											if p1 := path.Join(dir, "storage", file.Path); len(p1) > 0 {
												if fi, err := os.Stat(p1); err == nil {
													name := product.Name + "-" + file.Name
													if len(name) > 32 {
														name = name[:32]
													}
													filename := fmt.Sprintf("%d-%s-%d%v", file.ID, name, fi.ModTime().Unix(), path.Ext(p1))
													logger.Infof("Copy %v => %v %v bytes", p1, path.Join("files", "products", filename), fi.Size())
													if url, err := common.STORAGE.PutFile(p1, path.Join("files", "products", filename)); err == nil {
														files = append(files, common.FilePF{
															Id:   file.ID,
															Type: file.Type,
															Name: file.Name,
															Path: url,
															Size: file.Size,
														})
													} else {
														logger.Warningf("%v", err)
													}
												}
											}
										}
									}
									productView.Files = files
								}
								//
								if len(product.Parameters) > 0 {
									for _, parameter := range product.Parameters {
										parameterView := common.ParameterPF{
											Id:    parameter.ID,
											Name:  parameter.Name,
											Title: parameter.Title,
										}
										if parameter.Value != nil {
											parameterView.Value = common.ValuePPF{
												Id:    parameter.Value.ID,
												Title: parameter.Value.Title,
												//Thumbnail: "",
												Value: parameter.Value.Value,
												Availability: parameter.Value.Availability,
											}
										} else {
											parameterView.CustomValue = parameter.CustomValue
										}
										productView.Parameters = append(productView.Parameters, parameterView)
									}
								}
								productFile.BasePrice = fmt.Sprintf("%.2f", product.BasePrice)
								//productView.BasePrice = product.BasePrice
								if product.SalePrice > 0 && product.Start.Before(now) && product.End.After(now){
									productFile.SalePrice = fmt.Sprintf("%.2f", product.SalePrice)
									//productView.SalePrice = product.SalePrice
									productFile.Start = &product.Start
									//productView.Start = &product.Start
									productFile.End = &product.End
									//productView.End = &product.End
								}
								//productView.Dimensions = product.Dimensions
								//productView.Weight = product.Weight
								//productView.Availability = product.Availability
								//productView.Sending = product.Sending
								productView.CustomParameters = []common.CustomParameterPF{}
								if product.CustomParameters != "" {
									for _, line := range strings.Split(strings.TrimSpace(product.CustomParameters), "\n"){
										if res := reKV.FindAllStringSubmatch(strings.TrimSpace(line), -1); len(res) > 0 && len(res[0]) > 1 {
											parameter := common.CustomParameterPF{
												Key:   res[0][1],
											}
											if len(res[0]) > 2 {
												parameter.Value = res[0][2]
											}
											productView.CustomParameters = append(productView.CustomParameters, parameter)
										}
									}
								}
								if p := path.Join(p1, product.Name); p != "" {
									if _, err := os.Stat(p); err != nil {
										if err = os.MkdirAll(p, 0755); err != nil {
											logger.Warningf("%v", err)
										}
									}
								}
								var basePriceMin float64
								//
								var variations []string
								variation := &models.Variation{
									ID:           0,
									Name:         "default",
									Title:        "Default",
									Thumbnail:    product.Thumbnail,
									Properties:   product.Properties,
									BasePrice:    product.BasePrice,
									SalePrice:    product.SalePrice,
									Start:        product.Start,
									End:          product.End,
									Prices: product.Prices,
									Dimensions: product.Dimensions,
									Width:        product.Width,
									Height:       product.Height,
									Depth:        product.Depth,
									Volume:       product.Volume,
									Weight:       product.Weight,
									Availability: product.Availability,
									Time:         product.Time,
									Sku:          product.Sku,
									ProductId:    product.ID,
								}
								if product.Variation != "" {
									variation.Title = product.Variation
								}
								variations2 := []*models.Variation{variation}
								for _, variation2 := range product.Variations {
									if variation2.Name != "default" {
										variations2 = append(variations2, variation2)
									}
								}
								product.Variations = variations2
								//
								if len(product.Variations) > 0 {
									productFile.BasePrice = fmt.Sprintf("%.2f", product.Variations[0].BasePrice)
									if product.Variations[0].SalePrice > 0 && product.Variations[0].Start.Before(now) && product.Variations[0].End.After(now){
										productFile.SalePrice = fmt.Sprintf("%.2f", product.Variations[0].SalePrice)
										productFile.Start = &product.Variations[0].Start
										productFile.End = &product.Variations[0].End
									}
									for _, variation := range product.Variations {
										variationView := common.VariationPF{
											Id:    variation.ID,
											Name:  variation.Name,
											Title: variation.Title,
											//Thumbnail:   variation.Thumbnail,
											Description: variation.Description,
											BasePrice:   variation.BasePrice,
											//Dimensions: variation.Dimensions,
											Pattern: variation.Pattern,
											Dimensions: variation.Dimensions,
											Width: variation.Width,
											Height: variation.Height,
											Depth: variation.Depth,
											Volume: variation.Volume,
											Weight: variation.Weight,
											Availability: variation.Availability,
											Sku: variation.Sku,
											Selected:    len(productView.Variations) == 0,
										}
										//
										for _, price := range variation.Prices {
											var ids []uint
											for _, rate := range price.Rates {
												ids = append(ids, rate.ID)
											}
											variationView.Prices = append(variationView.Prices, common.PricePF{
												Ids: ids,
												Price: price.Price,
												Availability: price.Availability,
												Sku: price.Sku,
											})
										}
										//
										if variation.Time != nil {
											variationView.Time = variation.Time.Title
										}
										// Images
										if variationView.Id == 0 {
											variationView.Images = images
										}else if len(variation.Images) > 0 {
											var images []string
											for _, image := range variation.Images {
												if image.Path != "" {
													if p1 := path.Join(dir, "storage", image.Path); len(p1) > 0 {
														if fi, err := os.Stat(p1); err == nil {
															name := product.Name + "-" + variation.Name
															if len(name) > 32 {
																name = name[:32]
															}
															filename := fmt.Sprintf("%d-%s-%d%v", image.ID, name, fi.ModTime().Unix(), path.Ext(p1))
															logger.Infof("Copy %v => %v %v bytes", p1, path.Join("images", "variations", filename), fi.Size())
															if images2, err := common.STORAGE.PutImage(p1, path.Join("images", "variations", filename), common.Config.Resize.Thumbnail.Size); err == nil {
																images = append(images, strings.Join(images2, ","))
															} else{
																logger.Warningf("%v", err)
															}
														}
													}
												}
											}
											variationView.Images = images
											productView.Images = append(productView.Images, images...)
										}
										// Files
										if variationView.Id == 0 {
											variationView.Files = files
										}else{
											// Copy files
											var files []common.FilePF
											if len(variation.Files) > 0 {
												for _, file := range variation.Files {
													if file.Path != "" {
														if p1 := path.Join(dir, "storage", file.Path); len(p1) > 0 {
															if fi, err := os.Stat(p1); err == nil {
																name := product.Name + "-" + file.Name
																if len(name) > 32 {
																	name = name[:32]
																}
																filename := fmt.Sprintf("%d-%s-%d%v", file.ID, name, fi.ModTime().Unix(), path.Ext(p1))
																logger.Infof("Copy %v => %v %v bytes", p1, path.Join("files", "variations", filename), fi.Size())
																if url, err := common.STORAGE.PutFile(p1, path.Join("files", "variations", filename)); err == nil {
																	files = append(files, common.FilePF{
																		Id:   file.ID,
																		Type: file.Type,
																		Name: file.Name,
																		Path: url,
																		Size: file.Size,
																	})
																} else {
																	logger.Warningf("%v", err)
																}
															}
														}
													}
												}
												variationView.Files = files
												productView.Files = append(productView.Files, files...)
											}
										}

										if variation.SalePrice > 0 && variation.Start.Before(now) && variation.End.After(now){
											variationView.SalePrice = variation.SalePrice
											variationView.Start = &variation.Start
											variationView.End = &variation.End
										}

										if basePriceMin > variation.BasePrice || basePriceMin == 0 {
											basePriceMin = variation.BasePrice
										}
										// Thumbnail
										if variation.Thumbnail != "" {
											if p1 := path.Join(dir, "storage", variation.Thumbnail); len(p1) > 0 {
												if fi, err := os.Stat(p1); err == nil {
													filename := filepath.Base(p1)
													filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
													logger.Infof("Copy %v => %v %v bytes", p1, path.Join("images", "variations", filename), fi.Size())
													if thumbnails, err := common.STORAGE.PutImage(p1, path.Join("images", "variations", filename), common.Config.Resize.Thumbnail.Size); err == nil {
														variationView.Thumbnail = strings.Join(thumbnails, ",")
													} else {
														logger.Warningf("%v", err)
													}
												}
											}
										}
										variations = append(variations, strings.Join([]string{fmt.Sprintf("%d", variation.ID), variation.Title}, ","))
										for _, property := range variation.Properties {
											propertyView := common.PropertyPF{
												Id:    property.ID,
												Type:  property.Type,
												Name:  property.Name,
												Title: property.Title,
											}
											for h, price := range property.Rates {
												valueView := common.ValuePF{
													Id:      price.Value.ID,
													Enabled: price.Enabled,
													Title:   price.Value.Title,
													Description: price.Value.Description,
													//Thumbnail: price.Value.Thumbnail,
													Value: price.Value.Value,
													Availability: price.Value.Availability,
													Price: common.RatePF{
														Id:    price.ID,
														Price: price.Price,
														Availability: price.Availability,
														Sku: price.Sku,
													},
													Selected: h == 0,
												}
												// Thumbnail
												if price.Value.Thumbnail != "" {
													if p1 := path.Join(dir, "storage", price.Value.Thumbnail); len(p1) > 0 {
														if fi, err := os.Stat(p1); err == nil {
															filename := filepath.Base(p1)
															filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
															logger.Infof("Copy %v => %v %v bytes", p1, path.Join("images", "values", filename), fi.Size())
															if thumbnails, err := common.STORAGE.PutImage(p1, path.Join("images", "values", filename), common.Config.Resize.Thumbnail.Size); err == nil {
																valueView.Thumbnail = strings.Join(thumbnails, ",")
															} else {
																logger.Warningf("%v", err)
															}
															//
															if !models.HasCacheValueByValueId(common.Database, price.Value.ID) {
																// Cache
																if _, err = models.CreateCacheValue(common.Database, &models.CacheValue{
																	ValueID:   price.Value.ID,
																	Title:     price.Value.Title,
																	Thumbnail: valueView.Thumbnail,
																	Value:     price.Value.Value,
																}); err != nil {
																	logger.Warningf("%v", err)
																}
																if key := fmt.Sprintf("%v", price.Value.ID); key != "" {
																	if !VALUES.Has(key) {
																		VALUES.Set(key, valueView.Thumbnail)
																	}
																}
															}
														}
													}
												}
												propertyView.Values = append(propertyView.Values, valueView)
											}
											if len(propertyView.Values) > 0 {
												variationView.Properties = append(variationView.Properties, propertyView)
											}
										}
										productView.Variations = append(productView.Variations, variationView)
										//variations = append(variations, strings.Join([]string{fmt.Sprintf("%d", variation.ID), variationView.Thumbnail}, ","))
										// Cache
										if _, err = models.CreateCacheVariation(common.Database, &models.CacheVariation{
											VariationID: variation.ID,
											Name:        variation.Name,
											Title:       variation.Title,
											Description: variation.Description,
											Thumbnail:   variationView.Thumbnail,
											BasePrice:   variation.BasePrice,
										}); err != nil {
											logger.Warningf("%v", err)
										}
									}
								}
								productView.Path = "/" + path.Join(append(names, product.Name)...) + "/"
								productView.Pattern = product.Pattern
								productView.Dimensions = product.Dimensions
								productView.Volume = product.Volume
								productView.Weight = product.Weight
								productView.Availability = product.Availability
								if product.Time != nil {
									productView.Time = product.Time.Title
								}
								productFile.Product = productView
								for _, tag := range product.Tags {
									if tag.Enabled {
										productFile.Tags = append(productFile.Tags, tag.Name)
									}
								}
								//productFile.Max = math.Round(product.Max * 100) / 100
								//productFile.Votes = product.Votes
								var max, votes int
								if comments, err := models.GetCommentsByProduct(common.Database, product.ID); err == nil {
									for _, comment := range comments {
										if comment.Enabled {
											commentPF := common.CommentPF{
												Id:    comment.ID,
												Uuid:  comment.Uuid,
												Title: comment.Title,
												Body:  comment.Body,
												Max:   comment.Max,
											}
											if user, err := models.GetUser(common.Database, int(comment.UserId)); err == nil {
												commentPF.Author = fmt.Sprintf("%s %s", user.Name, user.Lastname)
											}
											productFile.Comments = append(productFile.Comments, commentPF)
											max += comment.Max
											votes++
										}
									}
								}
								productFile.Max = math.Round((float64(max) / float64(votes)) * 100) / 100
								productFile.Votes = votes
								productFile.Content = product.Content
								//
								for _, language := range languages {
									file := path.Join(p1, product.Name, fmt.Sprintf("index%s.html", language.Suffix))
									if _, err := os.Stat(path.Dir(file)); err != nil {
										if err = os.MkdirAll(path.Dir(file), 0755); err != nil {
											logger.Errorf("%v", err)
											return
										}
									}
									if common.Config.FlatUrl {
										productFile.Url = "/" + path.Join(append(names[1:], product.Name)...) + "/"
										productFile.Aliases = append(productFile.Aliases, "/" + path.Join(append(names, product.Name)...) + "/")
									}
									productFile.Widgets = append(allWidgets, allProductsWidgets...)
									if widgets, err := models.GetWidgetsByProduct(common.Database, product.ID); err == nil {
										for _, widget := range widgets {
											if widget.Enabled {
												productFile.Widgets = append(productFile.Widgets, common.WidgetCF{
													Name:     widget.Name,
													Title:    widget.Title,
													Content:  widget.Content,
													Location: widget.Location,
													ApplyTo:  widget.ApplyTo,
												})
											}
										}
									}
									productFile.Sku = product.Sku
									// Sort
									if rows, err := common.Database.Debug().Table("categories_products_sort").Select("Value").Where("CategoryId = ? and ProductId = ?", category.ID, product.ID).Rows(); err == nil {
										for rows.Next() {
											var r struct {
												Value int
											}
											if err = common.Database.ScanRows(rows, &r); err == nil {
												productFile.Sort = r.Value
											}
										}
									}
									//
									if err = common.WriteProductFile(file, productFile); err != nil {
										logger.Errorf("%v", err)
									}
								}
								// Cache
								if _, err = models.CreateCacheProduct(common.Database, &models.CacheProduct{
									ProductID:   product.ID,
									Path:        fmt.Sprintf("/%s/", strings.Join(names, "/")),
									Name:        product.Name,
									Title:       product.Title,
									Description: product.Description,
									Thumbnail:   productView.Thumbnail,
									Images:      strings.Join(images, ";"),
									Variations:  strings.Join(variations, ";"),
									CategoryID:  category.ID,
									Price:       basePriceMin,
									Width:       product.Width,
									Height:      product.Height,
									Depth:       product.Depth,
									Volume:      product.Volume,
									Weight:      product.Weight,
								}); err != nil {
									logger.Warningf("%v", err)
								}
							}
						}
					}
				}
			}
		}else{
			logger.Errorf("%v", err)
			return
		}
		//
		// Options
		if options, err := models.GetOptions(common.Database); err == nil {
			if remove {
				if err := os.RemoveAll(path.Join(output, "options")); err != nil {
					logger.Infof("%v", err)
				}
			}
			// Payload
			for _, option := range options {
				if p1 := path.Join(output, "options", option.Name); len(p1) > 0 {
					if _, err := os.Stat(p1); err != nil {
						if err = os.MkdirAll(p1, 0755); err != nil {
							logger.Errorf("%v", err)
						}
					}
					for _, language := range languages {
						if p2 := path.Join(p1, fmt.Sprintf("_index%s.html", language.Suffix)); len(p2) > 0 {
							if _, err := os.Stat(p2); err != nil {
								content := option.Description
								optionFile := &common.OptionFile{
									ID:    option.ID,
									Date:  option.UpdatedAt,
									Title: option.Title,
									Type:    "options",
									Content: content,
								}
								if err = common.WriteOptionFile(p2, optionFile); err != nil {
									logger.Warningf("%v", err)
								}
							}
						}
					}
					//
					if values, err := models.GetValuesByOptionId(common.Database, int(option.ID)); err == nil {
						for _, value := range values {
							if p1 := path.Join(output, "options", option.Name, value.Value); len(p1) > 0 {
								if _, err := os.Stat(p1); err != nil {
									if err = os.MkdirAll(p1, 0755); err != nil {
										logger.Errorf("%v", err)
									}
								}
								for _, language := range languages {
									if p2 := path.Join(p1, fmt.Sprintf("index%s.html", language.Suffix)); len(p2) > 0 {
										if _, err := os.Stat(p2); err != nil {
											valueFile := &common.ValueFile{
												ID:    value.ID,
												Date:  value.UpdatedAt,
												Title: value.Title,
												Description: value.Description,
												Type:    "values",
												Value: value.Value,
											}
											if key := fmt.Sprintf("%v", value.ID); key != "" {
												if v, found := VALUES.Get(key); found {
													valueFile.Thumbnail = v.(string)
												}
											}
											if err = common.WriteValueFile(p2, valueFile); err != nil {
												logger.Warningf("%v", err)
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
		// Transports
		if transports, err := models.GetTransports(common.Database); err == nil {
			for _, transport := range transports {
				if transport.Thumbnail != "" {
					if p1 := path.Join(dir, "storage", transport.Thumbnail); len(p1) > 0 {
						if fi, err := os.Stat(p1); err == nil {
							filename := path.Base(p1)
							/*p2 := path.Join(dir, "hugo", "static", "images", "transports", path.Base(p1))
							logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
							if _, err := os.Stat(path.Dir(p2)); err != nil {
								if err = os.MkdirAll(path.Dir(p2), 0755); err != nil {
									logger.Warningf("%v", err)
								}
							}
							if err = common.Copy(p1, p2); err != nil {
								logger.Warningf("%v", err)
							}*/
							var thumbnails []string
							logger.Infof("Copy %v => %v %v bytes", p1, path.Join("images", "transport", filename), fi.Size())
							if thumbnails, err = common.STORAGE.PutImage(p1, path.Join("images", "categories", filename), common.Config.Resize.Thumbnail.Size); err != nil {
								logger.Warningf("%v", err)
							}
							//
							if _, err = models.CreateCacheTransport(common.Database, &models.CacheTransport{
								TransportID:   transport.ID,
								Name:        transport.Name,
								Title:       transport.Title,
								Thumbnail:   strings.Join(thumbnails, ","),
							}); err != nil {
								logger.Warningf("%v", err)
							}
						}
					}
				}
			}
		}
		// Menu
		if menus, err := models.GetMenus(common.Database); err == nil {
			views := []common.MenuView2{}
			for _, menu := range menus {
				if menu.Enabled {
					view := common.MenuView2{
						Name: menu.Name,
						Title: menu.Title,
						Location: menu.Location,
					}
					//
					root := &common.MenuItemView{}
					createMenu(root, []byte(fmt.Sprintf(`{"Children":%s}`, menu.Description)))
					view.Children = root.Children
					//
					views = append(views, view)
				}
			}
			if bts, err := json.MarshalIndent(views, " ", "   "); err == nil {
				p := path.Join(dir, "hugo", "data")
				if _, err = os.Stat(p); err != nil {
					if err = os.MkdirAll(p, 0755); err != nil {
						logger.Warningf("%+v", err)
					}
				}
				if err = ioutil.WriteFile(path.Join(p, "menus.json"), bts, 0755); err != nil {
					logger.Warningf("%+v", err)
				}
			}
		}
		logger.Infof("Rendered ~ %.3f ms", float64(time.Since(t1).Nanoseconds())/1000000)
	},
}

func createMenu(root *common.MenuItemView, bts []byte) {
	var raw struct {
		Name string
		Data struct {
			Id int
			Type string
			Path string
			Title string
			Thumbnail string
		}
		Children []interface{}
	}
	//
	if err := json.Unmarshal(bts, &raw); err == nil {
		if raw.Data.Type == "category" {
			if categoriesView, err := models.GetCategoriesView(common.Database, raw.Data.Id, 999, true, true); err == nil {
				root.Type = raw.Data.Type
				root.ID = categoriesView.ID
				root.Name = categoriesView.Name
				root.Title = categoriesView.Title
				root.Path = categoriesView.Path
				root.Thumbnail = categoriesView.Thumbnail
				for _, child := range categoriesView.Children {
					root.Children = append(root.Children, child)
				}
			}
		} else if raw.Data.Type == "product" {
			if cache, err := models.GetCacheProductByProductId(common.Database, uint(raw.Data.Id)); err == nil {
				root.Type = raw.Data.Type
				root.Url = fmt.Sprintf("%s%s", cache.Path, cache.Name)
				root.Title = cache.Title
				root.Thumbnail = cache.Thumbnail
			}else{
				logger.Warningf("%+v", err)
			}
		} else if raw.Data.Type == "page" {
			root.Type = raw.Data.Type
			root.Url = raw.Data.Path
			root.Title = raw.Data.Title
		} else if raw.Data.Type == "custom" {
			root.Type = raw.Data.Type
			root.Url = raw.Data.Path
			root.Title = raw.Data.Title
		}
		for _, child := range raw.Children {
			item := &common.MenuItemView{}
			if bts2, err := json.Marshal(child); err == nil {
				createMenu(item, bts2)
			}
			root.Children = append(root.Children, item)
		}
	}
}

func init() {
	RootCmd.AddCommand(renderCmd)
	renderCmd.Flags().StringP("products", "p", "products", "products output folder")
	renderCmd.Flags().BoolP("remove", "r", false, "remove all files during rendering")
}
