package engine

import (
	"asmroner/internal/consts"
	"asmroner/internal/model"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"asmroner/internal/logger"

	"github.com/go-resty/resty/v2"
)

// probeTimeout 单个站点测速超时
const probeTimeout = 5 * time.Second

// newProbeClient 构建测速用的 HTTP 客户端。
// 复用 buildRestyClient 的代理配置，保证测速走的是和实际请求相同的网络路径；
// 关闭重试并设置短超时，避免单个站点拖慢整体测速。
func newProbeClient() (*resty.Client, error) {
	client, err := buildRestyClient(model.AppConfig)
	if err != nil {
		return nil, err
	}
	return client.SetRetryCount(0).SetTimeout(probeTimeout), nil
}

// GetAsmrLatestUrls 获取 asmr.one 最新域名列表
func GetAsmrLatestUrls(client *resty.Client) ([]string, error) {
	officialPublishSite := "https://as.mr"
	cfProxyPublishSite := "https://as.131433.xyz"
	var latestPublishSite string

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

// probeCandidates 计算测速候选地址：
// 官方发布页的站点域名（如 https://asmr-300.com）派生为 API 域名（如 https://api.asmr-300.com），
// 并始终补充 api.asmr.one 与默认地址，去重后返回。
func probeCandidates(client *resty.Client) []string {
	candidates := []string{"https://api.asmr.one", consts.AsmrBaseApiUrl}
	seen := map[string]bool{}
	for _, c := range candidates {
		seen[c] = true
	}

	latestUrls, err := GetAsmrLatestUrls(client)
	if err != nil {
		logger.Warn("获取最新域名列表失败: %v", err)
	}
	for _, u := range latestUrls {
		api := strings.TrimRight(strings.Replace(u, "https://", "https://api.", 1), "/")
		if api != "" && !seen[api] {
			seen[api] = true
			candidates = append(candidates, api)
		}
	}
	return candidates
}

// probeFastestApiUrl 并发探测所有候选地址，返回响应最快的 API 地址。
// 收到任意 HTTP 响应（含 401/404）即视为可达；全部失败时返回错误。
func probeFastestApiUrl(client *resty.Client) (string, error) {
	candidates := probeCandidates(client)
	logger.Info("开始测速，候选地址: %d 个", len(candidates))

	type probeResult struct {
		url      string
		duration time.Duration
		status   int
	}

	results := make(chan probeResult, len(candidates))
	var wg sync.WaitGroup
	for _, u := range candidates {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			start := time.Now()
			resp, err := client.R().Get(url)
			if err != nil {
				logger.Info("测速失败: %s (%v)", url, err)
				return
			}
			results <- probeResult{url: url, duration: time.Since(start), status: resp.StatusCode()}
		}(u)
	}
	wg.Wait()
	close(results)

	var best *probeResult
	for r := range results {
		logger.Info("测速结果: %s -> %dms (HTTP %d)", r.url, r.duration.Milliseconds(), r.status)
		res := r
		if best == nil || res.duration < best.duration {
			best = &res
		}
	}
	if best == nil {
		return "", errors.New("所有候选地址均不可达")
	}

	logger.Info("最快响应站点: %s (%dms)", best.url, best.duration.Milliseconds())
	return best.url, nil
}

// ProbeFastestApiUrl 测速入口：并发探测所有候选 API 地址，返回响应最快的。
// 供 `asmroner ping` 命令显式调用；GetRespFastestSiteUrl 内部走同样的逻辑。
func ProbeFastestApiUrl() (string, error) {
	client, err := newProbeClient()
	if err != nil {
		return "", fmt.Errorf("构建测速客户端失败: %w", err)
	}
	return probeFastestApiUrl(client)
}

// GetRespFastestSiteUrl 决定本次运行使用的 API 地址，优先级从高到低：
//  1. 配置项 downloader.api_url（用户显式指定，跳过测速）
//  2. 访问官方发布页获取最新域名列表，并发测速取响应最快的
//  3. 发布页不可达或全部候选超时 → 回退默认地址 consts.AsmrBaseApiUrl
func GetRespFastestSiteUrl() string {
	if u := model.AppConfig.Downloader.ApiUrl; u != "" {
		logger.Info("使用配置指定的接口地址: %s", u)
		return strings.TrimRight(u, "/")
	}

	fastest, err := ProbeFastestApiUrl()
	if err != nil {
		logger.Warn("自动测速失败: %v，使用默认地址 %s", err, consts.AsmrBaseApiUrl)
		return consts.AsmrBaseApiUrl
	}
	return fastest
}
