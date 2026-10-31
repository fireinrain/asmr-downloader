package cmd

import (
	"asmroner/internal/engine"
	"asmroner/internal/logger"
	"asmroner/internal/model"
	"os"

	"github.com/spf13/cobra"
)

// ping 命令 —— 对 asmr.one 各镜像 API 地址测速
//
// 示例：
//
//	ping
var pingCmd = &cobra.Command{
	Use:   "ping",
	Short: "对 asmr.one 各镜像 API 地址测速，找出响应最快的",
	Long: `
ping 命令用于对 asmr.one 的各镜像 API 地址进行并发测速，帮助选择最快的接口地址。

执行流程：
  1. 访问官方发布页 as.mr 获取最新镜像域名列表（失败时仅测内置候选）
  2. 并发探测所有候选地址（含 api.asmr.one 与默认地址），超时 5 秒
  3. 逐站点输出耗时与状态码，给出最快站点

测速走配置文件 downloader.proxy_url 指定的代理（如有），
与实际下载请求的网络路径一致，结果可直接用于选站。

注意事项：
  - 收到任意 HTTP 响应（含 401/404）即视为可达，耗时只反映网络延迟
  - 若配置了 downloader.api_url，测速结果仅供参考，
    程序实际仍使用配置指定的地址；置空 api_url 才会启用自动选站
`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Step("开始 API 测速...")

		fastest, err := engine.ProbeFastestApiUrl()
		if err != nil {
			logger.Fail("测速失败: %v", err)
			os.Exit(1)
		}

		if u := model.AppConfig.Downloader.ApiUrl; u != "" {
			logger.Warn("当前配置 downloader.api_url = %s，实际请求不会使用测速结果", u)
			logger.Info("如需自动选择最快站点，请将 api_url 置空")
		} else {
			logger.Info("未配置 api_url，本次运行将自动使用: %s", fastest)
		}

		logger.Done("测速完成！")
	},
}

func init() {
	rootCmd.AddCommand(pingCmd)
}
