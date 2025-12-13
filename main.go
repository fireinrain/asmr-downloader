package main

import (
	"asmroner/cmd"
	"asmroner/internal/consts"
	"asmroner/internal/logger"
	"asmroner/internal/utils"
	"fmt"

	"github.com/spf13/cobra"
)

//TIP <p>To run your code, right-click the code and select <b>Run</b>.</p> <p>Alternatively, click
// the <icon src="AllIcons.Actions.Execute"/> icon in the gutter and select the <b>Run</b> menu item from here.</p>
import _ "net/http/pprof"

func main() {
	utils.EnSureDirExist(consts.MetaDataDir)
	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "显示程序版本信息",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("asmroner")
			fmt.Println("版本:      ", version)
			fmt.Println("构建时间:  ", buildTime)
			fmt.Println("开发者:    ", author)
		},
	}
	cmd.RegisterCmd(versionCmd)
	//初始化错误日志记录器
	logger.InitErrorLogger()
	defer logger.Close()
	cmd.Execute()
}
