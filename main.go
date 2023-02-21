package main

import (
	"asmr-downloader/config"
	"asmr-downloader/model"
	"asmr-downloader/spider"
	"asmr-downloader/storage"
	"asmr-downloader/utils"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

var pageDataChannel = make(chan model.PageResult, 4)
var subTitlePageDataChannel = make(chan model.PageResult, 4)
var collectPageDataChannel = make(chan model.PageResult, 8)

func main() {
	//获取程序传入的参数
	//简易下载模式
	if len(os.Args) >= 2 && os.Args[1] != "" {
		builder := strings.Builder{}
		container := []string{}

		for k, v := range os.Args {
			if k == 0 {
				continue
			}
			cleanValue := strings.TrimSpace(v)
			if !strings.HasPrefix(cleanValue, "RJ") {
				fmt.Errorf("")
				log.Fatal("参数格式有误,请重新输入参数并运行")
			}
			container = append(container, cleanValue)
			builder.WriteString(cleanValue + " ")
		}
		println("正在查询：", builder.String())
		SimpleModeDownload(container)
		return
	}

	println("------ASMR.ONE Downloader------")
	println("---------Power by Euler--------")
	println("---------version20230207--------")
	var globalConfig *config.Config
	//判断是否初次运行
	globalConfig = CheckIfFirstStart(config.ConfigFileName)
	_ = storage.GetDbInstance()
	fmt.Printf("GlobalConfig=%s\n", globalConfig.SafePrintInfoStr())
	asmrClient := spider.NewASMRClient(globalConfig.MaxWorker, globalConfig)
	err := asmrClient.Login()
	if err != nil {
		fmt.Println("登录失败:", err)
		return
	}
	fmt.Println("账号登录成功!")
	var authStr = asmrClient.Authorization
	//检查数据更新
	ifNeedUpdateMetadata, err := CheckIfNeedUpdateMetadata(authStr)
	if err != nil {
		fmt.Println("元数据检查更新失败: ", err)
	}
	// Get the current time
	now := time.Now()

	// Format the time using the standard format string
	currentTimeStr := now.Format("2006-01-02 15:04:05")

	if ifNeedUpdateMetadata {
		fmt.Printf("当前时间: %s,网站有新作品更新,正在进行更新...\n", currentTimeStr)
		FetchAllMetaData(authStr, asmrClient)
	} else {
		fmt.Printf("当前时间: %s,网站暂时无新作品...\n", currentTimeStr)
	}
	//获取首页
	//先获取有字幕数据
	//FetchMetaDataWithSub(authStr, asmrClient, globalConfig)
	//FetchAllMetaData(authStr, asmrClient)

	//检查是否需要进行下载作品MPS
	needUpdateDownload := CheckIfNeedUpdateDownload()
	if needUpdateDownload {
		input := utils.PromotForInput("ASMR作品本地与网站不同步.是否需要同步下载(Y/N,默认为Y)?:", "Y")
		if input == "Y" {
			//TODO do download task
			//检测破碎文件并下载
			fixBrokenDownloadFile := utils.CheckIfNeedFixBrokenDownloadFile()
			if fixBrokenDownloadFile {
				fmt.Println("发现上一次运行存在下载失败的媒体文件，正在进行修复下载...")
				utils.FixBrokenDownloadFile(asmrClient.GlobalConfig.MaxFailedRetry)
				fmt.Println("修复下载完成...")
			}
			fmt.Println("正在下载ASMR作品文件,请稍后...")
			DownloadItemHandler(asmrClient)
			fmt.Println("当前下载任务已完成...")
		} else {
			fmt.Println("你已取消下载,程序即将退出.")
		}

	} else {
		fmt.Println("ASMR作品本地与网站完全同步.当前无需下载")
	}
	//close db con
	_ = storage.StoreDb.Db.Close()
}

func SimpleModeDownload(idList []string) {
	c := &config.Config{
		Account:          "guset",
		Password:         "guest",
		MaxWorker:        6,
		BatchTaskCount:   1,
		BatchSleepTime:   1,
		AutoForNextBatch: false,
		DownloadDir:      "./",
		MetaDataDb:       "",
	}
	asmrClient := spider.NewASMRClient(6, c)
	err := asmrClient.Login()
	if err != nil {
		fmt.Println("登录失败:", err)
		return
	}
	fmt.Println("访客账号登录成功!")
	pool := asmrClient.WorkerPool
	for i := range idList {
		value := idList[i]
		pool.Do(func() error {
			asmrClient.SimpleDownloadItem(value)
			return nil
		})
	}
	_ = pool.Wait()
	fmt.Println("所有任务下载完成,程序即将退出 ")

}

// DownloadItemHandler
//
//	@Description: ASMR作品下载
//	@param asmrClient
func DownloadItemHandler(asmrClient *spider.ASMRClient) {
	batchCounter := 0
	//批量下载大小 默认为1, -1表示一次性全部下载
	var batchTaskCount = asmrClient.GlobalConfig.BatchTaskCount
	//批量下载完后休息多少秒(防止服务器ban你)
	var batchSleepTime = asmrClient.GlobalConfig.BatchSleepTime
	//是否自动执行 下一个批次
	var autoForNextBatch = asmrClient.GlobalConfig.AutoForNextBatch
	// 失败作品重试次数
	var maxRetry = asmrClient.GlobalConfig.MaxFailedRetry
	for {
		if batchCounter == batchTaskCount {
			fmt.Println("--------------------为下一批次下载休眠--------------------")
			time.Sleep(time.Duration(batchSleepTime) * time.Second)
			if !autoForNextBatch {
				//处理下载失败的链接
				utils.FixBrokenDownloadFile(maxRetry)
				_ = utils.PromotForInput("手动确认下一批次任务,按回车键继续进行任务: ", "")
			}
			//重置batchCounter
			batchCounter = 0
		}

		var id string
		var subtitleFlag int

		err := storage.StoreDb.Db.QueryRow("select item_prod_id,subtitle_flag from asmr_download where download_flag =0").Scan(&id, &subtitleFlag)
		if err != nil {
			if err == sql.ErrNoRows {
				//没有数据了
				break
			}
			log.Fatal("查询数据库失败: ", err)
		}

		asmrClient.DownloadItem(id, subtitleFlag)
		//更新ASMR数据下载状态
		UpdateItemDownStatus(id, subtitleFlag)
		batchCounter += 1
	}
}

// UpdateItemDownStatus
//
//	@Description: 下载完音频数据更新下载状态
//	@param itemProdId
//	@param subtitleFlag
func UpdateItemDownStatus(itemProdId string, subtitleFlag int) {
	tx, err := storage.StoreDb.Db.Begin()
	if err != nil {
		log.Fatal("开启事务失败: ", err)
	}
	_, err = tx.Exec("update asmr_download set download_flag = 1 where item_prod_id = ? and subtitle_flag = ?", itemProdId, subtitleFlag)
	if err != nil {
		tx.Rollback()
		fmt.Println("数据下载完成状态更新失败: ", err)
		fmt.Println("正在进行数据回滚...")
	}
	err = tx.Commit()
	if err != nil {
		fmt.Println("数据提交失败：", err)
	}
	var message = ""
	if subtitleFlag == 0 {
		message = "无字幕"
	}
	if subtitleFlag == 1 {
		message = "含字幕"
	}
	fmt.Printf("%s数据: RJ%s 下载完成...\n", message, itemProdId)

}

// CheckIfNeedUpdateDownload
//
//	@Description: 检查是否需要下载ASMR
//
// 当数据库中asmr_download的所有数据 download_flag 为1
// 则不需要下载否则需要下载
func CheckIfNeedUpdateDownload() bool {
	var metaDataStatics model.MetaDataStatics
	err := storage.StoreDb.Db.QueryRow("select a.total,\n       b.sub_total,\n       (a.total - b.sub_total)        "+
		"             as no_sub_total,\n       c.down_sub_total,\n       d.down_no_sub_total,\n       (b.sub_total - c.down_sub_total)      "+
		"      as undown_sub_total,\n       (a.total - b.sub_total - down_no_sub_total) as undown_no_sub_total,\n      "+
		" (c.down_sub_total+d.down_no_sub_total) as have_down_total,\n      "+
		" (a.total - (c.down_sub_total+d.down_no_sub_total)) as undown_total\nfrom (select count(*) as total from asmr_download) as a,\n  "+
		"   (select count(*) as sub_total from asmr_download where subtitle_flag = 1) as b,\n     (select count(*) as down_sub_total from asmr_download where subtitle_flag = 1 and download_flag = 1) as c,\n    "+
		" (select count(*) as down_no_sub_total from asmr_download where subtitle_flag = 0 and download_flag = 1) as d").Scan(
		&metaDataStatics.TotalCount,
		&metaDataStatics.SubTitleCount,
		&metaDataStatics.NoSubTitleCount,
		&metaDataStatics.SubTitleDownloaded,
		&metaDataStatics.NoSubTitleDownloaded,
		&metaDataStatics.SubTitleUnDownloaded,
		&metaDataStatics.NoSubTitleUnDownloaded,
		&metaDataStatics.HavenDownTotal,
		&metaDataStatics.UnDownTotal)
	if err != nil {
		if err == sql.ErrNoRows {
			//没有数据 ignore here
			return true
		}
		log.Fatal("查询统计信息出错: ", err)
	}
	staticsInfo := metaDataStatics.GetStaticsInfo()
	infoStr := staticsInfo.PrettyInfoStr()
	fmt.Println(infoStr)
	if metaDataStatics.TotalCount > (metaDataStatics.SubTitleDownloaded + metaDataStatics.NoSubTitleDownloaded) {
		return true
	}
	return false
}

// CheckIfNeedUpdateMetadata
//
//	@Description: 判断是否需要从网站跟下元数据
//	@param authStr
//	@return bool
//	@return error
func CheckIfNeedUpdateMetadata(authStr string) (bool, error) {
	indexPageInfo, err := spider.GetAllIndexPageInfo(authStr)
	if err != nil {
		log.Printf("ASMR one 首页数据获取失败: %s\n", err.Error())
	}
	//查询数据
	var total int
	err = storage.StoreDb.Db.QueryRow("select count(*) as total from asmr_download").Scan(&total)
	if err != nil {
		if err == sql.ErrNoRows {
			//没有数据
			total = 0
		} else {
			log.Fatal("查询总数据条数出错: ", err)

		}
	}
	if indexPageInfo.Pagination.TotalCount > total {
		return true, nil
	}
	return false, nil
}

// FetchAllMetaData
//
//	@Description: 提取所有元数据
//	@param authStr
//	@param asmrClient
func FetchAllMetaData(authStr string, asmrClient *spider.ASMRClient) {
	pageSg := &sync.WaitGroup{}
	pageSg.Add(2)
	go MetaDataAllTaskHandler(authStr, asmrClient, pageSg)
	time.Sleep(5 * time.Duration(time.Second))
	go ProcessAllCollectPageData(pageSg)
	pageSg.Wait()
}

// FetchMetaDataWithSub
//
//	@Description: 按照查询是否带字幕标签运行获取数据程序
//	@param authStr
//	@param asmrClient
//	@param globalConfig
func FetchMetaDataWithSub(authStr string, asmrClient *spider.ASMRClient, globalConfig *config.Config) {
	pageSg := &sync.WaitGroup{}
	pageSg.Add(2)
	go MetaDataTaskHandler(authStr, 1, asmrClient, pageSg)
	//无字幕数据
	asmrClient2 := spider.NewASMRClient(globalConfig.MaxWorker, globalConfig)
	go MetaDataTaskHandler(authStr, 0, asmrClient2, pageSg)
	pageSg.Wait()
	time.Sleep(5 * time.Duration(time.Second))
	ProcessCollectPageData()
}

// MetaDataAllTaskHandler
//
//	@Description: 下载所有元数据
//	@param authStr
//	@param asmrClient
//	@param wg
func MetaDataAllTaskHandler(authStr string, asmrClient *spider.ASMRClient, wg *sync.WaitGroup) {
	defer wg.Done()
	indexPageInfo, err := spider.GetAllIndexPageInfo(authStr)
	if err != nil {
		log.Printf("ASMR one 首页数据获取失败: %s\n", err.Error())
	}
	fmt.Printf("正在获取作品元数据...\n")
	//计算最大页数
	var totalCount = indexPageInfo.Pagination.TotalCount
	var pageSize = indexPageInfo.Pagination.PageSize
	maxPage := utils.CalculateMaxPage(totalCount, pageSize)
	//maxPage = 2
	pool := asmrClient.WorkerPool
	//接受数据
	//并发10
	//limiter := make(chan bool, 20)
	fetchWg := &sync.WaitGroup{}
	go func() {
		fetchWg.Add(1)
		defer fetchWg.Done()
		for i := 1; i <= maxPage; i++ { //开启20个请求
			pageIndex := i
			pool.Do(func() error {
				return PageAllDataTaskHandler(collectPageDataChannel, authStr, pageIndex)
			})
		}
		_ = pool.Wait()
		close(collectPageDataChannel)
	}()
	fetchWg.Wait()

}

// PageAllDataTaskHandler
//
//	@Description: 获取所有页面元数据
//	@param collectPageDataChannel
//	@param authStr
//	@param pageIndex
//	@return error
func PageAllDataTaskHandler(collectPageDataChannel chan model.PageResult, authStr string, pageIndex int) error {
	infoData, err2 := spider.GetPerPageInfo(authStr, pageIndex, -1)
	if err2 != nil {
		fmt.Printf("当前页: %d,访问失败\n", pageIndex)
		//TODO 记录失败的index
	}
	fmt.Printf("获取到数据页: %d\n", pageIndex)
	//发送给channel
	collectPageDataChannel <- *infoData
	//fmt.Printf("数据: %v\n", infoData)
	return nil
}

// MetaDataTaskHandler
//
//	@Description: 按照有无字幕获取接口数据
//	@param authStr
//	@param subTitleFlag
//	@param asmrClient
//	@param wg
func MetaDataTaskHandler(authStr string, subTitleFlag int, asmrClient *spider.ASMRClient, wg *sync.WaitGroup) {
	defer wg.Done()
	indexPageInfo, err := spider.GetIndexPageInfo(authStr, subTitleFlag)
	var targetChannel chan model.PageResult
	var message = ""
	if subTitleFlag == 0 {
		message = "无字幕"
		targetChannel = pageDataChannel
	}
	if subTitleFlag == 1 {
		message = "有字幕"
		targetChannel = subTitlePageDataChannel
	}
	if err != nil {
		log.Printf("ASMR one 首页(%s)获取失败: %s\n", message, err.Error())
	}
	fmt.Printf("正在获取%s作品元数据...\n", message)
	//计算最大页数
	var totalCount = indexPageInfo.Pagination.TotalCount
	var pageSize = indexPageInfo.Pagination.PageSize
	maxPage := utils.CalculateMaxPage(totalCount, pageSize)
	//maxPage = 2
	pool := asmrClient.WorkerPool
	//接受数据
	//并发10
	//limiter := make(chan bool, 20)
	fetchWg := &sync.WaitGroup{}
	go func() {
		fetchWg.Add(1)
		defer fetchWg.Done()
		for i := 1; i <= maxPage; i++ { //开启20个请求
			pageIndex := i
			pool.Do(func() error {
				return PageDataTaskHandler(targetChannel, authStr, pageIndex, subTitleFlag)
			})
		}
		_ = pool.Wait()
		close(targetChannel)
	}()
	fetchWg.Wait()

}

// PageDataTaskHandler
//
//	@Description: 页面元数据下载
//	@param dataChannel
//	@param authStr
//	@param pageIndex
//	@param subTitleFlag
//	@return error
func PageDataTaskHandler(dataChannel chan model.PageResult, authStr string, pageIndex int, subTitleFlag int) error {
	infoData, err2 := spider.GetPerPageInfo(authStr, pageIndex, subTitleFlag)
	if err2 != nil {
		fmt.Printf("当前页: %d,访问失败\n", pageIndex)
		//TODO 记录失败的index
	}
	var message = ""
	if subTitleFlag == 0 {
		message = "无字幕"
	}
	if subTitleFlag == 1 {
		message = "有字幕"
	}
	fmt.Printf("获取到%s数据页: %d\n", message, pageIndex)
	//发送给channel
	dataChannel <- *infoData
	//fmt.Printf("数据: %v\n", infoData)
	return nil
}

// ProcessAllCollectPageData
//
//	@Description: 一个channel处理所有数据
//	@param wg
func ProcessAllCollectPageData(wg *sync.WaitGroup) {
	defer wg.Done()
	fmt.Println("元数据处理中...")

	index := 0
	for rc := range collectPageDataChannel {
		index += 1
		//fmt.Printf("data: %v\n", rc)
		StoreTodb(rc)
	}
	fmt.Printf("采集元数据结束,共采集%d页数据\n", index)

}

// ProcessCollectPageData
//
//	@Description: 分两个channel处理有/无字幕数据
func ProcessCollectPageData() {
	fmt.Println("元数据处理中...")

	indexSubtitle := 0
	for rc := range subTitlePageDataChannel {
		indexSubtitle += 1
		//fmt.Printf("data: %v\n", rc)
		StoreTodb(rc)
	}
	//fmt.Printf("采集结束,共采集%d页数据\n", indexSubtitle)
	index := 0
	for rc := range pageDataChannel {
		index += 1
		//fmt.Printf("data: %v\n", rc)
		StoreTodb(rc)
	}
	total := indexSubtitle + index
	fmt.Printf("采集元数据结束,共采集%d页数据\n", total)

	//loop:
	//	for {
	//		select {
	//		case value := <-pageDataChannel:
	//			counter += 1
	//			fmt.Printf("data: %v\n", value)
	//		case value := <-subTitlePageDataChannel:
	//			counter += 1
	//			fmt.Printf("data: %v\n", value)
	//		default:
	//			break loop
	//		}
	//	}

}

// StoreTodb
//
//	@Description: 将下载的元数据存储到sqlite3
//	@param data
func StoreTodb(data model.PageResult) {
	//查找数据库中是否存在 不存在插入 存在跳过
	for _, row := range data.Works {
		id := row.ID
		subtitle := row.HasSubtitle
		err := storage.StoreDb.Db.QueryRow(
			"select item_prod_id,subtitle_flag from asmr_download where item_prod_id = ? and subtitle_flag = ?", id, subtitle).Scan(&id, &subtitle)
		if err == sql.ErrNoRows {
			//插入数据
			tx, err := storage.StoreDb.Db.Begin()
			if err != nil {
				log.Fatal("开启事务失败: ", err)
			}
			rjid := fmt.Sprintf("RJ%d", row.ID)
			title := strings.TrimSpace(row.Title)
			subtitleFlag := row.HasSubtitle

			_, err = tx.Exec("insert into asmr_download(rjid,item_prod_id,title,subtitle_flag) values(?,?,?,?)", rjid, row.ID, title, subtitleFlag)
			if err != nil {
				tx.Rollback()
				fmt.Println("数据插入失败: ", err)
				fmt.Println("正在进行数据回滚...")
			}
			err = tx.Commit()
			if err != nil {
				fmt.Println("数据提交失败：", err)
			}
			fmt.Println("新增数据: ", rjid)

		} else if err != nil {
			log.Fatal("查询数据库出现错误: ", err)
			return
		} else {
			//fmt.Printf("数据库已存在: id = %d--subtitle_flag = %t 该条数据,跳过处理\n\n", id, subtitle)
			//ignore here
		}

	}

}

// CheckIfFirstStart
//
//	@Description: 检测是否是第一次运行
//	@param configFile
//	@return *config.Config
func CheckIfFirstStart(configFile string) *config.Config {
	if utils.FileOrDirExists(configFile) {
		fmt.Println("程序已初始化完成,正在启动运行...")
	} else {
		fmt.Println("检测到初次运行,请进行相关设置...")
	}
	return config.GetConfig()
}
