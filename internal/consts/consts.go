package consts

import (
	"os"
	"path/filepath"
	"regexp"
)

// MetaDataDirName 元数据存储目录名。
const MetaDataDirName = ".asmroner-data"

// MetaDataDir 返回元数据存储目录。
// 用户主目录优先；主目录没有配置文件时，兼容当前目录下已有的便携式配置。
func MetaDataDir() string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		homeDir := filepath.Join(home, MetaDataDirName)
		if _, err := os.Stat(filepath.Join(homeDir, ConfigFileName)); err == nil {
			return homeDir
		}

		localDir := filepath.Join(".", MetaDataDirName)
		if _, err := os.Stat(filepath.Join(localDir, ConfigFileName)); err == nil {
			return localDir
		}
		return homeDir
	}

	return filepath.Join(".", MetaDataDirName)
}

// ConfigFileName 配置文件名
const ConfigFileName = "config.toml"

// DbName 数据库文件名
const DbName = "asmroner.db"

// FailedLogName 下载错误日志文件名
const FailedLogName = "download_errors.log"

// DefaultFolderNameFormat 默认的作品下载目录命名格式
// 占位符: {rjid} {date} {subtitle} {title}，与历史版本的硬编码命名保持一致
const DefaultFolderNameFormat = "{rjid}-{date}-{subtitle}-{title}"

// asmr.one id类型
var AsmrOneIDRegex = regexp.MustCompile(`(?i)^(RJ|VJ|BJ|AJ|CJ|DL|NP|AL|KN)\d+$`)

// UserAgent 自定义User-Agent
var UserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36 Edg/139.0.0.0",
	"Mozilla/5.0 (X11; Fedora; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/118.0",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/60.0.6602.1146 Mobile Safari/537.36 (Note: Apple strings can be less specific)",
	"Mozilla/5.0 (Linux; Android 5.0; SM-G900P Build/LRX21T) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/48.0.4893.1163 Mobile Safari/537.36 (Android device)",
}

// AsmrBaseApiUrl 默认 API 地址
const AsmrBaseApiUrl = "https://api.asmr-300.com"

// AsmrApiPath 一些接口路径
var AsmrApiPath = struct {
	WorkinfoPath string `json:"workinfo_path"`
	TracksPath   string `json:"tracks_path"`
	LoginPath    string `json:"login_path"`
	SearchPath   string `json:"search_path"`
	HotPath      string `json:"hot_path"`
	// 同步元数据路径
	SyncMetaPath string `json:"sync_meta_path"`
}{
	WorkinfoPath: "/api/work/",
	TracksPath:   "/api/tracks/",
	LoginPath:    "/api/auth/me",
	SearchPath:   "/api/search/",
	SyncMetaPath: "/api/works?order=release&sort=desc&page=1&pageSize=1",
	HotPath:      "/api/recommender/popular",
}
