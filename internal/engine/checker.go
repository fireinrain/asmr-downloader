package engine

import (
	"asmroner/internal/consts"
	"asmroner/internal/model"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"asmroner/internal/logger"
	"asmroner/internal/utils"

	"github.com/go-resty/resty/v2"
)

// GetAsmrLatestUrls 获取 asmr.one 最新域名列表
func GetAsmrLatestUrls() ([]string, error) {
	officialPublishSite := "https://as.mr"
	cfProxyPublishSite := "https://as.131433.xyz"
	var latestPublishSite string

	client := resty.New().
		SetTimeout(10*time.Second).
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.198 Safari/537.36")

	// 先尝试官方站点
	resp, err := client.R().Get(officialPublishSite)
	if err != nil || resp.StatusCode() != 200 {
		if err != nil {
			logger.Warn("尝试访问asmr.one最新站点发布页as.mr失败: %v", err)
		}
		logger.Info("当前使用as.131433.xyz代理访问最新站点发布页...")
		latestPublishSite = cfProxyPublishSite
	} else {
		logger.Info("当前使用as.mr访问最新站点发布页...")
		latestPublishSite = officialPublishSite
	}

	// 访问最新发布页获取 HTML
	resp, err = client.R().Get(latestPublishSite)
	if err != nil {
		logger.Error("访问asmr.one最新域名发布页出现错误: %v", err)
		return nil, err
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("访问asmr.one最新域名发布页返回状态码: %d", resp.StatusCode())
	}
	bodyText := resp.String()

	// 正则匹配 JS 文件路径
	pattern := `<script type="module" crossorigin src="(/assets/index\.[a-f0-9]+\.js)"></script>`
	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(bodyText)

	var jsFilePath string
	if len(match) > 1 {
		jsFilePath = match[1]
	} else {
		logger.Warn("JavaScript file path not found in HTML")
		return nil, fmt.Errorf("js file path not found")
	}

	jsContentUrl := latestPublishSite + jsFilePath
	resp, err = client.R().Get(jsContentUrl)
	if err != nil {
		logger.Error("访问asmr.one最新域名发布页js resource出现错误: %v", err)
		return nil, err
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf("获取JS资源失败, 状态码: %d", resp.StatusCode())
	}
	jsText := resp.String()

	// 从 JS 内容提取快速响应的站点 URL
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
		}
	}

	return result, nil
}

// GetRespFastestSiteUrl 获取最快的响应 API 地址
func GetRespFastestSiteUrl() string {
	u := model.AppConfig.Downloader.ApiUrl
	if u != "" {
		return u
	}
	if consts.AsmrBaseApiUrl != "" {
		return consts.AsmrBaseApiUrl
	}

	latestUrls, err := GetAsmrLatestUrls()
	if err != nil {
		logger.Warn("获取最新域名列表失败: %v，使用默认地址", err)
		return "https://api.asmr.one"
	}

	var wg sync.WaitGroup
	ch := make(chan string, len(latestUrls))

	for _, url := range latestUrls {
		wg.Add(1)
		go utils.FastFetch(url, &wg, ch)
	}

	// Close channel after all goroutines finish
	go func() {
		wg.Wait()
		close(ch)
	}()

	// The first result from the channel is the fastest responder
	fastestURL, ok := <-ch
	if !ok || fastestURL == "" {
		logger.Warn("未找到最快响应的站点，使用默认地址")
		return "https://api.asmr.one"
	}

	logger.Info("最快响应站点: %s", fastestURL)
	fastestURL = strings.TrimRight(fastestURL, "/")
	apiUrl := strings.Replace(fastestURL, "https://", "https://api.", 1)
	return apiUrl
}
