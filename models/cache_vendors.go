package models

import "gorm.io/gorm"

type CacheVendor struct {
	gorm.Model
	VendorID   uint
	Title       string
	Name       string
	Thumbnail   string
}

func (CacheVendor) TableName() string {
	return "cache_vendors"
}

func CreateCacheVendor(connector *gorm.DB, value *CacheVendor) (uint, error) {
	db := connector
	db.Create(&value)
	if err := db.Error; err != nil {
		return 0, err
	}
	return value.ID, nil
}

func GetCacheVendorByVendorId(connector *gorm.DB, valueId uint) (*CacheVendor, error){
	db := connector
	var cacheVendor CacheVendor
	db.Where("vendor_id = ?", valueId).First(&cacheVendor)
	return &cacheVendor, db.Error
}

func HasCacheVendorByVendorId(connector *gorm.DB, valueId uint) bool {
	db := connector
	var count int64
	db.Model(CacheVendor{}).Where("vendor_id = ?", valueId).Count(&count)
	return count > 0
}

func DeleteCacheVendorByVendorId(connector *gorm.DB, vendorId uint) error {
	db := connector
	db.Unscoped().Where("vendor_id = ?", vendorId).Delete(&CacheVendor{})
	if err := db.Error; err != nil {
		return err
	}
	return nil
}