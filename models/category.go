package models

import (
	"gorm.io/gorm"
	"time"
)

type Category struct {
	gorm.Model
	Name string `gorm:"size:255;index:idx_category_name,unique"`
	Title string
	Description string
	Thumbnail string
	Content string
	Products []*Product `gorm:"many2many:categories_products;"`
	//
	Parent *Category `gorm:"foreignKey:ParentId"`
	ParentId uint
}

func GetCategories(connector *gorm.DB) ([]*Category, error) {
	db := connector
	var categories []*Category
	db.Debug().Find(&categories)
	if err := db.Error; err != nil {
		return nil, err
	}
	return categories, nil
}

func GetCategory(connector *gorm.DB, id int) (*Category, error) {
	db := connector
	var category Category
	if err := db.Where("id = ?", id).First(&category).Error; err != nil {
		return nil, err
	}
	return &category, nil
}

func GetCategoryByName(connector *gorm.DB, name string) (*Category, error) {
	db := connector
	var category Category
	if err := db.Where("name = ?", name).First(&category).Error; err != nil {
		return nil, err
	}
	return &category, nil
}

func CreateCategory(connector *gorm.DB, category *Category) (uint, error) {
	db := connector
	db.Debug().Create(&category)
	if err := db.Error; err != nil {
		return 0, err
	}
	return category.ID, nil
}

func UpdateCategory(connector *gorm.DB, category *Category) error {
	db := connector
	db.Debug().Save(&category)
	return db.Error
}

func GetBreadcrumbs(connector *gorm.DB, categoryId uint) []*Category {
	breadcrumbs := &[]*Category{}
	var f3 func(connector *gorm.DB, id uint)
	f3 = func(connector *gorm.DB, id uint) {
		if id != 0 {
			if category, err := GetCategory(connector, int(id)); err == nil {
				if category.Thumbnail == "" {
					if len(*breadcrumbs) > 0 {
						category.Thumbnail = (*breadcrumbs)[0].Thumbnail
					}
				}
				*breadcrumbs = append([]*Category{category}, *breadcrumbs...)
				f3(connector, category.ParentId)
			}
		}
	}
	f3(connector, categoryId)
	*breadcrumbs = append([]*Category{{Name: "products", Title: "Products", Model: gorm.Model{UpdatedAt: time.Now()}}}, *breadcrumbs...)
	return *breadcrumbs
}

func GetChildrenCategories(connector *gorm.DB, category *Category) []*Category {
	categories := &[]*Category{}
	getChildrenCategories(connector, category.ID, categories)
	return *categories
}

func getChildrenCategories(connector *gorm.DB, id uint, categories *[]*Category) {
	for _, category := range GetChildrenOfCategoryById(connector, id) {
		getChildrenCategories(connector, category.ID, categories)
		*categories = append(*categories, category)
	}
}

func GetRootCategories(connector *gorm.DB) []*Category {
	db := connector
	var categories []*Category
	db.Where("parent_id = ?", 0).Find(&categories)
	return categories
}

func GetCategoriesFromCategory(connector *gorm.DB, category *Category) []*Category {
	db := connector
	var categories []*Category
	var id uint
	if category != nil {
		id = category.ID
	}
	db.Where("parent_id = ?", id).Find(&categories)
	return categories
}

func GetParentFromCategory(connector *gorm.DB, category *Category) *Category {
	db := connector
	var parent Category
	db.Where("id = ?", category.ParentId).First(&parent)
	return &parent
}

func GetChildrenOfCategory(connector *gorm.DB, category *Category) []*Category {
	db := connector
	var children []*Category
	db.Where("parent_id = ?", category.ID).Find(&children)
	return children
}

func GetChildrenOfCategoryById(connector *gorm.DB, id uint) []*Category {
	db := connector
	var children []*Category
	db.Where("parent_id = ?", id).Find(&children)
	return children
}

func GetProductsFromCategory(connector *gorm.DB, category *Category) ([]*Product, error) {
	db := connector
	var products []*Product
	if err := db.Debug().Model(&category).Association("Products").Find(&products); err != nil {
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

func AddProductToTag(connector *gorm.DB, tag *Tag, product *Product) error {
	db := connector
	return db.Model(&tag).Association("Products").Append(product)
}

/*func AddOptionToCategory(connector *gorm.DB, category *Category, option *Option) error {
	db := connector
	return db.Model(&category).Association("Options").Append(option)
}

func DeleteOptionFromCategory(connector *gorm.DB, category *Category, option *Option) error {
	db := connector
	return db.Model(&category).Association("Options").Delete(option)
}*/

func DeleteCategory(connector *gorm.DB, category *Category) error {
	db := connector
	db.Debug().Unscoped().Delete(&category)
	return db.Error
}