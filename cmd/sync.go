package cmd

import (
	"asmroner/internal/database"
	"asmroner/internal/engine"
	"asmroner/internal/model"
	"asmroner/internal/utils"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
			log.Println("❌ 同步元数据失败:", err)
			return
		}
		time.Sleep(2 * time.Second)
		log.Println("✅ 作品元数据同步完成")
	},
}

func doSyncMetadata() error {
	engineManager, err := engine.NewEngineManager()
	if err != nil {
		log.Fatalf("❌创建下载引擎管理器失败: %v\n", err)
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
			//use default download folder
			downloadFolder = model.AppConfig.Downloader.SyncDataFolder
		}
		// 同步元数据
		err := doSyncMetadata()
		if err != nil {
			log.Println("❌ 同步元数据失败:", err)
			return
		}
		time.Sleep(2 * time.Second)
		doSyncDownload(downloadFolder)
		log.Println("✅ 文件已成功下载到", downloadFolder)
	},
}

func doSyncDownload(dir string) {
	//当所有在work_sync_infos 表中的status 为 SUCCESS 的作品目录数据总大小小于 配置文件的设定值 时,进行循环下载
	// 获取数据库连接
	db := database.Database
	if db == nil {
		log.Println("❌ 数据库连接未初始化")
		return
	}
	//获取下载大小限制
	downloadLimitSize, err := utils.FileSize2Byte(model.AppConfig.Downloader.SyncWantedSize)
	if err != nil {
		log.Println("❌ 解析SyncWantedSize失败:", err)
		// 下载大小限制默认值为 1GB
		downloadLimitSize = 1024 * 1024 * 1024
	}
	//移除所有status 为 Pending 的作品目录数据
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
		log.Printf("✅ 已下载的数据大小: %d byte, 下载限制: %d byte\n", hasDownSize, downloadLimitSize)
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
		log.Println("❌ 查询metadata_works失败:", result.Error)
		return
	}
	// 如果没有需要同步的作品,则直接返回
	if needSyncCount == 0 {
		log.Println("✅ 没有需要同步下载的新作品")
		return
	}
	batchCounts := int(math.Ceil(float64(needSyncCount) / float64(batchSize)))
	log.Printf("📥 找到 %d 个需要同步下载的作品,共需要 %d 批次下载", needSyncCount, batchCounts)
	// 1. 查询五条metadata_work,并且id不在 work_sync_infos 表中的数据
	var metadataWorks []model.MetadataWork
	result = db.Table("metadata_works").
		Where("id NOT IN (SELECT metadata_work_id FROM work_sync_infos)").
		Limit(batchSize).
		Find(&metadataWorks)

	if result.Error != nil {
		log.Println("❌ 查询metadata_works失败:", result.Error)
		return
	}

	if len(metadataWorks) == 0 {
		log.Println("✅ 没有需要同步下载的新作品")
		return
	}
	log.Printf("🔨 正在执行第 %d 次批量(%d)下载...\n", batchCount, batchSize)

	log.Printf("📥 找到 %d 个需要同步的新作品", len(metadataWorks))

	// 2. 插入到work_sync_infos 表中,并初始化下载状态
	var workSyncInfos []model.WorkSyncInfo
	for _, work := range metadataWorks {
		valid, prefix, number, err := utils.IsValidDlsiteID(work.SourceID)
		if err != nil || !valid {
			log.Printf("❌ 无效的作品ID: %s, 跳过...\n", work.SourceID)
			continue
		}
		hasSubtitle := "nosub"
		if work.HasSubtitle {
			hasSubtitle = "sub"
		}
		//新建下载目录名
		folderName := fmt.Sprintf(
			"%s%s-%s-%s-%s",
			strings.ToUpper(prefix),
			number,
			strings.ReplaceAll(work.Release, "-", ""),
			hasSubtitle,
			//修正标题 移除目录不支持的特殊字符
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
			FailReason:     "",
			RetryCount:     0,
		}
		workSyncInfos = append(workSyncInfos, workSyncInfo)
	}

	result = db.Create(&workSyncInfos)
	if result.Error != nil {
		log.Println("❌ 插入work_sync_infos失败:", result.Error)
		return
	}

	// 在函数内部添加以下修改：

	// 3. 循环发送数据到下载chan
	// 创建下载通道和结果通道
	downloadChan := make(chan model.WorkSyncInfo, len(workSyncInfos))
	resultChan := make(chan struct {
		SyncInfo model.WorkSyncInfo
		Error    error
		Size     int64
	}, len(workSyncInfos))
	cancelChan := make(chan struct{})
	var wg sync.WaitGroup

	// 启动下载工作池
	manager, err := engine.NewEngineManager()
	if err != nil {
		log.Fatalf("❌创建下载引擎管理器失败: %v\n", err)
	}
	ctx := context.Background()
	workerCount := batchSize // 可配置的工作线程数
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(downDir string) {
			defer wg.Done()
			for {
				select {
				case syncInfo, ok := <-downloadChan:
					if !ok {
						return
					}

					log.Printf("🚀 开始下载作品: %s", syncInfo.SourceId)
					// 这里应该调用实际的下载函数
					// 模拟下载延迟
					downError := manager.DownloadOne(ctx, syncInfo.SourceId, downDir)
					if downError != nil {
						log.Printf("❌ 下载作品 %s 失败: %v", syncInfo.SourceId, downError)
						syncInfo.Status = "FAILED"
						syncInfo.FailReason = downError.Error()
						syncInfo.FailedAt = time.Now()
						continue
					} else {
						syncInfo.Status = "COMPLETED"
						//计算下载完成的目录的大小
						size, err := utils.GetDirSize(syncInfo.FilePath)
						if err != nil {
							log.Printf("❌ 计算目录大小失败: %v", err)
							// 注意：这里不再使用continue，而是设置一个默认值
							syncInfo.DirSize = 0
						}
						syncInfo.DirSize = size
					}
					// 模拟下载结果
					//time.Sleep(1 * time.Second)
					//var err error
					//var size int64 = 1024 * 1024 // 模拟1MB大小
					//
					//if rand.Intn(10) < 2 { // 20%失败概率
					//	err = fmt.Errorf("模拟下载失败")
					//	syncInfo.Status = "FAILED"
					//	syncInfo.FailReason = err.Error()
					//	syncInfo.FailedAt = time.Now()
					//} else {
					//	syncInfo.Status = "COMPLETED"
					//	syncInfo.DirSize = size
					//}
					syncInfo.UpdatedAt = time.Now()

					// 发送结果前检查cancelChan是否已关闭
					select {
					case <-cancelChan:
						return
					case resultChan <- struct {
						SyncInfo model.WorkSyncInfo
						Error    error
						Size     int64
					}{syncInfo, downError, syncInfo.DirSize}:
						// 结果发送成功
					}
				case <-cancelChan:
					return
				}
			}
		}(downDir)
	}

	// 将工作发送到下载通道
	for _, syncInfo := range workSyncInfos {
		downloadChan <- syncInfo
	}

	// 关闭downloadChan，表示所有工作已经发送完毕
	close(downloadChan)

	// 4. 下载chan 下载完 推送 result channel
	// 5. 从result chan 读取数据并更新work_sync_infos 表中的status
	var totalDownloadedSize int64 = 0
	var maxSize = downloadLimitSize
	var needCancel bool

	doneCount := 0
	for doneCount < len(workSyncInfos) && !needCancel {
		select {
		case result := <-resultChan:
			doneCount++

			// 更新数据库状态
			updateResult := db.Model(&model.WorkSyncInfo{}).
				Where("metadata_work_id = ?", result.SyncInfo.MetadataWorkId).
				Updates(map[string]interface{}{
					"status":       result.SyncInfo.Status,
					"dir_size":     result.SyncInfo.DirSize,
					"updated_at":   result.SyncInfo.UpdatedAt,
					"fail_reason":  result.SyncInfo.FailReason,
					"retry_count":  result.SyncInfo.RetryCount,
					"failed_at":    result.SyncInfo.FailedAt,
					"has_subtitle": result.SyncInfo.HasSubtitle,
				})

			if updateResult.Error != nil {
				log.Printf("❌ 更新work_sync_infos失败(作品ID: %d): %v", result.SyncInfo.MetadataWorkId, updateResult.Error)
			} else {
				if result.SyncInfo.Status == "COMPLETED" {
					totalDownloadedSize += result.Size
					log.Printf("✅ 作品ID: %d 下载完成, 大小: %d bytes", result.SyncInfo.MetadataWorkId, result.Size)
				} else {
					log.Printf("❌ 作品ID: %d 下载失败: %v", result.SyncInfo.MetadataWorkId, result.Error)
				}
			}

			// 6. 当下载完的总数据大小 等于配置文件的设定值 取消所有下载任务
			if totalDownloadedSize >= maxSize {
				log.Printf("📦 已达到下载目标大小 (%d bytes), 取消剩余下载任务", totalDownloadedSize)
				close(cancelChan)
				needCancel = true
				// 不立即关闭resultChan，而是等待正在处理的任务完成
			}
		}
	}

	// 等待所有工作池goroutine完成
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// 继续处理剩余的结果
	for result := range resultChan {
		if !needCancel { // 只有在未取消的情况下才更新计数
			doneCount++
		}

		// 更新数据库状态
		updateResult := db.Model(&model.WorkSyncInfo{}).
			Where("metadata_work_id = ?", result.SyncInfo.MetadataWorkId).
			Updates(map[string]interface{}{
				"status":       result.SyncInfo.Status,
				"dir_size":     result.SyncInfo.DirSize,
				"updated_at":   result.SyncInfo.UpdatedAt,
				"fail_reason":  result.SyncInfo.FailReason,
				"retry_count":  result.SyncInfo.RetryCount,
				"failed_at":    result.SyncInfo.FailedAt,
				"has_subtitle": result.SyncInfo.HasSubtitle,
			})

		if updateResult.Error != nil {
			log.Printf("❌ 更新work_sync_infos失败(作品ID: %d): %v", result.SyncInfo.MetadataWorkId, updateResult.Error)
		} else {
			if result.SyncInfo.Status == "COMPLETED" && !needCancel {
				totalDownloadedSize += result.Size
				log.Printf("✅ 作品ID: %d 下载完成, 大小: %d bytes", result.SyncInfo.MetadataWorkId, result.Size)
			} else {
				log.Printf("❌ 作品ID: %d 下载失败: %v", result.SyncInfo.MetadataWorkId, result.Error)
			}
		}
	}
	// done标签不再需要
	log.Printf("✅ 单次批量同步下载完成, 下载大小: %d bytes", totalDownloadedSize)
}

func cleanSyncDownPendingData(db *gorm.DB) {
	//移除所有status 为 Pending 的作品目录数据
	var pendingSyncInfos []model.WorkSyncInfo
	tx := db.Table("work_sync_infos").Where("status = ?", "PENDING").Find(&pendingSyncInfos)
	if tx.Error != nil {
		log.Println("❌ 查询work_sync_infos失败:", tx.Error)
		return
	}
	for _, info := range pendingSyncInfos {
		err := os.RemoveAll(info.FilePath)
		if err != nil {
			log.Println("❌ 删除目录失败:", err)
		}
	}
	//删除所有status 为 PENDING 的作品目录数据
	t := db.Table("work_sync_infos").Where("status = ?", "PENDING").Delete(&model.WorkSyncInfo{})
	if t.Error != nil {
		log.Println("❌ 删除work_sync_infos失败:", t.Error)
		return
	}
	return
}

func checkIfNeedSyncDownload(db *gorm.DB, downloadLimitSize int64) (bool, int64, error) {
	// 1. 查询所有在work_sync_infos 表中的status 为 SUCCESS 的作品目录数据总大小
	var totalSize int64
	// 使用Raw SQL查询，参数化查询避免SQL注入
	result := db.Raw("SELECT COALESCE(SUM(dir_size), 0) as total_size FROM work_sync_infos WHERE status = ?", "COMPLETED").
		Scan(&totalSize)

	if result.Error != nil {
		log.Println("❌ 查询work_sync_infos失败:", result.Error)
		return false, totalSize, result.Error
	}
	// 2. 比较总大小与配置文件的设定值
	if totalSize >= downloadLimitSize {
		log.Println("✅ 已下载的数据大小已超过配置的设定值, 无需继续下载")
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
			log.Println("❌ 数据库连接未初始化")
			return
		}
		// 3. 查询所有在work_sync_infos 表中的status 为 FAILED 的作品目录数据
		var failedSyncInfos []model.WorkSyncInfo
		tx := db.Table("work_sync_infos").Where("status = ?", "FAILED").Find(&failedSyncInfos)
		if tx.Error != nil {
			log.Println("❌ 查询work_sync_infos失败:", tx.Error)
			return
		}
		if len(failedSyncInfos) == 0 {
			log.Println("✅ 没有需要重试下载的文件")
			return
		}
		log.Printf("❌ 有 %d 个文件需要重试下载", len(failedSyncInfos))
		for _, info := range failedSyncInfos {
			// 4. 重试下载该目录下的所有文件
			log.Printf("🔄 重试下载作品 %s", info.SourceId)
			err := doSyncFailedDownload(db, info)
			if err != nil {
				log.Printf("❌ 重试下载作品 %s 失败: %v", info.SourceId, err)
			}
		}
		log.Println("✅ 重试下载完成，目录：", downloadFolder)
	},
}

func doSyncFailedDownload(db *gorm.DB, info model.WorkSyncInfo) error {
	err := os.RemoveAll(info.FilePath)
	if err != nil {
		return err
	}
	manager, err := engine.NewEngineManager()
	if err != nil {
		log.Fatalf("❌创建下载引擎管理器失败: %v\n", err)
	}
	ctx := context.Background()
	err = manager.DownloadOne(ctx, info.SourceId, filepath.Dir(info.FilePath))
	if err != nil {
		return err
	}
	// 更新数据库状态为 COMPLETED
	info.Status = "COMPLETED"
	info.FailReason = ""
	info.RetryCount++
	info.FailedAt = time.Now()
	tx := db.Table("work_sync_infos").Where("id = ?", info.ID).Updates(info)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
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
			log.Println("❌ 数据库连接未初始化")
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

		log.Printf("✅ 已导出 %s 记录到 %s\n", exportStatus, syncExportFile)
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
		log.Println("❌ 查询work_sync_infos失败:", tx.Error)
		return
	}
	if len(syncInfos) == 0 {
		log.Println("✅ 没有需要导出的作品文件记录")
		return
	}
	log.Printf("✅ 有 %d 个作品文件记录需要导出", len(syncInfos))
	// 3. 导出为 JSON 文件
	f, err := os.Create(file)
	if err != nil {
		log.Println("❌ 创建 JSON 文件失败:", err)
		return
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(syncInfos); err != nil {
		log.Println("❌ 写入 JSON 文件失败:", err)
		return
	}

	log.Printf("✅ 已成功导出 %d 条记录到 %s", len(syncInfos), file)
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
		log.Println("❌ 查询work_sync_infos失败:", tx.Error)
		return
	}
	if len(syncInfos) == 0 {
		log.Println("✅ 没有需要导出的作品文件记录")
		return
	}
	log.Printf("✅ 有 %d 个作品文件记录需要导出", len(syncInfos))
	// 2. 导出为 CSV 文件
	// 2.1 定义 CSV 列头
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
		log.Println("❌ 创建 CSV 文件失败:", err)
		return
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	// 写入列头
	if err := w.Write(headers); err != nil {
		log.Println("❌ 写入 CSV 列头失败:", err)
		return
	}

	// 写入数据行
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			log.Println("❌ 写入 CSV 数据行失败:", err)
			return
		}
	}

	log.Printf("✅ 已成功导出 %d 条记录到 %s", len(rows), file)
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
		log.Println("✅ 相关统计数据如下：")
		//打印统计信息 元数据总量  元数据中带字幕的数量 不带字幕的数量,合并成一个sql查询
		db := database.Database
		if db == nil {
			log.Println("❌ 数据库连接未初始化")
			return
		}
		var total, withSubtitle, withoutSubtitle int64
		tx := db.Table("metadata_works").
			Select("COUNT(*) AS total, COUNT(CASE WHEN has_subtitle THEN 1 END) AS withSubtitle, COUNT(CASE WHEN NOT has_subtitle THEN 1 END) AS withoutSubtitle").
			Row()
		if tx.Err() != nil {
			log.Println("❌ 查询metadata_works失败:", tx.Err())
			return
		}
		tx.Scan(&total, &withSubtitle, &withoutSubtitle)
		log.Printf("✅ 元数据总量: %d, 带字幕数量: %d, 不带字幕数量: %d\n", total, withSubtitle, withoutSubtitle)
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
			log.Println("❌ 查询work_sync_infos失败:", tx.Err())
			return
		}
		tx.Scan(&syncDownloaded, &syncFailed, &syncPending, &syncWithSubtitle, &syncWithoutSubtitle)
		log.Printf("✅ 同步下载数量: %d, 带字幕数量: %d, 不带字幕数量: %d,失败数量: %d, 等待下载数量: %d\n", syncDownloaded, syncWithSubtitle, syncWithoutSubtitle, syncFailed, syncPending)

		//计算同步进度
		// 同步进度 = 已下载数量 / 总数量,包含总同步进度，带字幕进度，不带字幕进度
		syncProgress := float64(syncDownloaded) / float64(total) * 100
		syncWithSubtitleProgress := float64(syncWithSubtitle) / float64(withSubtitle) * 100
		syncWithoutSubtitleProgress := float64(syncWithoutSubtitle) / float64(withoutSubtitle) * 100
		log.Printf("✅ 同步进度: %.2f%%, 带字幕进度: %.2f%%, 不带字幕进度: %.2f%%\n", syncProgress, syncWithSubtitleProgress, syncWithoutSubtitleProgress)

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
