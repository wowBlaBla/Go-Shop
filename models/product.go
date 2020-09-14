package models

import "gorm.io/gorm"

type Product struct {
	gorm.Model
	Name string `gorm:"size:255;index:idx_name,unique"`
	Title string
	Description string
	Thumbnail string
	Categories []*Category `gorm:"many2many:category_products;"`
	Offers []*Offer `gorm:"foreignKey:ProductId"` // Extend Product with extra variations of it affect to price or availability
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