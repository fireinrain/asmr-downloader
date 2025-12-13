package cmd

import (
	"asmroner/internal/engine"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	hotDownloadDir string
	hotCount       int
)

var downloadCmd = &cobra.Command{
	Use:   "download [RJID 或 RJID1,RJID2,RJID3 或 hot100]",
	Short: "下载资源（支持单个、多项RJID 或热门作品）",
	Long: `
download 命令用于下载音声资源，支持单个 RJID、多项 RJID 批量下载，也支持热门作品（hot100）模式。

参数说明：
  [RJID 或 RJID1,RJID2,RJID3 或 hot100]
    - 输入单个 RJID：下载对应作品
    - 输入多个 RJID（逗号分隔）：执行批量下载
    - 输入 hot100：下载热门作品（需结合 -n 设置数量）

选项：
  -d, --dir <目录路径>
      指定文件保存目录（默认当前目录）。
      示例：
        asmroner download RJ01000001 -d ./downloads

  -n, --number <数量>
      热门模式下载数量（仅在 hot100 模式下生效）。
      示例：
        asmroner download hot100 -n 20

适用场景：
  - 指定 RJID 下载单个作品
  - 批量下载多个作品
  - 按热门榜下载指定数量的热门作品
  - 配合 -d 自定义保存路径

说明：
  - 批量下载会自动处理请求调度、限流、重试等机制
  - hot100 模式会按热门榜顺序下载前 N 个作品
`,

	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		keyword := args[0]

		// 解析保存路径

		hotDownloadDir, _ = filepath.Abs(hotDownloadDir)

		// 创建目录
		if err := os.MkdirAll(hotDownloadDir, os.ModePerm); err != nil {
			log.Fatalf("❌创建下载目录失败: %v\n", err)
		}
		engineManager := engine.NewEngineManager()

		// ------------------------------------
		// 模式 1：下载热门 hot
		// ------------------------------------
		if keyword == "hot100" {
			if hotCount <= 0 {
				log.Println("❌请使用 -n 参数设置要下载的热门作品数量，例如：-n 10")
				return
			}
			log.Printf("🔥 正在下载 %d 个热门作品到目录：%s\n", hotCount, hotDownloadDir)

			err := engineManager.DownloadHot100(hotCount, hotDownloadDir)
			if err != nil {
				log.Printf("❌热门作品下载失败: %v\n", err)
				return
			}
			log.Println("✅ 热门作品下载完成！")
			return
		}

		// ------------------------------------
		// 模式 2：下载一个或多个 rjId
		// ------------------------------------
		rjIds := strings.Split(keyword, ",")
		log.Printf("📥 正在下载以下资源：%v\n", rjIds)
		log.Printf("📂 保存路径：%s\n", hotDownloadDir)

		err := engineManager.SimpleDownload(rjIds, hotDownloadDir)
		if err != nil {
			log.Printf("❌资源下载失败: %v\n", err)
			return
		}
		log.Println("✅ 资源下载完成！")
	},
}

func init() {
	rootCmd.AddCommand(downloadCmd)

	downloadCmd.Flags().StringVarP(&hotDownloadDir, "dir", "d", "./", "文件保存目录（默认当前目录）")
	downloadCmd.Flags().IntVarP(&hotCount, "number", "n", 1, "下载热门作品数量（当输入hot100 时生效）")
}
