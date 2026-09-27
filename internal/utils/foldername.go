package utils

import (
	"asmroner/internal/consts"
	"strings"
)

// BuildFolderName 按 format 构建作品下载目录名（纯函数）。
//
// 可用占位符:
//
//	{rjid}     作品ID, 如 RJ01037721
//	{date}     发售日期 YYYYMMDD
//	{subtitle} sub / nosub
//	{title}    作品标题
//
// format 为空时使用 consts.DefaultFolderNameFormat（与历史版本硬编码命名一致）。
// 未知占位符原样保留；结果整体做文件名非法字符净化。
func BuildFolderName(format, workID, release string, hasSubtitle bool, title string) string {
	if strings.TrimSpace(format) == "" {
		format = consts.DefaultFolderNameFormat
	}

	subtitle := "nosub"
	if hasSubtitle {
		subtitle = "sub"
	}

	name := strings.NewReplacer(
		"{rjid}", strings.ToUpper(workID),
		"{date}", strings.ReplaceAll(release, "-", ""),
		"{subtitle}", subtitle,
		"{title}", title,
	).Replace(format)

	// 整体做文件名非法字符净化（含路径分隔符移除，防止产生嵌套目录）
	name = SanitizeFileName(name)

	// 格式串全是非法字符等极端情况，回退默认格式
	if name == "" {
		return BuildFolderName(consts.DefaultFolderNameFormat, workID, release, hasSubtitle, title)
	}
	return name
}
