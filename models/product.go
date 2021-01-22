package models

import (
	"gorm.io/gorm"
)

type Product struct {
	gorm.Model
	Enabled bool
	Name        string `gorm:"size:255;index:idx_product_name,unique"`
	Title       string
	Description string
	Thumbnail   string
	Parameters  []*Parameter `gorm:"foreignKey:ProductId"`
	Content 	string
	// ONLY TO USE AS DEFAULT VALUE FOR VIRIATIONS
	BasePrice float64          `sql:"type:decimal(8,2);"`
	//
	Categories  []*Category  `gorm:"many2many:categories_products;"`
	Variations  []*Variation `gorm:"foreignKey:ProductId"`
	Files       []*File     `gorm:"many2many:products_files;"`
	Images      []*Image     `gorm:"many2many:products_images;"`
	Tags        []*Tag `gorm:"many2many:products_tags;"`
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

func GetProductsByCategoryId(connector *gorm.DB, id uint) ([]*Product, error) {
	db := connector
	var products []*Product
	db.Model(&Product{}).Joins("inner join categories_products on categories_products.product_id = products.id").Where("categories_products.category_id = ?", id).Find(&products)
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
	if err := db.Preload("Categories").Preload("Parameters").Preload("Parameters.Option").Preload("Parameters.Value").Preload("Files").Preload("Images").Preload("Variations").Preload("Variations.Properties").Preload("Variations.Properties.Option").Preload("Variations.Properties.Prices").Preload("Variations.Properties.Prices.Value").Preload("Tags").First(&product, id).Error; err != nil {
		return nil, err
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

func AddFileToProduct(connector *gorm.DB, product *Product, file *File) error {
	db := connector
	return db.Model(&product).Association("Files").Append(file)
}

func AddImageToProduct(connector *gorm.DB, product *Product, image *Image) error {
	db := connector
	return db.Model(&product).Association("Images").Append(image)
}

func DeleteAllCategoriesFromProduct(connector *gorm.DB, product *Product) error {
	db := connector
	return db.Debug().Unscoped().Model(&product).Association("Categories").Clear()
}

func DeleteAllTagsFromProduct(connector *gorm.DB, product *Product) error {
	db := connector
	return db.Debug().Unscoped().Model(&product).Association("Tags").Clear()
}

func UpdateProduct(connector *gorm.DB, product *Product) error {
	db := connector
	db.Debug().Unscoped().Save(&product)
	return db.Error
}

func DeleteProduct(connector *gorm.DB, product *Product) error {
	db := connector
	db.Debug().Unscoped().Delete(&product)
	return db.Error
}