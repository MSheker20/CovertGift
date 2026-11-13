package models

import "gorm.io/gorm"

// Migrate 5 temel tabloyu, bildirimleri, grup kapaklarını ve ilişkilerini AutoMigrate ile senkronize eder.
// API ve seed komutu aynı şemayı kullanır.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&User{}, &Group{}, &GroupMember{}, &GiftIdea{}, &Vote{}, &Notification{}, &GroupCover{})
}
