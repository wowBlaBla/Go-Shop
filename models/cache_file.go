package models

import "gorm.io/gorm"

type CacheFile struct {
	gorm.Model
	FileId uint
	Name string
	File string
}

func (CacheFile) TableName() string {
	return "cache_files"
}

func GetCacheFileByFileId(connector *gorm.DB, fileId uint) (*CacheFile, error){
	db := connector
	var cacheFile CacheFile
	db.Where("file_id = ?", fileId).First(&cacheFile)
	return &cacheFile, db.Error
}

func CreateCacheFile(connector *gorm.DB, file *CacheFile) (uint, error) {
	db := connector
	db.Create(&file)
	if err := db.Error; err != nil {
		return 0, err
	}
	return file.ID, nil
}

func DeleteCacheFileByFileId(connector *gorm.DB, fileId uint) error {
	db := connector
	db.Unscoped().Where("file_id = ?", fileId).Delete(&CacheFile{})
	if err := db.Error; err != nil {
		return err
	}
	return nil
}