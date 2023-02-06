package main

import (
	"asmr-downloader/config"
	"asmr-downloader/spider"
	"asmr-downloader/utils"
	"fmt"
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

}

func checkIfFirstStart(configFile string) *config.Config {
	if utils.FileOrDirExists(configFile) {
		fmt.Println("程序已初始化完成,正在启动运行...")
	} else {
		fmt.Println("检测到初次运行,请进行相关设置...")
	}
	return config.GetConfig()

}
