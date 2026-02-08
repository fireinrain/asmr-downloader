package cmd

import (
	"asmroner/internal/consts"
	"asmroner/internal/database"
	"asmroner/internal/logger"
	"asmroner/internal/model"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd 代表基础命令，即使没有子命令也会运行
var rootCmd = &cobra.Command{
	Use:   "asmroner",
	Short: "强大的命令行asmr.one下载管理工具",
	Long: `
   ___  ____  __  __ ____  _   _ ___ _   _ _____ 
  / _ \|  _ \|  \/  |  _ \| \ | |_ _| \ | | ____|
 | | | | | | | |\/| | | | |  \| || ||  \| |  _|  
 | |_| | |_| | |  | | |_| | |\  || || |\  | |___ 
  \___/|____/|_|  |_|____/|_| \_|___|_| \_|_____|
                            Powered by fireinrain

asmroner 是一个基于Go的多功能命令行工，提供以下功能：
1. 搜索与下载资源（支持单个、多项 RJID 或热门作品）
2. 同步站点元数据，管理本地下载文件
3. 导出资源信息到 CSV / JSON 文件
4. 支持配置管理，交互式初始化或重置配置
5. 支持限流、抖动、代理等高级下载控制

使用示例：
  asmroner config
      初始化或重置配置文件（交互式）

  asmroner search RJ01037721
      按 RJID 或关键字搜索资源

  asmroner download RJ01037721
      下载单个资源到默认目录

  asmroner sync
      同步站点元数据

  asmroner sync download -d ./downloads
      同步元数据并下载文件到指定目录

  asmroner sync retry-failed -d ./downloads
      重试指定目录下下载失败的文件

  asmroner sync export -s failed -f failed.csv
      导出失败文件列表到 CSV

注意事项：
  - 除 config/help 命令外，所有命令执行前必须确保配置文件已初始化
  - 配置文件默认位置：~/.asmroner/config.toml
  - 目录不存在时，程序会自动创建
`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if cmd.Name() == "config" || cmd.Name() == "help" {
			return
		}

		viper.AddConfigPath(consts.MetaDataDir)
		viper.SetConfigName("config")
		viper.SetConfigType("toml")

		if err := viper.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); ok {
				logger.Fail("配置文件未找到 (config.toml)")
				logger.Info("请先运行 'asmroner config' 初始化配置文件")
				os.Exit(1)
			} else {
				logger.Fail("读取配置文件失败: %v", err)
				os.Exit(1)
			}
		}

		config := model.NewDefaultConfig()
		if err := viper.Unmarshal(config); err != nil {
			logger.Fail("配置解析失败: %v", err)
			os.Exit(1)
		}

		_, err := database.InitDB()
		if err != nil {
			logger.Fail("数据库初始化失败: %v", err)
			os.Exit(1)
		}

		if _, err := os.Stat(config.Downloader.SyncDataFolder); os.IsNotExist(err) {
			if err := os.MkdirAll(config.Downloader.SyncDataFolder, 0755); err != nil {
				logger.Fail("创建同步数据目录失败: %v", err)
				os.Exit(1)
			}
		}
	},
}

// Execute 将所有子命令添加到 root 命令并设置标志。
// 这是 main.main() 调用的唯一入口。
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logger.Error("%v", err)
		os.Exit(1)
	}
}

func RegisterCmd(cmd *cobra.Command) {
	rootCmd.AddCommand(cmd)
}
