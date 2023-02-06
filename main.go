package main

import (
	"asmr-downloader/config"
	"asmr-downloader/model"
	"asmr-downloader/spider"
	"asmr-downloader/storage"
	"asmr-downloader/utils"
	"fmt"
	"log"
	"sync"
	"time"
)

//	func init() {
//		fmt.Println("------ASMR.ONE Downloader------")
//		fmt.Println("---------Power by Euler--------")
//	}
var pageDataChannel = make(chan model.PageResult, 5)

func main() {
	println("------ASMR.ONE Downloader------")
	println("---------Power by Euler--------")
	println("---------version20230207--------")
	var globalConfig *config.Config
	//判断是否初次运行
	globalConfig = CheckIfFirstStart(config.ConfigFileName)
	var storageDb = storage.GetDbInstance()
	println(storageDb)
	fmt.Printf("GlobalConfig=%s\n", globalConfig.SafePrintInfoStr())
	asmrClient := spider.NewASMRClient(globalConfig.MaxWorker, globalConfig)
	err := asmrClient.Login()
	if err != nil {
		fmt.Println("登录失败:", err)
		return
	}
	fmt.Println("账号登录成功!")
	var authStr = asmrClient.Authorization
	//获取首页
	indexPageInfo, err := spider.GetIndexPageInfo(authStr)
	if err != nil {
		log.Println("ASMR one 首页获取失败: ", err.Error())
	}
	//计算最大页数
	var totalCount = indexPageInfo.Pagination.TotalCount
	var pageSize = indexPageInfo.Pagination.PageSize
	maxPage := utils.CalculateMaxPage(totalCount, pageSize)
	var subTitleFlag = 0
	maxPage = 10
	pool := asmrClient.WorkerPool
	//接受数据
	//并发10
	//limiter := make(chan bool, 20)
	fetchWg := &sync.WaitGroup{}
	go func() {
		fetchWg.Add(1)
		defer fetchWg.Done()
		defer close(pageDataChannel)
		for i := 1; i <= maxPage; i++ { //开启20个请求
			pageIndex := i
			pool.Do(func() error {
				return PageDataTaskHandler(authStr, pageIndex, subTitleFlag)
			})
		}
		_ = pool.Wait()
	}()
	fetchWg.Wait()

	processWg := &sync.WaitGroup{}
	go ProcessCollectPageData(processWg)
	processWg.Wait()

	time.Sleep(10 * time.Second)

}

func PageDataTaskHandler(authStr string, pageIndex int, subTitleFlag int) error {
	infoData, err2 := spider.GetPerPageInfo(authStr, pageIndex, subTitleFlag)
	if err2 != nil {
		fmt.Printf("当前页: %d,访问失败", pageIndex)
		//TODO 记录失败的index
	}
	fmt.Printf("获取到数据页: %d", pageIndex)
	//发送给channel
	pageDataChannel <- *infoData
	//fmt.Printf("数据: %v\n", infoData)
	return nil
}

func ProcessCollectPageData(wg *sync.WaitGroup) {
	wg.Add(1)
	defer wg.Done()
	fmt.Println("process data...")
	for rc := range pageDataChannel {
		fmt.Printf("data: %v", rc)
	}
	fmt.Println("采集结束")
	//index := 0
	//for rc := range pageDataChannel {
	//	index += 1
	//	fmt.Printf("序号: %d\n\n", index)
	//	fmt.Printf("data: %v\n", rc)
	//}
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
