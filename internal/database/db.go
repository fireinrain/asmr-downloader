package database

import (
	"asmroner/internal/consts"
	"asmroner/internal/model"
	"os"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var Database *gorm.DB

func InitDB() (*gorm.DB, error) {
	// 使用 SQLite 存储状态，文件名为 meta.db
	dbPath := consts.MetaDataDir + string(os.PathSeparator) + consts.DbName
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // Silent 不打印
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
