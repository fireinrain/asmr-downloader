package config

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"go.uber.org/zap"

	"asmr-downloader/log"
	"asmr-downloader/utils"
)

const ConfigFileName = "config.json"
const MetaDataDb = "asmr.db"

// AsmroneStartPageUrl https://api.asmr.one/api/works?order=create_date&sort=desc&page=1&seed=92&subtitle=0
// const AsmrOneStartPageUrl = "https://api.asmr.one"
//const Asmr100StartPageUrl = "https://api.asmr-100.com"
//const Asmr200StartPageUrl = "https://api.asmr-200.com"

var AsmrBaseApiUrl = ""

func init() {
	//访问asmr.one
	url := GetRespFastestSiteUrl()
	AsmrBaseApiUrl = url
}

// GetAsmrLatestUrls
//
//	@Description: 获取asmr.one最新域名列表
//	@return []string
//	@return error
func GetAsmrLatestUrls() ([]string, error) {
	//访问asmr.one 最新域名发布页
	// official : https://as.mr
	// cf worker proxy: https://as.131433.xyz
	var officialPublishSite = "https://as.mr"
	var cfProxyPublishSite = "https://as.131433.xyz"
	var latestPublishSite = ""
	client := utils.Client.Get().(*http.Client)
	req, _ := http.NewRequest("GET", officialPublishSite, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.198 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		log.AsmrLog.Info("尝试访问asmr.one最新站点发布页as.mr失败: ", zap.String("error", err.Error()))
		log.AsmrLog.Info("当前使用as.131433.xyz代理访问最新站点发布页")
		latestPublishSite = cfProxyPublishSite
	} else {
		log.AsmrLog.Info("当前使用as.mr访问最新站点发布页...")
		latestPublishSite = officialPublishSite
	}
	utils.Client.Put(client)
	defer resp.Body.Close()

	client = utils.Client.Get().(*http.Client)
	req, _ = http.NewRequest("GET", latestPublishSite, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.198 Safari/537.36")
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		log.AsmrLog.Error("访问asmr.one最新域名发布页出现错误: ", zap.String("error", err.Error()))
		return nil, err
	}
	utils.Client.Put(client)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.AsmrLog.Error("Error reading response body:", zap.String("error", err.Error()))
		return nil, err
	}

	// Convert the response body to a string and print it
	bodyText := string(body)
	//fmt.Println("Response Text:", bodyText)

	pattern := `<script type="module" crossorigin src="(/assets/index\.[a-f0-9]+\.js)"></script>`
	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(bodyText)

	var jsFilePath = ""
	if len(match) > 1 {
		jsFilePath = match[1]
		//fmt.Println("JavaScript file path:", jsFilePath)
	} else {
		//fmt.Println("JavaScript file path not found.")
	}

	jsContentUrl := latestPublishSite + jsFilePath
	client = utils.Client.Get().(*http.Client)
	req, _ = http.NewRequest("GET", jsContentUrl, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.198 Safari/537.36")
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		log.AsmrLog.Error("访问asmr.one最新域名发布页js resource出现错误: ", zap.String("error", err.Error()))
		return nil, err
	}
	utils.Client.Put(client)
	defer resp.Body.Close()
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		//fmt.Println("Error reading response body:", err)
		return nil, err
	}
	jsText := string(body)
	//fmt.Println("Response Text:", jsText)

	//extract rapid resp site url from js file text
	sitePattern := `link:\s*"([^"]+)"`
	re = regexp.MustCompile(sitePattern)
	matches := re.FindAllStringSubmatch(jsText, -1)
	var result []string
	for _, match := range matches {
		if len(match) > 1 {
			link := match[1]
			if strings.HasPrefix(link, "https://") {
				result = append(result, link)
			}
			//fmt.Println("Link:", link)
		}
	}
	return result, nil
}

func GetRespFastestSiteUrl() string {
	latestUrls, err := GetAsmrLatestUrls()
	if err != nil {
		log.AsmrLog.Error("获取最新域名列表失败: ", zap.String("error", err.Error()))
		//as the default
		return "https://api.asmr.one"
	}
	var wg sync.WaitGroup
	ch := make(chan string, len(latestUrls))

	for _, url := range latestUrls {
		wg.Add(1)
		go utils.FastFetch(url, &wg, ch)
	}

	// Use a goroutine to wait for all fetches to complete and close the channel
	go func() {
		wg.Wait()
		close(ch)
	}()

	// Wait for the fastest response
	var fastestResponse string
	for response := range ch {
		if fastestResponse == "" || len(response) < len(fastestResponse) {
			fastestResponse = response
		}
		log.AsmrLog.Info("Checking Fast Response:", zap.String("response", response))
	}

	log.AsmrLog.Info("Fastest Response is:", zap.String("response", fastestResponse))
	fastUrls := strings.Split(fastestResponse, "|")
	url := fastUrls[0]
	url = strings.Trim(url, "/")
	// convert to api
	apiUrl := strings.Replace(url, "https://", "https://api.", 1)
	return apiUrl
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
		log.AsmrLog.Error("序列化配置出错: ", zap.String("error", err.Error()))
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
		log.AsmrLog.Error("格式输入错误: ", zap.String("error", err.Error()))
	}
	customConfig.MaxWorker = maxWorkerInt
	//最大失败文件下载次数
	maxFailedRetry := utils.PromotForInput("请输入文件下载失败时最大重试次数(默认为3): ", strconv.Itoa(customConfig.MaxFailedRetry))
	maxFailedRetryInt, err2 := strconv.Atoi(maxFailedRetry)
	if err2 != nil {
		log.AsmrLog.Error("格式输入错误: ", zap.String("error", err2.Error()))
	}
	customConfig.MaxFailedRetry = maxFailedRetryInt

	batchTaskCount := utils.PromotForInput("请输出批量下载作品数量(默认为1): ", strconv.Itoa(customConfig.BatchTaskCount))
	i, err := strconv.Atoi(batchTaskCount)
	if err != nil {
		log.AsmrLog.Error("格式输入错误: ", zap.String("error", err.Error()))
	}
	customConfig.BatchTaskCount = i

	batchSleepTime := utils.PromotForInput("请输出批量下载间隔，单位为秒(默认为1): ", strconv.Itoa(customConfig.BatchSleepTime))
	ii, err := strconv.Atoi(batchSleepTime)
	if err != nil {
		log.AsmrLog.Error("格式输入错误: ", zap.String("error", err.Error()))
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
		log.AsmrLog.Info("设置的下载目录不存在,尝试自动生成: " + dowwnloadDir)
		subtitleDir := filepath.Join(dowwnloadDir, "subtitle")
		nosubtitleDir := filepath.Join(dowwnloadDir, "nosubtitle")

		err := os.MkdirAll(subtitleDir, os.ModePerm)
		if err != nil {
			log.AsmrLog.Error("自动创建下载目录失败: " + subtitleDir)
		}
		err = os.MkdirAll(nosubtitleDir, os.ModePerm)
		if err != nil {
			log.AsmrLog.Error("自动创建下载目录失败: " + subtitleDir)
		}
	}
	customConfig.DownloadDir = dowwnloadDir

	config, err := json.Marshal(customConfig)
	if err != nil {
		log.AsmrLog.Error("序列化配置出错: ", zap.String("error", err.Error()))
		os.Exit(0)
	}
	_ = os.WriteFile("config.json", config, 0644)
	log.AsmrLog.Info("已生成配置文件config.json, 如果您之前不做任何输入，默认以访客模式访问。")
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
		log.AsmrLog.Error("打开配置文件失败: ", zap.String("error", err.Error()))
		os.Exit(0)
	}
	defer func() { _ = file.Close() }()
	all, err := io.ReadAll(file)
	if err != nil {
		log.AsmrLog.Error("读取配置文件失败: ", zap.String("error", err.Error()))
		os.Exit(0)
	}
	var config Config
	err = json.Unmarshal(all, &config)
	if err != nil {
		log.AsmrLog.Error("解析配置文件失败: ", zap.String("error", err.Error()))
		os.Exit(0)
	}
	return &config
}
