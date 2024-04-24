package storage

import (
	"database/sql"
	"sync"

	"go.uber.org/zap"
	_ "modernc.org/sqlite"

	"asmr-downloader/config"
	"asmr-downloader/log"
)

var StoreDb *SqliteStoreEngine

var once sync.Once

// GetDbInstance
//
//	@Description: 单例存储实例
//	@return *SqliteStoreEngine
func GetDbInstance() *SqliteStoreEngine {
	db, err := sql.Open("sqlite", config.MetaDataDb)
	if err != nil {
		log.AsmrLog.Error("", zap.String("error", err.Error()))
		return nil
	}
	//defer db.Close()
	once.Do(func() {
		StoreDb = &SqliteStoreEngine{
			DbFilePath: config.MetaDataDb,
			Db:         db,
		}
		//初始化db
		err := StoreDb.initDbTables()
		if err != nil {
			log.AsmrLog.Error("数据库表初始化失败: ", zap.String("error", err.Error()))
		}
	})
	return StoreDb
}

// SqliteStoreEngine
//
//	@Description: sqlite holder
type SqliteStoreEngine struct {
	//db文件路径
	DbFilePath string
	//db指针
	Db *sql.DB
}

// initDbTables
//
//	@Description: 初始化db数据库表结构
//	@receiver receiver
//	@return error
func (receiver *SqliteStoreEngine) initDbTables() error {
	_, err := receiver.Db.Exec(`
		CREATE TABLE IF NOT EXISTS [item_product] (
		  [id] TEXT PRIMARY KEY,
		  [title] TEXT,
		  [circle_id] INT,
		  [name] TEXT,
		  [nsfw] INT,
		  [release] TEXT,
		  [dl_count] INT,
		  [price] INT,
		  [source_type] TEXT,
		  [source_id] TEXT,
		  [source_url] TEXT,
		  [review_count] INT,
		  [rate_count] INT,
		  [rate_average_2dp] REAL,
		  [rate_count_detail] TEXT,
		  [rank] TEXT,
		  [has_subtitle] INT,
		  [create_date] TEXT,
		  [vas] TEXT,
		  [tags] TEXT,
		  [userRating] TEXT NULL,
		  [circle.id] INT,
		  [circle.name] TEXT,
		  [samCoverUrl] TEXT,
		  [thumbnailCoverUrl] TEXT,
		  [mainCoverUrl] TEXT
		);`)

	_, _ = receiver.Db.Exec(`
		
	CREATE TABLE if not exists asmr_download (id integer PRIMARY KEY autoincrement,
                                                   rjid text ,
                                                             item_prod_id text ,
                                                                                  download_flag integer default 0, title text,subtitle_flag integer);
-- 
	CREATE INDEX asmr_download__index_item_prod_id ON asmr_download (item_prod_id);
    CREATE INDEX asmr_download__index_rjid ON asmr_download (rjid);
	`)

	return err
}
