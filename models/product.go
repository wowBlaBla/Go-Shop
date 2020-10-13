package models

import "gorm.io/gorm"

type Product struct {
	gorm.Model
	Name string `gorm:"size:255;index:idx_name,unique"`
	Title string
	Description string
	Thumbnail string
	Categories []*Category `gorm:"many2many:categories_products;"`
	Offers []*Offer `gorm:"foreignKey:ProductId"`
	Images []*Image `gorm:"many2many:products_images;"`
}

func SearchProducts(connector *gorm.DB, term string, limit int) ([]*Product, error) {
	db := connector
	var products []*Product
	db.Debug().Preload("Categories").Preload("Offers").Where("name like ? OR title like ? OR description like ?", term, term, term).Limit(limit).Find(&products)
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
	db.Preload("Images").Preload("Offers").Preload("Offers.Properties").Preload("Offers.Properties.Option").Preload("Offers.Properties.Prices").Preload("Offers.Properties.Prices.Value").Find(&product, id)
	if err := db.Error; err != nil {
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

func GetProductOffers(connector *gorm.DB, id int) ([]*Offer, error) {
	db := connector
	var product Product
	db.Debug().Preload("Offers").Preload("Offers.Properties").Preload("Offers.Properties.Option").Preload("Offers.Properties.Prices").Preload("Offers.Properties.Prices.Value").Find(&product, id)
	if err := db.Error; err != nil {
		return nil, err
	}
	return product.Offers, nil
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

func GetOffersFromProduct(connector *gorm.DB, product *Product) ([]*Offer, error) {
	db := connector
	var offers []*Offer
	if err := db.Model(&product).Association("Offers").Find(&offers); err != nil {
		return nil, err
	}
	return offers, nil
}

func AddOfferToProduct(connector *gorm.DB, product *Product, offer *Offer) error {
	db := connector
	return db.Model(&product).Association("Offers").Append(offer)
}

func AddImageToProduct(connector *gorm.DB, product *Product, image *Image) error {
	db := connector
	return db.Model(&product).Association("Images").Append(image)
}

func DeleteImageFromProduct(connector *gorm.DB, product *Product, image *Image) error {
	db := connector
	return db.Model(&product).Association("Images").Delete(image)
}

func UpdateProduct(connector *gorm.DB, product *Product) error {
	db := connector
	db.Debug().Save(&product)
	return db.Error
}