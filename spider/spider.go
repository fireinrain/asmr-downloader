package spider

import (
	"asmr-downloader/config"
	"asmr-downloader/utils"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/xxjwxc/gowp/workpool"
	"io"
	"net/http"
)

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

// Login 登入获取授权信息
func (ac *ASMRClient) Login() error {
	payload, err := json.Marshal(map[string]string{
		"name":     ac.GlobalConfig.Account,
		"password": ac.GlobalConfig.Password,
	})
	if err != nil {
		fmt.Println("登录失败, 配置文件有误。")
		return err
	}
	client := utils.Client.Get().(*http.Client)
	req, _ := http.NewRequest("POST", "https://api.asmr.one/api/auth/me", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Referer", "https://www.asmr.one/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.198 Safari/537.36")
	resp, err := client.Do(req)
	utils.Client.Put(client)
	if err != nil {
		fmt.Println("登录失败, 网络错误。请尝试通过环境变量的方式设置代理。")
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	all, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("登录失败, 读取响应失败。")
		return err
	}
	res := make(map[string]string)
	err = json.Unmarshal(all, &res)
	ac.Authorization = "Bearer " + res["token"]
	return nil
}
