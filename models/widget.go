package models

import (
	"gorm.io/gorm"
)

type Widget struct {
	gorm.Model
	Enabled bool
	Name string
	Title string
	Description string `json:",omitempty"`
	Content string
	//
	Location string
	ApplyTo string
	Categories  []*Category  `gorm:"many2many:categories_widgets;"`
	Products  []*Product  `gorm:"many2many:products_widgets;"`
}

func GetWidgets(connector *gorm.DB) ([]*Widget, error) {
	db := connector
	var widgets []*Widget
	db.Debug().Find(&widgets)
	if err := db.Error; err != nil {
		return nil, err
	}
	return widgets, nil
}

func GetWidgetsByApplyTo(connector *gorm.DB, applyTo string) ([]*Widget, error) {
	db := connector
	var widgets []*Widget
	if err := db.Where("apply_to = ?", applyTo).Find(&widgets).Error; err != nil {
		return nil, err
	}
	return widgets, nil
}

func GetWidgetsByCategory(connector *gorm.DB, categoryId uint) ([]*Widget, error) {
	db := connector
	var widgets []*Widget
	if err := db.Where("apply_to = ? and categories_widgets.category_id = ?", "categories", categoryId).Joins("left join categories_widgets on categories_widgets.widget_id = widgets.id").Find(&widgets).Error; err != nil {
		return nil, err
	}
	return widgets, nil
}

func GetWidgetsByProduct(connector *gorm.DB, productId uint) ([]*Widget, error) {
	db := connector
	var widgets []*Widget
	if err := db.Where("apply_to = ? and products_widgets.product_id = ?", "products", productId).Joins("left join products_widgets on products_widgets.widget_id = widgets.id").Find(&widgets).Error; err != nil {
		return nil, err
	}
	return widgets, nil
}

func CreateWidget(connector *gorm.DB, widget *Widget) (uint, error) {
	db := connector
	db.Debug().Create(&widget)
	if err := db.Error; err != nil {
		return 0, err
	}
	return widget.ID, nil
}

func GetWidget(connector *gorm.DB, id int) (*Widget, error) {
	db := connector
	var widget Widget
	if err := db.Preload("Categories").Preload("Products").Where("id = ?", id).First(&widget).Error; err != nil {
		return nil, err
	}
	return &widget, nil
}

func AddCategoryToWidget(connector *gorm.DB, widget *Widget, category *Category) error {
	db := connector
	return db.Model(&widget).Association("Categories").Append(category)
}

func DeleteAllCategoriesFromWidget(connector *gorm.DB, widget *Widget) error {
	db := connector
	return db.Debug().Unscoped().Model(&widget).Association("Categories").Clear()
}

func AddProductToWidget(connector *gorm.DB, widget *Widget, product *Product) error {
	db := connector
	return db.Model(&widget).Association("Products").Append(product)
}

func DeleteAllProductsFromWidget(connector *gorm.DB, widget *Widget) error {
	db := connector
	return db.Debug().Unscoped().Model(&widget).Association("Products").Clear()
}

func UpdateWidget(connector *gorm.DB, widget *Widget) error {
	db := connector
	db.Debug().Save(&widget)
	return db.Error
}

func DeleteWidget(connector *gorm.DB, widget *Widget) error {
	db := connector
	db.Unscoped().Debug().Delete(&widget)
	return db.Error
}