package main

import (
	"asmroner/cmd"
	"asmroner/internal/consts"
	"asmroner/internal/logger"
	"asmroner/internal/utils"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// hintLegacyDataDir 兼容提示：旧版本把配置/元数据放在“当前目录”，新版统一读取 $HOME
func hintLegacyDataDir() {
	legacy := filepath.Join(consts.MetaDataDirName, consts.ConfigFileName)
	if _, err := os.Stat(legacy); err == nil {
		if _, err := os.Stat(filepath.Join(consts.MetaDataDir(), consts.ConfigFileName)); err != nil {
			fmt.Printf("⚠️  检测到当前目录存在旧版配置 %s，新版程序从 %s 读取配置。\n    可将该目录下的文件复制到新位置后删除旧目录。\n", legacy, consts.MetaDataDir())
		}
	}
}

func main() {
	hintLegacyDataDir()

	utils.EnsureDirExist(consts.MetaDataDir())

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
