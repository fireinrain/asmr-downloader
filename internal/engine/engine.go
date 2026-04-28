package engine

import (
	"asmroner/internal/consts"
	"asmroner/internal/database"
	"asmroner/internal/logger"
	"asmroner/internal/model"
	"asmroner/internal/utils"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alitto/pond/v2"
	"github.com/go-resty/resty/v2"
	"golang.org/x/net/proxy"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// EngineManager 下载器管理结构
type EngineManager struct {
	DB           *gorm.DB
	DownLimiter  *SmartLimiter
	Config       *model.Config
	WorkerPool   pond.Pool // work 间并行下载池，限速器控制提交速率
	DownloadPool pond.Pool // 单 work 内文件并行下载池
	Client       *resty.Client
	JWTToken     string
	ApiUrl       string
}

var defaultHeaders = map[string]string{
	"accept":          "application/json, text/plain, */*",
	"accept-encoding": "gzip",
	"accept-language": "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7",
	"cache-control":   "no-cache",
	"content-type":    "application/json",
	"origin":          "https://asmr.one",
	"pragma":          "no-cache",
	"referer":         "https://asmr.one/",
	"user-agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36",
}

// NewEngineManager 构造函数，增加 error 返回以符合 Go 惯例
func NewEngineManager(r float64, burst int, minMs int, maxMs int) (*EngineManager, error) {
	config := model.AppConfig
	if config == nil {
		return nil, errors.New("application config is not initialized")
	}

	workers := config.Downloader.MaxWorkers
	workerPool := pond.NewPool(workers)
	downloadPool := pond.NewPool(workers)

	client, err := buildRestyClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to build resty client: %w", err)
	}

	apiUrl := GetRespFastestSiteUrl()

	engine := &EngineManager{
		DB:           database.Database,
		DownLimiter:  NewSmartLimiter(r, burst, minMs, maxMs),
		Config:       config,
		WorkerPool:   workerPool,
		DownloadPool: downloadPool,
		Client:       client,
		ApiUrl:       apiUrl,
	}

	// 默认初始化登录
	if err := engine.AuthLogin(context.Background()); err != nil {
		logger.Warn("初始登录失败: %v", err)
	}

	return engine, nil
}

func buildRestyClient(config *model.Config) (*resty.Client, error) {
	proxyStr := config.Downloader.ProxyUrl
	retries := config.Downloader.MaxRetries
	r := resty.New()
	//http://112.123.45.67:8080
	if strings.Contains(proxyStr, "http") || strings.Contains(proxyStr, "https") {
		r.SetProxy(proxyStr)
	}
	// 如果没有使用代理，配置默认 Transport 以优化连接稳定性
	if proxyStr == "" {
		r.SetTransport(&http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			ForceAttemptHTTP2:     true,
		})
	}

	//socks5://user123:pass456@112.123.45.67:8080
	if strings.Contains(proxyStr, "socks5") {
		if strings.Contains(proxyStr, "@") {
			//use auth
			//user123:pass456@112.123.45.67:8080
			authStr := strings.Split(proxyStr, "@")[0]
			proxyAddr := strings.Split(proxyStr, "@")[1]
			username := strings.Split(authStr, ":")[0]
			password := strings.Split(authStr, ":")[1]
			auth := &proxy.Auth{
				User:     username,
				Password: password,
			}

			dialer, err := proxy.SOCKS5("tcp", proxyAddr, auth, proxy.Direct)
			if err != nil {
				return nil, fmt.Errorf("create socks5 dialer failed: %w", err)
			}
			r.SetTransport(&http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return dialer.Dial(network, addr)
				},
			})

		} else {
			//no auth
			proxyAddr := strings.Split(proxyStr, "://")[1]
			dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
			if err != nil {
				return nil, fmt.Errorf("create socks5 dialer failed: %w", err)
			}
			r.SetTransport(&http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return dialer.Dial(network, addr)
				},
			})
		}

	}
	client := r.
		SetHeader("User-Agent", utils.RandomUserAgent(consts.UserAgents)).
		SetRetryCount(retries).
		SetRetryWaitTime(2 * time.Second)
	return client, nil
}

//func (m *EngineManager) CheckIfMetadataWorkBatchMode() {
//	var data model.MetadataWork
//	has := m.DB.Model(&model.MetadataWork{}).Limit(1).Find(&data).RowsAffected > 0
//	if has {
//		m.MetadataWorkBatchMode = false
//	}
//}

// AuthLogin 登录获取JWT Token
func (m *EngineManager) AuthLogin(ctx context.Context) error {
	headers := defaultHeaders
	user := struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}{
		Name:     m.Config.User.Account,
		Password: m.Config.User.Password,
	}
	result := make(map[string]interface{})

	response, err2 := m.Client.R().
		SetContext(ctx).
		SetHeaders(headers).
		SetResult(&result).
		SetBody(&user).
		Post(m.ApiUrl + consts.AsmrApiPath.LoginPath)
	//fmt.Println(string(response.Body()))
	if !response.IsSuccess() {
		return errors.New("auth login error: " + response.Status())
	}

	if err2 != nil {
		return errors.New("auth login error: " + err2.Error())
	}
	// 检查响应是否包含 token
	token, ok := result["token"].(string)
	if !ok || token == "" {
		return errors.New("auth login error: token not found in response")
	}
	m.JWTToken = "Bearer " + token
	return nil
}

// SimpleDownload 并行下载，限速器控制提交速率，WorkerPool 控制最大并发数
func (m *EngineManager) SimpleDownload(ctx context.Context, ids []string, storeBaseDir string) error {
	group := m.WorkerPool.NewGroup()
	for _, id := range ids {
		// 限速：在主 goroutine 中等待令牌，控制提交速率
		if err := m.DownLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("限流器等待失败: %w", err)
		}
		group.SubmitErr(func() error {
			if err := m.DownloadOne(ctx, id, storeBaseDir); err != nil {
				logger.Fail("下载 %s 失败: %s", id, logger.SummarizeError(err))
				return err
			}
			return nil
		})
	}
	return group.Wait()
}

func (m *EngineManager) DownloadOne(ctx context.Context, id string, storeBaseDir string) error {
	valid, prefix, number, err := utils.IsValidDlsiteID(id)
	if err != nil || !valid {
		return err
	}

	task := logger.NewTask(id)

	// 获取作品信息
	task.Info("正在获取作品信息...")
	workInfo, err := m.GetWorkInfo(ctx, number)
	if err != nil {
		task.Error("获取作品信息失败: %s", logger.SummarizeError(err))
		return err
	}
	task.Info("作品: %s", workInfo.Title)

	// 获取所有的 tracks
	tracks, err := m.GetVoiceTracks(number)
	if err != nil {
		task.Error("获取音轨列表失败: %s", logger.SummarizeError(err))
		return err
	}
	task.Info("音轨数: %d", len(tracks))

	hasSubtitle := ""
	if workInfo.HasSubtitle {
		hasSubtitle = "sub"
	} else {
		hasSubtitle = "nosub"
	}

	// 新建下载目录名
	folderName := fmt.Sprintf(
		"%s%s-%s-%s-%s",
		strings.ToUpper(prefix),
		number,
		strings.ReplaceAll(workInfo.Release, "-", ""),
		hasSubtitle,
		utils.NormalDirPathStr(strings.ReplaceAll(workInfo.Title, "/", "")),
	)
	storeFileDir := filepath.Join(storeBaseDir, folderName)
	defer func() {
		utils.RemoveEmptyDirs(storeFileDir)
	}()
	task.Info("目标目录: %s", folderName)
	needDownloadUrls, err := m.ensureDirExists(tracks, storeFileDir)
	if err != nil {
		return err
	}
	//过滤掉不需要的格式
	needDownloadUrls = m.filterTargetAudioFormat(needDownloadUrls)
	//并行下载
	group := m.DownloadPool.NewGroup()
	for _, url := range needDownloadUrls {
		//log.Println("Download file:", url[2])
		group.SubmitErr(func() error {
			return m.downloadFile(url[0], url[1], url[2])
			//return nil
		})
	}
	err = group.Wait()

	return err
}

func (m *EngineManager) filterTargetAudioFormat(urls [][]string) [][]string {
	// 1. 如果配置是 all，直接返回原文件列表
	config := m.Config.Downloader.PreferMedia
	if strings.ToLower(config) == "all" {
		return urls
	}
	// 2. 解析优先规则（例如 "mp3>wav>flac"）
	rules := strings.Split(strings.ToLower(config), ">")

	// 定义格式与后缀映射
	extMap := map[string][]string{
		"mp3":  {".mp3", ".mp3.vtt"},
		"wav":  {".wav", ".wav.vtt"},
		"flac": {".flac", ".flac.vtt"},
	}
	// 分成 groupA（支持的音频格式） 和 groupB（其它文件）
	groupA := make([][]string, 0)
	groupB := make([][]string, 0)

	allExtList := []string{
		".mp3", ".mp3.vtt",
		".wav", ".wav.vtt",
		".flac", ".flac.vtt",
	}

	for _, f := range urls {
		lf := strings.ToLower(f[2])

		found := false
		for _, ext := range allExtList {
			if strings.HasSuffix(lf, ext) {
				groupA = append(groupA, f)
				found = true
				break
			}
		}
		if !found {
			groupB = append(groupB, f)
		}
	}

	// 3. 按优先顺序过滤 groupA
	for _, rule := range rules {
		targetExts, ok := extMap[rule]
		if !ok {
			continue // 未知格式直接跳过
		}

		// 抽取符合该格式的文件
		selected := make([][]string, 0)
		for _, f := range groupA {
			lf := strings.ToLower(f[2])
			for _, ext := range targetExts {
				if strings.HasSuffix(lf, ext) {
					selected = append(selected, f)
					break
				}
			}
		}
		// 如果选到文件，则直接返回：选中文件 + groupB
		if len(selected) > 0 {
			return append(selected, groupB...)
		}
	}
	// 如果一个也没选到，则返回 groupB
	return groupB

}

func (m *EngineManager) ensureDirExists(tracks []model.Track, storeBaseDir string) ([][]string, error) {
	path := storeBaseDir
	path = utils.NormalDirPathStr(path)
	_ = os.MkdirAll(path, os.ModePerm)
	//url,path,title
	var needDownloadUrls [][]string

	for _, t := range tracks {
		if t.Type != "folder" {
			needDownloadUrls = append(needDownloadUrls, []string{t.MediaDownloadURL, path, t.Title})
		} else {
			needDownUrl, _ := m.ensureDirExists(t.Children, fmt.Sprintf("%s/%s", path, t.Title))
			needDownloadUrls = append(needDownloadUrls, needDownUrl...)
		}
	}
	return needDownloadUrls, nil
}

func (m *EngineManager) GetVoiceTracks(id string) ([]model.Track, error) {
	url := m.ApiUrl + consts.AsmrApiPath.TracksPath + id
	headers := defaultHeaders

	var result []model.Track

	resp, err := m.Client.R().
		SetHeader("Authorization", m.JWTToken).
		SetHeaders(headers).
		SetResult(&result).
		Get(url)

	if err != nil {
		logger.Error("获取音轨信息失败: %s", logger.SummarizeError(err))
		return nil, err
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf("获取音轨信息HTTP错误, 状态码: %d", resp.StatusCode())
	}
	return result, nil
}

func (m *EngineManager) GetWorkInfo(ctx context.Context, id string) (model.WorkInfo, error) {
	url := m.ApiUrl + consts.AsmrApiPath.WorkinfoPath + id
	headers := defaultHeaders

	var result = model.WorkInfo{}

	resp, err := m.Client.R().
		SetContext(ctx).
		SetHeader("Authorization", m.JWTToken).
		SetHeaders(headers).
		SetResult(&result).
		Get(url)

	if err != nil {
		logger.Error("获取作品信息失败: %s", logger.SummarizeError(err))
		return result, err
	}
	if !resp.IsSuccess() {
		return result, fmt.Errorf("获取作品信息HTTP错误, 状态码: %d", resp.StatusCode())
	}
	return result, nil
}

func (m *EngineManager) SyncMetadata(ctx context.Context) error {
	url := m.ApiUrl + consts.AsmrApiPath.SyncMetaPath

	allPageResult, err := m.fetchMetaDataResp(url)
	if err != nil {
		return fmt.Errorf("获取所有元数据首页信息失败: %w", err)
	}

	allSubPageResult, err := m.fetchMetaDataResp(url + "&subtitle=1")
	if err != nil {
		return fmt.Errorf("获取带字幕元数据首页信息失败: %w", err)
	}

	siteAll, localAll := m.printSyncMetadataStatics(allPageResult, allSubPageResult)
	if siteAll == localAll {
		logger.Done("网页数据与本地数据一致，无需同步")
		return nil
	}
	if siteAll < localAll {
		logger.Warn("本地数据存在逻辑错误，请检查数据库是否存在重复数据")
	}
	if siteAll > localAll {
		confirm := utils.PromptConfirm("网页数据有更新,是否进行同步操作?")
		if !confirm {
			return nil
		}
	}

	urls := m.buildMetaDataWorkUrls(allPageResult.Pagination.TotalCount, 100)
	totalBatches := len(urls)

	// Use a mutex-protected counter for progress tracking
	var mu sync.Mutex
	savedCount := 0
	const maxRetries = 3

	syncPool := pond.NewPool(2)
	group := syncPool.NewGroup()

	for _, u := range urls {
		pageURL := u
		group.SubmitErr(func() error {
			var resp *model.MetadataWorkResponse
			var fetchErr error

			// Retry with backoff
			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					logger.DownloadRetry(attempt, maxRetries, pageURL, fetchErr)
					time.Sleep(time.Duration(attempt*5) * time.Second)
				}
				resp, fetchErr = m.fetchMetaDataResp(pageURL)
				if fetchErr == nil {
					break
				}
			}
			if fetchErr != nil {
				return fmt.Errorf("获取分页元数据最终失败: %s: %w", pageURL, fetchErr)
			}

			metadataWorks := resp.BuildMetadataWork()
			if len(metadataWorks) == 0 {
				return nil
			}

			// Store to database
			tx := m.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&metadataWorks)
			if tx.Error != nil {
				return fmt.Errorf("批量保存元数据失败: %w", tx.Error)
			}

			mu.Lock()
			savedCount++
			logger.Progress(savedCount, totalBatches, "同步元数据")
			mu.Unlock()
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		logger.Warn("同步元数据过程中出现错误: %v", err)
		return err
	}

	return nil
}

func (m *EngineManager) fetchMetaDataResp(url string) (*model.MetadataWorkResponse, error) {
	// 限速：每次 API 请求前等待令牌
	if err := m.DownLimiter.Wait(context.Background()); err != nil {
		return nil, fmt.Errorf("限流器等待失败: %w", err)
	}

	headers := defaultHeaders

	var result = model.MetadataWorkResponse{}

	resp, err := m.Client.R().
		SetHeader("Authorization", m.JWTToken).
		SetHeaders(headers).
		SetResult(&result).
		Get(url)
	if err != nil {
		logger.Error("获取元数据信息失败: %s", logger.SummarizeError(err))
		return nil, err
	}
	if !resp.IsSuccess() {
		logger.Warn("API 请求被拒绝, HTTP 状态码: %d", resp.StatusCode())
		return nil, fmt.Errorf("HTTP %d: 请求被服务器拒绝 (可能触发了速率限制)", resp.StatusCode())
	}
	return &result, nil
}

func (m *EngineManager) buildMetaDataWorkUrls(totalCount int, pageSize int) []string {
	urls := make([]string, 0)
	//page := totalCount / pageSize
	//if totalCount%pageSize != 0 {
	//	page++
	//}

	for i := 1; i <= (totalCount/pageSize)+1; i++ {
		pageStr := strings.ReplaceAll(consts.AsmrApiPath.SyncMetaPath, "page=1",
			fmt.Sprintf("page=%s", strconv.Itoa(i)))

		pageSizeStr := strings.ReplaceAll(pageStr, "pageSize=1",
			fmt.Sprintf("pageSize=%d", 100))
		url := m.ApiUrl + pageSizeStr
		urls = append(urls, url)
	}
	return urls
}

func (m *EngineManager) downloadFile(url string, path string, fileName string) error {
	storePath := filepath.Join(path, fileName)
	maxRetries := m.Config.Downloader.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			logger.DownloadRetry(attempt, maxRetries, fileName, lastErr)
			// 指数退避: 2s, 4s, 8s...
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			time.Sleep(backoff)
			// 清理可能的残留文件
			os.Remove(storePath)
		}

		logger.Debug("下载文件: %s", fileName)
		resp, err := m.Client.R().
			SetOutput(storePath).
			Get(url)
		if err != nil {
			lastErr = err
			// 仅对可重试的网络错误进行重试
			if isRetryableError(err) {
				continue
			}
			logger.Error("下载文件 %s 失败 (不可重试): %s", fileName, logger.SummarizeError(err))
			return err
		}
		if !resp.IsSuccess() {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode())
			if resp.StatusCode() >= 500 || resp.StatusCode() == 429 {
				continue // 服务端错误或限流，可重试
			}
			logger.Error("下载文件 %s 失败, HTTP 状态码: %d", fileName, resp.StatusCode())
			return lastErr
		}
		return nil // 成功
	}

	logger.Error("下载文件 %s 最终失败 (已重试 %d 次): %s", fileName, maxRetries, logger.SummarizeError(lastErr))
	return fmt.Errorf("下载 %s 失败 (重试 %d 次后): %w", fileName, maxRetries, lastErr)
}

// isRetryableError 判断错误是否可以重试
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	retryablePatterns := []string{
		"stream error",
		"INTERNAL_ERROR",
		"connection reset",
		"broken pipe",
		"EOF",
		"unexpected EOF",
		"i/o timeout",
		"TLS handshake timeout",
		"connection refused",
		"no such host",
	}
	for _, pattern := range retryablePatterns {
		if strings.Contains(msg, pattern) {
			return true
		}
	}
	return false
}

func (m *EngineManager) SearchForCountResult(ctx context.Context, asmrOneQueryStr string, count int) (model.SearchResult, error) {
	url := m.ApiUrl + consts.AsmrApiPath.SearchPath + asmrOneQueryStr
	headers := defaultHeaders

	var result = model.SearchResult{}

	resp, err := m.Client.R().
		SetContext(ctx).
		SetHeader("Authorization", m.JWTToken).
		SetHeaders(headers).
		SetResult(&result).
		Get(url)

	if err != nil {
		logger.Error("查询关键字信息失败: %s", logger.SummarizeError(err))
		return result, err
	}
	if !resp.IsSuccess() {
		return result, fmt.Errorf("搜索请求HTTP错误, 状态码: %d", resp.StatusCode())
	}
	logger.Info("作品搜索结果总数: %d", result.Pagination.TotalCount)
	// 如果结果比较少
	if result.Pagination.TotalCount > count && count < result.Pagination.PageSize {
		result.Works = result.Works[:count]
		return result, nil
	}
	if result.Pagination.TotalCount < count && count > result.Pagination.PageSize {
		count = result.Pagination.TotalCount
	}
	//如果结果比count大 但是比pageSize小 则直接返回
	if result.Pagination.TotalCount >= count {
		//计算分页
		page := count / result.Pagination.PageSize
		if count%result.Pagination.PageSize != 0 {
			page++
		}
		for i := 2; i <= page; i++ {
			// 构建分页URL
			var newResult model.SearchResult
			pageURL := strings.ReplaceAll(url, "&page=1", fmt.Sprintf("&page=%d", i))
			// 发送GET请求
			resp, err := m.Client.R().
				SetHeader("Authorization", m.JWTToken).
				SetHeaders(headers).
				SetResult(&newResult).
				Get(pageURL)
			if err != nil {
				logger.Error("查询分页信息失败: %s", logger.SummarizeError(err))
				return newResult, err
			}
			if !resp.IsSuccess() {
				return newResult, fmt.Errorf("查询分页HTTP错误, 状态码: %d", resp.StatusCode())
			}
			// 合并结果
			result.Works = append(result.Works, newResult.Works...)
			time.Sleep(500 * time.Millisecond)
		}
	}
	return result, nil
}

func (m *EngineManager) DownloadBatchMedias(ctx context.Context, works []model.SearchResultView, storePathDir string) error {
	group := m.WorkerPool.NewGroup()
	for _, work := range works {
		// 限速：在主 goroutine 中等待令牌，控制提交速率
		if err := m.DownLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("限流器等待失败: %w", err)
		}
		group.SubmitErr(func() error {
			if err := m.DownloadOne(ctx, work.SourceID, storePathDir); err != nil {
				logger.Fail("下载 %s 失败: %s", work.SourceID, logger.SummarizeError(err))
				return err
			}
			return nil
		})
	}
	return group.Wait()
}

func (m *EngineManager) DownloadMediaByBatchIds(ctx context.Context, worksId []string, storePathDir string) error {
	group := m.WorkerPool.NewGroup()
	for _, id := range worksId {
		// 限速：在主 goroutine 中等待令牌，控制提交速率
		if err := m.DownLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("限流器等待失败: %w", err)
		}
		group.SubmitErr(func() error {
			if err := m.DownloadOne(ctx, id, storePathDir); err != nil {
				logger.Fail("下载作品 %s 失败: %s", id, logger.SummarizeError(err))
				return err
			}
			return nil
		})
	}
	return group.Wait()
}

// 打印同步元数据统计信息
func (m *EngineManager) printSyncMetadataStatics(result *model.MetadataWorkResponse, result2 *model.MetadataWorkResponse) (int, int) {
	//输出当前数据库中存在的记录数
	//var allCount int64
	//tx := m.DB.Model(&model.MetadataWork{}).Count(&allCount)
	//if tx.Error != nil {
	//	log.Println("查询数据库出现错误" + tx.Error.Error())
	//}
	type StatResult struct {
		TotalCount        int
		SubtitleTrueCount int
	}
	var localResult StatResult
	m.DB.Raw(`
    SELECT 
        COUNT(*) AS total_count,
        SUM(CASE WHEN has_subtitle = 1 THEN 1 ELSE 0 END) AS subtitle_true_count
    FROM metadata_works`).Scan(&localResult)
	logger.Info("网站作品元数据数量 (所有/带字幕): %d/%d",
		result.Pagination.TotalCount, result2.Pagination.TotalCount)
	var syncRateTotal, syncRateSubtitle float64

	if localResult.TotalCount > 0 {
		// 总体同步率 = 已同步条目 / 总条目
		syncRateTotal = float64(localResult.TotalCount) / float64(result.Pagination.TotalCount)

		// 字幕同步率 = 带字幕条目 / 总条目
		syncRateSubtitle = float64(localResult.SubtitleTrueCount) / float64(result2.Pagination.TotalCount)
	} else {
		syncRateTotal = 0.0
		syncRateSubtitle = 0.0
	}

	logger.Info(
		"本地数据库中元数据数量 (所有/带字幕): %d/%d — 同步率 (总/字幕): %.2f%%/%.2f%%",
		localResult.TotalCount,
		localResult.SubtitleTrueCount,
		syncRateTotal*100,
		syncRateSubtitle*100,
	)

	return result.Pagination.TotalCount, localResult.TotalCount
}

// 按照指定数量下载热门100作品
func (m *EngineManager) DownloadHot100(ctx context.Context, count int, dir string) error {
	url := m.ApiUrl + consts.AsmrApiPath.HotPath
	headers := defaultHeaders

	var result = model.MetadataWorkResponse{}
	body := map[string]interface{}{
		"keyword":             "",
		"page":                1,
		"pageSize":            100,
		"subtitle":            0,
		"localSubtitledWorks": []interface{}{},
		"withPlaylistStatus":  []interface{}{},
	}

	resp, err := m.Client.R().
		SetHeader("Authorization", m.JWTToken).
		SetContext(ctx).
		SetBody(body).
		SetHeaders(headers).
		SetResult(&result).
		Post(url)

	if err != nil {
		logger.Error("获取热门作品列表失败: %s", logger.SummarizeError(err))
		logger.RecordFailure("DownloadHot100", url, err.Error())
		return err
	}
	if !resp.IsSuccess() {
		logger.RecordFailure("DownloadHot100", url, resp.Status())
		return fmt.Errorf("获取热门作品列表HTTP错误, 状态码: %d", resp.StatusCode())
	}
	if count <= 0 {
		return errors.New("下载数量选择必须大于0")
	}
	metadataWork := result.BuildMetadataWork()
	works := metadataWork[:count]
	var sourceIds []string
	for _, work := range works {
		sourceIds = append(sourceIds, work.SourceID)
	}
	// 下载热门100作品
	err = m.DownloadMediaByBatchIds(ctx, sourceIds, dir)
	if err != nil {
		logger.Fail("下载热门作品失败: %s", logger.SummarizeError(err))
		return err
	}
	return nil
}

// ExportLinksOnly 仅获取作品的所有下载链接，不执行实际下载，并按原始目录结构保存链接文件。
// ctx: 上下文
// id: 作品ID，例如 "RJ01544940"
// outputBaseDir: 输出根目录路径。若为空则使用当前目录。
// 返回每个文件夹下链接数量的统计信息，以及实际作品输出目录。
func (m *EngineManager) ExportLinksOnly(ctx context.Context, id string, outputBaseDir string) (map[string]int, string, error) {
	// 校验 ID
	valid, prefix, number, err := utils.IsValidDlsiteID(id)
	if err != nil || !valid {
		return nil, "", fmt.Errorf("无效的作品ID: %s", id)
	}

	// 限速等待
	if err := m.DownLimiter.Wait(ctx); err != nil {
		return nil, "", fmt.Errorf("限流器等待失败: %w", err)
	}

	// 获取作品信息
	workInfo, err := m.GetWorkInfo(ctx, number)
	if err != nil {
		logger.Warn("获取作品信息失败: %v，将使用ID作为标题", err)
		workInfo = model.WorkInfo{Title: id, Release: ""}
	}

	// 获取音轨列表
	tracks, err := m.GetVoiceTracks(number)
	if err != nil {
		return nil, "", fmt.Errorf("获取音轨列表失败: %w", err)
	}

	// 构建作品文件夹名（与下载逻辑一致）
	hasSubtitle := ""
	if workInfo.HasSubtitle {
		hasSubtitle = "sub"
	} else {
		hasSubtitle = "nosub"
	}
	folderName := fmt.Sprintf(
		"%s%s-%s-%s-%s",
		strings.ToUpper(prefix),
		number,
		strings.ReplaceAll(workInfo.Release, "-", ""),
		hasSubtitle,
		utils.NormalDirPathStr(strings.ReplaceAll(workInfo.Title, "/", "")),
	)

	// 确定输出根目录
	if outputBaseDir == "" {
		outputBaseDir = "."
	}
	workOutputDir := filepath.Join(outputBaseDir, folderName)

	// 递归收集每个文件夹下的文件 URL
	folderFiles := make(map[string][]string)
	var collect func([]model.Track, string) error
	collect = func(ts []model.Track, relPath string) error {
		for _, t := range ts {
			if t.Type != "folder" {
				folderFiles[relPath] = append(folderFiles[relPath], t.MediaDownloadURL)
			} else {
				subPath := filepath.Join(relPath, t.Title)
				if err := collect(t.Children, subPath); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := collect(tracks, ""); err != nil {
		return nil, "", err
	}

	// 创建输出目录结构并写入 links.txt，同时记录文件夹路径用于生成脚本
	stats := make(map[string]int)
	folderPaths := make(map[string]string) // 相对路径 -> 实际绝对路径
	for relPath, urls := range folderFiles {
		dir := workOutputDir
		if relPath != "" {
			dir = filepath.Join(workOutputDir, relPath)
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, "", fmt.Errorf("创建目录 %s 失败: %w", dir, err)
		}
		linkFile := filepath.Join(dir, "links.txt")
		content := strings.Join(urls, "\n")
		if err := os.WriteFile(linkFile, []byte(content), 0644); err != nil {
			return nil, "", fmt.Errorf("写入文件 %s 失败: %w", linkFile, err)
		}
		stats[relPath] = len(urls)
		folderPaths[relPath] = dir
	}
// 创建脚本文档夹
scriptsDir := filepath.Join(workOutputDir, "download_scripts")
if err := os.MkdirAll(scriptsDir, 0755); err != nil {
    return nil, "", fmt.Errorf("创建脚本目录失败: %w", err)
}

// 生成 IDM 批处理脚本（实际是引导 PowerShell 的 .bat）
if err := m.generateIDMScript(workOutputDir, folderPaths, scriptsDir); err != nil {
    logger.Warn("生成 IDM 脚本失败: %v", err)
}
// 生成 Aria2 下载脚本（跨平台）
if err := m.generateAria2Script(workOutputDir, folderPaths, scriptsDir); err != nil {
    logger.Warn("生成 Aria2 脚本失败: %v", err)
}

logger.Info("已导出链接文件，共 %d 个文件夹", len(folderFiles))
logger.Info("作品目录: %s", workOutputDir)

return stats, workOutputDir, nil
}

// ExportHotWorks 导出热门榜前 count 个作品的下载链接（增强版：显示标题）
func (m *EngineManager) ExportHotWorks(ctx context.Context, count int, outputBaseDir string) error {
	// 调用热门作品接口获取列表
	url := m.ApiUrl + consts.AsmrApiPath.HotPath
	headers := defaultHeaders

	var result = model.MetadataWorkResponse{}
	body := map[string]interface{}{
		"keyword":             "",
		"page":                1,
		"pageSize":            100,
		"subtitle":            0,
		"localSubtitledWorks": []interface{}{},
		"withPlaylistStatus":  []interface{}{},
	}

	resp, err := m.Client.R().
		SetHeader("Authorization", m.JWTToken).
		SetContext(ctx).
		SetBody(body).
		SetHeaders(headers).
		SetResult(&result).
		Post(url)

	if err != nil {
		logger.Error("获取热门作品列表失败: %s", logger.SummarizeError(err))
		return err
	}
	if !resp.IsSuccess() {
		return fmt.Errorf("获取热门作品列表HTTP错误, 状态码: %d", resp.StatusCode())
	}
	if count <= 0 {
		return errors.New("导出数量必须大于0")
	}

	// 限制数量
	works := result.BuildMetadataWork()
	if count > len(works) {
		count = len(works)
	}
	selectedWorks := works[:count]

	logger.Info("准备导出 %d 个热门作品", count)

	for i, work := range selectedWorks {
		id := work.SourceID
		title := work.Title
		logger.Info("[%d/%d] 正在导出作品: %s - %s", i+1, count, id, title)

		_, _, err := m.ExportLinksOnly(ctx, id, outputBaseDir)
		if err != nil {
			logger.Error("导出作品 %s 失败: %s", id, logger.SummarizeError(err))
			// 继续处理下一个
			continue
		}
	}

	logger.Done("已导出热门榜前 %d 个作品的链接", count)
	return nil
}

// generateIDMScript 生成 idm_download.bat 脚本（放入 scriptsDir）
func (m *EngineManager) generateIDMScript(baseDir string, folderPaths map[string]string, scriptsDir string) error {
    ps1Path := filepath.Join(scriptsDir, "idm_download.ps1")
    var psLines []string

    // ---------- 从配置读取 IDM 路径 ----------
    configIdmPath := ""
    if m.Config != nil {
        configIdmPath = m.Config.Downloader.IdmPath
    }

    if configIdmPath != "" {
        // 已配置路径
        escapedPath := strings.ReplaceAll(configIdmPath, `"`, "`\"")
        psLines = append(psLines, fmt.Sprintf(`$idmPath = "%s"`, escapedPath))
        psLines = append(psLines, `if (-not (Test-Path $idmPath)) {`)
        psLines = append(psLines, `    Write-Host "=============================================" -ForegroundColor Red`)
        psLines = append(psLines, `    Write-Host "错误：配置的 IDM 路径不存在！" -ForegroundColor Red`)
        psLines = append(psLines, `    Write-Host "路径: $idmPath"`)
        psLines = append(psLines, `    Write-Host ""`)
        psLines = append(psLines, `    Write-Host "请运行 'asmroner config' 重新设置，或编辑本 .ps1 文件中的 ` + "`$idmPath`" + ` 变量。"`)
        psLines = append(psLines, `    Write-Host "=============================================" -ForegroundColor Red`)
        psLines = append(psLines, `    Read-Host "按 Enter 键退出"`)
        psLines = append(psLines, `    exit 1`)
        psLines = append(psLines, `}`)
    } else {
        // 未配置路径
        psLines = append(psLines, `Write-Host "=============================================" -ForegroundColor Red`)
        psLines = append(psLines, `Write-Host "IDM 路径未在配置文件中设置！" -ForegroundColor Red`)
        psLines = append(psLines, `Write-Host ""`)
        psLines = append(psLines, `Write-Host "请按以下步骤之一操作："`)
        psLines = append(psLines, `Write-Host "1. 运行 'asmroner config' 重新配置，并输入您的 IDM 安装路径。"`)
		psLines = append(psLines, `Write-Host '2. 或用记事本打开本 .ps1 文件，将 ` + "`$idmPath = $null`" + ` 行改为实际路径，例如：'`)
        psLines = append(psLines, `Write-Host '   ` + "`$idmPath = \"E:\\idm\\IDM\\IDMan.exe\"`" + `'`)
        psLines = append(psLines, `Write-Host "=============================================" -ForegroundColor Red`)
        psLines = append(psLines, `Read-Host "按 Enter 键退出"`)
        psLines = append(psLines, `exit 1`)
    }

    psLines = append(psLines, `Write-Host "IDM 路径: $idmPath"`)
    psLines = append(psLines, ``)

    psLines = append(psLines, `$baseDir = (Get-Item $PSScriptRoot).Parent.FullName`)
    psLines = append(psLines, ``)

    psLines = append(psLines, `if (!(Test-Path $idmPath)) {`)
    psLines = append(psLines, `    Write-Host "错误：找不到 IDMan.exe，请修改脚本中的 idmPath 变量"`)
    psLines = append(psLines, `    Read-Host "按 Enter 键退出"`)
    psLines = append(psLines, `    exit 1`)
    psLines = append(psLines, `}`)
    psLines = append(psLines, ``)

    psLines = append(psLines, `$folders = @{`)
    for relPath, dir := range folderPaths {
        relSaveDir, _ := filepath.Rel(baseDir, dir)
        linksFile, _ := filepath.Rel(baseDir, filepath.Join(dir, "links.txt"))
        displayName := relPath
        if displayName == "" {
            displayName = "(根目录)"
        }
        escName := strings.ReplaceAll(displayName, `"`, "`\"")
        escSave := strings.ReplaceAll(relSaveDir, `"`, "`\"")
        escLinks := strings.ReplaceAll(linksFile, `"`, "`\"")
        psLines = append(psLines, fmt.Sprintf(`    "%s" = "%s|%s";`, escName, escSave, escLinks))
    }
    psLines = append(psLines, `}`)
    psLines = append(psLines, ``)

    psLines = append(psLines, `$totalAdded = 0`)
    psLines = append(psLines, `$maxRetries = 2       # 最多重试2次`)
    psLines = append(psLines, `$retryDelay = 300     # 重试间隔300毫秒`)
    psLines = append(psLines, `foreach ($entry in $folders.GetEnumerator()) {`)
    psLines = append(psLines, `    $name = $entry.Key`)
    psLines = append(psLines, `    $data = $entry.Value -split '\|'`)
    psLines = append(psLines, `    $saveDirRel = $data[0]`)
    psLines = append(psLines, `    $linksFileRel = $data[1]`)
    psLines = append(psLines, `    $saveDir   = if ($saveDirRel -eq ".") { $baseDir } else { Join-Path $baseDir $saveDirRel }`)
    psLines = append(psLines, `    $linksFile = Join-Path $baseDir $linksFileRel`)
    psLines = append(psLines, ``)
    psLines = append(psLines, `    Write-Host "处理文件夹: $name"`)
    psLines = append(psLines, `    $urls = Get-Content -Path $linksFile | Where-Object { $_.Trim() -ne "" } | ForEach-Object { $_.Trim() }`)
    psLines = append(psLines, `    foreach ($url in $urls) {`)
    psLines = append(psLines, `        $success = $false`)
    psLines = append(psLines, `        for ($retry = 0; $retry -le $maxRetries; $retry++) {`)
    psLines = append(psLines, `            try {`)
    psLines = append(psLines, `                $proc = Start-Process -FilePath $idmPath -ArgumentList '/d', $url, '/p', $saveDir, '/a' -Wait -NoNewWindow -PassThru`)
    psLines = append(psLines, `                if ($proc.ExitCode -eq 0) {`)
    psLines = append(psLines, `                    $success = $true`)
    psLines = append(psLines, `                    break`)
    psLines = append(psLines, `                } else {`)
    psLines = append(psLines, `                    Write-Host "  尝试 $($retry+1)/$($maxRetries+1) 失败，退出码: $($proc.ExitCode)" -ForegroundColor Yellow`)
    psLines = append(psLines, `                }`)
    psLines = append(psLines, `            } catch {`)
    psLines = append(psLines, `                Write-Host "  尝试 $($retry+1)/$($maxRetries+1) 失败: $($_.Exception.Message)" -ForegroundColor Yellow`)
    psLines = append(psLines, `            }`)
    psLines = append(psLines, `            if ($retry -lt $maxRetries) { Start-Sleep -Milliseconds $retryDelay }`)
    psLines = append(psLines, `        }`)
    psLines = append(psLines, `        if ($success) {`)
    psLines = append(psLines, `            $totalAdded++`)
    psLines = append(psLines, `        } else {`)
    psLines = append(psLines, `            Write-Host "  最终失败: $url" -ForegroundColor Red`)
    psLines = append(psLines, `        }`)
    psLines = append(psLines, `    }`)
    psLines = append(psLines, `}`)
    psLines = append(psLines, ``)
    psLines = append(psLines, `Write-Host "===================================="`)
    psLines = append(psLines, `Write-Host "全部处理完毕。成功添加 $totalAdded 个任务。"`)
    psLines = append(psLines, `Write-Host "请打开 IDM 主界面查看下载队列。"`)
    psLines = append(psLines, `Read-Host "按 Enter 键退出"`)

    content := strings.Join(psLines, "\r\n")
    bom := []byte{0xEF, 0xBB, 0xBF}
    if err := os.WriteFile(ps1Path, append(bom, []byte(content)...), 0644); err != nil {
        return err
    }

    // 2. 生成引导用的 .bat 脚本
    batPath := filepath.Join(scriptsDir, "idm_download.bat")
    batLines := []string{
        "@echo off",
        "chcp 65001 > nul",
        "echo 正在通过 PowerShell 向 IDM 添加下载任务...",
        "powershell -ExecutionPolicy Bypass -File \"%~dp0idm_download.ps1\"",
        "pause",
    }
    batContent := strings.Join(batLines, "\r\n")
    return os.WriteFile(batPath, append(bom, []byte(batContent)...), 0644)
}

func (m *EngineManager) generateAria2Script(baseDir string, folderPaths map[string]string, scriptsDir string) error {
    // 1. 生成 Windows PowerShell 直接下载脚本
    ps1Path := filepath.Join(scriptsDir, "aria2_download.ps1")
    var psLines []string
    psLines = append(psLines, `$aria2 = if (Test-Path "$PSScriptRoot\aria2c.exe") { "$PSScriptRoot\aria2c.exe" } else { "aria2c" }`)
    psLines = append(psLines, `$baseDir = (Get-Item $PSScriptRoot).Parent.FullName`)
    psLines = append(psLines, `Set-Location $baseDir`)
    psLines = append(psLines, `$global:foldersDone = 0`)

    psLines = append(psLines, `$folders = @{`)
    for relPath, dir := range folderPaths {
        relSaveDir, _ := filepath.Rel(baseDir, dir)
        linksFile, _ := filepath.Rel(baseDir, filepath.Join(dir, "links.txt"))
        displayName := relPath
        if displayName == "" {
            displayName = "(root)"
        }
        savePath := `.\`
        if relSaveDir != "." {
            savePath = fmt.Sprintf(`.\%s`, relSaveDir)
        }
        escName := strings.ReplaceAll(displayName, `"`, "`\"")
        escLinks := strings.ReplaceAll(linksFile, `"`, "`\"")
        escSave := strings.ReplaceAll(savePath, `"`, "`\"")
        psLines = append(psLines, fmt.Sprintf(`    "%s" = @("%s", "%s");`, escName, escLinks, escSave))
    }
    psLines = append(psLines, `}`)
    psLines = append(psLines, ``)

    psLines = append(psLines, `Write-Host "Starting Aria2 direct downloads..."`)
    psLines = append(psLines, `Write-Host "===================================="`)
    psLines = append(psLines, `foreach ($entry in $folders.GetEnumerator()) {`)
    psLines = append(psLines, `    $name = $entry.Key`)
    psLines = append(psLines, `    $linksFile = $entry.Value[0]`)
    psLines = append(psLines, `    $saveDir   = $entry.Value[1]`)
    psLines = append(psLines, `    Write-Host "Downloading: $name"`)

    psLines = append(psLines, `    $commonArgs = @("--max-concurrent-downloads=2", "--max-connection-per-server=4", "--split=4", "--user-agent=Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36", "--enable-http-keep-alive=false", "--check-certificate=false", "--console-log-level=notice", "--retry-wait=10", "--max-tries=5", "--timeout=30")`)
    psLines = append(psLines, `    $specificArgs = @("--dir=$saveDir", "--input-file=$linksFile")`)
    psLines = append(psLines, `    $args = $commonArgs + $specificArgs`)

    psLines = append(psLines, `    try {`)
    psLines = append(psLines, `        $process = Start-Process -FilePath $aria2 -ArgumentList $args -Wait -NoNewWindow -PassThru`)
    psLines = append(psLines, `        if ($process.ExitCode -ne 0) {`)
    psLines = append(psLines, `            Write-Host "  Warning: aria2c exited with code $($process.ExitCode)" -ForegroundColor Yellow`)
    psLines = append(psLines, `        }`)
    psLines = append(psLines, `        $global:foldersDone++`)
    psLines = append(psLines, `    } catch {`)
    psLines = append(psLines, `        Write-Host "  FAILED to start aria2c: $($_.Exception.Message)" -ForegroundColor Red`)
    psLines = append(psLines, `    }`)
    psLines = append(psLines, `}`)
    psLines = append(psLines, `Write-Host "===================================="`)
    psLines = append(psLines, `Write-Host "Processed $($global:foldersDone) folder(s)."`)
    psLines = append(psLines, `Write-Host "If some files failed, simply re-run this script to retry."`)
    psLines = append(psLines, `Read-Host "Press Enter to exit"`)

    content := strings.Join(psLines, "\r\n")
    bom := []byte{0xEF, 0xBB, 0xBF}
    if err := os.WriteFile(ps1Path, append(bom, []byte(content)...), 0644); err != nil {
        return err
    }

    // 2. 引导用的 .bat 脚本
    batPath := filepath.Join(scriptsDir, "aria2_download.bat")
    batLines := []string{
        "@echo off",
        "chcp 65001 > nul",
        "echo Starting Aria2 downloads via PowerShell...",
        "powershell -ExecutionPolicy Bypass -File \"%~dp0aria2_download.ps1\"",
        "pause",
    }
    batContent := strings.Join(batLines, "\r\n")
    if err := os.WriteFile(batPath, append(bom, []byte(batContent)...), 0644); err != nil {
        return err
    }

    // 3. Linux / macOS Shell 脚本 (原来的 RPC 方式)
    shPath := filepath.Join(scriptsDir, "aria2_download.sh")
    var shLines []string
    shLines = append(shLines, "#!/bin/bash")
    shLines = append(shLines, `BASE_DIR="$(cd "$(dirname "$0")/.." && pwd)"`)
    rpcPort := "6800"
    shLines = append(shLines, fmt.Sprintf("RPC_URL=\"http://localhost:%s/jsonrpc\"", rpcPort))
    shLines = append(shLines, "")
    shLines = append(shLines, "echo \"Make sure aria2 daemon is running on port " + rpcPort + "\"")
    shLines = append(shLines, "")
    for relPath, dir := range folderPaths {
        relLinksFile, _ := filepath.Rel(baseDir, filepath.Join(dir, "links.txt"))
        relSaveDir, _ := filepath.Rel(baseDir, dir)
        displayName := relPath
        if displayName == "" {
            displayName = "(root)"
        }
        saveDir := `"$BASE_DIR"`
        if relSaveDir != "." {
            saveDir = fmt.Sprintf(`"$BASE_DIR/%s"`, relSaveDir)
        }
        shLines = append(shLines, fmt.Sprintf("echo \"Processing: %s\"", displayName))
        shLines = append(shLines, fmt.Sprintf(
            "curl -s -X POST \"$RPC_URL\" -H \"Content-Type: application/json\" -d '{\"jsonrpc\":\"2.0\",\"method\":\"aria2.addUri\",\"id\":1,\"params\":[[$(cat \"$BASE_DIR/%s\" | sed '/^$/d' | paste -sd, - | sed 's/,/\",\"/g')],{\"dir\":%s,\"max-connection-per-server\":\"4\",\"split\":\"4\",\"continue\":\"true\"}]}'",
            relLinksFile, saveDir,
        ))
        shLines = append(shLines, "")
    }
    shLines = append(shLines, "echo \"All links added. Check AriaNg.\"")
    return os.WriteFile(shPath, []byte(strings.Join(shLines, "\n")), 0755)
}