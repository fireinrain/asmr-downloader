package main

import (
	"asmroner/cmd"
	"asmroner/internal/consts"
	"asmroner/internal/logger"
	"asmroner/internal/utils"
	"fmt"

	"github.com/spf13/cobra"
)

func main() {
	utils.EnsureDirExist(consts.MetaDataDir)

	logger.Init()
	defer logger.Close()

	cmd.RegisterCmd(&cobra.Command{
		Use:   "version",
		Short: "显示程序版本信息",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("asmroner")
			fmt.Println("版本:      ", version)
			fmt.Println("构建时间:  ", buildTime)
			fmt.Println("开发者:    ", author)
		},
	})

	cmd.Execute()
}
