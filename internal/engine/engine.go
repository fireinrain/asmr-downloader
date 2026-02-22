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
