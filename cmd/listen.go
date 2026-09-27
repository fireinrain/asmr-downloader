package cmd

import (
	"asmroner/internal/consts"
	"asmroner/internal/database"
	"asmroner/internal/logger"
	"asmroner/internal/model"
	"asmroner/webui"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/browser"
	"gorm.io/gorm"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
)

var listenPort int

// listen 命令
// 监听命令
// 用于启动一个 web UI 服务器，用于展示目录中的数据和简单管理
// 选项：
//
//	-p, --port int：服务器端口（默认 9999）
//	-d, --dir string：数据目录（默认是初始化配置的 syncdata 目录）
var listenCmd = &cobra.Command{
	Use:   "listen",
	Short: "启动WebUI以展示/播放下载的音声作品",
	Long: `
listen 命令用于启动一个 Web UI 服务器，用于展示和播放下载的音声作品。

参数说明：
  [dir]
    - 指定要展示的下载数据目录
    - 如果不提供，默认使用配置中 syncdata 目录
选项：
  -p, --port <端口号>
      指定服务器监听端口（默认 9999）。
      示例：
        asmroner listen -p 8080 ./syncdata
适用场景：
  - 可视化浏览下载的音声作品
  - 直接在浏览器播放或管理已下载的资源
  - 快速定位和查看同步数据目录中的作品
说明：
  - 会自动启动 HTTP 服务并提供静态文件访问
  - 提供简单 API 获取文件列表，可分页查询
  - 支持优雅关闭，按 Ctrl+C 停止服务器
  - 默认使用内嵌资源加载前端界面，无需额外配置
`,

	Run: func(cmd *cobra.Command, args []string) {
		// 优先使用命令行参数，args[0] 次之，默认 listenDir
		dataFolder := model.AppConfig.Downloader.SyncDataFolder
		if len(args) > 0 {
			dataFolder = args[0]
		}
		absDataFolder, err := filepath.Abs(dataFolder)
		if err != nil {
			logger.Fail("获取绝对路径失败: %v", err)
			return
		}

		if _, err := os.Stat(absDataFolder); os.IsNotExist(err) {
			logger.Fail("数据目录不存在: %s", absDataFolder)
			return
		}

		db := buildInmemoryDb(absDataFolder)

		folderName := filepath.Base(absDataFolder)
		port := listenPort
		if port == 0 {
			port = 9999
		}
		logger.Step("启动 Web UI，端口: %d，数据目录: %s", port, absDataFolder)

		// Gin Release 模式
		gin.SetMode(gin.ReleaseMode)
		r := gin.New()
		r.Use(gin.Logger(), gin.Recovery())

		fs := webui.GetFileSystem()
		r.StaticFS("/public", fs)

		// 首页
		r.GET("/", func(c *gin.Context) {
			content, err := webui.GetFileContent("index.html")
			if err != nil {
				c.String(http.StatusInternalServerError, "加载 index.html 失败")
				return
			}
			c.Data(http.StatusOK, "text/html; charset=utf-8", content)
		})

		// 静态文件服务
		r.StaticFS(fmt.Sprintf("/%s", folderName), gin.Dir(absDataFolder, true))

		// API: 获取文件列表
		r.GET("/api/list", func(c *gin.Context) {
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

			infos, total, err := getFolderInfoPage(db, page, pageSize, folderName)
			if err != nil {
				c.JSON(http.StatusInternalServerError, wrapResponse(err))
				return
			}
			// 补充音频计数
			for i := range infos {
				count := 0
				for _, f := range infos[i].Files {
					if !f.IsDir && isAudioFile(f.Name) {
						count++
					}
				}
				infos[i].AudioCount = count
			}
			c.JSON(http.StatusOK, wrapResponse(gin.H{
				"infos":    infos,
				"total":    total,
				"page":     page,
				"pageSize": pageSize,
			}))
		})

		// API: 搜索
		r.GET("/api/search", func(c *gin.Context) {
			q := strings.TrimSpace(c.Query("q"))
			page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
			pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))

			if q == "" {
				c.JSON(http.StatusOK, wrapResponse(gin.H{
					"infos":    []FolderInfo{},
					"total":    0,
					"page":     page,
					"pageSize": pageSize,
				}))
				return
			}

			infos, total, err := searchFolderInfo(db, q, page, pageSize)
			if err != nil {
				c.JSON(http.StatusInternalServerError, wrapResponse(err))
				return
			}
			c.JSON(http.StatusOK, wrapResponse(gin.H{
				"infos":    infos,
				"total":    total,
				"page":     page,
				"pageSize": pageSize,
			}))
		})

		addr := fmt.Sprintf(":%d", port)
		srv := &http.Server{
			Addr:              addr,
			Handler:           r,
			ReadTimeout:       10 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      0, // 大文件下载时不限制
			IdleTimeout:       120 * time.Second,
		}

		// 启动服务器
		go func() {
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("启动失败: %v", err)
			}
		}()

		// 延迟打开浏览器
		go func() {
			time.Sleep(500 * time.Millisecond) // 确保端口监听成功
			link := fmt.Sprintf("http://localhost:%d", port)
			browser.OpenURL(link)
		}()

		// 优雅退出
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		<-quit
		logger.Warn("接收到退出信号，正在优雅关闭服务器...")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("服务器强制关闭: %v", err)
		}

		logger.Done("服务器已成功关闭")
	},
}

func init() {
	rootCmd.AddCommand(listenCmd)
	listenCmd.Flags().IntVarP(&listenPort, "port", "p", 9999, "服务器端口")
}

// FolderInfo 用于 /api/list 的 JSON 输出
type FolderInfo struct {
	Id           int64      `gorm:"primaryKey" json:"id"`
	Name         string     `json:"name"`
	MediaId      string     `json:"mediaId"`
	Date         string     `json:"date"`
	HasSubtitles bool       `json:"hasSubtitles"`
	Title        string     `json:"title"`
	Vas          string     `json:"vas"`
	BaseDir      string     `json:"baseDir"`
	CoverPath    string     `json:"coverPath"`
	AudioCount   int        `gorm:"-" json:"audioCount"`
	Files        []FileInfo `gorm:"foreignKey:FolderId" json:"files"`
}

// FileInfo 文件信息
type FileInfo struct {
	Id       int64 `gorm:"primaryKey" json:"id"`
	FolderId int64 `json:"folderId"`

	Path  string `json:"path"`
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
	//ModTs int64  `json:"modTs"`
}

// scanDirectory 递归扫描目录，返回相对路径的文件列表（相对于 baseDir）
func scanDirectory(baseDir string) ([]FileInfo, error) {
	var list []FileInfo
	err := filepath.WalkDir(baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// 遇到错误继续（或返回 err 以中止）
			return err
		}
		// 跳过 baseDir 本身（如果你想包含可以去掉）
		if path == baseDir {
			return nil
		}
		//fi, err := d.Info()
		//if err != nil {
		//	return nil
		//}
		rel, err := filepath.Rel(baseDir, path)
		if err != nil {
			rel = d.Name()
		}
		// 统一为 slash 分隔，便于前端使用
		rel = filepath.ToSlash(rel)

		list = append(list, FileInfo{
			Path: rel,
			Name: d.Name(),
			//Size:  fi.Size(),
			IsDir: d.IsDir(),
			//ModTs: fi.ModTime().Unix(),
		})
		return nil
	})
	return list, err
}

// buildInmemoryDb 初始化内存数据库
func buildInmemoryDb(asbDataFolder string) *gorm.DB {
	//构建内存sqlite数据库
	db, err := database.NewInMemoryDb()
	if err != nil {
		log.Fatalf("Failed to create in-memory SQLite database: %v", err)
	}
	// 自动迁移数据库结构
	db.AutoMigrate(&FolderInfo{}, &FileInfo{})

	// 从主数据库加载声优映射 (mediaId -> vas)
	vasMap := loadVasMap()

	//defer db.Close()
	// 遍历asbDataFolder一级目录
	entries, err := os.ReadDir(asbDataFolder)
	baseDir := filepath.Base(asbDataFolder)
	if err != nil {
		log.Fatalf("Failed to read directory: %v", err)
	}
	//下载的数据目录命名格式可由用户配置（issue #46），
	// 这里用容错解析兼容旧格式与新格式
	for _, entry := range entries {
		if entry.IsDir() {
			mediaId, date, _, hasSubtitles, title, ok := parseFolderName(entry.Name())
			if !ok {
				continue
			}

			directory, err := scanDirectory(filepath.Join(asbDataFolder, entry.Name()))
			if err != nil {
				log.Fatalf("Failed to scan directory: %v", err)
			}
			// 检测封面图
			coverPath := detectCover(directory)
			// 统计音频文件数量
			audioCount := 0
			for _, f := range directory {
				if !f.IsDir && isAudioFile(f.Name) {
					audioCount++
				}
			}

			// 构建 FolderInfo
			folder := FolderInfo{
				MediaId:      mediaId,
				Date:         date,
				HasSubtitles: hasSubtitles,
				Title:        title,
				Vas:          vasMap[mediaId],
				Name:         entry.Name(),
				Files:        directory,
				BaseDir:      baseDir,
				CoverPath:    coverPath,
				AudioCount:   audioCount,
			}
			// 保存到数据库
			if err := db.Create(&folder).Error; err != nil {
				log.Fatalf("Failed to save folder %s to database: %v", folder.Name, err)
			}
		}
	}

	return db
}

// loadVasMap 从主数据库加载 source_id -> vas 的映射
func loadVasMap() map[string]string {
	m := make(map[string]string)
	mainDB, err := database.InitDB()
	if err != nil {
		log.Printf("主数据库未就绪，跳过声优信息加载: %v", err)
		return m
	}
	var works []model.MetadataWork
	if err := mainDB.Select("source_id, vas").Find(&works).Error; err != nil {
		log.Printf("查询声优信息失败: %v", err)
		return m
	}
	for _, w := range works {
		if w.Vas != "" {
			m[w.SourceID] = w.Vas
		}
	}
	return m
}

// detectCover 从文件列表中查找封面图（优先找封面命名的图片）
func detectCover(files []FileInfo) string {
	coverNames := map[string]bool{
		"cover.jpg": true, "cover.png": true, "cover.webp": true,
		"folder.jpg": true, "folder.png": true, "front.jpg": true,
	}
	for _, f := range files {
		if f.IsDir {
			continue
		}
		lower := strings.ToLower(f.Name)
		if coverNames[lower] {
			return f.Path
		}
	}
	// 没找到封面命名，用第一张图片
	for _, f := range files {
		if f.IsDir {
			continue
		}
		lower := strings.ToLower(f.Name)
		if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") ||
			strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".webp") {
			return f.Path
		}
	}
	return ""
}

// isAudioFile 判断文件名是否为音频文件
func isAudioFile(name string) bool {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".mp3"):
		return true
	case strings.HasSuffix(lower, ".wav"):
		return true
	case strings.HasSuffix(lower, ".flac"):
		return true
	case strings.HasSuffix(lower, ".m4a"):
		return true
	case strings.HasSuffix(lower, ".ogg"):
		return true
	case strings.HasSuffix(lower, ".aac"):
		return true
	}
	return false
}

// parseFolderName 尽力从目录名中解析作品信息，兼容新旧命名格式（issue #46）。
// 仅要求首段是合法作品ID；date/subtitle/title 允许缺失。
// ok=false 表示无法识别（非作品目录，应跳过）。
func parseFolderName(name string) (mediaId, date, subtitle string, hasSub bool, title string, ok bool) {
	tokens := strings.Split(name, "-")
	if len(tokens) == 0 {
		return "", "", "", false, "", false
	}

	// 首段必须是合法作品ID（唯一的硬性过滤条件）
	if !consts.AsmrOneIDRegex.MatchString(tokens[0]) {
		return "", "", "", false, "", false
	}
	mediaId = tokens[0]

	// 判断日期段：连续8位数字
	isDate := func(s string) bool {
		if len(s) != 8 {
			return false
		}
		for _, c := range s {
			if c < '0' || c > '9' {
				return false
			}
		}
		return true
	}

	rest := tokens[1:]
	// 在剩余 token 中寻找 sub/nosub 段
	for i, tok := range rest {
		if tok != "sub" && tok != "nosub" {
			continue
		}
		subtitle = tok
		hasSub = tok == "sub"
		if i > 0 && isDate(rest[0]) {
			date = rest[0]
		}
		title = strings.Join(rest[i+1:], "-")
		return mediaId, date, subtitle, hasSub, title, true
	}

	// 没有 sub/nosub 段：date 在首则 title 在次，否则整段视为 title
	if len(rest) > 0 && isDate(rest[0]) {
		date = rest[0]
		title = strings.Join(rest[1:], "-")
	} else {
		title = strings.Join(rest, "-")
	}
	return mediaId, date, subtitle, hasSub, title, true
}

// getFolderInfoPage 分页查询文件夹信息
func getFolderInfoPage(db *gorm.DB, page, pageSize int, baseDir string) ([]FolderInfo, int64, error) {
	var folders []FolderInfo
	var total int64

	// 先统计总数
	if err := db.Model(&FolderInfo{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 查询指定页的数据，并预加载 Files
	if err := db.Preload("Files").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&folders).Error; err != nil {
		return nil, 0, err
	}

	return folders, total, nil
}

// searchFolderInfo 搜索文件夹（按名称、MediaId、Title 模糊匹配）
func searchFolderInfo(db *gorm.DB, q string, page, pageSize int) ([]FolderInfo, int64, error) {
	var folders []FolderInfo
	var total int64

	like := "%" + q + "%"
	query := db.Model(&FolderInfo{}).
		Where("name LIKE ? OR media_id LIKE ? OR title LIKE ?", like, like, like)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Preload("Files").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&folders).Error; err != nil {
		return nil, 0, err
	}

	// 补充音频计数和封面
	for i := range folders {
		count := 0
		for _, f := range folders[i].Files {
			if !f.IsDir && isAudioFile(f.Name) {
				count++
			}
		}
		folders[i].AudioCount = count
	}

	return folders, total, nil
}

type ResultResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

// 包装resp
func wrapResponse(data any) ResultResp {
	//判断data 是不是error
	e, ok := data.(error)
	if ok {
		return ResultResp{
			Code: 500,
			Msg:  e.Error(),
			Data: nil,
		}
	}
	return ResultResp{
		Code: 200,
		Msg:  "",
		Data: data,
	}
}
