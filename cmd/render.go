package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/google/logger"
	"github.com/spf13/cobra"
	cmap "github.com/streamrail/concurrent-map"
	"github.com/yonnic/goshop/common"
	"github.com/yonnic/goshop/config"
	"github.com/yonnic/goshop/handler"
	"github.com/yonnic/goshop/models"
	"github.com/yonnic/goshop/storage"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gorm_logger "gorm.io/gorm/logger"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	CACHE = cmap.New()
	//VALUES = cmap.New()
	CACHE_CATEGORIES = cmap.New()
	CACHE_IMAGES = cmap.New()
	CACHE_VALUES = cmap.New()
	CACHE_PRICES = cmap.New()
	reKV = regexp.MustCompile(`^([^\:]+):\s*(.*)$`)
	reTags = regexp.MustCompile(`<.*?>`)
	reSanitizeFilename = regexp.MustCompile(`[+]`)
	reSpace = regexp.MustCompile(`\s{2,}`)
	reTrimEmail = regexp.MustCompile(`^(.+)@.*$`)
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
		clear := true
		if flagClear := cmd.Flag("clear").Value.String(); flagClear == "false" {
			clear = false
		}
		logger.Infof("clear: %v", clear)
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

		common.Database, err = gorm.Open(dialer, &gorm.Config{
			Logger: gorm_logger.New(
				log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
				gorm_logger.Config{
					SlowThreshold:             100 * time.Millisecond,
					LogLevel:                  gorm_logger.Silent,
					IgnoreRecordNotFoundError: true,
					Colorful:                  true,
				},
			),
		})
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
		common.Database.Unscoped().Where("ID > ?", 0).Delete(&models.CacheVariation{})
		if clear {
			common.Database.Unscoped().Where("ID > ?", 0).Delete(&models.CacheImage{})
			common.Database.Unscoped().Where("ID > ?", 0).Delete(&models.CacheFile{})
		}
		common.Database.Unscoped().Where("ID > ?", 0).Delete(&models.CacheValue{})
		common.Database.Unscoped().Where("ID > ?", 0).Delete(&models.CachePrice{})
		common.Database.Unscoped().Where("ID > ?", 0).Delete(&models.CacheTag{})
		common.Database.Unscoped().Where("ID > ?", 0).Delete(&models.CacheTransport{})
		common.Database.Unscoped().Where("ID > ?", 0).Delete(&models.CacheVendor{})
		//
		common.STORAGE, err = storage.NewLocalStorage(path.Join(dir, "hugo"), common.Config.Resize.Enabled, common.Config.Resize.Quality)
		if err != nil {
			logger.Warningf("%+v", err)
		}
		if common.Config.Storage.Enabled {
			if common.Config.Storage.S3.Enabled {
				if common.STORAGE, err = storage.NewAWSS3Storage(common.Config.Storage.S3.AccessKeyID,common.Config.Storage.S3.SecretAccessKey, common.Config.Storage.S3.Region, common.Config.Storage.S3.Bucket, common.Config.Storage.S3.Prefix, path.Join(dir, "temp", "s3"), common.Config.Resize.Enabled, common.Config.Resize.Quality, common.Config.Storage.S3.CDN, common.Config.Storage.S3.Rewrite); err != nil {
					logger.Warningf("%+v", err)
				}
				defer common.STORAGE.Close()
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
		// Tags
		if tags, err := models.GetTags(common.Database); err == nil {
			if remove {
				if err := os.RemoveAll(path.Join(output, "tags")); err != nil {
					logger.Infof("%v", err)
				}
			}
			// Payload
			for _, tag := range tags {
				if p1 := path.Join(output, "tags", tag.Name); len(p1) > 0 {
					if _, err := os.Stat(p1); err != nil {
						if err = os.MkdirAll(p1, 0755); err != nil {
							logger.Errorf("%v", err)
						}
					}
					for _, language := range languages {
						if p2 := path.Join(p1, fmt.Sprintf("_index%s.html", language.Suffix)); len(p2) > 0 {
							content := tag.Description
							tagFile := &common.TagFile{
								ID:      tag.ID,
								Name:   tag.Name,
								Title:   tag.Title,
								Type:    "tags",
								Content: content,
							}
							//
							// Thumbnail
							if tag.Thumbnail != "" {
								if p1 := path.Join(dir, "storage", tag.Thumbnail); len(p1) > 0 {
									if fi, err := os.Stat(p1); err == nil {
										filename := filepath.Base(p1)
										filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
										logger.Infof("Copy %v => %v %v bytes", p1, path.Join("images", "tags", filename), fi.Size())
										if thumbnails, err := common.STORAGE.PutImage(p1, path.Join("images", "tags", filename), common.Config.Resize.Thumbnail.Size); err == nil {
											tagFile.Thumbnail = strings.Join(thumbnails, ",")
										} else {
											logger.Warningf("%v", err)
										}
										//
										if _, err = models.CreateCacheTag(common.Database, &models.CacheTag{
											TagID:   tag.ID,
											Title:     tag.Title,
											Name:     tag.Name,
											Thumbnail: tagFile.Thumbnail,
										}); err != nil {
											logger.Warningf("%v", err)
										}
									}
								}
							}
							if err = common.WriteTagFile(p2, tagFile); err != nil {
								logger.Warningf("%v", err)
							}
						}
					}
				}
			}
		}
		// Vendors
		if vendors, err := models.GetVendors(common.Database); err == nil {
			if remove {
				if err := os.RemoveAll(path.Join(output, "vendors")); err != nil {
					logger.Infof("%v", err)
				}
			}
			// Payload
			for _, vendor := range vendors {
				if p1 := path.Join(output, "vendors", vendor.Name); len(p1) > 0 {
					if _, err := os.Stat(p1); err != nil {
						if err = os.MkdirAll(p1, 0755); err != nil {
							logger.Errorf("%v", err)
						}
					}
					for _, language := range languages {
						if p2 := path.Join(p1, fmt.Sprintf("_index%s.html", language.Suffix)); len(p2) > 0 {
							content := vendor.Content
							vendorFile := &common.VendorFile{
								ID:      vendor.ID,
								Name:   vendor.Name,
								Title:   vendor.Title,
								Type:    "vendors",
								Content: content,
							}
							//
							// Thumbnail
							if vendor.Thumbnail != "" {
								if p1 := path.Join(dir, "storage", vendor.Thumbnail); len(p1) > 0 {
									if fi, err := os.Stat(p1); err == nil {
										filename := filepath.Base(p1)
										filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
										logger.Infof("Copy %v => %v %v bytes", p1, path.Join("images", "vendors", filename), fi.Size())
										if thumbnails, err := common.STORAGE.PutImage(p1, path.Join("images", "vendors", filename), common.Config.Resize.Thumbnail.Size); err == nil {
											vendorFile.Thumbnail = strings.Join(thumbnails, ",")
										} else {
											logger.Warningf("%v", err)
										}
										//
										if _, err = models.CreateCacheVendor(common.Database, &models.CacheVendor{
											VendorID:   vendor.ID,
											Title:     vendor.Title,
											Name:     vendor.Name,
											Thumbnail: vendorFile.Thumbnail,
										}); err != nil {
											logger.Warningf("%v", err)
										}
									}
								}
							}
							if err = common.WriteVendorFile(p2, vendorFile); err != nil {
								logger.Warningf("%v", err)
							}
						}
					}
				}
			}
		}
		// Values
		if values, err := models.GetValues(common.Database); err == nil {
			for _, value := range values {
				var thumbnail string
				if value.Thumbnail != "" {
					if _, found := CACHE_VALUES.Get(value.Thumbnail); !found {
						if p1 := path.Join(dir, "storage", value.Thumbnail); len(p1) > 0 {
							if fi, err := os.Stat(p1); err == nil {
								filename := filepath.Base(p1)
								filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
								logger.Infof("Copy %v => %v %v bytes", p1, path.Join("images", "values", filename), fi.Size())
								if thumbnails, err := common.STORAGE.PutImage(p1, path.Join("images", "values", filename), common.Config.Resize.Thumbnail.Size); err == nil {
									thumbnail = strings.Join(thumbnails, ",")
									CACHE_VALUES.Set(value.Thumbnail, thumbnail)
									// Cache
									if _, err = models.CreateCacheValue(common.Database, &models.CacheValue{
										ValueID:   value.ID,
										Title:     value.Title,
										Thumbnail: thumbnail,
										Value:     value.Value,
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
			}
		}
		// Prices
		if prices, err := models.GetPrices(common.Database); err == nil {
			for _, price := range prices {
				var thumbnail string
				if price.Thumbnail != "" {
					if _, found := CACHE_PRICES.Get(price.Thumbnail); !found {
						if p1 := path.Join(dir, "storage", price.Thumbnail); len(p1) > 0 {
							if fi, err := os.Stat(p1); err == nil {
								filename := filepath.Base(p1)
								filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
								logger.Infof("Copy %v => %v %v bytes", p1, path.Join("images", "prices", filename), fi.Size())
								if thumbnails, err := common.STORAGE.PutImage(p1, path.Join("images", "prices", filename), common.Config.Resize.Thumbnail.Size); err == nil {
									thumbnail = strings.Join(thumbnails, ",")
									CACHE_PRICES.Set(price.Thumbnail, thumbnail)
									// Cache
									if _, err = models.CreateCachePrice(common.Database, &models.CachePrice{
										PriceID:   price.ID,
										Thumbnail: thumbnail,
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
			}
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
				if tree, err := models.GetCategoriesView(common.Database, 0, 999, true, true, false); err == nil {
					categoryFile.Count = tree.Count
				}else{
					logger.Warningf("%+v", err)
				}
				if err = common.WriteCategoryFile(path.Join(p2, "_index.html"), categoryFile); err != nil {
					logger.Warningf("%v", err)
				}
			}
			logger.Infof("Categories found: %v", len(categories))
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
								location := path.Join("images", "categories", filename)
								t2 := time.Now()
								if thumbnails, err = common.STORAGE.PutImage(p1, location, common.Config.Resize.Thumbnail.Size); err != nil {
									logger.Warningf("%v", err)
								}
								logger.Infof("Copy %v => %v %v bytes in ~ %.3f ms", p1, location, fi.Size(), float64(time.Since(t2).Nanoseconds())/1000000)
							}
						}
					}
					for _, language := range languages {
						if p2 := path.Join(append(append([]string{output}, names...), fmt.Sprintf("_index%s.html", language.Suffix))...); len(p2) > 0 {
							if _, err := os.Stat(p2); err != nil {
								categoryFile := &common.CategoryFile{
									ID:    category.ID,
									Date:  category.UpdatedAt,
									Title: category.Title,
									Description: category.Description,
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
								categoryFile.Thumbnail = strings.Join(thumbnails, ",")
								if tree, err := models.GetCategoriesView(common.Database, int(category.ID), 999, true, true, false); err == nil {
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
		t2 := time.Now()
		if products, err := models.GetProducts(common.Database); err == nil {
			logger.Infof("Products found: %v", len(products))
			/*if _, err := os.Stat(path.Join(dir, "temp", "products")); err != nil {
				if err = os.MkdirAll(path.Join(dir, "temp", "products"), 0755); err != nil {
					logger.Warningf("%+v", err)
				}
			}*/
			for i, product := range products {
				if product.Enabled {
					logger.Infof("[%d] Product ID: %+v Name: %v Title: %v", i, product.ID, product.Name, product.Title)

					product, _ = models.GetProductFull(common.Database, int(product.ID))
					/*p := path.Join(dir, "temp", "products", fmt.Sprintf("%d-%v.json", product.ID, product.Name))
					if fi, err := os.Stat(p); err != nil || !product.UpdatedAt.Equal(fi.ModTime()){
						logger.Infof("[MISS]")
						t2 := time.Now()
						product, _ = models.GetProductFull(common.Database, int(product.ID))
						if bts, err := json.Marshal(product); err == nil {
							if err = ioutil.WriteFile(p, bts, 0755); err == nil {
								if err = os.Chtimes(p, product.UpdatedAt, product.UpdatedAt); err != nil {
									logger.Warningf("%+v", err)
								}
							} else {
								logger.Warningf("%+v", err)
							}
						}
						logger.Infof("Cached in ~ %.3f ms", float64(time.Since(t2).Nanoseconds())/1000000)
					} else {
						t2 := time.Now()
						if bts, err := ioutil.ReadFile(p); err == nil {
							if err = json.Unmarshal(bts, &product); err != nil {
								logger.Warningf("%+v", err)
							}
						}
						logger.Infof("Uncached in ~ %.3f ms", float64(time.Since(t2).Nanoseconds())/1000000)
					}*/
					t3 := time.Now()
					// Common actions independent of category
					productFile := &common.ProductFile{
						ID:         product.ID,
						Date:       time.Now(),
						Title:      product.Title,
						Type:       "products",
					}
					productView := common.ProductPF{
						Id:         product.ID,
						Name:       product.Name,
						Title:      product.Title,
					}
					if product.Type != "" {
						productView.Type = product.Type
					}else {
						productView.Type = "swatch"
					}
					if product.Size != "" {
						productView.Size = product.Size
					}else {
						productView.Size = "medium"
					}
					productView.Description = reTags.ReplaceAllString(product.Description, "")
					productView.Description = reSpace.ReplaceAllString(product.Description, " ")
					if len(productView.Description) > 160 {
						productView.Description = productView.Description[:160]
					}
					// Process thumbnail
					//var thumbnails []string
					if product.Image != nil {
						if p1 := path.Join(dir, "storage", product.Image.Path); len(p1) > 0 {
							if fi, err := os.Stat(p1); err == nil {
								name := product.Name
								if len(name) > 32 {
									name = name[:32]
								}
								name = reSanitizeFilename.ReplaceAllString(name, "_")
								filename := fmt.Sprintf("%d-%s-%d%v", product.ID, name, fi.ModTime().Unix(), path.Ext(p1))
								location := path.Join("images", filename)
								t2 := time.Now()
								if thumbnails, err := common.STORAGE.PutImage(p1, location, common.Config.Resize.Image.Size); err == nil {
									productFile.Thumbnail = strings.Join(thumbnails, ",")
									productView.Thumbnail = strings.Join(thumbnails, ",")
								} else {
									logger.Warningf("%v", err)
								}
								logger.Infof("Copy %v => %v %v bytes in ~ %.3f ms", p1, location, fi.Size(), float64(time.Since(t2).Nanoseconds())/1000000)
							}
						}
					}
					// Parameters
					if len(product.Parameters) > 0 {
						for _, parameter := range product.Parameters {
							parameterView := common.ParameterPF{
								Id:    parameter.ID,
								Name:  parameter.Name,
								Title: parameter.Title,
							}
							if parameter.Value != nil {
								parameterView.Value = &common.ValuePPF{
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
					var minPrice float64
					for _, variation := range product.Variations {
						if variation.BasePrice != 0 && (minPrice == 0 || variation.BasePrice < minPrice) {
							minPrice = variation.BasePrice
						}
						for _, price := range variation.Prices {
							if price.BasePrice != 0 && (minPrice == 0 || price.BasePrice < minPrice) {
								minPrice = price.BasePrice
							}
						}
					}
					productFile.BasePrice = fmt.Sprintf("%.2f", minPrice)
					//productView.BasePrice = product.BasePrice
					if product.SalePrice > 0 && (product.End.IsZero() || (product.Start.Before(now) && product.End.After(now))) {
						var minPrice float64
						for _, variation := range product.Variations {
							if variation.SalePrice != 0 && (minPrice == 0 || variation.SalePrice < minPrice) {
								minPrice = variation.SalePrice
							}
							for _, price := range variation.Prices {
								if price.SalePrice != 0 && (minPrice == 0 || price.SalePrice < minPrice) {
									minPrice = price.SalePrice
								}
							}
						}
						productFile.SalePrice = fmt.Sprintf("%.2f", minPrice)
					}
					if product.ItemPrice > 0 {
						productFile.ItemPrice = fmt.Sprintf("%.2f", product.ItemPrice)
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
					// Related
					rows, err := common.Database.Table("products_relations").Select("ProductIdL, ProductIdR").Where("ProductIdL = ? or ProductIdR = ?", product.ID, product.ID).Rows()
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
					// Copy images
					var images []string
					if len(product.Images) > 0 {
						for i, image := range product.Images {
							if image.Path != "" {
								if v, found := CACHE_IMAGES.Get(image.Path); !found {
									if p1 := path.Join(dir, "storage", image.Path); len(p1) > 0 {
										if fi, err := os.Stat(p1); err == nil {
											name := product.Name
											if len(name) > 32 {
												name = name[:32]
											}
											name = reSanitizeFilename.ReplaceAllString(name, "_")
											filename := fmt.Sprintf("%d-%s-%d%v", image.ID, name, fi.ModTime().Unix(), path.Ext(p1))
											location := path.Join("images", filename)
											t2 := time.Now()
											if images2, err := common.STORAGE.PutImage(p1, location, common.Config.Resize.Image.Size); err == nil {
												// Generate thumbnail
												thumbnail := strings.Join(images2, ",")
												if i == 0 || product.ImageId == image.ID {
													productFile.Thumbnail = thumbnail
													productView.Thumbnail = thumbnail
													CACHE_IMAGES.Set(image.Path, thumbnail)
												}
												//
												images = append(images, strings.Join(images2, ","))
												//
												// Cache
												key := fmt.Sprintf("%v", image.ID)
												if !CACHE_IMAGES.Has(key) && (clear || !models.HasCacheImageByImageId(common.Database, image.ID)) {
													if _, err = models.CreateCacheImage(common.Database, &models.CacheImage{
														ImageId:   image.ID,
														Name:      image.Name,
														Thumbnail: strings.Join(images2, ","),
													}); err == nil {
														CACHE_IMAGES.Set(key, true)
													} else {
														logger.Warningf("%v", err)
													}
												}
											}
											logger.Infof("Copy %v => %v %v bytes in ~ %.3f ms", p1, location, fi.Size(), float64(time.Since(t2).Nanoseconds())/1000000)
										}
									}
								}else{
									thumbnail := v.(string)
									if i == 0 || product.ImageId == image.ID {
										productFile.Thumbnail = thumbnail
										productView.Thumbnail = thumbnail
									}
									//
									images = append(images, thumbnail)
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
										location := path.Join("files", filename)
										t2 := time.Now()
										if url, err := common.STORAGE.PutFile(p1, location); err == nil {
											files = append(files, common.FilePF{
												Id:   file.ID,
												Type: file.Type,
												Name: file.Name,
												Path: url,
												Size: file.Size,
											})
											// Cache
											if found := models.HasCacheFileByFileId(common.Database, file.ID); !found {
												if _, err = models.CreateCacheFile(common.Database, &models.CacheFile{
													FileId: file.ID,
													Name:   file.Name,
													File:   url,
												}); err != nil {
													logger.Warningf("%v", err)
												}
											}
											logger.Infof("Copy %v => %v %v bytes in ~ %.3f ms", p1, location, fi.Size(), float64(time.Since(t2).Nanoseconds())/1000000)
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
					var variations []string
					if !product.Container {
						variation := &models.Variation{
							ID:           0,
							Enabled: true,
							Name:         "default",
							Title:        "Default",
							Thumbnail:    product.Thumbnail,
							Properties:   product.Properties,
							BasePrice:    product.BasePrice,
							ManufacturerPrice: product.ManufacturerPrice,
							SalePrice:    product.SalePrice,
							Start:        product.Start,
							End:          product.End,
							ItemPrice: product.ItemPrice,
							MinQuantity: product.MinQuantity,
							MaxQuantity: product.MaxQuantity,
							PurchasableMultiply: product.PurchasableMultiply,
							Prices: product.Prices,
							Pattern: product.Pattern,
							Dimensions: product.Dimensions,
							DimensionUnit: product.DimensionUnit,
							Width:        product.Width,
							Height:       product.Height,
							Depth:        product.Depth,
							Volume:       product.Volume,
							Weight:       product.Weight,
							WeightUnit: product.WeightUnit,
							Packages:     product.Packages,
							Availability: product.Availability,
							Time:         product.Time,
							Sku:          product.Sku,
							ProductId:    product.ID,
						}
						if variation.DimensionUnit == "" && common.Config.DimensionUnit != "" {
							variation.DimensionUnit = common.Config.DimensionUnit
						}
						if variation.WeightUnit == "" && common.Config.WeightUnit != "" {
							variation.WeightUnit = common.Config.WeightUnit
						}
						if product.Variation != "" {
							variation.Title = product.Variation
						}
						product.Variations = append([]*models.Variation{variation}, product.Variations...)
					}
					var basePriceMin float64
					if len(product.Variations) > 0 {
						var minPrice float64
						for _, variation := range product.Variations {
							if variation.BasePrice != 0 && (minPrice == 0 || variation.BasePrice < minPrice) {
								minPrice = variation.BasePrice
							}
							for _, price := range variation.Prices {
								if price.BasePrice != 0 && (minPrice == 0 || price.BasePrice < minPrice) {
									minPrice = price.BasePrice
								}
							}
						}
						productFile.BasePrice = fmt.Sprintf("%.2f", minPrice)
						if product.Variations[0].SalePrice > 0 && (product.Variations[0].End.IsZero() || (product.Variations[0].Start.Before(now) && product.Variations[0].End.After(now))){
							var minPrice float64
							for _, variation := range product.Variations {
								if variation.SalePrice != 0 && (minPrice == 0 || variation.SalePrice < minPrice) {
									minPrice = variation.SalePrice
								}
								for _, price := range variation.Prices {
									if price.SalePrice != 0 && (minPrice == 0 || price.SalePrice < minPrice) {
										minPrice = price.SalePrice
									}
								}
							}
							productFile.SalePrice = fmt.Sprintf("%.2f", minPrice)
						}
						if product.Variations[0].ItemPrice > 0 {
							productFile.ItemPrice = fmt.Sprintf("%.2f", product.Variations[0].ItemPrice)
						}
						for _, variation := range product.Variations {
							if variation.Enabled {
								variationView := common.VariationPF{
									Id:    variation.ID,
									Name:  variation.Name,
									Title: variation.Title,
									//Thumbnail:   variation.Thumbnail,
									Description: variation.Description,
									BasePrice:   variation.BasePrice,
									ItemPrice:   variation.ItemPrice,
									MinQuantity:   variation.MinQuantity,
									MaxQuantity:   variation.MaxQuantity,
									PurchasableMultiply:   variation.PurchasableMultiply,
									//Dimensions: variation.Dimensions,
									Pattern:      variation.Pattern,
									Dimensions:   variation.Dimensions,
									DimensionUnit:   variation.DimensionUnit,
									Width:        variation.Width,
									Height:       variation.Height,
									Depth:        variation.Depth,
									Volume:       variation.Volume,
									Weight:       variation.Weight,
									WeightUnit:   variation.WeightUnit,
									Packages:     variation.Packages,
									Availability: variation.Availability,
									Sku:          variation.Sku,
									Selected:     len(productView.Variations) == 0,
								}
								if variationView.DimensionUnit == "" && common.Config.DimensionUnit != "" {
									variationView.DimensionUnit = common.Config.DimensionUnit
								}
								if variationView.WeightUnit == "" && common.Config.WeightUnit != "" {
									variationView.WeightUnit = common.Config.WeightUnit
								}
								//
								for _, price := range variation.Prices {
									var ids []uint
									for _, rate := range price.Rates {
										ids = append(ids, rate.ID)
									}
									pricePF := common.PricePF{
										Ids:          ids,
										Thumbnail:    price.Thumbnail,
										BasePrice:        price.BasePrice,
										Availability: price.Availability,
										Sku:          price.Sku,
									}
									if price.SalePrice > 0 {
										pricePF.SalePrice = price.SalePrice
									}
									if v, found := CACHE_PRICES.Get(pricePF.Thumbnail); found {
										pricePF.Thumbnail = v.(string)
									}
									variationView.Prices = append(variationView.Prices, pricePF)
								}
								//
								if variation.Time != nil {
									variationView.Time = variation.Time.Title
								}
								// Images
								if variationView.Id == 0 {
									variationView.Images = images
								} else if len(variation.Images) > 0 {
									var images []string
									for _, image := range variation.Images {
										if image.Path != "" {
											if p1 := path.Join(dir, "storage", image.Path); len(p1) > 0 {
												if fi, err := os.Stat(p1); err == nil {
													name := product.Name + "-" + variation.Name
													if len(name) > 32 {
														name = name[:32]
													}
													name = reSanitizeFilename.ReplaceAllString(name, "_")
													filename := fmt.Sprintf("%d-%s-%d%v", image.ID, name, fi.ModTime().Unix(), path.Ext(p1))
													t2 := time.Now()
													if images2, err := common.STORAGE.PutImage(p1, path.Join("images", filename), common.Config.Resize.Thumbnail.Size); err == nil {
														images = append(images, strings.Join(images2, ","))
														// Cache
														key := fmt.Sprintf("%v", image.ID)
														if !CACHE_IMAGES.Has(key) && (clear || !models.HasCacheImageByImageId(common.Database, image.ID)) {
															if _, err = models.CreateCacheImage(common.Database, &models.CacheImage{
																ImageId:   image.ID,
																Name:      image.Name,
																Thumbnail: strings.Join(images2, ","),
															}); err == nil {
																CACHE_IMAGES.Set(key, true)
															} else {
																logger.Warningf("%v", err)
															}
														}
													} else {
														logger.Warningf("%v", err)
													}
													logger.Infof("Copy %v => %v %v bytes in ~ %.3f ms", p1, path.Join("images", filename), fi.Size(), float64(time.Since(t2).Nanoseconds())/1000000)
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
								} else {
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
														location := path.Join("files", filename)
														t2 := time.Now()
														if url, err := common.STORAGE.PutFile(p1, location); err == nil {
															files = append(files, common.FilePF{
																Id:   file.ID,
																Type: file.Type,
																Name: file.Name,
																Path: url,
																Size: file.Size,
															})
															// Cache
															if found := models.HasCacheFileByFileId(common.Database, file.ID); !found {
																if _, err = models.CreateCacheFile(common.Database, &models.CacheFile{
																	FileId: file.ID,
																	Name:   file.Name,
																	File:   url,
																}); err != nil {
																	logger.Warningf("%v", err)
																}
															}
														} else {
															logger.Warningf("%v", err)
														}
														logger.Infof("Copy %v => %v %v bytes in ~ %.3f ms", p1, location, fi.Size(), float64(time.Since(t2).Nanoseconds())/1000000)
													}
												}
											}
										}
										variationView.Files = files
										productView.Files = append(productView.Files, files...)
									}
								}

								if variation.SalePrice > 0 && (variation.End.IsZero() || (variation.Start.Before(now) && variation.End.After(now))) {
									variationView.SalePrice = variation.SalePrice
								}

								if basePriceMin > variation.BasePrice || basePriceMin < 0.01 {
									basePriceMin = variation.BasePrice
								}

								for _, price := range variation.Prices {
									if price.BasePrice != 0 && (basePriceMin == 0 || price.BasePrice < basePriceMin) {
										basePriceMin = price.BasePrice
									}
								}
								// Thumbnail
								if variation.Thumbnail != "" {
									if p1 := path.Join(dir, "storage", variation.Thumbnail); len(p1) > 0 {
										if fi, err := os.Stat(p1); err == nil {
											filename := filepath.Base(p1)
											filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
											t2 := time.Now()
											if thumbnails, err := common.STORAGE.PutImage(p1, path.Join("images", "variations", filename), common.Config.Resize.Thumbnail.Size); err == nil {
												variationView.Thumbnail = strings.Join(thumbnails, ",")
											} else {
												logger.Warningf("%v", err)
											}
											logger.Infof("Copy %v => %v %v bytes in ~ %.3f ms", p1, path.Join("images", "variations", filename), fi.Size(), float64(time.Since(t2).Nanoseconds())/1000000)
										}
									}
								}
								variations = append(variations, strings.Join([]string{fmt.Sprintf("%d", variation.ID), variation.Title}, ","))
								for _, property := range variation.Properties {
									propertyView := common.PropertyPF{
										Id:    property.ID,
										Type:  property.Type,
										Size:  property.Size,
										Name:  property.Name,
										Title: property.Title,
									}
									for h, rate := range property.Rates {
										valueView := common.ValuePF{
											Id:          rate.Value.ID,
											Enabled:     rate.Enabled,
											Title:       rate.Value.Title,
											Description: rate.Value.Description,
											Color:       rate.Value.Color,
											//Thumbnail: price.Value.Thumbnail,
											Value:        rate.Value.Value,
											Availability: rate.Value.Availability,
											Price: common.RatePF{
												Id:           rate.ID,
												Price:        rate.Price,
												Availability: rate.Availability,
												Sku:          rate.Sku,
											},
											Selected: h == 0,
										}
										// Thumbnail
										if rate.Value.Thumbnail != "" {
											// p1 => thumbnails
											if v, found := CACHE_VALUES.Get(rate.Value.Thumbnail); !found {
												if p1 := path.Join(dir, "storage", rate.Value.Thumbnail); len(p1) > 0 {
													if fi, err := os.Stat(p1); err == nil {
														filename := filepath.Base(p1)
														filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
														location := path.Join("images", "values", filename)
														t2 := time.Now()
														if thumbnails, err := common.STORAGE.PutImage(p1, location, common.Config.Resize.Thumbnail.Size); err == nil {
															valueView.Thumbnail = strings.Join(thumbnails, ",")
															CACHE_VALUES.Set(rate.Value.Thumbnail, valueView.Thumbnail)
															//
															if _, err = models.CreateCacheValue(common.Database, &models.CacheValue{
																ValueID:   rate.Value.ID,
																Title:     rate.Value.Title,
																Thumbnail: valueView.Thumbnail,
																Value:     rate.Value.Value,
															}); err != nil {
																logger.Warningf("%v", err)
															}
														} else {
															logger.Warningf("%v", err)
														}
														logger.Infof("Copy %v => %v %v bytes in ~ %.3f ms", p1, location, fi.Size(), float64(time.Since(t2).Nanoseconds())/1000000)
													}
												}
											} else {
												valueView.Thumbnail = v.(string)
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
									SalePrice:   variation.SalePrice,
								}); err != nil {
									logger.Warningf("%v", err)
								}
							}
						}
					}
					productView.Pattern = product.Pattern
					productView.Dimensions = product.Dimensions
					productView.DimensionUnit = product.DimensionUnit
					productView.Volume = product.Volume
					productView.Weight = product.Weight
					productView.WeightUnit = product.WeightUnit
					productView.Availability = product.Availability
					if product.Vendor != nil {
						productView.Vendor = common.VendorPF{
							Id:          product.Vendor.ID,
							Name:        product.Vendor.Name,
							Title:       product.Vendor.Title,
							Thumbnail:   product.Vendor.Thumbnail,
							Description: product.Vendor.Description,
						}
						if product.Vendor.Thumbnail != "" {
							if cache, err := models.GetCacheVendorByVendorId(common.Database, product.VendorId); err == nil {
								productView.Vendor.Thumbnail = cache.Thumbnail
							}
						}
					}
					if product.Time != nil {
						productView.Time = product.Time.Title
					}
					productFile.Product = productView
					for _, productTag := range product.Tags {
						if productTag.Enabled {
							tag := common.TagPF{Id: productTag.ID, Name: productTag.Name, Title: productTag.Title}
							if productTag.Thumbnail != "" {
								if cache, err := models.GetCacheTagByTagId(common.Database, productTag.ID); err == nil {
									tag.Thumbnail = cache.Thumbnail
								}
							}
							productFile.Tags = append(productFile.Tags, tag)
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
								if cache, err := models.GetCacheCommentByCommentId(common.Database, comment.ID); err == nil {
									if cache.Images != "" {
										commentPF.Images = strings.Split(cache.Images, ";")
									}
								}
								if user, err := models.GetUser(common.Database, int(comment.UserId)); err == nil {
									if user.Name != "" || user.Lastname != "" {
										commentPF.Author = strings.TrimSpace(fmt.Sprintf("%s %s", user.Name, user.Lastname))
									}else {
										commentPF.Author = reTrimEmail.ReplaceAllString(user.Email, "$1")
									}
								}
								productFile.Comments = append(productFile.Comments, commentPF)
								max += comment.Max
								votes++
							}
						}
					}
					if max > 0 && votes > 0 {
						productFile.Max = math.Round((float64(max) / float64(votes)) * 100) / 100
					}
					productFile.Votes = votes
					productFile.Description = product.Content
					productFile.Content = product.Content
					if categories, err := models.GetCategoriesOfProduct(common.Database, product); err == nil {
						var canonical string
						for i, category := range categories {
							breadcrumbs := &[]*models.Category{}
							createBreadcrumbs(common.Database, category.ID, breadcrumbs, product)
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
								productFile.CategoryId = category.ID
								if i > 0 {
									productFile.Canonical = canonical
								}
								//
								var arr = []string{}
								t6 := time.Now()
								for _, category := range *breadcrumbs {
									arr = append(arr, category.Name)
									for _, language := range languages {
										if p2 := path.Join(append(append([]string{output}, arr...), fmt.Sprintf("_index%s.html", language.Suffix))...); len(p2) > 0 {
											// Update category file
											if categoryFile, err := common.ReadCategoryFile(p2); err == nil {
												variations := append([]*models.Variation{{
													BasePrice: product.BasePrice,
													Dimensions: product.Dimensions,
													DimensionUnit: product.DimensionUnit,
													Width: product.Width,
													WeightUnit: product.WeightUnit,
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
																					location := path.Join("images", "values", filename)
																					t2 := time.Now()
																					if thumbnails, err := common.STORAGE.PutImage(p1, location, common.Config.Resize.Thumbnail.Size); err == nil {
																						thumbnail = strings.Join(thumbnails, ",")
																					} else {
																						logger.Warningf("%v", err)
																					}
																					logger.Infof("Copy %v => %v %v bytes in ~ %.3f ms", p1, location, fi.Size(), float64(time.Since(t2).Nanoseconds())/1000000)
																				}
																			}
																		}
																		//
																		categoryFile.Options[i].Values = append(categoryFile.Options[i].Values, &common.ValueCF{
																			ID:        parameter.Value.ID,
																			Color:     parameter.Value.Color,
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
																			location := path.Join("images", "values", filename)
																			t2 := time.Now()
																			if thumbnails, err := common.STORAGE.PutImage(p1, location, common.Config.Resize.Thumbnail.Size); err == nil {
																				thumbnail = strings.Join(thumbnails, ",")
																			} else {
																				logger.Warningf("%v", err)
																			}
																			logger.Infof("Copy %v => %v %v bytes in ~ %.3f ms", p1, location, fi.Size(), float64(time.Since(t2).Nanoseconds())/1000000)
																		}
																	}
																}
																//
																opt.Values = append(opt.Values, &common.ValueCF{
																	ID:        parameter.Value.ID,
																	Color:     parameter.Value.Color,
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
																	for _, rate := range property.Rates {
																		var found bool
																		for _, value := range opt.Values {
																			if value.ID == rate.Value.ID {
																				found = true
																				break
																			}
																		}
																		if !found {
																			//
																			var thumbnail string
																			if rate.Value.Thumbnail != "" {
																				if p1 := path.Join(dir, "storage", rate.Value.Thumbnail); len(p1) > 0 {
																					if fi, err := os.Stat(p1); err == nil {
																						filename := filepath.Base(p1)
																						filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
																						location := path.Join("images", "values", filename)
																						t2 := time.Now()
																						if thumbnails, err := common.STORAGE.PutImage(p1, location, common.Config.Resize.Thumbnail.Size); err == nil {
																							thumbnail = strings.Join(thumbnails, ",")
																						} else {
																							logger.Warningf("%v", err)
																						}
																						logger.Infof("Copy %v => %v %v bytes in ~ %.3f ms", p1, location, fi.Size(), float64(time.Since(t2).Nanoseconds())/1000000)
																					}
																				}
																			}
																			// Only if the value is a part of some Option
																			if rate.Value.OptionId > 0 {
																				categoryFile.Options[i].Values = append(categoryFile.Options[i].Values, &common.ValueCF{
																					ID:        rate.Value.ID,
																					Color: rate.Value.Color,
																					Thumbnail: thumbnail,
																					Title:     rate.Value.Title,
																					Value:     rate.Value.Value,
																				})
																			}
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
																for _, rate := range property.Rates {
																	if rate.Enabled {
																		//
																		var thumbnail string
																		if rate.Value.Thumbnail != "" {
																			if p1 := path.Join(dir, "storage", rate.Value.Thumbnail); len(p1) > 0 {
																				if fi, err := os.Stat(p1); err == nil {
																					filename := filepath.Base(p1)
																					filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
																					location := path.Join("images", "values", filename)
																					t2 := time.Now()
																					if thumbnails, err := common.STORAGE.PutImage(p1, location, common.Config.Resize.Thumbnail.Size); err == nil {
																						thumbnail = strings.Join(thumbnails, ",")
																					} else {
																						logger.Warningf("%v", err)
																					}
																					logger.Infof("Copy %v => %v %v bytes in ~ %.3f ms", p1, location, fi.Size(), float64(time.Since(t2).Nanoseconds())/1000000)
																				}
																			}
																		}
																		// Only if the value is a part of some Option
																		if rate.Value.OptionId > 0 {
																			opt.Values = append(opt.Values, &common.ValueCF{
																				ID:        rate.Value.ID,
																				Color:     rate.Value.Color,
																				Thumbnail: thumbnail,
																				Title:     rate.Value.Title,
																				Value:     rate.Value.Value,
																			})
																		}
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
																	for _, rate := range property.Rates {
																		var found bool
																		for _, value := range opt.Values {
																			if value.ID == rate.Value.ID {
																				found = true
																				break
																			}
																		}
																		if !found {
																			//
																			var thumbnail string
																			if rate.Value.Thumbnail != "" {
																				if p1 := path.Join(dir, "storage", rate.Value.Thumbnail); len(p1) > 0 {
																					if fi, err := os.Stat(p1); err == nil {
																						filename := filepath.Base(p1)
																						filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
																						location := path.Join("images", "values", filename)
																						t2 := time.Now()
																						if thumbnails, err := common.STORAGE.PutImage(p1, location, common.Config.Resize.Thumbnail.Size); err == nil {
																							thumbnail = strings.Join(thumbnails, ",")
																						} else {
																							logger.Warningf("%v", err)
																						}
																						logger.Infof("Copy %v => %v %v bytes in ~ %.3f ms", p1, location, fi.Size(), float64(time.Since(t2).Nanoseconds())/1000000)
																					}
																				}
																			}
																			// Only if the value is a part of some Option
																			if rate.Value.OptionId > 0 {
																				categoryFile.Options[i].Values = append(categoryFile.Options[i].Values, &common.ValueCF{
																					ID:        rate.Value.ID,
																					Color:     rate.Value.Color,
																					Thumbnail: thumbnail,
																					Title:     rate.Value.Title,
																					Value:     rate.Value.Value,
																				})
																			}
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
																for _, rate := range property.Rates {
																	if rate.Enabled {
																		//
																		var thumbnail string
																		if rate.Value.Thumbnail != "" {
																			if p1 := path.Join(dir, "storage", rate.Value.Thumbnail); len(p1) > 0 {
																				if fi, err := os.Stat(p1); err == nil {
																					filename := filepath.Base(p1)
																					filename = fmt.Sprintf("%v-%d%v", filename[:len(filename)-len(filepath.Ext(filename))], fi.ModTime().Unix(), filepath.Ext(filename))
																					location := path.Join("images", "values", filename)
																					if thumbnails, err := common.STORAGE.PutImage(p1, location, common.Config.Resize.Thumbnail.Size); err == nil {
																						thumbnail = strings.Join(thumbnails, ",")
																					} else {
																						logger.Warningf("%v", err)
																					}
																					logger.Infof("Copy %v => %v %v bytes in ~ %.3f ms", p1, location, fi.Size(), float64(time.Since(t2).Nanoseconds())/1000000)
																				}
																			}
																		}
																		//
																		opt.Values = append(opt.Values, &common.ValueCF{
																			ID:        rate.Value.ID,
																			Color:     rate.Value.Color,
																			Thumbnail: thumbnail,
																			Title:     rate.Value.Title,
																			Value:     rate.Value.Value,
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
								logger.Infof("Breadcrumbs ~ %.3f ms", float64(time.Since(t6).Nanoseconds())/1000000)
								t6 = time.Now()
								productFile.Product.CategoryId = category.ID

								/*if p := path.Join(p1, product.Name); p != "" {
									if _, err := os.Stat(p); err != nil {
										if err = os.MkdirAll(p, 0755); err != nil {
											logger.Warningf("%v", err)
										}
									}
								}*/
								productFile.Product.Path = "/" + path.Join(append(names, product.Name)...) + "/"

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
									productFile.Sku = product.Sku
									// Sort
									if rows, err := common.Database.Table("categories_products_sort").Select("Value").Where("CategoryId = ? and ProductId = ?", category.ID, product.ID).Rows(); err == nil {
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
								cacheProduct := &models.CacheProduct{
									ProductID:   product.ID,
									Path:        fmt.Sprintf("/%s/", strings.Join(names, "/")),
									Name:        product.Name,
									Title:       product.Title,
									Description: product.Description,
									Thumbnail:   productView.Thumbnail,
									Images:      strings.Join(images, ";"),
									Variations:  strings.Join(variations, ";"),
									CategoryID:  category.ID,
									Width:       product.Width,
									Height:      product.Height,
									Depth:       product.Depth,
									Volume:      product.Volume,
									Weight:      product.Weight,
									Sku: product.Sku,
								}
								if len(product.Variations) > 0 {
									// BasePrice
									var minPrice float64
									for _, variation := range product.Variations {
										if variation.BasePrice != 0 && (minPrice == 0 || variation.BasePrice < minPrice) {
											minPrice = variation.BasePrice
										}
										for _, price := range variation.Prices {
											if price.BasePrice != 0 && (minPrice == 0 || price.BasePrice < minPrice) {
												minPrice = price.BasePrice
											}
										}
									}
									cacheProduct.BasePrice = minPrice
									//
									minPrice = 0
									for _, variation := range product.Variations {
										if variation.SalePrice != 0 && (minPrice == 0 || variation.SalePrice < minPrice) {
											minPrice = variation.SalePrice
										}
										for _, price := range variation.Prices {
											if price.SalePrice != 0 && (minPrice == 0 || price.SalePrice < minPrice) {
												minPrice = price.SalePrice
											}
										}
									}
									cacheProduct.SalePrice = minPrice
								}else{
									var minPrice = product.BasePrice
									for _, price := range product.Prices {
										if price.BasePrice != 0 && (minPrice == 0 || price.BasePrice < minPrice) {
											minPrice = price.BasePrice
										}
									}
									cacheProduct.BasePrice = minPrice
									minPrice = product.SalePrice
									for _, price := range product.Prices {
										if price.SalePrice != 0 && (minPrice == 0 || price.SalePrice < minPrice) {
											minPrice = price.SalePrice
										}
									}
									cacheProduct.SalePrice = minPrice
								}
								t7 := time.Now()
								if _, err = models.CreateCacheProduct(common.Database, cacheProduct); err != nil {
									logger.Warningf("%v", err)
								}
								logger.Infof("CreateCache ~ %.3f ms", float64(time.Since(t7).Nanoseconds())/1000000)
								logger.Infof("Rest ~ %.3f ms", float64(time.Since(t6).Nanoseconds())/1000000)
							}
						}
					}
					logger.Infof("[%d] Product ID: %+v ~ %.3f ms", i, product.ID, float64(time.Since(t3).Nanoseconds())/1000000)
				}
			}
		}else{
			logger.Errorf("%v", err)
			return
		}
		logger.Infof("Products ~ %.3f ms", float64(time.Since(t2).Nanoseconds())/1000000)
		//
		common.THUMBNAILS.Flush()
		// Catalog
		if tree, err := models.GetCategoriesView(common.Database, 0, 999, false, true, true); err == nil {
			if bts, err := json.Marshal(tree); err == nil {
				// Data
				p := path.Join(dir, "hugo", "data")
				if _, err = os.Stat(p); err != nil {
					if err = os.MkdirAll(p, 0755); err != nil {
						logger.Warningf("%+v", err)
					}
				}
				if err = ioutil.WriteFile(path.Join(p, "catalog.json"), bts, 0755); err != nil {
					logger.Warningf("%+v", err)
				}
				// Static / catalog.json
				p = path.Join(dir, "hugo", "static")
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
		// Options
		/*if options, err := models.GetOptions(common.Database); err == nil {
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
											if v, found := CACHE_VALUES.Get(value.Thumbnail); found {
												valueFile.Thumbnail = v.(string)
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
		}*/
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
			if bts, err := json.Marshal(views); err == nil {
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
		// Options
		if options, err := models.GetOptionsFull(common.Database); err == nil {
			var data handler.OptionsFullView
			if bts, err := json.Marshal(options); err == nil {
				if err = json.Unmarshal(bts, &data); err == nil {
					for i, option := range data {
						for j, value := range option.Values {
							if cache, err := models.GetCacheValueByValueId(common.Database, value.ID); err == nil {
								data[i].Values[j].Thumbnail = cache.Thumbnail
							}else{
								logger.Warningf("%+v", err)
							}
						}
					}
				}else{
					logger.Warningf("%+v", err)
				}
			}
			if bts, err := json.Marshal(data); err == nil {
				p := path.Join(dir, "hugo", "data")
				if _, err = os.Stat(p); err != nil {
					if err = os.MkdirAll(p, 0755); err != nil {
						logger.Warningf("%+v", err)
					}
				}
				if err = ioutil.WriteFile(path.Join(p, "options.json"), bts, 0755); err != nil {
					logger.Warningf("%+v", err)
				}
			}
		}
		// Data
		var data struct {
			Plugins map[string]interface{}
		}

		var instagram InstagramData

		for _, url := range []string{
			"https://cdn.dasmoebelhaus.de/theme/instagram/1-min.jpg",
			"https://cdn.dasmoebelhaus.de/theme/instagram/2-min.jpg",
			"https://cdn.dasmoebelhaus.de/theme/instagram/3-min.jpg",
			"https://cdn.dasmoebelhaus.de/theme/instagram/4-min.jpg",
			"https://cdn.dasmoebelhaus.de/theme/instagram/5-min.jpg",
			"https://cdn.dasmoebelhaus.de/theme/instagram/6-min.jpg",
			"https://cdn.dasmoebelhaus.de/theme/instagram/7-min.jpg",
			"https://cdn.dasmoebelhaus.de/theme/instagram/8-min.jpg",
		}{
			instagram.Posts = append(instagram.Posts, InstagramPost{
				Url: url,
			})
		}

		data.Plugins = make(map[string]interface{})
		data.Plugins["instagram"] = instagram

		if bts, err := json.Marshal(data); err == nil {
			p := path.Join(dir, "hugo", "data")
			if _, err = os.Stat(p); err != nil {
				if err = os.MkdirAll(p, 0755); err != nil {
					logger.Warningf("%+v", err)
				}
			}
			if err = ioutil.WriteFile(path.Join(p, "data.json"), bts, 0755); err != nil {
				logger.Warningf("%+v", err)
			}
		}

		logger.Infof("Rendered ~ %.3f ms", float64(time.Since(t1).Nanoseconds())/1000000)
	},
}

type InstagramData struct {
	Posts []InstagramPost
}

type InstagramPost struct {
	Url string
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
			if categoriesView, err := models.GetCategoriesView(common.Database, raw.Data.Id, 999, true, true, false); err == nil {
				root.Type = raw.Data.Type
				root.ID = categoriesView.ID
				root.Name = categoriesView.Name
				root.Title = categoriesView.Title
				root.Path = categoriesView.Path
				root.Count = categoriesView.Count
				if v, found := common.THUMBNAILS.Get(categoriesView.Thumbnail); !found {
					if cache, err := models.GetCacheCategoryByCategoryId(common.Database, categoriesView.ID); err == nil {
						common.THUMBNAILS.Set(categoriesView.Thumbnail, cache.Thumbnail, time.Duration(60 + rand.Intn(60)) * time.Second)
						root.Thumbnail = cache.Thumbnail
					}else{
						common.THUMBNAILS.Set(categoriesView.Thumbnail, "", 3 * time.Second)
					}
				}else if thumbnail := v.(string); thumbnail != "" {
					root.Thumbnail = thumbnail
				}
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

func createBreadcrumbs(connector *gorm.DB, id uint, breadcrumbs *[]*models.Category, product *models.Product) {
	if id != 0 {
		key := fmt.Sprintf("%+v", id)
		var category *models.Category
		if v, found := CACHE_CATEGORIES.Get(key); found {
			category = v.(*models.Category)
		}else{
			var err error
			if category, err = models.GetCategory(common.Database, int(id)); err == nil {
				if category.Thumbnail == "" {
					if len(*breadcrumbs) > 0 {
						category.Thumbnail = (*breadcrumbs)[0].Thumbnail
					} else if product.Image != nil {
						category.Thumbnail = product.Image.Url
					}
				}
				CACHE_CATEGORIES.Set(key, category)
			}
		}
		if category != nil {
			*breadcrumbs = append([]*models.Category{category}, *breadcrumbs...)
			createBreadcrumbs(connector, category.ParentId, breadcrumbs, product)
		}
	}
}

func init() {
	RootCmd.AddCommand(renderCmd)
	renderCmd.Flags().StringP("products", "p", "products", "products output folder")
	renderCmd.Flags().BoolP("clear", "c", false, "clear cache")
	renderCmd.Flags().BoolP("remove", "r", false, "remove all files during rendering")
}
