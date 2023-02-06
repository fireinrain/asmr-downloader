package main

import (
	"asmr-downloader/config"
	"asmr-downloader/spider"
	"asmr-downloader/utils"
	"fmt"
	"log"
	"time"
)

//func init() {
//	fmt.Println("------ASMR.ONE Downloader------")
//	fmt.Println("---------Power by Euler--------")
//}

func main() {
	println("------ASMR.ONE Downloader------")
	println("---------Power by Euler--------")
	println("---------version20230207--------")
	var globalConfig *config.Config
	//判断是否初次运行
	globalConfig = checkIfFirstStart(config.ConfigFileName)
	fmt.Printf("GlobalConfig=%s\n", globalConfig.SafePrintInfoStr())
	asmrClient := spider.NewASMRClient(globalConfig.MaxWorker, globalConfig)
	err := asmrClient.Login()
	if err != nil {
		fmt.Println("登录失败:", err)
		return
	}
	fmt.Println("账号登录成功!")
	//获取首页
	indexPageInfo, err := spider.GetIndexPageInfo(asmrClient.Authorization)
	if err != nil {
		log.Println("ASMR one 首页获取失败: ", err.Error())
	}
	//计算最大页数
	var totalCount = indexPageInfo.Pagination.TotalCount
	var pageSize = indexPageInfo.Pagination.PageSize
	maxPage := utils.CalculateMaxPage(totalCount, pageSize)
	pool := asmrClient.WorkerPool
	for i := 2; i < maxPage; i++ { //开启20个请求
		ii := i
		pool.Do(func() error {
			for j := 0; j < 10; j++ { //每次打印0-10的值
				fmt.Println(fmt.Sprintf("%v->\t%v", ii, j))
				time.Sleep(1 * time.Second)
			}
			//time.Sleep(1 * time.Second)
			return nil
		})
	}
	pool.Wait()

	pool.Wait()

}

func checkIfFirstStart(configFile string) *config.Config {
	if utils.FileOrDirExists(configFile) {
		fmt.Println("程序已初始化完成,正在启动运行...")
	} else {
		fmt.Println("检测到初次运行,请进行相关设置...")
	}
	return config.GetConfig()

}
