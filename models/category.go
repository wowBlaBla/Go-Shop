package models

import (
	"gorm.io/gorm"
)

type Category struct {
	gorm.Model
	Name string `gorm:"size:255;index:idx_name,unique"`
	Title string
	Description string
	Thumbnail string
	Products []*Product `gorm:"many2many:categories_products;"`
	Options []*Option `gorm:"many2many:categories_options;"`
	//
	Parent *Category `gorm:"foreignKey:ParentId"`
	ParentId uint
}

func CreateCategory(connector *gorm.DB, category *Category) (uint, error) {
	db := connector
	db.Debug().Create(&category)
	if err := db.Error; err != nil {
		return 0, err
	}
	return category.ID, nil
}

func GetCategoriesFromCategory(connector *gorm.DB, category *Category) []*Category {
	db := connector
	var categories []*Category
	db.Where("ParentId = ?", category.ID).Find(&categories)
	return categories
}

func GetParentFromCategory(connector *gorm.DB, category *Category) *Category {
	db := connector
	var parent Category
	db.Where("id = ?", category.ParentId).First(&parent)
	return &parent
}

func GetProductsFromCategory(connector *gorm.DB, category *Category) ([]*Product, error) {
	db := connector
	var products []*Product
	if err := db.Model(&category).Association("Products").Find(&products); err != nil {
		return nil, err
	}
	return products, nil
}

func GetSubcategoriesFromCategory(connector *gorm.DB, category *Category) ([]*Category, error) {
	db := connector
	var subcategories []*Category
	if err := db.Model(&category).Association("Subcategories").Find(&subcategories); err != nil {
		return nil, err
	}
	return subcategories, nil
}

/*func AddSubcategoryToCategory(connector *gorm.DB, category *Category, subcategory *Category) error {
	db := connector
	return db.Model(&category).Association("Subcategories").Append(subcategory)
}*/

func DeleteSubcategoryFromCategory(connector *gorm.DB, category *Category, subcategory *Category) error {
	db := connector
	return db.Model(&category).Association("Subcategories").Delete(subcategory)
}

func AddProductToCategory(connector *gorm.DB, category *Category, product *Product) error {
	db := connector
	return db.Model(&category).Association("Products").Append(product)
}

func DeleteProductFromCategory(connector *gorm.DB, category *Category, product *Product) error {
	db := connector
	return db.Model(&category).Association("Products").Delete(product)
}

func AddOptionToCategory(connector *gorm.DB, category *Category, option *Option) error {
	db := connector
	return db.Model(&category).Association("Options").Append(option)
}

func DeleteOptionFromCategory(connector *gorm.DB, category *Category, option *Option) error {
	db := connector
	return db.Model(&category).Association("Options").Delete(option)
}