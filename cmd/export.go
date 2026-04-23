package cmd

import (
	"asmroner/internal/engine"
	"asmroner/internal/logger"
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	exportOutput string
	exportNumber int
)

var exportCmd = &cobra.Command{
	Use:   "export [workID 或 hot100]",
	Short: "导出作品的下载链接列表",
	Long: `导出单个作品或热门榜前N个作品的下载链接。
每个作品将按原始目录结构生成 links.txt 和 idm_download.bat。

示例：
  # 导出单个作品
  asmroner export RJ01544940 -o ./downloads/

  # 导出热门榜前20个作品
  asmroner export hot100 -n 20 -o ./downloads/

  # 导出热门榜前10个作品到指定目录
  asmroner export hot100 -n 10 -o F:\ASMR\`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		eng, err := engine.NewEngineManager(10, 5, 500, 2000)
		if err != nil {
			logger.Fail("初始化引擎失败: %v", err)
			return
		}

		ctx := context.Background()
		target := args[0]

		// 热门榜模式
		if target == "hot100" {
			if exportNumber <= 0 {
				logger.Fail("请使用 -n 指定导出数量（必须大于0）")
				return
			}
			err = eng.ExportHotWorks(ctx, exportNumber, exportOutput)
			if err != nil {
				logger.Fail("导出热门作品失败: %v", err)
				return
			}
			logger.Done("已导出热门榜前 %d 个作品的链接", exportNumber)
			fmt.Printf("所有作品保存在: %s\n", exportOutput)
			return
		}

		// 单作品模式
		id := target
		stats, workDir, err := eng.ExportLinksOnly(ctx, id, exportOutput)
		if err != nil {
			logger.Fail("导出失败: %v", err)
			return
		}

		logger.Done("成功导出链接")
		fmt.Printf("作品目录: %s\n", workDir)
		fmt.Println("各文件夹链接数量:")
		for folder, count := range stats {
			display := folder
			if display == "" {
				display = "(根目录)"
			}
			fmt.Printf("  %s: %d 个链接\n", display, count)
		}
	},
}

func init() {
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "输出根目录路径（可选，默认为当前目录）")
	exportCmd.Flags().IntVarP(&exportNumber, "number", "n", 0, "热门模式下导出数量（仅在 hot100 模式下生效）")
	RegisterCmd(exportCmd)
}