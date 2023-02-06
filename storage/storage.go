package storage

import "asmr-downloader/config"

type SqliteStoreEngine struct {
}

func NewCon(dbFilePath string) SqliteStoreEngine {
	return SqliteStoreEngine{}
}

var StoreDb = NewCon(config.ConfigFileName)
