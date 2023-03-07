package config

import (
	"asmr-downloader/utils"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

const ConfigFileName = "config.json"
const MetaDataDb = "asmr.db"

// AsmroneStartPageUrl https://api.asmr.one/api/works?order=create_date&sort=desc&page=1&seed=92&subtitle=0
const AsmrOneStartPageUrl = "https://api.asmr.one"
const Asmr100StartPageUrl = "https://api.asmr-100.com"

var AsmrBaseApiUrl = ""

func init() {
	//访问asmr.one
	client := utils.Client.Get().(*http.Client)
	req, _ := http.NewRequest("GET", "https://asmr.one", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.198 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		fmt.Println("尝试访问asmr.one失败: ", err)
		fmt.Println("当前使用asmr-100.com访问")
		AsmrBaseApiUrl = Asmr100StartPageUrl
	} else {
		fmt.Println("当前使用asmr.one访问...")
		AsmrBaseApiUrl = AsmrOneStartPageUrl
	}
	utils.Client.Put(client)
	defer func() { _ = resp.Body.Close() }()

}

// Config
//
//	Config
//	@Description: 配置结构体
type Config struct {
	Account   string `json:"account"`
	Password  string `json:"password"`
	MaxWorker int    `json:"max_worker"`
	//批量下载次数
	BatchTaskCount int `json:"batch_task_count"`
	//批量下载完后休息多少秒(防止服务器ban你)
	BatchSleepTime int `json:"batch_sleep_time"`
	//是否自动执行 下一个批次
	AutoForNextBatch bool `json:"auto_for_next_batch"`
	//下载目录
	DownloadDir string `json:"download_dir"`
	//元数据库
	MetaDataDb string `json:"meta_data_db"`
	//最大重试次数
	MaxFailedRetry int `json:"max_failed_retry"`
}

// SafePrintInfoStr
//
//	@Description: 格式配置信息
//	@receiver receiver
//	@return string
func (receiver *Config) SafePrintInfoStr() string {
	config := Config{
		Account:          receiver.Account,
		Password:         utils.MosaicStr(receiver.Password, "*"),
		MaxWorker:        receiver.MaxWorker,
		BatchTaskCount:   receiver.BatchTaskCount,
		BatchSleepTime:   receiver.BatchSleepTime,
		AutoForNextBatch: receiver.AutoForNextBatch,
		DownloadDir:      receiver.DownloadDir,
		MetaDataDb:       receiver.MetaDataDb,
		MaxFailedRetry:   receiver.MaxFailedRetry,
	}
	marshal, err := json.Marshal(config)
	if err != nil {
		fmt.Println("序列化配置出错: ", err)
	}
	return string(marshal)
}

// generateDefaultConfig
//
//	@Description: 生成默认配置
func generateDefaultConfig() {
	var customConfig = Config{
		Account:          "guest",
		Password:         "guest",
		MaxWorker:        6,
		BatchTaskCount:   1,
		BatchSleepTime:   2,
		AutoForNextBatch: false,
		DownloadDir:      "data",
		MetaDataDb:       "asmr.db",
		MaxFailedRetry:   3,
	}

	//提示用户输入用户名
	account := utils.PromotForInput("请输入您的账号(默认为guest): ", customConfig.Account)
	customConfig.Account = account
	password := utils.PromotForInput("请输入您的密码(默认为guest): ", customConfig.Password)
	customConfig.Password = password
	maxWorker := utils.PromotForInput("请输入并发下载数(默认为6): ", strconv.Itoa(customConfig.MaxWorker))
	maxWorkerInt, err := strconv.Atoi(maxWorker)
	if err != nil {
		fmt.Println("格式输入错误: ", err)
	}
	customConfig.MaxWorker = maxWorkerInt
	//最大失败文件下载次数
	maxFailedRetry := utils.PromotForInput("请输入文件下载失败时最大重试次数(默认为3): ", strconv.Itoa(customConfig.MaxFailedRetry))
	maxFailedRetryInt, err2 := strconv.Atoi(maxFailedRetry)
	if err2 != nil {
		fmt.Println("格式输入错误: ", err2)
	}
	customConfig.MaxFailedRetry = maxFailedRetryInt

	batchTaskCount := utils.PromotForInput("请输出批量下载作品数量(默认为1): ", strconv.Itoa(customConfig.BatchTaskCount))
	i, err := strconv.Atoi(batchTaskCount)
	if err != nil {
		fmt.Println("格式输入错误: ", err)
	}
	customConfig.BatchTaskCount = i

	batchSleepTime := utils.PromotForInput("请输出批量下载间隔，单位为秒(默认为1): ", strconv.Itoa(customConfig.BatchSleepTime))
	ii, err := strconv.Atoi(batchSleepTime)
	if err != nil {
		fmt.Println("格式输入错误: ", err)
	}
	customConfig.BatchSleepTime = ii

	autoForNextBatch := utils.PromotForInput("是否自动执行下一批次下载(Y/N)(默认为N): ", "N")
	if autoForNextBatch == "Y" {
		customConfig.AutoForNextBatch = true
	} else {
		customConfig.AutoForNextBatch = false
	}
	dowwnloadDir := utils.PromotForInput("请输入数据下载目录(默认为data): ", customConfig.DownloadDir)
	exists := utils.FileOrDirExists(dowwnloadDir)
	if !exists {
		fmt.Println("设置的下载目录不存在,尝试自动生成: " + dowwnloadDir)
		subtitleDir := filepath.Join(dowwnloadDir, "subtitle")
		nosubtitleDir := filepath.Join(dowwnloadDir, "nosubtitle")

		err := os.MkdirAll(subtitleDir, os.ModePerm)
		if err != nil {
			fmt.Println("自动创建下载目录失败: " + subtitleDir)
		}
		err = os.MkdirAll(nosubtitleDir, os.ModePerm)
		if err != nil {
			fmt.Println("自动创建下载目录失败: " + subtitleDir)
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

// GetConfig
//
//	@Description: 获取配置
//	@return *Config
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
