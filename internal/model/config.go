package model

import (
	"github.com/spf13/viper"
)

type User struct {
	// 必须用 mapstructure
	Account  string `mapstructure:"account"`
	Password string `mapstructure:"password"`
}

type Downloader struct {
	ApiUrl         string `mapstructure:"api_url"`
	ProxyUrl       string `mapstructure:"proxy_url"`
	MaxWorkers     int    `mapstructure:"max_workers"`
	MaxRetries     int    `mapstructure:"max_retries"`
	SyncDataFolder string `mapstructure:"sync_data_folder"`
	SyncWantedSize string `mapstructure:"sync_wanted_size"`
	PreferMedia    string `mapstructure:"prefer_media"`
	// 扩展名白名单，如 ".mp3,.jpg"（逗号分隔，可不带点，大小写不敏感）；空不筛选
	// 支持嵌套扩展名：".vtt" 命中所有字幕，".mp3.vtt" 只命中 mp3 的字幕
	IncludeExt string `mapstructure:"include_ext"`
	// 扩展名黑名单，如 ".mp4,.webm"；空不过滤，与白名单同时配置时先白后黑
	ExcludeExt string `mapstructure:"exclude_ext"`
	IdmPath    string `mapstructure:"idm_path"`
	// 下载目录命名格式，占位符: {rjid} {date} {subtitle} {title}；空用默认值
	FolderNameFormat string `mapstructure:"folder_name_format"`
}

type Limit struct {
	SyncQPS           float64 `mapstructure:"sync_qps"`
	SyncJitterMin     int     `mapstructure:"sync_jitter_min"`
	SyncJitterMax     int     `mapstructure:"sync_jitter_max"`
	DownloadQPS       float64 `mapstructure:"download_qps"`
	DownloadJitterMin int     `mapstructure:"download_jitter_min"`
	DownloadJitterMax int     `mapstructure:"download_jitter_max"`
}

type Config struct {
	User       User       `mapstructure:"user"`
	Downloader Downloader `mapstructure:"downloader"`
	Limit      Limit      `mapstructure:"limit"`
}

// AppConfig 全局变量
var AppConfig *Config

func NewDefaultConfig() *Config {
	AppConfig = &Config{}
	return AppConfig
}

// LoadConfig 读取配置
func LoadConfig(configPath string) (*Config, error) {
	viper.SetConfigName("config") // 文件名 config
	viper.SetConfigType("toml")
	viper.AddConfigPath(configPath)

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}
	config := NewDefaultConfig()
	if err := viper.Unmarshal(config); err != nil {
		return nil, err
	}
	AppConfig = config
	return AppConfig, nil
}
