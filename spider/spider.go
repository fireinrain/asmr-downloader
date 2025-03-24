package spider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/xxjwxc/gowp/workpool"
	"go.uber.org/zap"

	"asmr-downloader/config"
	"asmr-downloader/log"
	"asmr-downloader/model"
	"asmr-downloader/utils"
)

var ctx = context.Background()

// ASMRClient ASMR 客户端
type ASMRClient struct {
	GlobalConfig  *config.Config
	Authorization string
	WorkerPool    *workpool.WorkPool
}

// 音轨
type track struct {
	Type             string  `json:"type"`
	Title            string  `json:"title"`
	Children         []track `json:"children,omitempty"`
	Hash             string  `json:"hash,omitempty"`
	WorkTitle        string  `json:"workTitle,omitempty"`
	MediaStreamURL   string  `json:"mediaStreamUrl,omitempty"`
	MediaDownloadURL string  `json:"mediaDownloadUrl,omitempty"`
}

// NewASMRClient 初始化ASMR客户端
func NewASMRClient(maxWorker int, globalConfig *config.Config) *ASMRClient {
	return &ASMRClient{
		WorkerPool:   utils.NewWorkerPool(maxWorker),
		GlobalConfig: globalConfig,
	}
}

func HeadersInit(r *http.Request) *http.Request {
	r.Header.Set("Referer", "https://www.asmr.one/")
	r.Header.Set("Origin", "https://www.asmr.one")
	r.Header.Set("Host", strings.Split(config.AsmrBaseApiUrl, "//")[1])
	r.Header.Set("Connection", "keep-alive")
	r.Header.Set("Sec-Fetch-Mode", "cors")
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	r.Header.Set("Sec-Fetch-Dest", "empty")
	r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.108 Safari/537.36")
	return r
}

// Login 登入获取授权信息
func (asmrClient *ASMRClient) Login() error {
	payload, err := json.Marshal(map[string]string{
		"name":     asmrClient.GlobalConfig.Account,
		"password": asmrClient.GlobalConfig.Password,
	})
	if err != nil {
		log.AsmrLog.Error("登录失败, 配置文件有误。")
		return err
	}
	client := utils.Client.Get().(*http.Client)
	req, _ := http.NewRequest("POST", config.AsmrBaseApiUrl+"/api/auth/me", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	req = HeadersInit(req)
	resp, err := client.Do(req)
	utils.Client.Put(client)
	if err != nil {
		log.AsmrLog.Error("登录失败, 网络错误。请尝试通过环境变量的方式设置代理。")
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	all, err := io.ReadAll(resp.Body)
	if err != nil {
		log.AsmrLog.Error("登录失败, 读取响应失败。")
		return err
	}
	res := make(map[string]string)
	err = json.Unmarshal(all, &res)
	asmrClient.Authorization = "Bearer " + res["token"]
	return nil
}

func (asmrClient *ASMRClient) GetVoiceTracks(id string) ([]track, error) {
	client := utils.Client.Get().(*http.Client)
	req, _ := http.NewRequest("GET", config.AsmrBaseApiUrl+"/api/tracks/"+id, nil)
	req.Header.Set("Authorization", asmrClient.Authorization)
	req = HeadersInit(req)
	resp, err := client.Do(req)
	utils.Client.Put(client)
	if err != nil {
		log.AsmrLog.Error("获取音声信息失败:", zap.String("error", err.Error()))
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	all, err := io.ReadAll(resp.Body)
	if err != nil {
		log.AsmrLog.Error("获取音声信息失败: ", zap.String("error", err.Error()))
		return nil, err
	}
	res := make([]track, 0)
	err = json.Unmarshal(all, &res)
	return res, nil
}

func (asmrClient *ASMRClient) DownloadItem(id string, subtitleFlag int) {
	rjId := "RJ" + id
	log.AsmrLog.Info("作品 RJ 号: ", zap.String("info", rjId))
	tracks, err := asmrClient.GetVoiceTracks(id)
	if err != nil {
		log.AsmrLog.Error(fmt.Sprintf("获取作品: %s音轨失败: %s\n", err.Error()))
		return
	}
	basePath := config.GetConfig().DownloadDir
	if subtitleFlag == 1 {
		basePath = filepath.Join(basePath, "subtitle")
	} else if subtitleFlag == 0 {
		basePath = filepath.Join(basePath, "nosubtitle")
	}
	itemStorePath := filepath.Join(basePath, "RJ"+id)
	asmrClient.EnsureFileDirsExist(tracks, itemStorePath)

}

// SimpleDownloadItem
//
//	@Description: 简易下载模式  下载单个RJ作品
//	@receiver asmrClient
//	@param id
func (asmrClient *ASMRClient) SimpleDownloadItem(id string) {
	realId := strings.ReplaceAll(id, "RJ", "")
	rjId := "RJ" + realId
	log.AsmrLog.Info("作品 RJ 号: ", zap.String("info", rjId))
	tracks, err := asmrClient.GetVoiceTracks(realId)
	if err != nil {
		log.AsmrLog.Error(fmt.Sprintf("获取作品: %s音轨失败: %s\n", err.Error()))
		return
	}
	basePath := asmrClient.GlobalConfig.DownloadDir
	itemStorePath := filepath.Join(basePath, id)
	asmrClient.EnsureFileDirsExist(tracks, itemStorePath)

}

// EnsureFileDirsExist
//
//	@Description: 确保文件路径存在 存在就下载文件
//	@receiver asmrClient
//	@param tracks
//	@param basePath
func (asmrClient *ASMRClient) EnsureFileDirsExist(tracks []track, basePath string) {
	path := basePath
	//windows 目录错误
	if runtime.GOOS == "windows" {
		for _, str := range []string{"?", "<", ">", ":", "*", "|", " "} {
			path = strings.Replace(path, str, "_", -1)
		}
	}
	_ = os.MkdirAll(path, os.ModePerm)

	// 根据下载类型处理
	switch asmrClient.GlobalConfig.DownloadType {
	case "all":
		// 下载所有文件
		for _, t := range tracks {
			if t.Type != "folder" {
				asmrClient.DownloadFile(t.MediaDownloadURL, path, t.Title)
			} else {
				asmrClient.EnsureFileDirsExist(t.Children, fmt.Sprintf("%s/%s", path, t.Title))
			}
		}
	case "prioritizemp3":
		// 优先下载MP3文件
		// 第一遍：收集所有MP3文件标题
		mp3Titles := make(map[string]bool)
		var collectMP3Titles func([]track, string)
		collectMP3Titles = func(tracks []track, currentPath string) {
			mp3Path := currentPath
			//windows 目录错误
			if runtime.GOOS == "windows" {
				for _, str := range []string{"?", "<", ">", ":", "*", "|", " "} {
					mp3Path = strings.Replace(mp3Path, str, "_", -1)
				}
			}
			_ = os.MkdirAll(mp3Path, os.ModePerm)
			for _, t := range tracks {
				if t.Type == "folder" {
					collectMP3Titles(t.Children, fmt.Sprintf("%s/%s", mp3Path, t.Title))
				} else if strings.HasSuffix(strings.ToLower(t.Title), ".mp3") {
					baseTitle := strings.TrimSuffix(t.Title, filepath.Ext(t.Title))
					mp3Titles[baseTitle] = true
				}
			}
		}
		collectMP3Titles(tracks, path)

		// 第二遍：下载文件，如果存在MP3版本则跳过WAV/FLAC文件
		var processFiles func([]track, string)
		processFiles = func(tracks []track, currentPath string) {
			allPath := currentPath
			//windows 目录错误
			if runtime.GOOS == "windows" {
				for _, str := range []string{"?", "<", ">", ":", "*", "|", " "} {
					allPath = strings.Replace(allPath, str, "_", -1)
				}
			}
			_ = os.MkdirAll(allPath, os.ModePerm)
			for _, t := range tracks {
				if t.Type == "folder" {
					processFiles(t.Children, fmt.Sprintf("%s/%s", currentPath, t.Title))
				} else {
					baseTitle := strings.TrimSuffix(t.Title, filepath.Ext(t.Title))
					ext := strings.ToLower(filepath.Ext(t.Title))

					// 如果是WAV/FLAC文件且存在MP3版本，则跳过
					if (ext == ".wav" || ext == ".flac") && mp3Titles[baseTitle] {
						log.AsmrLog.Info(fmt.Sprintf("跳过 %s 因为存在 MP3 版本", t.Title))
						continue
					}

					asmrClient.DownloadFile(t.MediaDownloadURL, currentPath, t.Title)
				}
			}
		}
		processFiles(tracks, path)
	default:
		// 默认行为，下载所有文件
		for _, t := range tracks {
			if t.Type != "folder" {
				asmrClient.DownloadFile(t.MediaDownloadURL, path, t.Title)
			} else {
				asmrClient.EnsureFileDirsExist(t.Children, fmt.Sprintf("%s/%s", path, t.Title))
			}
		}
	}
}

// DownloadFile
//
//	@Description: 文件下载
//	@receiver asmrClient
//	@param url
//	@param dirPath
//	@param fileName
func (asmrClient *ASMRClient) DownloadFile(url string, dirPath string, fileName string) {
	if runtime.GOOS == "windows" {
		for _, str := range []string{"?", "<", ">", ":", "/", "\\", "*", "|", " "} {
			fileName = strings.Replace(fileName, str, "_", -1)
		}
	}
	savePath := dirPath + "/" + fileName
	if utils.FileOrDirExists(savePath) {
		log.AsmrLog.Info(fmt.Sprintf("文件: %s 已存在, 跳过下载...\n", savePath))
		return
	}
	log.AsmrLog.Info("正在下载 ", zap.String("info", savePath))
	_ = utils.NewFileDownloader(url, dirPath, fileName)()

}

// GetPerPageInfo 获取每页的信息
//
//	@Description:
//	@param authorStr 授权字符串
//	@param pageIndex 分页需要
//	@param subtitleFlag 是否选择字幕
//	@return *model.PageResult
//	@return error
func GetPerPageInfo(authorStr string, pageIndex int, subtitleFlag int) (*model.PageResult, error) {
	var seed int = utils.GenerateReqSeed()
	//log.Printf("Random: %s\n", randomUserAgent)
	//var reqUrl = "https://api.asmr.one/api/works?order=create_date&sort=desc&page=1&seed=" + strconv.Itoa(seed) + "&subtitle=0"
	var reqUrl = ""
	if subtitleFlag == -1 {
		reqUrl = fmt.Sprintf(config.AsmrBaseApiUrl+"/api/works?order=id&sort=desc&page=%d&seed=%d", pageIndex, seed)
	} else {
		reqUrl = fmt.Sprintf(config.AsmrBaseApiUrl+"/api/works?order=id&sort=desc&page=%d&seed=%d&subtitle=%d", pageIndex, seed, subtitleFlag)
	}
	var resp = new(model.PageResult)
	client := &http.Client{}
	req, err := http.NewRequest("GET", reqUrl, nil)
	if err != nil {
		// Handle error
		// ignore here
	}
	req = HeadersInit(req)
	req.Header.Set("Authorization", authorStr)

	respond, respError := client.Do(req.WithContext(context.Background()))
	utils.Client.Put(client)

	if respError != nil {
		log.AsmrLog.Error("请求失败: ", zap.String("error", respError.Error()))
		return nil, respError
	}
	defer func() { _ = respond.Body.Close() }()
	all, err := io.ReadAll(respond.Body)
	if err != nil {
		log.AsmrLog.Error("获取接口数据失败: ", zap.String("error", err.Error()))
		return nil, err
	}
	err = json.Unmarshal(all, resp)
	return resp, nil
}

// GetIndexPageInfo
//
// @Description: 获取首页信息
// @param authorStr
// @param subTitleFlag
// @return *model.PageResult
// @return error
func GetIndexPageInfo(authorStr string, subTitleFlag int) (*model.PageResult, error) {
	return GetPerPageInfo(authorStr, 1, subTitleFlag)
}

// GetAllIndexPageInfo
//
//	@Description: 获取所有数据首页信息
//	@param authorStr
//	@return *model.PageResult
//	@return error
func GetAllIndexPageInfo(authorStr string) (*model.PageResult, error) {
	return GetPerPageInfo(authorStr, 1, -1)
}

//func CollectPagesData(reqUrls []string) []model.PageResult {
//	var result []string
//	//执行的 这里要注意  需要指针类型传入  否则会异常
//	wg := &sync.WaitGroup{}
//	//并发控制
//	limiter := make(chan bool, 10)
//	defer close(limiter)
//
//	response := make(chan string, 20)
//	wgResponse := &sync.WaitGroup{}
//	//处理结果 接收结果
//	go func() {
//		wgResponse.Add(1)
//		for rc := range response {
//			result = append(result, rc)
//		}
//		wgResponse.Done()
//	}()
//	//开启协程处理请求
//	for _, url := range urls {
//		//计数器
//		wg.Add(1)
//		//并发控制 10
//		limiter <- true
//		go httpGet(url, response, limiter, wg)
//	}
//	//发送任务
//	wg.Wait()
//	close(response) //关闭 并不影响接收遍历
//	//处理接收结果
//	wgResponse.Wait()
//	return result
//}
