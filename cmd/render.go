package cmd

import (
	"fmt"
	"github.com/google/logger"
	"github.com/spf13/cobra"
	cmap "github.com/streamrail/concurrent-map"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/config"
	"github.com/yonnic/goshop/models"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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
		if p := path.Join(dir, "hugo", "assets", "images", "variations"); p != "" {
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
		}
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
		common.Database.Unscoped().Where("ID > ?", 0).Delete(&models.CacheProduct{})
		common.Database.Unscoped().Where("ID > ?", 0).Delete(&models.CacheImage{})
		common.Database.Unscoped().Where("ID > ?", 0).Delete(&models.CacheVariation{})
		common.Database.Unscoped().Where("ID > ?", 0).Delete(&models.CacheValue{})
		// Files
		if files, err := models.GetFiles(common.Database); err == nil {
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
		}
		// Categories
		if categories, err := models.GetCategories(common.Database); err == nil {
			// Clear existing "products" folder
			if common.Config.Products != "" {
				if err := os.RemoveAll(path.Join(output, strings.ToLower(common.Config.Products))); err != nil {
					logger.Infof("%v", err)
				}
			}
			logger.Infof("Categories found: %v", len(categories))
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
					var arr = []string{}
					for _, category := range *breadcrumbs {
						arr = append(arr, category.Name)
						for _, language := range languages {
							if p2 := path.Join(append(append([]string{output}, arr...), fmt.Sprintf("_index%s.html", language.Suffix))...); len(p2) > 0 {
								if _, err := os.Stat(p2); err != nil {
									categoryFile := &common.CategoryFile{
										ID:    category.ID,
										Date:  category.UpdatedAt,
										Title: category.Title,
										//Thumbnail: category.Thumbnail,
										Path:    "/" + path.Join(arr...),
										Type:    "categories",
										Content: category.Content,
									}
									if common.Config.FlatUrl {
										if len(arr) == 1 && arr[0] == strings.ToLower(common.Config.Products) {
											categoryFile.Url = "/" + strings.ToLower(common.Config.Products)
										}else{
											categoryFile.Url = "/" + path.Join(names[1:]...) + "/"
										}
									}
									//
									if category.Thumbnail != "" {
										//p0 := path.Join(p1, product.Country)
										if p1 := path.Join(dir, "storage", category.Thumbnail); len(p1) > 0 {
											if fi, err := os.Stat(p1); err == nil {
												filename := fmt.Sprintf("%d-thumbnail-%d%v", category.ID, fi.ModTime().Unix(), path.Ext(p1))
												//p2 := path.Join(path.Dir(p2), filename)
												p2 := path.Join(dir, "hugo", "static", "images", "categories", filename)
												logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
												if _, err := os.Stat(path.Dir(p2)); err != nil {
													if err = os.MkdirAll(path.Dir(p2), 0755); err != nil {
														logger.Warningf("%v", err)
													}
												}
												if err = common.Copy(p1, p2); err == nil {
													//thumbnails = append(thumbnails, fmt.Sprintf("/%s/%s", strings.Join(append(names, product.Country), "/"), filename))
													//thumbnails := []string{fmt.Sprintf("/%s/%s", strings.Join(names, "/"), filename)}
													thumbnails := []string{fmt.Sprintf("/%s/%s", strings.Join([]string{"images", "categories"}, "/"), filename)}
													if common.Config.Resize.Enabled && common.Config.Resize.Thumbnail.Enabled {
														if images, err := common.ImageResize(p2, common.Config.Resize.Thumbnail.Size); err == nil {
															for _, image := range images {
																//thumbnails = append(thumbnails, fmt.Sprintf("/%s/resize/%s %s", strings.Join(names, "/"), image.Filename, image.Size))
																thumbnails = append(thumbnails, fmt.Sprintf("/%s/resize/%s %s", strings.Join([]string{"images", "categories"}, "/"), image.Filename, image.Size))
															}
														} else {
															logger.Warningf("%v", err)
														}
													}
													categoryFile.Thumbnail = strings.Join(thumbnails, ",")
												} else {
													logger.Warningf("%v", err)
												}
											}
										}
									}
									//
									if err = common.WriteCategoryFile(p2, categoryFile); err != nil {
										logger.Warningf("%v", err)
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
								view := &common.ProductFile{
									ID:         product.ID,
									Date:       time.Now(),
									Title:      product.Title,
									Type:       "products",
									CategoryId: category.ID,
								}
								if i > 0 {
									view.Canonical = canonical
								}
								//
								//
								var arr = []string{}
								for _, category := range *breadcrumbs {
									arr = append(arr, category.Name)
									for _, language := range languages {
										if p2 := path.Join(append(append([]string{output}, arr...), fmt.Sprintf("_index%s.html", language.Suffix))...); len(p2) > 0 {
											// Update category file
											if categoryFile, err := common.ReadCategoryFile(p2); err == nil {
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
													// Price
													if categoryFile.Price.Min > variation.BasePrice || categoryFile.Price.Min == 0 {
														categoryFile.Price.Min = variation.BasePrice
													}
													if categoryFile.Price.Max < variation.BasePrice {
														categoryFile.Price.Max = variation.BasePrice
													}
													// Width
													if categoryFile.Dimensions.Width.Min > variation.Width || categoryFile.Dimensions.Width.Min == 0 {
														categoryFile.Dimensions.Width.Min = variation.Width
													}
													if categoryFile.Dimensions.Width.Max < variation.Width {
														categoryFile.Dimensions.Width.Max = variation.Width
													}
													// Height
													if categoryFile.Dimensions.Height.Min > variation.Height || categoryFile.Dimensions.Height.Min == 0 {
														categoryFile.Dimensions.Height.Min = variation.Height
													}
													if categoryFile.Dimensions.Height.Max < variation.Height {
														categoryFile.Dimensions.Height.Max = variation.Height
													}
													// Depth
													if categoryFile.Dimensions.Depth.Min > variation.Depth || categoryFile.Dimensions.Depth.Min == 0 {
														categoryFile.Dimensions.Depth.Min = variation.Depth
													}
													if categoryFile.Dimensions.Depth.Max < variation.Depth {
														categoryFile.Dimensions.Depth.Max = variation.Depth
													}
													// Weight
													if categoryFile.Weight.Min > variation.Weight || categoryFile.Weight.Min == 0 {
														categoryFile.Weight.Min = variation.Weight
													}
													if categoryFile.Weight.Max < variation.Weight {
														categoryFile.Weight.Max = variation.Weight
													}
													// Products parameters
													for _, parameter := range product.Parameters {
														if parameter.ID > 0 && parameter.Filtering && parameter.Option != nil {
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
																					p2 := path.Join(dir, "hugo", "assets", "images", "values", filename)
																					thumbnail = "/" + path.Join("images", "values", filename)
																					if _, err = os.Stat(p2); err != nil {
																						logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
																						if err = common.Copy(p1, p2); err != nil {
																							logger.Warningf("%v", err)
																						}
																					}
																					if common.Config.Resize.Enabled && common.Config.Resize.Thumbnail.Enabled {
																						p2 := path.Join(dir, "hugo", "static", "images", "values", filename)
																						if _, err = os.Stat(p2); err != nil {
																							logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
																							if err = common.Copy(p1, p2); err != nil {
																								logger.Warningf("%v", err)
																							}
																						}
																						if images, err := common.ImageResize(p2, common.Config.Resize.Thumbnail.Size); err == nil {
																							thumbnails := []string{fmt.Sprintf("/images/values/resize/%s", filename)}
																							for _, image := range images {
																								thumbnails = append(thumbnails, fmt.Sprintf("/images/values/resize/%s %s", image.Filename, image.Size))
																							}
																							thumbnail = strings.Join(thumbnails, ",")
																						} else {
																							logger.Warningf("%v", err)
																						}
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
																			p2 := path.Join(dir, "hugo", "assets", "images", "values", filename)
																			thumbnail = "/" + path.Join("images", "values", filename)
																			if _, err = os.Stat(p2); err != nil {
																				logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
																				if err = common.Copy(p1, p2); err != nil {
																					logger.Warningf("%v", err)
																				}
																			}
																			if common.Config.Resize.Enabled && common.Config.Resize.Thumbnail.Enabled {
																				p2 := path.Join(dir, "hugo", "static", "images", "values", filename)
																				if _, err = os.Stat(p2); err != nil {
																					logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
																					if err = common.Copy(p1, p2); err != nil {
																						logger.Warningf("%v", err)
																					}
																				}
																				if images, err := common.ImageResize(p2, common.Config.Resize.Thumbnail.Size); err == nil {
																					thumbnails := []string{fmt.Sprintf("/images/values/resize/%s", filename)}
																					for _, image := range images {
																						//thumbnail = "/" + path.Join("images", "values", "resize", filename)
																						//thumbnail = fmt.Sprintf("/images/values/resize/%s %s", image.Filename, image.Size)
																						thumbnails = append(thumbnails, fmt.Sprintf("/images/values/resize/%s %s", image.Filename, image.Size))
																						//break
																					}
																					thumbnail = strings.Join(thumbnails, ",")
																				} else {
																					logger.Warningf("%v", err)
																				}
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
																	for _, price := range property.Prices {
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
																						p2 := path.Join(dir, "hugo", "assets", "images", "values", filename)
																						thumbnail = "/" + path.Join("images", "values", filename)
																						if _, err = os.Stat(p2); err != nil {
																							logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
																							if err = common.Copy(p1, p2); err != nil {
																								logger.Warningf("%v", err)
																							}
																						}
																						if common.Config.Resize.Enabled && common.Config.Resize.Thumbnail.Enabled {
																							p2 := path.Join(dir, "hugo", "static", "images", "values", filename)
																							if _, err = os.Stat(p2); err != nil {
																								logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
																								if err = common.Copy(p1, p2); err != nil {
																									logger.Warningf("%v", err)
																								}
																							}
																							if images, err := common.ImageResize(p2, common.Config.Resize.Thumbnail.Size); err == nil {
																								thumbnails := []string{fmt.Sprintf("/images/values/resize/%s", filename)}
																								for _, image := range images {
																									//thumbnail = "/" + path.Join("images", "values", "resize", filename)
																									//thumbnail = fmt.Sprintf("/images/values/resize/%s %s", image.Filename, image.Size)
																									thumbnails = append(thumbnails, fmt.Sprintf("/images/values/resize/%s %s", image.Filename, image.Size))
																									//break
																								}
																								thumbnail = strings.Join(thumbnails, ",")
																							} else {
																								logger.Warningf("%v", err)
																							}
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
																for _, price := range property.Prices {
																	if price.Enabled {
																		//
																		var thumbnail string
																		if price.Value.Thumbnail != "" {
																			if p1 := path.Join(dir, "storage", price.Value.Thumbnail); len(p1) > 0 {
																				if fi, err := os.Stat(p1); err == nil {
																					filename := filepath.Base(p1)
																					filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
																					p2 := path.Join(dir, "hugo", "assets", "images", "values", filename)
																					thumbnail = "/" + path.Join("images", "values", filename)
																					if _, err = os.Stat(p2); err != nil {
																						logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
																						if err = common.Copy(p1, p2); err != nil {
																							logger.Warningf("%v", err)
																						}
																					}
																					if common.Config.Resize.Enabled && common.Config.Resize.Thumbnail.Enabled {
																						p2 := path.Join(dir, "hugo", "static", "images", "values", filename)
																						if _, err = os.Stat(p2); err != nil {
																							logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
																							if err = common.Copy(p1, p2); err != nil {
																								logger.Warningf("%v", err)
																							}
																						}
																						if images, err := common.ImageResize(p2, common.Config.Resize.Thumbnail.Size); err == nil {
																							thumbnails := []string{fmt.Sprintf("/images/values/resize/%s", filename)}
																							for _, image := range images {
																								//thumbnail = "/" + path.Join("images", "values", "resize", filename)
																								//thumbnail = fmt.Sprintf("/images/values/resize/%s %s", image.Filename, image.Size)
																								thumbnails = append(thumbnails, fmt.Sprintf("/images/values/resize/%s %s", image.Filename, image.Size))
																								//break
																							}
																							thumbnail = strings.Join(thumbnails, ",")
																						} else {
																							logger.Warningf("%v", err)
																						}
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
																	for _, price := range property.Prices {
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
																						p2 := path.Join(dir, "hugo", "assets", "images", "values", filename)
																						thumbnail = "/" + path.Join("images", "values", filename)
																						if _, err = os.Stat(p2); err != nil {
																							logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
																							if err = common.Copy(p1, p2); err != nil {
																								logger.Warningf("%v", err)
																							}
																						}
																						if common.Config.Resize.Enabled && common.Config.Resize.Thumbnail.Enabled {
																							p2 := path.Join(dir, "hugo", "static", "images", "values", filename)
																							if _, err = os.Stat(p2); err != nil {
																								logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
																								if err = common.Copy(p1, p2); err != nil {
																									logger.Warningf("%v", err)
																								}
																							}
																							if images, err := common.ImageResize(p2, common.Config.Resize.Thumbnail.Size); err == nil {
																								thumbnails := []string{fmt.Sprintf("/images/values/resize/%s", filename)}
																								for _, image := range images {
																									//thumbnail = "/" + path.Join("images", "values", "resize", filename)
																									//thumbnail = fmt.Sprintf("/images/values/resize/%s %s", image.Filename, image.Size)
																									thumbnails = append(thumbnails, fmt.Sprintf("/images/values/resize/%s %s", image.Filename, image.Size))
																									//break
																								}
																								thumbnail = strings.Join(thumbnails, ",")
																							} else {
																								logger.Warningf("%v", err)
																							}
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
																for _, price := range property.Prices {
																	if price.Enabled {
																		//
																		var thumbnail string
																		if price.Value.Thumbnail != "" {
																			if p1 := path.Join(dir, "storage", price.Value.Thumbnail); len(p1) > 0 {
																				if fi, err := os.Stat(p1); err == nil {
																					filename := filepath.Base(p1)
																					filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
																					p2 := path.Join(dir, "hugo", "assets", "images", "values", filename)
																					thumbnail = "/" + path.Join("images", "values", filename)
																					if _, err = os.Stat(p2); err != nil {
																						logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
																						if err = common.Copy(p1, p2); err != nil {
																							logger.Warningf("%v", err)
																						}
																					}
																					if common.Config.Resize.Enabled && common.Config.Resize.Thumbnail.Enabled {
																						p2 := path.Join(dir, "hugo", "static", "images", "values", filename)
																						if _, err = os.Stat(p2); err != nil {
																							logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
																							if err = common.Copy(p1, p2); err != nil {
																								logger.Warningf("%v", err)
																							}
																						}
																						if images, err := common.ImageResize(p2, common.Config.Resize.Thumbnail.Size); err == nil {
																							thumbnails := []string{fmt.Sprintf("/images/values/resize/%s", filename)}
																							for _, image := range images {
																								//thumbnail = "/" + path.Join("images", "values", "resize", filename)
																								//thumbnail = fmt.Sprintf("/images/values/resize/%s %s", image.Filename, image.Size)
																								thumbnails = append(thumbnails, fmt.Sprintf("/images/values/resize/%s %s", image.Filename, image.Size))
																								//break
																							}
																							thumbnail = strings.Join(thumbnails, ",")
																						} else {
																							logger.Warningf("%v", err)
																						}
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
												categoryFile.BasePriceMin = min
												categoryFile.BasePriceMax = max
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
									view.Categories = append(view.Categories, category.Title)
								}
								productView := common.ProductPF{
									Id:         product.ID,
									CategoryId: category.ID,
									Name:       product.Name,
									Title:      product.Title,
								}
								// Process thumbnail
								var thumbnails []string
								if product.Image != nil {
									if p1 := path.Join(dir, "storage", product.Image.Path); len(p1) > 0 {
										if fi, err := os.Stat(p1); err == nil {
											filename := fmt.Sprintf("%d-thumbnail-%d%v", product.ID, fi.ModTime().Unix(), path.Ext(p1))
											//p2 := path.Join(p0, filename)
											p2 := path.Join(dir, "hugo", "static", "images", "products", filename)
											logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
											if _, err := os.Stat(path.Dir(p2)); err != nil {
												if err = os.MkdirAll(path.Dir(p2), 0755); err != nil {
													logger.Warningf("%v", err)
												}
											}
											if err = common.Copy(p1, p2); err == nil {
												//thumbnails = append(thumbnails, fmt.Sprintf("/%s/%s", strings.Join(append(names, product.Name), "/"), filename))
												thumbnails = append(thumbnails, fmt.Sprintf("/%s/%s", strings.Join([]string{"images", "products"}, "/"), filename))
												if common.Config.Resize.Enabled && common.Config.Resize.Thumbnail.Enabled {
													if images, err := common.ImageResize(p2, common.Config.Resize.Thumbnail.Size); err == nil {
														for _, image := range images {
															thumbnails = append(thumbnails, fmt.Sprintf("/%s/resize/%s %s", strings.Join([]string{"images", "products"}, "/"), image.Filename, image.Size))
														}
													} else {
														logger.Warningf("%v", err)
													}
												}
												view.Thumbnail = strings.Join(thumbnails, ",")
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
													filename := fmt.Sprintf("%d-image-%d%v", image.ID, fi.ModTime().Unix(), path.Ext(p1))
													// p2 := path.Join(p0, filename)
													p2 := path.Join(dir, "hugo", "static", "images", "products", filename)
													logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
													if _, err := os.Stat(path.Dir(p2)); err != nil {
														if err = os.MkdirAll(path.Dir(p2), 0755); err != nil {
															logger.Warningf("%v", err)
														}
													}
													if err = common.Copy(p1, p2); err == nil {
														images2 := []string{fmt.Sprintf("/%s/%s", strings.Join([]string{"images", "products"}, "/"), filename)}
														if common.Config.Resize.Enabled && common.Config.Resize.Image.Enabled {
															if images, err := common.ImageResize(p2, common.Config.Resize.Image.Size); err == nil {
																for _, image := range images {
																	images2 = append(images2, fmt.Sprintf("/%s/resize/%s %s", strings.Join([]string{"images", "products"}, "/"), image.Filename, image.Size))
																}
															} else {
																logger.Warningf("%v", err)
															}
														}
														// Generate thumbnail
														if i == 0 || product.ImageId == image.ID {
															view.Thumbnail = strings.Join(images2, ",")
															productView.Thumbnail = strings.Join(images2, ",")
														}
														//
														images = append(images, strings.Join(images2, ","))
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
													filename := fmt.Sprintf("%d-file-%d%v", file.ID, fi.ModTime().Unix(), path.Ext(p1))
													p2 := path.Join(dir, "hugo", "static", "files", "products", filename)
													logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
													if _, err := os.Stat(path.Dir(p2)); err != nil {
														if err = os.MkdirAll(path.Dir(p2), 0755); err != nil {
															logger.Warningf("%v", err)
														}
													}
													if err = common.Copy(p1, p2); err == nil {
														files = append(files, common.FilePF{
															Id:   file.ID,
															Type: file.Type,
															Name: file.Name,
															Path: file.Path,
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
												Sending: parameter.Value.Sending,
											}
										} else {
											parameterView.CustomValue = parameter.CustomValue
										}
										productView.Parameters = append(productView.Parameters, parameterView)
									}
								}
								view.BasePrice = fmt.Sprintf("%.2f", product.BasePrice)
								//productView.BasePrice = product.BasePrice
								if product.SalePrice > 0 && product.Start.Before(now) && product.End.After(now){
									view.SalePrice = fmt.Sprintf("%.2f", product.SalePrice)
									//productView.SalePrice = product.SalePrice
									view.Start = &product.Start
									//productView.Start = &product.Start
									view.End = &product.End
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
									Width:        product.Width,
									Height:       product.Height,
									Depth:        product.Depth,
									Weight:       product.Weight,
									Availability: product.Availability,
									Sending:      product.Sending,
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
									view.BasePrice = fmt.Sprintf("%.2f", product.Variations[0].BasePrice)
									if product.Variations[0].SalePrice > 0 && product.Variations[0].Start.Before(now) && product.Variations[0].End.After(now){
										view.SalePrice = fmt.Sprintf("%.2f", product.Variations[0].SalePrice)
										view.Start = &product.Variations[0].Start
										view.End = &product.Variations[0].End
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
											Width: variation.Width,
											Height: variation.Height,
											Depth: variation.Depth,
											Weight: variation.Weight,
											Availability: variation.Availability,
											Sending: variation.Sending,
											Selected:    len(productView.Variations) == 0,
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
															filename := fmt.Sprintf("%d-image-%d%v", image.ID, fi.ModTime().Unix(), path.Ext(p1))
															p2 := path.Join(dir, "hugo", "static", "images", "variations", filename)
															logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
															if _, err := os.Stat(path.Dir(p2)); err != nil {
																if err = os.MkdirAll(path.Dir(p2), 0755); err != nil {
																	logger.Warningf("%v", err)
																}
															}
															if err = common.Copy(p1, p2); err == nil {
																images2 := []string{fmt.Sprintf("/%s/%s", strings.Join([]string{"images", "variations"}, "/"), filename)}
																if common.Config.Resize.Enabled && common.Config.Resize.Image.Enabled {
																	if images, err := common.ImageResize(p2, common.Config.Resize.Image.Size); err == nil {
																		for _, image := range images {
																			images2 = append(images2, fmt.Sprintf("/%s/resize/%s %s", strings.Join([]string{"images", "variations"}, "/"), image.Filename, image.Size))
																		}
																	} else {
																		logger.Warningf("%v", err)
																	}
																}
																images = append(images, strings.Join(images2, ","))
															} else {
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
																filename := fmt.Sprintf("%d-file-%d%v", file.ID, fi.ModTime().Unix(), path.Ext(p1))
																p2 := path.Join(dir, "hugo", "static", "files", "variations", filename)
																logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
																if _, err := os.Stat(path.Dir(p2)); err != nil {
																	if err = os.MkdirAll(path.Dir(p2), 0755); err != nil {
																		logger.Warningf("%v", err)
																	}
																}
																if err = common.Copy(p1, p2); err == nil {
																	files = append(files, common.FilePF{
																		Id:   file.ID,
																		Type: file.Type,
																		Name: file.Name,
																		Path: file.Path,
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
													p2 := path.Join(dir, "hugo", "static", "images", "variations", filename)
													//
													if _, err = os.Stat(path.Dir(p2)); err != nil {
														if err = os.MkdirAll(path.Dir(p2), 0755); err != nil {
															logger.Warningf("%v", err)
														}
													}
													//
													logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
													if err = common.Copy(p1, p2); err == nil {
														thumbnails := []string{fmt.Sprintf("/images/variations/%s", filename)}
														if common.Config.Resize.Enabled && common.Config.Resize.Image.Enabled {
															if images, err := common.ImageResize(p2, common.Config.Resize.Image.Size); err == nil {
																for _, image := range images {
																	thumbnails = append(thumbnails, fmt.Sprintf("/images/variations/resize/%s %s", image.Filename, image.Size))
																}
															} else {
																logger.Warningf("%v", err)
															}
														}
														variationView.Thumbnail = strings.Join(thumbnails, ",")
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
											for h, price := range property.Prices {
												valueView := common.ValuePF{
													Id:      price.Value.ID,
													Enabled: price.Enabled,
													Title:   price.Value.Title,
													//Thumbnail: price.Value.Thumbnail,
													Value: price.Value.Value,
													Availability: price.Value.Availability,
													Sending: price.Value.Sending,
													Price: common.PricePF{
														Id:    price.ID,
														Price: price.Price,
														Availability: price.Availability,
														Sending: price.Sending,
													},
													Selected: h == 0,
												}
												// Thumbnail
												if price.Value.Thumbnail != "" {
													if p1 := path.Join(dir, "storage", price.Value.Thumbnail); len(p1) > 0 {
														if fi, err := os.Stat(p1); err == nil {
															filename := filepath.Base(p1)
															filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
															thumbnails := []string{fmt.Sprintf("/images/values/%s", filename)}
															if common.Config.Resize.Enabled && common.Config.Resize.Image.Enabled {
																p2 := path.Join(dir, "hugo", "static", "images", "values", filename)
																if _, err = os.Stat(p2); err != nil {
																	logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
																	if err = common.Copy(p1, p2); err != nil {
																		logger.Warningf("%v", err)
																	}
																}
																if images, err := common.ImageResize(p2, common.Config.Resize.Image.Size); err == nil {
																	for _, image := range images {
																		thumbnails = append(thumbnails, fmt.Sprintf("/images/values/resize/%s %s", image.Filename, image.Size))
																	}
																} else {
																	logger.Warningf("%v", err)
																}
															}
															valueView.Thumbnail = strings.Join(thumbnails, ",")
															//
															if !models.HasCacheVariationByValueId(common.Database, price.Value.ID) {
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
											variationView.Properties = append(variationView.Properties, propertyView)
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
								view.Product = productView
								for _, tag := range product.Tags {
									if tag.Enabled {
										view.Tags = append(view.Tags, tag.Name)
									}
								}
								view.Content = product.Content
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
										view.Url = "/" + path.Join(append(names[1:], product.Name)...) + "/"
									}
									if err = common.WriteProductFile(file, view); err != nil {
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
									Thumbnail:   strings.Join(thumbnails, ","),
									Images:      strings.Join(images, ";"),
									Variations:  strings.Join(variations, ";"),
									CategoryID:  category.ID,
									Price:       basePriceMin,
									Width:       product.Width,
									Height:      product.Height,
									Depth:       product.Depth,
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
							p2 := path.Join(dir, "hugo", "static", "images", "transports", path.Base(p1))
							logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
							if _, err := os.Stat(path.Dir(p2)); err != nil {
								if err = os.MkdirAll(path.Dir(p2), 0755); err != nil {
									logger.Warningf("%v", err)
								}
							}
							if err = common.Copy(p1, p2); err != nil {
								logger.Warningf("%v", err)
							}
						}
					}
				}
			}
		}
		logger.Infof("Rendered ~ %.3f ms", float64(time.Since(t1).Nanoseconds())/1000000)
	},
}

func init() {
	RootCmd.AddCommand(renderCmd)
	renderCmd.Flags().StringP("products", "p", "products", "products output folder")
	renderCmd.Flags().BoolP("remove", "r", false, "remove all files during rendering")
}

/**/

type CategoryView struct {
	ID uint
	Date time.Time
	Title string
	Thumbnail string
	Path string
	Type string
}

/*type PageView struct {
	ID uint
	Type       string
	ZIP      string
	Date       time.Time
	Tags       []string
	Canonical  string
	Categories []string
	CategoryId  uint
	Images     []string
	Thumbnail  string
	BasePrice  string
	Products    ProductView
}

type ProductView struct {
	Id         uint `json:"Id"`
	CategoryId uint
	Country       string
	ZIP      string
	Thumbnail  string
	Path       string
	Variations []VariationView
}

type VariationView struct {
	Id uint
	Country string
	ZIP string
	Thumbnail string
	Description string
	BasePrice float64
	Properties []PropertyView
	Selected bool
}

type PropertyView struct {
	Id uint
	Type string
	Country string
	ZIP string
	Description string
	Values []ValueView
}

type ValueView struct {
	Id uint
	Enabled bool
	ZIP string
	Thumbnail string
	Value string
	Price PriceView
	Selected bool
}

type PriceView struct {
	Id uint
	Price float64
}*/
