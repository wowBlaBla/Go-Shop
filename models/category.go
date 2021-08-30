package models

import (
	"fmt"
	"github.com/yonnic/goshop/common"
	"gorm.io/gorm"
	"math/rand"
	"path"
	"strings"
	"time"
)

type Category struct {
	gorm.Model
	Name          string
	Title         string
	Description   string
	Thumbnail     string
	Content       string
	Products      []*Product `gorm:"many2many:categories_products;"`
	Customization string
	Sort int
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
	db.Where("parent_id = ?", id).Order("Title asc").Find(&children)
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
	return db.Debug().Model(&category).Association("Products").Append(product)
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

/**/

type CatalogView []CatalogItemView

type CatalogItemView struct {
	ID uint
	Name string
	Path string
	Title string
	Thumbnail string            `json:",omitempty"`
	Description string          `json:",omitempty"`
	Type string                 `json:",omitempty"` // "category", "product"
	Children []*CatalogItemView `json:",omitempty"`
	Parents []*CatalogItemView  `json:",omitempty"`
	BasePrice float64 			`json:",omitempty"`
	Products int64              `json:",omitempty"`
	Count int                   `json:",omitempty"`
	Sold int                   `json:",omitempty"`
	Sort int                    `json:",omitempty"`
}

func (c *CatalogItemView) SetCount() {

}

func GetCategoriesView(connector *gorm.DB, id int, depth int, noProducts bool, count bool, stat bool) (*CatalogItemView, error) {
	productsCache := make(map[uint]string)
	//
	if id == 0 {
		return getChildrenCategoriesView(productsCache, connector, &CatalogItemView{Path: "/", Name: strings.ToLower(common.Config.Products), Title: common.Config.Products, Type: "category"}, depth, noProducts, count, stat), nil
	} else {
		if category, err := GetCategory(connector, id); err == nil {
			chunks := &[]string{}
			if err := getCategoryPath(connector, int(category.ParentId), chunks); err != nil {
				return nil, err
			}
			view := getChildrenCategoriesView(productsCache, connector, &CatalogItemView{ID: category.ID, Path: fmt.Sprintf("/%s", strings.Join(*chunks, "/")), Name: category.Name, Title: category.Title, Thumbnail: category.Thumbnail, Description: category.Description, Type: "category", Sort: category.Sort}, depth, noProducts, count, stat)
			if view != nil {
				if err = getParentCategoriesView(connector, view, category.ParentId); err != nil {
					return nil, err
				}
			}
			return view, nil
		}else{
			return nil, err
		}
	}
}

func getCategoryPath(connector *gorm.DB, pid int, chunks *[]string) error {
	if pid == 0 {
		return nil
	} else {
		if category, err := GetCategory(connector, pid); err == nil {
			*chunks = append([]string{category.Name}, *chunks...)
			return getCategoryPath(connector, int(category.ParentId), chunks)
		} else {
			return err
		}
	}
}

func getChildrenCategoriesView(productsCache map[uint]string, connector *gorm.DB, root *CatalogItemView, depth int, noProducts bool, count bool, stat bool) *CatalogItemView {
	for _, category := range GetChildrenOfCategoryById(connector, root.ID) {
		if category.Thumbnail != "" {
			if v, found := common.THUMBNAILS.Get(category.Thumbnail); !found {
				if cache, err := GetCacheCategoryByCategoryId(common.Database, category.ID); err == nil {
					common.THUMBNAILS.Set(category.Thumbnail, cache.Thumbnail, time.Duration(60 + rand.Intn(60)) * time.Second)
					category.Thumbnail = cache.Thumbnail
				}else{
					common.THUMBNAILS.Set(category.Thumbnail, "", 3 * time.Second)
				}
			}else if thumbnail := v.(string); thumbnail != "" {
				category.Thumbnail = thumbnail
			}
		}
		if depth > 0 {
			child := getChildrenCategoriesView(productsCache, connector, &CatalogItemView{ID: category.ID, Path: path.Join(root.Path, root.Name), Name: category.Name, Title: category.Title, Thumbnail: category.Thumbnail, Description: category.Description, Type: "category", Sort: category.Sort}, depth - 1, noProducts, count, stat)
			root.Children = append(root.Children, child)
			if count {
				root.Count += child.Count
			}
		}
	}
	if count || !noProducts {
		if products, err := GetProductsByCategoryId(connector, root.ID); err == nil {
			for _, product := range products {
				if product.Thumbnail != "" {
					if v, found := common.THUMBNAILS.Get(product.Thumbnail); !found {
						if cache, err := GetCacheProductByProductId(common.Database, product.ID); err == nil {
							common.THUMBNAILS.Set(product.Thumbnail, cache.Thumbnail, time.Duration(60 + rand.Intn(60)) * time.Second)
							product.Thumbnail = cache.Thumbnail
						}else{
							common.THUMBNAILS.Set(product.Thumbnail, "", 3 * time.Second)
						}
					}else if thumbnail := v.(string); thumbnail != "" {
						product.Thumbnail = thumbnail
					}
				}
				if !noProducts {
					item := &CatalogItemView{ID: product.ID, Path: path.Join(root.Path, root.Name), Name: product.Name, Title: product.Title, Thumbnail: product.Thumbnail, Description: product.Description, BasePrice: product.BasePrice, Type: "product"}
					if stat {
						if count, err := GetItemsCountByProductId(common.Database, product.ID); err == nil {
							item.Sold = int(count)
						}
					}
					root.Children = append(root.Children, item)
				}
			}
			if count {
				root.Products = int64(len(products))
				root.Count += len(products)
			}
		}
	}
	return root
}

func getParentCategoriesView(connector *gorm.DB, node *CatalogItemView, pid uint) error {
	if pid == 0 {
		node.Parents = append([]*CatalogItemView{{Path: "/", Name: strings.ToLower(common.Config.Products), Title: common.Config.Products}}, node.Parents...)
	} else {
		if category, err := GetCategory(connector, int(pid)); err == nil {
			node.Parents = append([]*CatalogItemView{{ID: category.ID, Name: category.Name, Title: category.Title, Thumbnail: category.Thumbnail, Description: category.Description}}, node.Parents...)
			return getParentCategoriesView(connector, node, category.ParentId)
		} else {
			return err
		}
	}
	return nil
}