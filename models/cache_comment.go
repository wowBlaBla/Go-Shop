package models

import "gorm.io/gorm"

type CacheComment struct {
	gorm.Model
	CommentID  uint
	Title      string
	Body       string
	Max 	   int
	Images     string
}

func (CacheComment) TableName() string {
	return "cache_comments"
}

func CreateCacheComment(connector *gorm.DB, value *CacheComment) (uint, error) {
	db := connector
	db.Create(&value)
	if err := db.Error; err != nil {
		return 0, err
	}
	return value.ID, nil
}

func GetCacheCommentByCommentId(connector *gorm.DB, commentId uint) (*CacheComment, error){
	db := connector
	var cacheComment CacheComment
	db.Where("comment_id = ?", commentId).First(&cacheComment)
	return &cacheComment, db.Error
}

func HasCacheCommentByCommentId(connector *gorm.DB, commentId uint) bool {
	db := connector
	var count int64
	db.Model(CacheComment{}).Where("comment_id = ?", commentId).Count(&count)
	return count > 0
}