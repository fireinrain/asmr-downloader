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
	"strings"
	"sync"
	"time"
)

//	func init() {
//		fmt.Println("------ASMR.ONE Downloader------")
//		fmt.Println("---------Power by Euler--------")
//	}
var pageDataChannel = make(chan model.PageResult, 4)
var subTitlePageDataChannel = make(chan model.PageResult, 4)
var collectPageDataChannel = make(chan model.PageResult, 8)

func main() {
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

	//获取首页
	//先获取有字幕数据
	//FetchMetaDataWithSub(authStr, asmrClient, globalConfig)
	FetchAllMetaData(authStr, asmrClient)

	time.Sleep(10 * time.Second)

}

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

		} else if err != nil {
			log.Fatal("查询数据库出现错误: ", err)
			return
		} else {
			fmt.Printf("数据库已存在: id = %d--subtitle_flag = %t 该条数据,跳过处理\n\n", id, subtitle)
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
