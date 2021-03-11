package models

import (
	"encoding/json"
	"gorm.io/gorm"
	"sort"
	"time"
)

type Product struct {
	gorm.Model
	Enabled bool
	Name        string `gorm:"size:255;index:idx_product_name,unique"`
	Title       string
	Description string
	Thumbnail   string
	Parameters  []*Parameter `gorm:"foreignKey:ProductId"`
	CustomParameters 	string
	Content 	string
	// ONLY TO USE AS DEFAULT VALUE FOR VIRIATIONS
	Variation string
	BasePrice float64          `sql:"type:decimal(8,2);"`
	SalePrice float64          `sql:"type:decimal(8,2);"`
	Start time.Time
	End time.Time
	//
	Pattern string
	Dimensions string
	Width float64 `sql:"type:decimal(8,2);"`
	Height float64 `sql:"type:decimal(8,2);"`
	Depth float64 `sql:"type:decimal(8,2);"`
	Weight float64 `sql:"type:decimal(8,2);"`
	Availability string
	Sending string
	Sku string
	//
	Properties []*Property `gorm:"foreignKey:ProductId"`
	//
	Categories  []*Category  `gorm:"many2many:categories_products;"`
	Variations  []*Variation `gorm:"foreignKey:ProductId"`
	Files       []*File     `gorm:"many2many:products_files;"`
	//
	ImageId uint
	Image       *Image `gorm:"foreignKey:image_id;"`
	//
	Images      []*Image     `gorm:"many2many:products_images;"`
	Tags        []*Tag `gorm:"many2many:products_tags;"`
	//
	//RelatedProducts []*Product `gorm:"many2many:products_related;"`
	//
	Customization string
}

func SearchProducts(connector *gorm.DB, term string, limit int) ([]*Product, error) {
	db := connector
	var products []*Product
	db.Debug().Preload("Categories").Preload("Variations").Where("name like ? OR title like ? OR description like ?", term, term, term).Limit(limit).Find(&products)
	if err := db.Error; err != nil {
		return nil, err
	}
	return products, nil
}

func GetProducts(connector *gorm.DB) ([]*Product, error) {
	db := connector
	var products []*Product
	db.Debug().Find(&products)
	if err := db.Error; err != nil {
		return nil, err
	}
	return products, nil
}

func GetProductsWithImages(connector *gorm.DB) ([]*Product, error) {
	db := connector
	var products []*Product
	db.Debug().Preload("Image").Preload("Images").Find(&products)
	if err := db.Error; err != nil {
		return nil, err
	}
	return products, nil
}

func GetProductsByCategoryId(connector *gorm.DB, id uint) ([]*Product, error) {
	db := connector
	var products []*Product
	db.Model(&Product{}).Preload("Image").Joins("inner join categories_products on categories_products.product_id = products.id").Where("categories_products.category_id = ?", id).Find(&products)
	if err :=  db.Error; err != nil {
		return nil, err
	}
	return products, nil
}

func GetProduct(connector *gorm.DB, id int) (*Product, error) {
	db := connector
	var product Product
	db.Debug().Find(&product, id)
	if err := db.Error; err != nil {
		return nil, err
	}
	return &product, nil
}

func GetProductFull(connector *gorm.DB, id int) (*Product, error) {
	db := connector
	var product Product
	if err := db.Debug().Preload("Categories").Preload("Parameters").Preload("Parameters.Option").Preload("Parameters.Value").Preload("Properties").Preload("Properties.Option").Preload("Properties.Prices").Preload("Properties.Prices.Value").Preload("Files").Preload("Images").Preload("Variations").Preload("Variations.Properties").Preload("Variations.Properties.Option").Preload("Variations.Properties.Prices").Preload("Variations.Properties.Prices.Value").Preload("Variations.Images").Preload("Variations.Files").Preload("Tags").First(&product, id).Error; err != nil {
		return nil, err
	}
	var customization struct {
		Images struct {
			Order []uint
		}
	}
	// Parameters
	parameters := product.Parameters
	sort.SliceStable(parameters, func(i, j int) bool {
		if parameters[i].Option != nil {
			if parameters[j].Option != nil {
				return parameters[j].Option.Sort > parameters[i].Option.Sort
			}
			return true
		}
		return false
	})
	product.Parameters = parameters
	// Customization
	if err := json.Unmarshal([]byte(product.Customization), &customization); err == nil {
		images := product.Images
		sort.SliceStable(images, func(i, j int) bool {
			var x, y = -1, -1
			for k, id := range customization.Images.Order {
				if id == images[i].ID {
					x = k
				}
				if id == images[j].ID {
					y = k
				}
			}
			if x == -1 || y == -1 {
				return images[i].ID < images[j].ID
			}else{
				return x < y
			}
		})
		product.Images = images
	}
	//
	if len(product.Variations) > 0 {
		for i, variation := range product.Variations {
			if err := json.Unmarshal([]byte(variation.Customization), &customization); err == nil {
				images := variation.Images
				sort.SliceStable(images, func(i, j int) bool {
					var x, y = -1, -1
					for k, id := range customization.Images.Order {
						if id == images[i].ID {
							x = k
						}
						if id == images[j].ID {
							y = k
						}
					}
					if x == -1 || y == -1 {
						return images[i].ID < images[j].ID
					} else {
						return x < y
					}
				})
				product.Variations[i].Images = images
			}
		}
	}
	return &product, nil
}

func GetProductByName(connector *gorm.DB, name string) (*Product, error) {
	db := connector
	var product Product
	if err := db.Where("name = ?", name).First(&product).Error; err != nil {
		return nil, err
	}
	return &product, nil
}

func GetProductVariations(connector *gorm.DB, id int) ([]*Variation, error) {
	db := connector
	var product Product
	db.Debug().Preload("Variations").Preload("Variations.Properties").Preload("Variations.Properties.Option").Preload("Variations.Properties.Prices").Preload("Variations.Properties.Prices.Value").Find(&product, id)
	if err := db.Error; err != nil {
		return nil, err
	}
	return product.Variations, nil
}

func CreateProduct(connector *gorm.DB, product *Product) (uint, error) {
	db := connector
	db.Debug().Create(&product)
	if err := db.Error; err != nil {
		return 0, err
	}
	return product.ID, nil
}

func GetCategoriesOfProduct(connector *gorm.DB, product *Product) ([]*Category, error) {
	db := connector
	var categories []*Category
	if err := db.Model(&product).Association("Categories").Find(&categories); err != nil {
		return nil, err
	}
	return categories, nil
}

func GetFilesOfProduct(connector *gorm.DB, product *Product) ([]*File, error) {
	db := connector
	var files []*File
	if err := db.Model(&product).Association("Files").Find(&files); err != nil {
		return nil, err
	}
	return files, nil
}

func AddFileToProduct(connector *gorm.DB, product *Product, file *File) error {
	db := connector
	return db.Model(&product).Association("Files").Append(file)
}

func AddImageToProduct(connector *gorm.DB, product *Product, image *Image) error {
	db := connector
	return db.Model(&product).Association("Images").Append(image)
}

func DeleteAllImagesFromProduct(connector *gorm.DB, product *Product) error {
	db := connector
	return db.Debug().Unscoped().Model(&product).Association("Images").Clear()
}

func DeleteAllCategoriesFromProduct(connector *gorm.DB, product *Product) error {
	db := connector
	return db.Debug().Unscoped().Model(&product).Association("Categories").Clear()
}

func DeleteAllTagsFromProduct(connector *gorm.DB, product *Product) error {
	db := connector
	return db.Debug().Unscoped().Model(&product).Association("Tags").Clear()
}

/*func AddProductToCategory(connector *gorm.DB, category *Category, product *Product) error {
	db := connector
	return db.Model(&category).Association("Products").Append(product)
}*/

func AddProductToProduct(connector *gorm.DB, product1 *Product, product2 *Product) error {
	db := connector
	return db.Model(&product1).Association("RelatedProducts").Append(product2)
}

func DeleteAllProductsFromProduct(connector *gorm.DB, product *Product) error {
	db := connector
	return db.Debug().Unscoped().Model(&product).Association("RelatedProducts").Clear()
}

func UpdateProduct(connector *gorm.DB, product *Product) error {
	db := connector
	db.Debug().Unscoped().Save(&product)
	return db.Error
}

func DeleteProduct(connector *gorm.DB, product *Product) error {
	db := connector
	db.Debug().Unscoped().Model(&product).Association("Categories").Clear()
	db.Debug().Unscoped().Model(&product).Association("Files").Clear()
	db.Debug().Unscoped().Model(&product).Association("Images").Clear()
	db.Debug().Unscoped().Model(&product).Association("Tags").Clear()
	db.Debug().Unscoped().Delete(&product)
	return db.Error
}