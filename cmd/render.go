package cmd

import (
	"fmt"
	"github.com/google/logger"
	"github.com/nfnt/resize"
	"github.com/spf13/cobra"
	cmap "github.com/streamrail/concurrent-map"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/config"
	"github.com/yonnic/goshop/models"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"image/jpeg"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	VALUES = cmap.New()
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
		if _, err := common.Database.DB(); err != nil {
			logger.Fatalf("%v", err)
		}
		//
		logger.Infof("Configure Hugo Theme index")
		if p := path.Join(dir, "hugo", "themes", common.Config.Hugo.Theme, "layouts", "partials", "scripts.html"); len(p) > 0 {
			if _, err = os.Stat(p); err == nil {
				logger.Infof("p: %+v", p)
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
									// Copy image
									/*if category.Thumbnail != "" {
										if p1 := path.Join(dir, category.Thumbnail); len(p1) > 0 {
											if fi, err := os.Stat(p1); err == nil {
												p2 := path.Join(path.Dir(p2), fmt.Sprintf("thumbnail-%d%v", fi.ModTime().Unix(), path.Ext(p1)))
												logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
												if err = common.Copy((p1, p2); err != nil {
													logger.Warningf("%v", err)
												}
												//
												if common.Config.Resize.Enabled && common.Config.Resize.Thumbnail.Enabled {
													if images, err := imageResize(p2, common.Config.Resize.Thumbnail.Size); err == nil {
														thumbnails := []string{category.Thumbnail}
														for _, image := range images {
															thumbnails = append(thumbnails, fmt.Sprintf("/%s/resize/%s %s", path.Dir(category.Thumbnail), image.Filename, image.Size))
														}
														categoryFile.Thumbnail = strings.Join(thumbnails, ",")
													} else {
														logger.Warningf("%v", err)
													}
												}
											}
										}
									}*/
									//
									if category.Thumbnail != "" {
										//p0 := path.Join(p1, product.Country)
										if p1 := path.Join(dir, category.Thumbnail); len(p1) > 0 {
											if fi, err := os.Stat(p1); err == nil {
												filename := fmt.Sprintf("thumbnail-%d%v", fi.ModTime().Unix(), path.Ext(p1))
												p2 := path.Join(path.Dir(p2), filename)
												logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
												if _, err := os.Stat(path.Dir(p2)); err != nil {
													if err = os.MkdirAll(path.Dir(p2), 0755); err != nil {
														logger.Warningf("%v", err)
													}
												}
												if err = common.Copy(p1, p2); err == nil {
													//thumbnails = append(thumbnails, fmt.Sprintf("/%s/%s", strings.Join(append(names, product.Country), "/"), filename))
													thumbnails := []string{fmt.Sprintf("/%s/%s", strings.Join(names, "/"), filename)}
													if common.Config.Resize.Enabled && common.Config.Resize.Thumbnail.Enabled {
														if images, err := imageResize(p2, common.Config.Resize.Thumbnail.Size); err == nil {
															for _, image := range images {
																thumbnails = append(thumbnails, fmt.Sprintf("/%s/resize/%s %s", strings.Join(names, "/"), image.Filename, image.Size))
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
										//*names = append([]string{category.Country}, *names...)
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
													// Product parameters
													for _, parameter := range product.Parameters {
														if parameter.ID > 0 && parameter.Filtering {
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
																			if p1 := path.Join(dir, parameter.Value.Thumbnail); len(p1) > 0 {
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
																						if images, err := imageResize(p2, common.Config.Resize.Thumbnail.Size); err == nil {
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
																	Type:  "Product",
																	Name:  parameter.Option.Name,
																	Title: parameter.Option.Title,
																}
																var thumbnail string
																if parameter.Value.Thumbnail != "" {
																	if p1 := path.Join(dir, parameter.Value.Thumbnail); len(p1) > 0 {
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
																				if images, err := imageResize(p2, common.Config.Resize.Thumbnail.Size); err == nil {
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
																				if p1 := path.Join(dir, price.Value.Thumbnail); len(p1) > 0 {
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
																							if images, err := imageResize(p2, common.Config.Resize.Thumbnail.Size); err == nil {
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
																			if p1 := path.Join(dir, price.Value.Thumbnail); len(p1) > 0 {
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
																						if images, err := imageResize(p2, common.Config.Resize.Thumbnail.Size); err == nil {
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
												// Sort to put Product options above Variation options
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
								if product.Thumbnail != "" {
									p0 := path.Join(p1, product.Name)
									if p1 := path.Join(dir, product.Thumbnail); len(p1) > 0 {
										if fi, err := os.Stat(p1); err == nil {
											filename := fmt.Sprintf("thumbnail-%d%v", fi.ModTime().Unix(), path.Ext(p1))
											p2 := path.Join(p0, filename)
											logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
											if _, err := os.Stat(path.Dir(p2)); err != nil {
												if err = os.MkdirAll(path.Dir(p2), 0755); err != nil {
													logger.Warningf("%v", err)
												}
											}
											if err = common.Copy(p1, p2); err == nil {
												thumbnails = append(thumbnails, fmt.Sprintf("/%s/%s", strings.Join(append(names, product.Name), "/"), filename))
												if common.Config.Resize.Enabled && common.Config.Resize.Thumbnail.Enabled {
													if images, err := imageResize(p2, common.Config.Resize.Thumbnail.Size); err == nil {
														for _, image := range images {
															thumbnails = append(thumbnails, fmt.Sprintf("/%s/resize/%s %s", strings.Join(append(names, product.Name), "/"), image.Filename, image.Size))
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
											p0 := path.Join(p1, product.Name)
											if _, err = os.Stat(p0); err != nil {
												if err = os.MkdirAll(p0, 0755); err != nil {
													logger.Warningf("%v", err)
												}
											}
											if p1 := path.Join(dir, image.Path); len(p1) > 0 {
												if fi, err := os.Stat(p1); err == nil {
													filename := fmt.Sprintf("image-%d-%d%v", i+1, fi.ModTime().Unix(), path.Ext(p1))
													p2 := path.Join(p0, filename)
													logger.Infof("Copy %v => %v %v bytes", p1, p2, fi.Size())
													if _, err := os.Stat(path.Dir(p2)); err != nil {
														if err = os.MkdirAll(path.Dir(p2), 0755); err != nil {
															logger.Warningf("%v", err)
														}
													}
													if err = common.Copy(p1, p2); err == nil {
														images2 := []string{fmt.Sprintf("/%s/%s", strings.Join(append(names, product.Name), "/"), filename)}
														if common.Config.Resize.Enabled && common.Config.Resize.Image.Enabled {
															if images, err := imageResize(p2, common.Config.Resize.Image.Size); err == nil {
																for _, image := range images {
																	images2 = append(images2, fmt.Sprintf("/%s/%s %s", strings.Join(append(names, product.Name, "resize"), "/"), image.Filename, image.Size))
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
									productView.Images = images
								}

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
											}
										} else {
											parameterView.CustomValue = parameter.CustomValue
										}
										productView.Parameters = append(productView.Parameters, parameterView)
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
								var variations []string
								if len(product.Variations) > 0 {
									view.BasePrice = fmt.Sprintf("%.2f", product.Variations[0].BasePrice)
									for i, variation := range product.Variations {
										variationView := common.VariationPF{
											Id:    variation.ID,
											Name:  variation.Name,
											Title: variation.Title,
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
															if images, err := imageResize(p2, common.Config.Resize.Image.Size); err == nil {
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
													Price: common.PricePF{
														Id:    price.ID,
														Price: price.Price,
													},
													Selected: h == 0,
												}
												// Thumbnail
												if price.Value.Thumbnail != "" {
													if p1 := path.Join(dir, price.Value.Thumbnail); len(p1) > 0 {
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
																if images, err := imageResize(p2, common.Config.Resize.Image.Size); err == nil {
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
									BasePrice:   basePriceMin,
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
			if err := os.RemoveAll(path.Join(output, "options")); err != nil {
				logger.Infof("%v", err)
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

		/*result := struct {
			ZIP string
			Date time.Time
			Tags []string
			Categories []string
			Images []string
			Thumbnail string
			BasePrice string
			ComparePrice *models.Price
			InStock bool
		}{
			ZIP: "Duke2",
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
	Product    ProductView
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



type Image struct {
	Filename string
	Size string
}

func imageResize(src, sizes string) ([]Image, error) {
	var images []Image
	file, err := os.Open(src)
	if err != nil {
		return images, err
	}
	img, err := jpeg.Decode(file)
	if err != nil {
		return images, err
	}
	file.Close()
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
		m := resize.Resize(uint(width), uint(height), img, resize.Lanczos3)
		filename := path.Base(src)
		filename = filename[:len(filename) - len(filepath.Ext(filename))]
		filename = fmt.Sprintf("%s_%dx%d%s", filename, width, height, filepath.Ext(src))
		images = append(images, Image{Filename: filename, Size: fmt.Sprintf("%dw", width)})
		out, err := os.Create(path.Join(path.Dir(src), "resize", filename))
		if err != nil {
			return images, err
		}
		defer out.Close()
		if err = jpeg.Encode(out, m, &jpeg.Options{Quality: common.Config.Resize.Quality}); err != nil {
			return images, err
		}
	}
	return images, nil
}