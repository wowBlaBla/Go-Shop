package models

import "gorm.io/gorm"

type Product struct {
	gorm.Model
	Name string `gorm:"size:255;index:idx_name,unique"`
	Title string
	Description string
	Thumbnail string
	Categories []*Category `gorm:"many2many:categories_products;"`
	//
	// Optionally use "Offers" for final count of product variations
	Offers []*Offer `gorm:"foreignKey:ProductId"`
	// OR use "Properties" for endless number of product variations
	//Properties []*ProductProperty `gorm:"foreignKey:ProductId"`
	// For example:
	// (1) Offers - iPhone have final number of variations in case of iPhone 11 it is color and storage:
	// 'black' and '64Gb'
	// 'black' and '256Gb'
	// 'black' and '512Gb'
	// 'black' and '1024Gb'
	// 'white' and '64Gb'
	// ...
	// 'red' and '1024Gb'
	// You should to create 'Offer' for each of such combination and use it.
	// (2) Properties - Furniture has huge number of variations depend on material, color, texture. Each of such 'option' linearly effect to price y increasing or decreasing value, if body color:
	// 'black' $+75
	// 'white' $+150
	// plate color:
	// 'black' $+50
	// 'white' $+100
	// ...
	// final price is build like constructor
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