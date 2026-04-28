package cmd

import (
	"asmroner/internal/consts"
	"asmroner/internal/logger"
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "初始化或重置配置文件（交互式）",
	Long: `
config 命令用于初始化或重置本程序的配置文件，并以“交互式”方式指导用户逐项填写必要的环境与账号信息。

功能说明：
  - 创建或重置配置文件（TOML 格式）
  - 逐项询问账号、接口地址、限流参数等关键配置
  - 自动生成标准化配置结构
  - 支持覆盖已有配置（需确认）

交互式配置内容包括：
  - 用户账号与密码
  - API 接口地址
  - 代理服务地址
  - 最大并发数与最大重试次数
  - 同步数据存放目录
  - 同步容量限制（如 200MB、2GB）
  - 优先媒体格式（如 flac、mp3）
  - 同步/下载的 QPS 限流配置
  - 请求抖动（Jitter）设置，用于分散负载、降低风控风险
  - IDM 安装路径（可选，用于自动生成下载脚本）

配置文件路径：
  ~/.asmroner/config.toml

适用场景：
  - 首次安装本程序时初始化配置
  - 修改账号、限流策略、下载目录等参数
  - 修复损坏或不完整的配置文件
  - 重置已有配置让程序恢复默认行为

注意事项：
  - 本命令为“交互模式”，不适用于 CI/CD 或自动化脚本。
  - 若需要在脚本中生成配置，请自行编写或拷贝 TOML 文件。
`,

	Run: func(cmd *cobra.Command, args []string) {
		configFile := filepath.Join(consts.MetaDataDir, consts.ConfigFileName)
		reader := bufio.NewReader(os.Stdin)

		// 创建目录
		if err := os.MkdirAll(consts.MetaDataDir, 0755); err != nil {
			log.Fatalf("❌ 无法创建配置目录: %v", err)
		}

		// 如果配置存在 → 询问是否重置
		if _, err := os.Stat(configFile); err == nil {
			ans := prompt(reader, "⚠️ 检测到配置文件已存在，是否要重置？[y/N]: ", "n")
			if strings.ToLower(ans) != "y" {
				logger.Info("已取消操作")
				return
			}
			os.Remove(configFile)
		}

		InitConfig(reader, configFile)
	},
}

// ------------------------- 核心初始化函数 -------------------------
func InitConfig(reader *bufio.Reader, configFile string) {
	logger.Step("正在初始化配置...")

	account := prompt(reader, "用户账号（默认：guest）: ", "guest")
	password := prompt(reader, "用户密码（默认：guest）: ", "guest")

	apiURL := prompt(reader, "接口地址（默认：自动获取）: ", "")
	proxyURL := prompt(reader, "代理地址（可选）: ", "")
	maxWorkers := promptInt(reader, "最大并发数（默认：5）: ", 5)
	maxRetries := promptInt(reader, "最大重试次数（默认：3）: ", 3)
	syncDataFolder := prompt(reader, "同步数据存放目录（默认：./syncdata）: ", "./syncdata")

	syncWantedSize := prompt(reader, "同步容量限制（1MB/GB/TB/PB，默认：200MB）: ", "200MB")
	preferMedia := prompt(reader, "优先媒体格式 [all | mp3>wav>flac]（默认：all）: ", "all")

	// ----- 新增：IDM 安装路径 -----
	idmPath := prompt(reader, "IDM 安装路径（留空则生成手动配置提示，例如 E:\\idm\\IDM\\IDMan.exe）: ", "")
	// 简单验证文件是否存在（可选）
	if idmPath != "" {
		if _, err := os.Stat(idmPath); os.IsNotExist(err) {
			fmt.Printf("⚠️ 警告：指定的文件 '%s' 不存在，但仍将保存此路径。\n", idmPath)
		}
	}
	// ----------------------------

	syncQPS := promptFloat(reader, "同步请求 QPS（默认：2）: ", 2)
	syncJitterMin := promptInt(reader, "同步请求抖动最小值（毫秒，默认：100）: ", 100)
	syncJitterMax := promptInt(reader, "同步请求抖动最大值（毫秒，默认：500）: ", 500)

	downloadQPS := promptFloat(reader, "下载请求 QPS（默认：0.2）: ", 0.2)
	downloadJitterMin := promptInt(reader, "下载抖动最小值（毫秒，默认：2000）: ", 2000)
	downloadJitterMax := promptInt(reader, "下载抖动最大值（毫秒，默认：5000）: ", 5000)

	// ------------------------- 写入配置 -------------------------
	viper.Set("user.account", account)
	viper.Set("user.password", password)

	viper.Set("downloader.site_url", apiURL)
	viper.Set("downloader.proxy_url", proxyURL)
	viper.Set("downloader.max_workers", maxWorkers)
	viper.Set("downloader.max_retries", maxRetries)
	viper.Set("downloader.sync_data_folder", syncDataFolder)
	viper.Set("downloader.sync_wanted_size", syncWantedSize)
	viper.Set("downloader.prefer_media", preferMedia)
	viper.Set("downloader.idm_path", idmPath)          // 新增

	viper.Set("limit.sync_qps", syncQPS)
	viper.Set("limit.sync_jitter_min", syncJitterMin)
	viper.Set("limit.sync_jitter_max", syncJitterMax)
	viper.Set("limit.download_qps", downloadQPS)
	viper.Set("limit.download_jitter_min", downloadJitterMin)
	viper.Set("limit.download_jitter_max", downloadJitterMax)

	writeConfig(configFile)

	logger.Done("配置文件已成功保存！")
}

// ------------------------- 输入封装函数 -------------------------

// 普通输入
func prompt(reader *bufio.Reader, text, def string) string {
	fmt.Print(text)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return def
	}
	return input
}

// 要求整数
func promptInt(reader *bufio.Reader, text string, def int) int {
	for {
		s := prompt(reader, text, fmt.Sprint(def))
		n, err := strconv.Atoi(s)
		if err == nil {
			return n
		}
		fmt.Println("❌ 输入的不是合法整数，请重新输入。")
	}
}

// 要求浮点数
func promptFloat(reader *bufio.Reader, text string, def float64) float64 {
	for {
		s := prompt(reader, text, fmt.Sprint(def))
		n, err := strconv.ParseFloat(s, 64)
		if err == nil {
			return n
		}
		fmt.Println("❌ 输入的不是合法浮点数，请重新输入。")
	}
}

// ------------------------- 写入配置文件 -------------------------
func writeConfig(configFile string) {
	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath(consts.MetaDataDir)

	// SafeWriteConfig 不存在才写
	if err := viper.SafeWriteConfig(); err != nil {
		if os.IsExist(err) || strings.Contains(err.Error(), "already exists") {
			// 已存在 → 删除并重写
			os.Remove(configFile)
			if err := viper.WriteConfig(); err != nil {
				log.Fatalf("❌ 写入配置文件失败: %v", err)
			}
		} else {
			log.Fatalf("❌ 保存配置文件失败: %v", err)
		}
	}
}

func init() {
	rootCmd.AddCommand(configCmd)
}