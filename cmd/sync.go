package cmd

import (
	"asmroner/internal/database"
	"asmroner/internal/engine"
	"asmroner/internal/logger"
	"asmroner/internal/model"
	"asmroner/internal/utils"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

// 命令
// 1. xxx sync 执行sync同步元数据
// 2. xxx sync download -d <download_folder> 执行sync元数据并下载文件到指定目录 不指定使用默认folder,下载总数受到配置容量限制
// 3. xxx sync retry -d <download_folder> 重试指定目录下失败的文件 不指定使用默认folder
// 4. xxx sync export -s <failed|success> -f <export_file> 导出指定状态的文件到指定文件,默认导出失败文件
var downloadFolder string
var syncExportFile string
var exportStatus string

// syncCmd 是根 sync 命令
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "同步元数据并管理文件下载",
	Long: `
sync 命令用于同步资源元数据，并管理文件下载、失败重试及导出操作。

可用子命令：
  download       同步元数据并下载文件
  retry          重试指定目录下下载失败的文件
  export         导出指定状态的文件列表（failed/success）
  report         打印相关统计数据

示例：
  asmroner sync
      仅同步元数据，不下载文件

  asmroner sync download -d ./downloads
      同步元数据并下载文件到指定目录

  asmroner sync retry -d ./downloads
      重试指定目录下失败的下载文件

  asmroner sync export -s failed -f failed_files.csv
      导出失败文件列表到 CSV 文件

  asmroner sync export
      打印相关统计数据
`,
	Run: func(cmd *cobra.Command, args []string) {
		err := doSyncMetadata()
		if err != nil {
			logger.Fail("同步元数据失败: %v", err)
			return
		}
		time.Sleep(2 * time.Second)
		logger.Done("作品元数据同步完成")
	},
}

func doSyncMetadata() error {
	engineManager, err := engine.NewEngineManager(
		model.AppConfig.Limit.SyncQPS, 1,
		model.AppConfig.Limit.SyncJitterMin,
		model.AppConfig.Limit.SyncJitterMax,
	)
	if err != nil {
		return fmt.Errorf("创建下载引擎管理器失败: %w", err)
	}
	ctx := context.Background()
	return engineManager.SyncMetadata(ctx)
}

// ------------------------- download 子命令 -------------------------
var syncDownloadCmd = &cobra.Command{
	Use:   "download",
	Short: "同步元数据并下载文件",
	Long: `
sync download 子命令用于在同步元数据的同时下载文件。

可用选项：
  -d, --folder <目录路径>
      指定下载文件保存目录，默认当前目录。
      示例：asmroner sync download -d ./downloads

适用场景：
  - 同步并下载新资源
  - 批量更新已有资源
`,
	Run: func(cmd *cobra.Command, args []string) {
		if downloadFolder == "" {
			downloadFolder = model.AppConfig.Downloader.SyncDataFolder
		}
		err := doSyncMetadata()
		if err != nil {
			logger.Fail("同步元数据失败: %v", err)
			return
		}
		time.Sleep(2 * time.Second)
		doSyncDownload(downloadFolder)
		logger.Done("文件已成功下载到 %s", downloadFolder)
	},
}

func doSyncDownload(dir string) {
	db := database.Database
	if db == nil {
		logger.Fail("数据库连接未初始化")
		return
	}
	downloadLimitSize, err := utils.FileSize2Byte(model.AppConfig.Downloader.SyncWantedSize)
	if err != nil {
		logger.Warn("解析SyncWantedSize失败: %v，使用默认值 1GB", err)
		downloadLimitSize = 1024 * 1024 * 1024
	}
	cleanSyncDownPendingData(db)

	var batchSize = 1
	var batchCount = 1
	for {
		// 检查是否需要同步下载
		needSync, hasDownSize, err := checkIfNeedSyncDownload(db, downloadLimitSize)
		if err != nil {
			return
		}
		if !needSync {
			break
		}
		logger.Info("已下载: %s, 限制: %s",
			utils.Byte2FileSize(hasDownSize), utils.Byte2FileSize(downloadLimitSize))
		time.Sleep(3 * time.Second)
		doBatchSyncDownload(dir, batchSize, batchCount, downloadLimitSize, db)
		batchCount++
	}

}

func doBatchSyncDownload(downDir string, batchSize int, batchCount int, downloadLimitSize int64, db *gorm.DB) {
	var needSyncCount int64
	result := db.Table("metadata_works").
		Where("id NOT IN (SELECT metadata_work_id FROM work_sync_infos)").
		Count(&needSyncCount)
	if result.Error != nil {
		logger.Fail("查询metadata_works失败: %v", result.Error)
		return
	}
	if needSyncCount == 0 {
		logger.Done("没有需要同步下载的新作品")
		return
	}
	logger.Info("找到 %d 个需要同步下载的作品", needSyncCount)

	// 查询一批待下载的作品
	var metadataWorks []model.MetadataWork
	result = db.Table("metadata_works").
		Where("id NOT IN (SELECT metadata_work_id FROM work_sync_infos)").
		Limit(batchSize).
		Find(&metadataWorks)
	if result.Error != nil {
		logger.Fail("查询待下载作品失败: %v", result.Error)
		return
	}
	if len(metadataWorks) == 0 {
		logger.Done("没有需要同步下载的新作品")
		return
	}
	logger.Step("正在执行第 %d 次批量(%d)下载...", batchCount, batchSize)

	// 构建 WorkSyncInfo 记录
	var workSyncInfos []model.WorkSyncInfo
	for _, work := range metadataWorks {
		valid, prefix, number, err := utils.IsValidDlsiteID(work.SourceID)
		if err != nil || !valid {
			logger.Warn("无效的作品ID: %s，跳过...", work.SourceID)
			continue
		}
		hasSubtitle := "nosub"
		if work.HasSubtitle {
			hasSubtitle = "sub"
		}
		folderName := fmt.Sprintf(
			"%s%s-%s-%s-%s",
			strings.ToUpper(prefix),
			number,
			strings.ReplaceAll(work.Release, "-", ""),
			hasSubtitle,
			utils.NormalDirPathStr(strings.ReplaceAll(work.Title, "/", "")),
		)

		workSyncInfo := model.WorkSyncInfo{
			MetadataWorkId: work.ID,
			SourceId:       work.SourceID,
			HasSubtitle:    work.HasSubtitle,
			DirSize:        0,
			Status:         "PENDING",
			FilePath:       filepath.Join(downDir, folderName),
			UpdatedAt:      time.Now(),
		}
		workSyncInfos = append(workSyncInfos, workSyncInfo)
	}

	result = db.Create(&workSyncInfos)
	if result.Error != nil {
		logger.Fail("插入work_sync_infos失败: %v", result.Error)
		return
	}

	manager, err := engine.NewEngineManager(
		model.AppConfig.Limit.DownloadQPS, 1,
		model.AppConfig.Limit.DownloadJitterMin,
		model.AppConfig.Limit.DownloadJitterMax,
	)
	if err != nil {
		logger.Fail("创建下载引擎管理器失败: %v", err)
		return
	}
	ctx := context.Background()

	// 逐个下载，每次提交前限速，下载后检查大小限制
	for i := range workSyncInfos {
		info := &workSyncInfos[i]

		// 限速：在提交下载前等待令牌
		if err := manager.DownLimiter.Wait(ctx); err != nil {
			logger.Fail("限流器等待失败: %v", err)
			break
		}

		task := logger.NewTask(info.SourceId)
		task.Info("开始下载")
		downErr := manager.DownloadOne(ctx, info.SourceId, downDir)

		if downErr != nil {
			task.Error("下载失败: %s", logger.SummarizeError(downErr))
			info.Status = "FAILED"
			info.FailReason = downErr.Error()
			info.FailedAt = time.Now()
		} else {
			info.Status = "COMPLETED"
			size, sizeErr := utils.GetDirSize(info.FilePath)
			if sizeErr != nil {
				task.Warn("计算目录大小失败: %v", sizeErr)
			}
			info.DirSize = size
			task.Info("下载完成, 大小: %s", utils.Byte2FileSize(size))
		}
		info.UpdatedAt = time.Now()

		// 更新数据库状态
		updateResult := db.Model(&model.WorkSyncInfo{}).
			Where("metadata_work_id = ?", info.MetadataWorkId).
			Updates(map[string]interface{}{
				"status":       info.Status,
				"dir_size":     info.DirSize,
				"updated_at":   info.UpdatedAt,
				"fail_reason":  info.FailReason,
				"retry_count":  info.RetryCount,
				"failed_at":    info.FailedAt,
				"has_subtitle": info.HasSubtitle,
			})
		if updateResult.Error != nil {
			logger.Error("更新work_sync_infos失败 (作品ID: %d): %v", info.MetadataWorkId, updateResult.Error)
		}

		needSync, _, checkErr := checkIfNeedSyncDownload(db, downloadLimitSize)
		if checkErr != nil {
			logger.Error("检查下载限制失败: %v", checkErr)
			break
		}
		if !needSync {
			logger.Info("已达到下载目标大小限制，停止剩余下载任务")
			break
		}
	}

	logger.Done("单次批量同步下载完成")
}

func cleanSyncDownPendingData(db *gorm.DB) {
	//移除所有status 为 Pending 的作品目录数据
	var pendingSyncInfos []model.WorkSyncInfo
	tx := db.Table("work_sync_infos").Where("status = ?", "PENDING").Find(&pendingSyncInfos)
	if tx.Error != nil {
		logger.Error("查询work_sync_infos失败: %v", tx.Error)
		return
	}
	for _, info := range pendingSyncInfos {
		err := os.RemoveAll(info.FilePath)
		if err != nil {
			logger.Warn("删除目录失败: %v", err)
		}
	}
	t := db.Table("work_sync_infos").Where("status = ?", "PENDING").Delete(&model.WorkSyncInfo{})
	if t.Error != nil {
		logger.Error("删除work_sync_infos失败: %v", t.Error)
		return
	}
}

func checkIfNeedSyncDownload(db *gorm.DB, downloadLimitSize int64) (bool, int64, error) {
	// 1. 查询所有在work_sync_infos 表中的status 为 SUCCESS 的作品目录数据总大小
	var totalSize int64
	// 使用Raw SQL查询，参数化查询避免SQL注入
	result := db.Raw("SELECT COALESCE(SUM(dir_size), 0) as total_size FROM work_sync_infos WHERE status = ?", "COMPLETED").
		Scan(&totalSize)

	if result.Error != nil {
		logger.Error("查询work_sync_infos失败: %v", result.Error)
		return false, totalSize, result.Error
	}
	if totalSize >= downloadLimitSize {
		logger.Done("已下载的数据大小已超过配置的设定值，无需继续下载")
		return false, totalSize, nil
	}
	return true, totalSize, nil
}

// ------------------------- retry-failed 子命令 -------------------------
var retryFailedCmd = &cobra.Command{
	Use:   "retry",
	Short: "重试下载失败的文件",
	Long: `
sync retry 子命令用于重试指定目录下下载失败的文件。

可用选项：
  -d, --folder <目录路径>
      指定失败文件所在目录，如果不指定使用默认下载目录。
      示例：asmroner sync retry -d ./downloads

适用场景：
  - 网络异常或部分文件下载失败后重试
`,
	Run: func(cmd *cobra.Command, args []string) {
		if downloadFolder == "" {
			downloadFolder = model.AppConfig.Downloader.SyncDataFolder
		}
		db := database.Database
		if db == nil {
			logger.Fail("数据库连接未初始化")
			return
		}
		var failedSyncInfos []model.WorkSyncInfo
		tx := db.Table("work_sync_infos").Where("status = ?", "FAILED").Find(&failedSyncInfos)
		if tx.Error != nil {
			logger.Fail("查询work_sync_infos失败: %v", tx.Error)
			return
		}
		if len(failedSyncInfos) == 0 {
			logger.Done("没有需要重试下载的文件")
			return
		}
		logger.Info("有 %d 个文件需要重试下载", len(failedSyncInfos))

		manager, err := engine.NewEngineManager(
			model.AppConfig.Limit.DownloadQPS, 1,
			model.AppConfig.Limit.DownloadJitterMin,
			model.AppConfig.Limit.DownloadJitterMax,
		)
		if err != nil {
			logger.Fail("创建下载引擎管理器失败: %v", err)
			return
		}

		for _, info := range failedSyncInfos {
			task := logger.NewTask(info.SourceId)
			task.Info("重试下载")
			if err := manager.DownLimiter.Wait(context.Background()); err != nil {
				logger.Fail("限流器等待失败: %v", err)
				break
			}
			err := doSyncFailedDownload(db, manager, info)
			if err != nil {
				task.Error("重试下载失败: %s", logger.SummarizeError(err))
			}
		}
		logger.Done("重试下载完成，目录: %s", downloadFolder)
	},
}

func doSyncFailedDownload(db *gorm.DB, manager *engine.EngineManager, info model.WorkSyncInfo) error {
	if err := os.RemoveAll(info.FilePath); err != nil {
		return fmt.Errorf("删除旧目录失败: %w", err)
	}
	ctx := context.Background()
	err := manager.DownloadOne(ctx, info.SourceId, filepath.Dir(info.FilePath))
	if err != nil {
		return err
	}
	info.Status = "COMPLETED"
	info.FailReason = ""
	info.RetryCount++
	info.UpdatedAt = time.Now()
	tx := db.Table("work_sync_infos").Where("id = ?", info.ID).Updates(info)
	return tx.Error
}

// ------------------------- export 子命令 -------------------------
var syncExportCmd = &cobra.Command{
	Use:   "export",
	Short: "导出文件状态列表",
	Long: `
sync export 子命令用于将文件按状态导出为 CSV/JSON 文件，便于管理或统计。

参数说明：
  -s, --status <failed|success>
      指定要导出的文件状态：failed（失败）或 success（成功）
  -f, --file <文件路径>
      指定导出文件路径及文件名，支持 .csv/.json

示例：
  asmroner sync export -s failed -f failed_files.csv
      导出失败文件列表

  asmroner sync export -s success -f success_files.json
      导出成功文件列表

适用场景：
  - 统计成功或失败下载文件
  - 后续处理失败文件
`,
	Run: func(cmd *cobra.Command, args []string) {
		if exportStatus != "failed" && exportStatus != "success" {
			fmt.Println("❌ 状态无效，必须为 'failed' 或 'success'")
			os.Exit(1)
		}
		db := database.Database
		if db == nil {
			logger.Fail("数据库连接未初始化")
			return
		}
		if syncExportFile == "" {
			syncExportFile = fmt.Sprintf("%s_%s.csv", exportStatus, time.Now().Format("20060102150405"))
		}
		if strings.HasSuffix(syncExportFile, ".csv") {
			//export csv
			exportCSV(db, exportStatus, syncExportFile)
		} else if strings.HasSuffix(syncExportFile, ".json") {
			//export json
			exportJSON(db, exportStatus, syncExportFile)
		}

		logger.Done("已导出 %s 记录到 %s", exportStatus, syncExportFile)
	},
}

func exportJSON(db *gorm.DB, status string, file string) {
	if status == "success" {
		status = "COMPLETED"
	} else {
		status = "FAILED"
	}
	var syncInfos []model.WorkSyncInfo
	tx := db.Table("work_sync_infos").Where("status = ?", status).Find(&syncInfos)
	if tx.Error != nil {
		logger.Fail("查询work_sync_infos失败: %v", tx.Error)
		return
	}
	if len(syncInfos) == 0 {
		logger.Done("没有需要导出的作品文件记录")
		return
	}
	logger.Info("有 %d 个作品文件记录需要导出", len(syncInfos))
	f, err := os.Create(file)
	if err != nil {
		logger.Fail("创建 JSON 文件失败: %v", err)
		return
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(syncInfos); err != nil {
		logger.Fail("写入 JSON 文件失败: %v", err)
		return
	}

	logger.Done("已成功导出 %d 条记录到 %s", len(syncInfos), file)
}

func exportCSV(db *gorm.DB, status string, file string) {
	if status == "success" {
		status = "COMPLETED"
	} else {
		status = "FAILED"
	}
	// 1. 查询所有在work_sync_infos 表中的status 为 status 的作品目录数据
	var syncInfos []model.WorkSyncInfo
	tx := db.Table("work_sync_infos").Where("status = ?", status).Find(&syncInfos)
	if tx.Error != nil {
		logger.Fail("查询work_sync_infos失败: %v", tx.Error)
		return
	}
	if len(syncInfos) == 0 {
		logger.Done("没有需要导出的作品文件记录")
		return
	}
	logger.Info("有 %d 个作品文件记录需要导出", len(syncInfos))
	headers := []string{
		"ID", "MetadataWorkId", "SourceId", "DirSize", "Status", "FilePath", "UpdatedAt", "FailReason", "RetryCount", "FailedAt", "HasSubtitle",
	}
	// 2.2 定义 CSV 数据行
	rows := make([][]string, 0, len(syncInfos))
	for _, info := range syncInfos {
		rows = append(rows, []string{
			fmt.Sprintf("%d", info.ID),
			fmt.Sprintf("%d", info.MetadataWorkId),
			info.SourceId,
			fmt.Sprintf("%d", info.DirSize),
			info.Status,
			info.FilePath,
			info.UpdatedAt.Format(time.RFC3339),
			info.FailReason,
			fmt.Sprintf("%d", info.RetryCount),
			info.FailedAt.Format(time.RFC3339),
			fmt.Sprintf("%v", info.HasSubtitle),
		})
	}
	// 2.3 写入 CSV 文件
	f, err := os.Create(file)
	if err != nil {
		logger.Fail("创建 CSV 文件失败: %v", err)
		return
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write(headers); err != nil {
		logger.Fail("写入 CSV 列头失败: %v", err)
		return
	}

	for _, row := range rows {
		if err := w.Write(row); err != nil {
			logger.Fail("写入 CSV 数据行失败: %v", err)
			return
		}
	}

	logger.Done("已成功导出 %d 条记录到 %s", len(rows), file)
}

var syncReportCmd = &cobra.Command{
	Use:   "report",
	Short: "打印相关统计数据",
	Long: `
sync report 子命令用于打印相关统计数据。

示例：
  asmroner sync report

适用场景：
  - 查看相关统计数据
`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Step("相关统计数据如下:")
		db := database.Database
		if db == nil {
			logger.Fail("数据库连接未初始化")
			return
		}
		var total, withSubtitle, withoutSubtitle int64
		tx := db.Table("metadata_works").
			Select("COUNT(*) AS total, COUNT(CASE WHEN has_subtitle THEN 1 END) AS withSubtitle, COUNT(CASE WHEN NOT has_subtitle THEN 1 END) AS withoutSubtitle").
			Row()
		if tx.Err() != nil {
			logger.Fail("查询metadata_works失败: %v", tx.Err())
			return
		}
		tx.Scan(&total, &withSubtitle, &withoutSubtitle)
		logger.Info("元数据总量: %d, 带字幕数量: %d, 不带字幕数量: %d", total, withSubtitle, withoutSubtitle)
		//查询同步下载数量,总下载带字幕数量，总下载不带字幕数量,失败数量，等待下载数量
		var syncDownloaded, syncFailed, syncPending, syncWithSubtitle, syncWithoutSubtitle int64
		tx = db.Table("work_sync_infos").
			Joins("JOIN metadata_works ON work_sync_infos.metadata_work_id = metadata_works.id").
			Select("COUNT(CASE WHEN work_sync_infos.status = 'COMPLETED' THEN 1 END) AS syncDownloaded, " +
				"COUNT(CASE WHEN work_sync_infos.status = 'FAILED' THEN 1 END) AS syncFailed, " +
				"COUNT(CASE WHEN work_sync_infos.status = 'PENDING' THEN 1 END) AS syncPending, " +
				"COUNT(CASE WHEN work_sync_infos.status = 'COMPLETED' AND metadata_works.has_subtitle THEN 1 END) AS syncWithSubtitle, " +
				"COUNT(CASE WHEN work_sync_infos.status = 'COMPLETED' AND NOT metadata_works.has_subtitle THEN 1 END) AS syncWithoutSubtitle").
			Row()
		if tx.Err() != nil {
			logger.Fail("查询work_sync_infos失败: %v", tx.Err())
			return
		}
		tx.Scan(&syncDownloaded, &syncFailed, &syncPending, &syncWithSubtitle, &syncWithoutSubtitle)
		logger.Info("同步下载数量: %d, 带字幕: %d, 不带字幕: %d, 失败: %d, 等待: %d",
			syncDownloaded, syncWithSubtitle, syncWithoutSubtitle, syncFailed, syncPending)

		syncProgress := float64(syncDownloaded) / float64(total) * 100
		syncWithSubtitleProgress := float64(syncWithSubtitle) / float64(withSubtitle) * 100
		syncWithoutSubtitleProgress := float64(syncWithoutSubtitle) / float64(withoutSubtitle) * 100
		logger.Info("同步进度: %.2f%%, 带字幕进度: %.2f%%, 不带字幕进度: %.2f%%",
			syncProgress, syncWithSubtitleProgress, syncWithoutSubtitleProgress)

	},
}

func init() {
	rootCmd.AddCommand(syncCmd)

	// 添加子命令
	syncCmd.AddCommand(syncDownloadCmd)
	syncCmd.AddCommand(retryFailedCmd)
	syncCmd.AddCommand(syncExportCmd)
	syncCmd.AddCommand(syncReportCmd)

	// 添加 flag
	syncDownloadCmd.Flags().StringVarP(&downloadFolder, "folder", "d", "", "下载文件保存目录(默认配置目录)")
	retryFailedCmd.Flags().StringVarP(&downloadFolder, "folder", "d", "", "下载失败文件所在目录(默认为配置目录)")

	syncExportCmd.Flags().StringVarP(&exportStatus, "status", "s", "", "导出文件状态（failed|success）")
	syncExportCmd.Flags().StringVarP(&syncExportFile, "file", "f", "", "导出文件路径（CSV/JSON）")
	syncExportCmd.MarkFlagRequired("status")
	syncExportCmd.MarkFlagRequired("file")
}
