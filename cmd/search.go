package cmd

import (
	"asmroner/internal/engine"
	"asmroner/internal/logger"
	"asmroner/internal/model"
	"asmroner/internal/utils"
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var searchCount int

// search 命令 —— 支持按 “RJID / 查询语法” 搜索
//
// 示例：
//
//	search RJ01037721
//	search 护士,-中出@duration:1h -c 50
//	search download 护士,-中出 -s 20 -d ./downloads/
//	search export 护士 -n 100 -f data.csv
var searchCmd = &cobra.Command{
	Use:   "search [ID/查询字符串]",
	Short: "按 RJID 或查询字符串搜索资源（支持高级搜索语法）",
	Long: `
search 命令用于按 RJID 或查询字符串搜索资源，支持 asmr.one 的高级搜索语法。

参数说明：
  [ID/查询字符串]
    - 单个 RJID：搜索指定作品
    - 多个 RJID（用逗号分隔）：批量搜索指定作品
    - 高级查询语法：如关键字过滤、排除词、时长限制等
      示例：
        search 护士,-中出@duration:1h -c 50

可用选项：
  -c, --count <数量>
      指定搜索结果条数（默认 10）
      示例：
        search 护士 -c 20

子命令：
  download [查询字符串]
      搜索并下载资源
      可用选项：
        -d, --dir <目录路径>：下载保存目录，默认当前目录
        -s, --size <数量>：下载数量，默认 100
      示例：
        search download 护士 -d ./downloads -s 20

  export [查询字符串]
      搜索并导出结果到 CSV 或 JSON
      可用选项：
        -f, --file <文件名>：导出文件名，支持 .csv/.json，默认 data.csv
        -n, --num <数量>：导出数量，默认 100
      示例：
        search export 护士 -n 50 -f data.json

适用场景：
  - 快速定位和筛选作品
  - 批量下载或导出搜索结果
  - 支持复杂查询语法，满足高级用户需求

注意事项：
  - 高级查询语法请参考 asmr.one 搜索规范
  - download/export 子命令需要正确设置目录或文件路径
  - count/size/num 参数用于限制操作数量，避免批量过大导致网络或存储压力
`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		keyword := args[0]
		count := searchCount

		logger.Step("开始搜索: 关键字='%s', 条数=%d", keyword, count)
		doSearchTask(keyword, count)
	},
}

// --------------------- 搜索主逻辑 ---------------------
func doSearchTask(keyword string, count int) {
	queryParams := model.NewQueryParams(strings.TrimSpace(keyword))

	if err := queryParams.ParseQueryStr(); err != nil {
		logger.Fail("解析查询字符串失败: %v", err)
		return
	}

	asmrOneQueryStr, err := queryParams.BuildAsmrOneQueryStr()
	if err != nil {
		logger.Fail("构建 asmr.one 查询语法失败: %v", err)
		return
	}

	engineManager, err := engine.NewEngineManager(
		model.AppConfig.Limit.SyncQPS, 1,
		model.AppConfig.Limit.SyncJitterMin,
		model.AppConfig.Limit.SyncJitterMax,
	)
	if err != nil {
		logger.Fail("创建下载引擎管理器失败: %v", err)
		return
	}
	ctx := context.Background()

	result, err := engineManager.SearchForCountResult(ctx, asmrOneQueryStr, count)
	if err != nil {
		logger.Fail("搜索失败: %s", logger.SummarizeError(err))
		return
	}

	logger.Info("搜索完毕，共找到 %d 条数据", result.Pagination.TotalCount)

	views := genTableViewData(&result)
	genTableView(views)

	logger.Done("搜索任务完成！")
}

// --------------------- 表格输出 ---------------------
func genTableView(results []model.SearchResultView) {
	for i, r := range results {
		results[i].Title = utils.NextlineRune(r.Title, 30)
	}

	runewidth.DefaultCondition.EastAsianWidth = true

	table := tablewriter.NewWriter(os.Stdout)
	headers := []string{"ID", "日期", "评分", "销量", "字幕", "标题"}
	table.Header(headers)

	for _, row := range results {
		v := reflect.ValueOf(row)
		var cols []string
		for i := 0; i < v.NumField(); i++ {
			cols = append(cols, fmt.Sprintf("%v", v.Field(i).Interface()))
		}
		table.Append(cols)
	}

	table.Bulk(true)
	table.Render()
}

func genTableViewData(result *model.SearchResult) []model.SearchResultView {
	views := make([]model.SearchResultView, 0, len(result.Works))
	for _, item := range result.Works {
		views = append(views, model.SearchResultView{
			Title:          item.Title,
			Release:        item.Release,
			DlCount:        item.DlCount,
			RateAverage2Dp: item.RateAverage2Dp,
			HasSubtitle:    item.HasSubtitle,
			SourceID:       item.SourceID,
		})
	}
	return views
}

//
// =========================================================
//                  download 子命令
// =========================================================
//

var (
	downloadDir string
	downloadCnt int
)

// ---------------------- download 子命令 ----------------------
var searchDownloadCmd = &cobra.Command{
	Use:   "download [查询字符串]",
	Short: "搜索并下载资源",
	Long: `
search download 子命令用于根据搜索结果批量下载音声资源。

参数说明：
  [查询字符串]：搜索关键字或高级查询语法
    示例：
      search download 护士,-中出@duration:1h

可用选项：
  -d, --dir <目录路径>：下载保存目录，默认当前目录
      示例：
        search download 护士 -d ./downloads
  -s, --size <数量>：下载数量，默认 100
      示例：
        search download 护士 -s 50

适用场景：
  - 批量下载特定关键词作品
  - 下载指定数量的搜索结果

注意事项：
  - 下载目录必须存在或可创建
  - 下载数量过大可能导致网络压力或存储不足
`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			logger.Warn("请提供搜索关键字，例如: search download 护士")
			return
		}

		keyword := args[0]
		logger.Step("开始下载: 关键字=%s, 保存目录=%s, 数量=%d", keyword, downloadDir, downloadCnt)

		doSearchDownload(keyword, downloadDir, downloadCnt)
	},
}

func doSearchDownload(keyword string, downloadDir string, count int) {
	queryParams := model.NewQueryParams(strings.TrimSpace(keyword))
	if err := queryParams.ParseQueryStr(); err != nil {
		logger.Fail("解析查询参数失败: %v", err)
		return
	}

	asmrOneQueryStr, err := queryParams.BuildAsmrOneQueryStr()
	if err != nil {
		logger.Fail("构建搜索语法失败: %v", err)
		return
	}

	engineManager, err := engine.NewEngineManager(
		model.AppConfig.Limit.DownloadQPS, 1,
		model.AppConfig.Limit.DownloadJitterMin,
		model.AppConfig.Limit.DownloadJitterMax,
	)
	if err != nil {
		logger.Fail("创建下载引擎管理器失败: %v", err)
		return
	}
	ctx := context.Background()

	result, err := engineManager.SearchForCountResult(ctx, asmrOneQueryStr, count)

	if err != nil {
		logger.Fail("搜索失败: %s", logger.SummarizeError(err))
		return
	}

	views := genTableViewData(&result)

	logger.Info("搜索到 %d 条，开始批量下载...", len(views))

	engineManager.DownloadBatchMedias(ctx, views, downloadDir)

	logger.Done("下载完成！")
}

//
// =========================================================
//                  export 子命令
// =========================================================
//

var (
	exportFile  string
	exportCount int
)

// ---------------------- export 子命令 ----------------------
var searchExportCmd = &cobra.Command{
	Use:   "export [查询字符串]",
	Short: "搜索并导出结果到 CSV/JSON",
	Long: `
search export 子命令用于将搜索结果导出为 CSV 或 JSON 文件，便于离线查看或数据处理。

参数说明：
  [查询字符串]：搜索关键字或高级查询语法
    示例：
      search export 护士,-中出@duration:1h

可用选项：
  -f, --file <文件名>：导出文件名，支持 .csv/.json，默认 data.csv
      示例：
        search export 护士 -f result.csv
  -n, --num <数量>：导出数量，默认 100
      示例：
        search export 护士 -n 50

适用场景：
  - 离线查看搜索结果
  - 将数据用于统计或二次处理
  - 批量导出特定关键词作品信息

注意事项：
  - 文件名需包含正确后缀
  - 导出数量过大可能导致文件过大
`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			logger.Warn("请提供搜索关键字，例如: search export 护士")
			return
		}

		keyword := args[0]
		logger.Step("导出资源: 关键字=%s, 文件=%s, 数量=%d", keyword, exportFile, exportCount)

		doSearchExport(keyword)
	},
}

func doSearchExport(keyword string) {
	queryParams := model.NewQueryParams(strings.TrimSpace(keyword))

	if err := queryParams.ParseQueryStr(); err != nil {
		logger.Fail("解析失败: %v", err)
		return
	}
	asmrOneQueryStr, err := queryParams.BuildAsmrOneQueryStr()
	if err != nil {
		logger.Fail("构建搜索语法失败: %v", err)
		return
	}

	engineManager, err := engine.NewEngineManager(
		model.AppConfig.Limit.SyncQPS, 1,
		model.AppConfig.Limit.SyncJitterMin,
		model.AppConfig.Limit.SyncJitterMax,
	)
	if err != nil {
		logger.Fail("创建下载引擎管理器失败: %v", err)
		return
	}
	ctx := context.Background()

	result, err := engineManager.SearchForCountResult(ctx, asmrOneQueryStr, exportCount)

	if err != nil {
		logger.Fail("搜索失败: %s", logger.SummarizeError(err))
		return
	}

	views := genTableViewData(&result)

	// 自动判断格式
	var format = "csv"
	if strings.HasSuffix(exportFile, ".json") {
		format = "json"
	}

	if format == "csv" {
		utils.ExportToCSV(views, exportFile)
	} else {
		utils.ExportToJSON(views, exportFile)
	}

	logger.Done("导出成功！")
}

// ---------------------- cobra 初始化 ----------------------
func init() {
	// search -c
	searchCmd.Flags().IntVarP(&searchCount, "count", "c", 10, "搜索数量")

	// search download
	searchDownloadCmd.Flags().StringVarP(&downloadDir, "dir", "d", ".", "下载目录")
	searchDownloadCmd.Flags().IntVarP(&downloadCnt, "size", "s", 100, "下载数量")
	searchCmd.AddCommand(searchDownloadCmd)

	// search export
	searchExportCmd.Flags().StringVarP(&exportFile, "file", "f", "data.csv", "导出文件名(.csv/.json)")
	searchExportCmd.Flags().IntVarP(&exportCount, "num", "n", 100, "导出数量")
	searchCmd.AddCommand(searchExportCmd)

	rootCmd.AddCommand(searchCmd)
}
