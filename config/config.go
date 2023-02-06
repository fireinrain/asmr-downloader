package config

import (
	"asmr-downloader/utils"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
)

const ConfigFileName = "config.json"
const MetaDataDb = "asmr.db"

// AsmroneStartPageUrl https://api.asmr.one/api/works?order=create_date&sort=desc&page=1&seed=92&subtitle=0
const AsmroneStartPageUrl = "https://api.asmr.one"

type Config struct {
	Account   string `json:"account"`
	Password  string `json:"password"`
	MaxWorker int    `json:"max_worker"`
	MaxThread int    `json:"max_thread"`
	//下载目录
	DownloadDir string `json:"download_dir"`
	//元数据库
	MetaDataDb string `json:"meta_data_db"`
}

func (receiver *Config) SafePrintInfoStr() string {
	config := Config{
		Account:     receiver.Account,
		Password:    utils.MosaicStr(receiver.Password, "*"),
		MaxWorker:   receiver.MaxWorker,
		MaxThread:   receiver.MaxThread,
		DownloadDir: receiver.DownloadDir,
	}
	marshal, err := json.Marshal(config)
	if err != nil {
		fmt.Println("序列化配置出错: ", err)
	}
	return string(marshal)
}

func generateDefaultConfig() {
	var customConfig = Config{
		Account:     "guest",
		Password:    "guest",
		MaxWorker:   1,
		MaxThread:   1,
		DownloadDir: "data",
		MetaDataDb:  "asmr.db",
	}

	//提示用户输入用户名
	account := utils.PromotForInput("请输入您的账号(默认为guest): ", customConfig.Account)
	customConfig.Account = account
	password := utils.PromotForInput("请输入您的密码(默认为guest): ", customConfig.Password)
	customConfig.Password = password
	maxWorker := utils.PromotForInput("请输入并发下载数(默认为1): ", strconv.Itoa(customConfig.MaxWorker))
	maxWorkerInt, err := strconv.Atoi(maxWorker)
	if err != nil {
		fmt.Println("格式输入错误: ", err)
	}
	customConfig.MaxWorker = maxWorkerInt
	dowwnloadDir := utils.PromotForInput("请输入数据下载目录(默认为data): ", customConfig.DownloadDir)
	exists := utils.FileOrDirExists(dowwnloadDir)
	if !exists {
		fmt.Println("设置的下载目录不存在,尝试自动生成: " + dowwnloadDir)
		err := os.MkdirAll(dowwnloadDir, os.ModePerm)
		if err != nil {
			fmt.Println("自动创建下载目录失败: " + dowwnloadDir)
		}
	}
	customConfig.DownloadDir = dowwnloadDir

	config, err := json.Marshal(customConfig)
	if err != nil {
		fmt.Print("序列化配置出错: ", err)
		os.Exit(0)
	}
	_ = os.WriteFile("config.json", config, 0644)
	fmt.Println("已生成配置文件config.json, 如果您之前不做任何输入，默认以访客模式访问。")
	//os.Exit(0)
}

func GetConfig() *Config {
	if _, err := os.Stat("config.json"); os.IsNotExist(err) {
		generateDefaultConfig()
	}
	file, err := os.Open("config.json")
	if err != nil {
		fmt.Println("打开配置文件失败", err)
		os.Exit(0)
	}
	defer func() { _ = file.Close() }()
	all, err := io.ReadAll(file)
	if err != nil {
		fmt.Println("读取配置文件失败", err)
		os.Exit(0)
	}
	var config Config
	err = json.Unmarshal(all, &config)
	if err != nil {
		fmt.Println("解析配置文件失败", err)
		os.Exit(0)
	}
	return &config
}
