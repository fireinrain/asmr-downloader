package engine

import (
	"path/filepath"
	"strings"
)

// 音频格式与后缀映射（prefer_media 支持的格式及其嵌套字幕后缀）
var audioExtMap = map[string][]string{
	"mp3":  {".mp3", ".mp3.vtt"},
	"wav":  {".wav", ".wav.vtt"},
	"flac": {".flac", ".flac.vtt"},
}

// audioExtList 受 prefer_media 管控的全部音频后缀
var audioExtList = []string{
	".mp3", ".mp3.vtt",
	".wav", ".wav.vtt",
	".flac", ".flac.vtt",
}

// isAudioExt 文件名（已小写）是否属于受 prefer_media 管控的音频文件
func isAudioExt(lf string) bool {
	for _, ext := range audioExtList {
		if strings.HasSuffix(lf, ext) {
			return true
		}
	}
	return false
}

// hitAudioFormat 文件名（已小写）是否命中指定音频格式的任一后缀
func hitAudioFormat(lf string, format string) bool {
	exts, ok := audioExtMap[format]
	if !ok {
		return false
	}
	for _, ext := range exts {
		if strings.HasSuffix(lf, ext) {
			return true
		}
	}
	return false
}

// parseKeywordList 解析逗号分隔的关键词配置为切片（小写、去空白），如 "SEなし, no se"
func parseKeywordList(raw string) []string {
	keywords := make([]string, 0)
	for _, part := range strings.Split(raw, ",") {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "" {
			continue
		}
		keywords = append(keywords, part)
	}
	return keywords
}

// hitKeyword 相对路径（已小写）是否包含任一关键词
func hitKeyword(relPath string, keywords []string) bool {
	if len(keywords) == 0 {
		return false
	}
	for _, kw := range keywords {
		if strings.Contains(relPath, kw) {
			return true
		}
	}
	return false
}

// relPathOf 计算文件相对作品目录的路径（目录分隔符统一为 /）；失败时回退绝对路径
func relPathOf(baseDir string, fileDir string, fileName string) string {
	rel, err := filepath.Rel(baseDir, fileDir)
	if err != nil {
		rel = fileDir
	}
	return filepath.ToSlash(filepath.Join(rel, fileName))
}

// filterIncludeKeyword 按白名单 downloader.include_keyword 保留相对路径命中关键词的文件，空配置原样返回
func (m *EngineManager) filterIncludeKeyword(urls [][]string, baseDir string) [][]string {
	keywords := parseKeywordList(m.Config.Downloader.IncludeKeyword)
	if len(keywords) == 0 {
		return urls
	}

	kept := make([][]string, 0, len(urls))
	for _, f := range urls {
		if hitKeyword(strings.ToLower(relPathOf(baseDir, f[1], f[2])), keywords) {
			kept = append(kept, f)
		}
	}
	return kept
}

// filterExcludeKeyword 按黑名单 downloader.exclude_keyword 排除相对路径命中关键词的文件，空配置原样返回
func (m *EngineManager) filterExcludeKeyword(urls [][]string, baseDir string) [][]string {
	keywords := parseKeywordList(m.Config.Downloader.ExcludeKeyword)
	if len(keywords) == 0 {
		return urls
	}

	kept := make([][]string, 0, len(urls))
	for _, f := range urls {
		if !hitKeyword(strings.ToLower(relPathOf(baseDir, f[1], f[2])), keywords) {
			kept = append(kept, f)
		}
	}
	return kept
}

// FilterStat 单条过滤规则的排除统计（供 list 命令展示过滤预览）
type FilterStat struct {
	Rule     string // 规则来源，如 prefer_media / include_keyword
	Detail   string // 规则内容
	Excluded int    // 被该规则排除的文件数
}

// PreviewFilterStats 按 DownloadOne 的真实过滤顺序（prefer_media → include_ext →
// exclude_ext → include_keyword → exclude_keyword）对相对路径列表做模拟过滤，
// 返回保留的路径和每条规则的排除统计。relPaths 为以 / 分隔的相对路径。
func (m *EngineManager) PreviewFilterStats(relPaths []string) ([]string, []FilterStat) {
	paths := relPaths
	stats := make([]FilterStat, 0)

	// 1. prefer_media：按优先级选第一个有命中的音频格式，其余音频文件被丢弃
	prefer := strings.ToLower(strings.TrimSpace(m.Config.Downloader.PreferMedia))
	if prefer != "" && prefer != "all" && len(paths) > 0 {
		chosen := ""
		for _, rule := range strings.Split(prefer, ">") {
			rule = strings.TrimSpace(rule)
			if rule == "" {
				continue
			}
			for _, p := range paths {
				if hitAudioFormat(strings.ToLower(filepath.Base(p)), rule) {
					chosen = rule
					break
				}
			}
			if chosen != "" {
				break
			}
		}
		kept := make([]string, 0, len(paths))
		for _, p := range paths {
			lf := strings.ToLower(filepath.Base(p))
			if isAudioExt(lf) {
				if chosen != "" && hitAudioFormat(lf, chosen) {
					kept = append(kept, p)
				}
			} else {
				kept = append(kept, p)
			}
		}
		if excluded := len(paths) - len(kept); excluded > 0 {
			stats = append(stats, FilterStat{Rule: "prefer_media", Detail: prefer, Excluded: excluded})
		}
		paths = kept
	}

	// 2. include_ext
	paths, stat := filterPathsByExtSet(paths, "include_ext", parseExtList(m.Config.Downloader.IncludeExt), true)
	if stat != nil {
		stats = append(stats, *stat)
	}
	// 3. exclude_ext
	paths, stat = filterPathsByExtSet(paths, "exclude_ext", parseExtList(m.Config.Downloader.ExcludeExt), false)
	if stat != nil {
		stats = append(stats, *stat)
	}
	// 4. include_keyword
	paths, stat = filterPathsByKeywordSet(paths, "include_keyword", parseKeywordList(m.Config.Downloader.IncludeKeyword), true)
	if stat != nil {
		stats = append(stats, *stat)
	}
	// 5. exclude_keyword
	paths, stat = filterPathsByKeywordSet(paths, "exclude_keyword", parseKeywordList(m.Config.Downloader.ExcludeKeyword), false)
	if stat != nil {
		stats = append(stats, *stat)
	}

	return paths, stats
}

// filterPathsByExtSet 按扩展名集合对相对路径列表过滤（keep=true 保留命中，keep=false 排除命中）；
// 命中判断基于文件名（与下载逻辑一致，避免目录名误命中扩展名）
func filterPathsByExtSet(paths []string, rule string, set map[string]bool, keep bool) ([]string, *FilterStat) {
	if len(set) == 0 {
		return paths, nil
	}
	kept := make([]string, 0, len(paths))
	for _, p := range paths {
		hit := hitExtSet(strings.ToLower(filepath.Base(p)), set)
		if hit == keep {
			kept = append(kept, p)
		}
	}
	if excluded := len(paths) - len(kept); excluded > 0 {
		return kept, &FilterStat{Rule: rule, Detail: joinExtSet(set), Excluded: excluded}
	}
	return paths, nil
}

// filterPathsByKeywordSet 按关键词集合对相对路径列表过滤（keep=true 保留命中，keep=false 排除命中）
func filterPathsByKeywordSet(paths []string, rule string, keywords []string, keep bool) ([]string, *FilterStat) {
	if len(keywords) == 0 {
		return paths, nil
	}
	kept := make([]string, 0, len(paths))
	for _, p := range paths {
		hit := hitKeyword(strings.ToLower(filepath.ToSlash(p)), keywords)
		if hit == keep {
			kept = append(kept, p)
		}
	}
	if excluded := len(paths) - len(kept); excluded > 0 {
		return kept, &FilterStat{Rule: rule, Detail: strings.Join(keywords, ","), Excluded: excluded}
	}
	return paths, nil
}

// joinExtSet 将扩展名集合转为逗号分隔字符串（用于统计展示）
func joinExtSet(set map[string]bool) string {
	exts := make([]string, 0, len(set))
	for ext := range set {
		exts = append(exts, ext)
	}
	// 稳定输出便于测试与展示
	for i := 0; i < len(exts); i++ {
		for j := i + 1; j < len(exts); j++ {
			if exts[j] < exts[i] {
				exts[i], exts[j] = exts[j], exts[i]
			}
		}
	}
	return strings.Join(exts, ",")
}
