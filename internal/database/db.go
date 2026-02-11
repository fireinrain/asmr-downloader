package database

import (
	"asmroner/internal/consts"
	"asmroner/internal/model"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var Database *gorm.DB

func InitDB() (*gorm.DB, error) {
	dbPath := filepath.Join(consts.MetaDataDir, consts.DbName)
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})

	if err != nil {
		return nil, err
	}
	// 自动建表
	db.AutoMigrate(&model.MetadataWork{}, &model.WorkSyncInfo{})
	Database = db
	return db, nil
}

func NewInMemoryDb() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	// 自动建表
	//db.AutoMigrate(&model.XXXX{})
	return db, nil
}
