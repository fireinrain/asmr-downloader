package cmd

import (
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
	"regexp"
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
	BaseDir      string     `json:"baseDir"`
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
	//defer db.Close()
	// 遍历asbDataFolder一级目录
	entries, err := os.ReadDir(asbDataFolder)
	baseDir := filepath.Base(asbDataFolder)
	if err != nil {
		log.Fatalf("Failed to read directory: %v", err)
	}
	//下载的数据目录中的文件名必须符合
	// xxx-8位数字-[sub/nosub]-xxxxx
	r := regexp.MustCompile(`^[^-\s]+-\d{8}-(sub|nosub)-[^-\s]+$`)

	for _, entry := range entries {
		if entry.IsDir() {
			if !r.MatchString(entry.Name()) {
				continue
			}
			//切分信息
			splitStr := strings.Split(entry.Name(), "-")
			mediaId := splitStr[0]
			date := splitStr[1]
			hasSubtitles := splitStr[2]
			title := splitStr[3]

			directory, err := scanDirectory(filepath.Join(asbDataFolder, entry.Name()))
			if err != nil {
				log.Fatalf("Failed to scan directory: %v", err)
			}
			// 构建 FolderInfo
			folder := FolderInfo{
				MediaId:      mediaId,
				Date:         date,
				HasSubtitles: hasSubtitles == "sub",
				Title:        title,
				Name:         entry.Name(),
				Files:        directory,
				BaseDir:      baseDir,
			}
			// 保存到数据库
			if err := db.Create(&folder).Error; err != nil {
				log.Fatalf("Failed to save folder %s to database: %v", folder.Name, err)
			}
		}
	}

	return db
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
