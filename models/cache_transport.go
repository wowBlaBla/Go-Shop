package models

import "gorm.io/gorm"

type CacheTransport struct {
	gorm.Model
	TransportID   uint
	Name       string
	Title       string
	Thumbnail   string
	Value        string
}

func (CacheTransport) TableName() string {
	return "cache_transport"
}

func CreateCacheTransport(connector *gorm.DB, value *CacheTransport) (uint, error) {
	db := connector
	db.Create(&value)
	if err := db.Error; err != nil {
		return 0, err
	}
	return value.ID, nil
}

func GetCacheTransportByTransportId(connector *gorm.DB, transportId uint) (*CacheTransport, error){
	db := connector
	var cacheTransport CacheTransport
	db.Where("transport_id = ?", transportId).First(&cacheTransport)
	return &cacheTransport, db.Error
}

func HasCacheTransportByTransportId(connector *gorm.DB, transportId uint) bool {
	db := connector
	var count int64
	db.Model(CacheValue{}).Where("transport_id = ?", transportId).Count(&count)
	return count > 0
}