package cmd

import (
	"asmroner/internal/engine"
	"asmroner/internal/logger"
	"asmroner/internal/model"
	"asmroner/internal/utils"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/fatih/color"
)

var listDetail bool

// list 命令 —— 查看作品资源目录内的文件类型 / 文件明细 / 过滤预览
//
// 示例：
//
//	list RJ01412863          列出该资源下所有文件的类型及数量，并按当前配置预览过滤效果
//	list RJ01412863 -d       列出该资源目录下的所有文件（标注每个文件是否会被过滤）
var listCmd = &cobra.Command{
	Use:   "list [RJID]",
	Short: "查看作品资源目录内的文件类型或文件列表",
	Long: `
list 命令用于在下载前查看指定作品资源目录内的文件构成，避免因过滤配置
（prefer_media / include_ext / exclude_ext / include_keyword / exclude_keyword）
与资源实际内容不匹配而"什么都下载不到"。

参数说明：
  [RJID]：作品ID，如 RJ01412863

可用选项：
  -d, --detail
      以树状结构列出资源目录下的所有文件（标注每个文件是否会被当前配置过滤）
      示例：
        list RJ01412863 -d

默认行为（不加 -d）：
  1. 按扩展名统计资源内的所有文件类型及数量，例如 mp3/wav/flac/jpg 等
  2. 按当前配置模拟一遍下载过滤流程，显示各规则的排除数量和最终会下载的文件数

适用场景：
  - 下载前确认资源内有哪些音频格式（如只有 wav 没有 mp3）
  - 检查 prefer_media / include_ext / exclude_ext / 关键词过滤配置能否命中该资源
  - 查看哪些目录/文件会被关键词规则（如 exclude_keyword=SEなし）排除
`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := strings.TrimSpace(args[0])
		doListTask(id, listDetail)
	},
}

type listFileEntry struct {
	dir  string
	name string
}

// relPath 相对路径（/ 分隔），与引擎 PreviewFilterStats 的输入格式一致
func (e listFileEntry) relPath() string {
	return filepath.ToSlash(filepath.Join(e.dir, e.name))
}

func doListTask(id string, detail bool) {
	valid, prefix, number, err := utils.IsValidDlsiteID(strings.ToUpper(id))
	if err != nil || !valid {
		logger.Fail("无效的作品ID: %s（示例: RJ01412863）", id)
		return
	}
	workID := strings.ToUpper(prefix) + number

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

	task := logger.NewTask(workID)

	// 获取作品信息（标题仅用于展示，失败不阻塞）
	workInfo, err := engineManager.GetWorkInfo(ctx, number)
	if err != nil {
		task.Warn("获取作品信息失败: %s", logger.SummarizeError(err))
	} else {
		task.Info("作品: %s", workInfo.Title)
	}

	// 获取音轨（资源目录树）
	tracks, err := engineManager.GetVoiceTracks(number)
	if err != nil {
		task.Error("获取音轨列表失败: %s", logger.SummarizeError(err))
		return
	}

	files := make([]listFileEntry, 0)
	var walk func(ts []model.Track, dir string)
	// 目录/文件名与下载侧 ensureDirExists 一致地做净化，保证关键词过滤预览与实际下载结果吻合
	walk = func(ts []model.Track, dir string) {
		for _, t := range ts {
			if t.Type == "folder" {
				name := utils.SanitizeFileName(t.Title)
				if name == "" {
					name = "untitled"
				}
				walk(t.Children, filepath.Join(dir, name))
			} else {
				files = append(files, listFileEntry{dir: dir, name: utils.SanitizeFileName(t.Title)})
			}
		}
	}
	walk(tracks, "")

	if len(files) == 0 {
		task.Warn("该资源目录下没有可下载的文件")
		return
	}
	task.Info("文件总数: %d", len(files))

	// 按下载的真实顺序模拟过滤，得到保留集合和各规则的排除统计
	relPaths := make([]string, 0, len(files))
	for _, f := range files {
		relPaths = append(relPaths, f.relPath())
	}
	keptPaths, stats, reasons := engineManager.PreviewFilterStats(relPaths)
	keptSet := make(map[string]bool, len(keptPaths))
	for _, p := range keptPaths {
		keptSet[p] = true
	}

	if detail {
		printListDetail(workID, files, keptSet, reasons)
	} else {
		printListTypes(files)
		printListPreview(len(files), keptPaths, stats)
	}

	logger.Done("查询完成！")
}

// fullExt 提取完整多级后缀（小写），如 "01.mp3.vtt" -> ".mp3.vtt"，无后缀返回 "(无后缀)"
func fullExt(name string) string {
	ln := strings.ToLower(name)
	i := strings.LastIndex(ln, ".")
	if i < 0 {
		return "(无后缀)"
	}
	// 泛字幕 .vtt 往前多取一级，展示成 .mp3.vtt / .wav.vtt
	if ln[i:] == ".vtt" {
		if j := strings.LastIndex(ln[:i], "."); j > 0 {
			return ln[j:]
		}
	}
	return ln[i:]
}

// printListTypes 按扩展名统计并输出文件类型概览
func printListTypes(files []listFileEntry) {
	counts := make(map[string]int)
	for _, f := range files {
		counts[fullExt(f.name)]++
	}

	exts := make([]string, 0, len(counts))
	for ext := range counts {
		exts = append(exts, ext)
	}
	// 数量多的排前面，数量相同按扩展名字母序
	sort.Slice(exts, func(i, j int) bool {
		if counts[exts[i]] != counts[exts[j]] {
			return counts[exts[i]] > counts[exts[j]]
		}
		return exts[i] < exts[j]
	})

	table := tablewriter.NewWriter(os.Stdout)
	table.Header([]string{"类型", "数量"})
	for _, ext := range exts {
		table.Append([]string{ext, fmt.Sprintf("%d", counts[ext])})
	}
	table.Render()
}

// printListPreview 输出过滤预览：各规则的排除数量与最终会下载的文件数
func printListPreview(total int, keptPaths []string, stats []engine.FilterStat) {
	if len(stats) == 0 {
		logger.Info("当前无过滤配置，全部 %d 个文件都会下载", total)
		return
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.Header([]string{"过滤规则", "内容", "排除文件数"})
	for _, s := range stats {
		table.Append([]string{s.Rule, s.Detail, fmt.Sprintf("%d", s.Excluded)})
	}
	table.Render()

	if len(keptPaths) == 0 {
		logger.Warn("按当前配置过滤后将一无所获（0/%d），请检查过滤配置或使用 list -d 查看明细！", total)
	} else {
		logger.Info("按当前配置过滤后: 将下载 %d/%d 个文件", len(keptPaths), total)
	}
}

// treeNode 树状输出的目录节点
type treeNode struct {
	dirs  map[string]*treeNode
	files []listFileEntry
}

func newTreeNode() *treeNode {
	return &treeNode{dirs: make(map[string]*treeNode)}
}

// buildFileTree 把扁平的文件列表按目录前缀挂到树上，还原资源目录层级
func buildFileTree(files []listFileEntry) *treeNode {
	root := newTreeNode()
	for _, f := range files {
		node := root
		if f.dir != "" {
			for part := range strings.SplitSeq(f.dir, string(filepath.Separator)) {
				child, ok := node.dirs[part]
				if !ok {
					child = newTreeNode()
					node.dirs[part] = child
				}
				node = child
			}
		}
		node.files = append(node.files, f)
	}
	return root
}

// printListDetail 以树状结构输出所有文件明细，并标注每个文件会被保留还是被哪条规则过滤
func printListDetail(workID string, files []listFileEntry, keptSet map[string]bool, reasons map[string]string) {
	root := buildFileTree(files)

	logger.Info("文件明细 (%s 保留 / %s 被 <规则名> 过滤):",
		color.GreenString("✓"), color.RedString("✗"))
	var b strings.Builder
	b.WriteString(workID)
	b.WriteString("/\n")
	renderTree(&b, root, "", keptSet, reasons)
	fmt.Print(b.String())
}

// renderTree 递归渲染目录树：目录按字母序在前，文件按名称序在后
func renderTree(b *strings.Builder, node *treeNode, prefix string, keptSet map[string]bool, reasons map[string]string) {
	dirNames := make([]string, 0, len(node.dirs))
	for name := range node.dirs {
		dirNames = append(dirNames, name)
	}
	sort.Strings(dirNames)

	files := make([]listFileEntry, len(node.files))
	copy(files, node.files)
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })

	total := len(dirNames) + len(files)
	idx := 0
	for _, name := range dirNames {
		connector, childPrefix := treeBranch(prefix, idx == total-1)
		b.WriteString(connector)
		b.WriteString(name)
		b.WriteString("/\n")
		renderTree(b, node.dirs[name], childPrefix, keptSet, reasons)
		idx++
	}
	for _, f := range files {
		connector, _ := treeBranch(prefix, idx == total-1)
		// 保留=绿色，被过滤=红色（fatih/color 在非终端输出时自动关闭颜色）
		var line string
		if keptSet[f.relPath()] {
			line = color.GreenString("%s  ✓ 保留", f.name)
		} else {
			// 标注命中规则：白名单（include_*）表示未命中被挡，黑名单（exclude_*）表示命中被排除
			if rule := reasons[f.relPath()]; rule != "" {
				line = color.RedString("%s  ✗ 被 %s 过滤", f.name, rule)
			} else {
				line = color.RedString("%s  ✗ 被过滤", f.name)
			}
		}
		b.WriteString(connector)
		b.WriteString(line)
		b.WriteString("\n")
		idx++
	}
}

// treeBranch 返回当前层级的连接符及子层级前缀
func treeBranch(prefix string, last bool) (connector, childPrefix string) {
	if last {
		return prefix + "└── ", prefix + "    "
	}
	return prefix + "├── ", prefix + "│   "
}

// ---------------------- cobra 初始化 ----------------------
func init() {
	listCmd.Flags().BoolVarP(&listDetail, "detail", "d", false, "列出资源目录下的所有文件")
	rootCmd.AddCommand(listCmd)
}
