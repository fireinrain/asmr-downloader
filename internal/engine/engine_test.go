package engine

import (
	"asmroner/internal/model"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEngineManager_FilterIncludeExt(t *testing.T) {
	urls := [][]string{
		{"u1", "d", "01 トラック.mp3"},
		{"u2", "d", "01 トラック.mp3.vtt"},
		{"u3", "d", "cover.jpg"},
		{"u4", "d", "sample.MP4"},
		{"u5", "d", "movie.webm"},
		{"u6", "d", "02.wav"},
		{"u7", "d", "おまけ.pdf"},
	}
	// 只保留 mp3 和 jpg；".mp3" 精确匹配，不自动带上字幕
	m := &EngineManager{Config: &model.Config{}}
	m.Config.Downloader.IncludeExt = "MP3, .jpg"

	got := m.filterIncludeExt(urls)
	want := []string{"01 トラック.mp3", "cover.jpg"}
	if len(got) != len(want) {
		t.Fatalf("filterIncludeExt() kept %d files, want %d: %v", len(got), len(want), names(got))
	}
	for i, f := range got {
		if f[2] != want[i] {
			t.Errorf("kept[%d] = %q, want %q", i, f[2], want[i])
		}
	}

	// 空配置不过滤
	m.Config.Downloader.IncludeExt = ""
	if got := m.filterIncludeExt(urls); len(got) != len(urls) {
		t.Errorf("empty include_ext: kept %d files, want %d", len(got), len(urls))
	}
}

func TestEngineManager_FilterIncludeThenExclude(t *testing.T) {
	urls := [][]string{
		{"u1", "d", "01.mp3"},
		{"u2", "d", "01.mp3.vtt"},
		{"u3", "d", "cover.jpg"},
		{"u4", "d", "movie.webm"},
	}
	// 先白后黑串联，".mp3.vtt" 显式带上 mp3 的字幕
	m := &EngineManager{Config: &model.Config{}}
	m.Config.Downloader.IncludeExt = ".mp3,.mp3.vtt,.jpg,.webm"
	m.Config.Downloader.ExcludeExt = "webm"

	got := m.filterExcludeExt(m.filterIncludeExt(urls))
	want := []string{"01.mp3", "01.mp3.vtt", "cover.jpg"}
	if len(got) != len(want) {
		t.Fatalf("include+exclude kept %d files, want %d: %v", len(got), len(want), names(got))
	}
	for i, f := range got {
		if f[2] != want[i] {
			t.Errorf("kept[%d] = %q, want %q", i, f[2], want[i])
		}
	}
}

func TestEngineManager_FilterExcludeExt(t *testing.T) {
	urls := [][]string{
		{"u1", "d", "01 トラック.mp3"},
		{"u2", "d", "01 トラック.mp3.vtt"},
		{"u3", "d", "cover.jpg"},
		{"u4", "d", "sample.MP4"},
		{"u5", "d", "movie.webm"},
		{"u6", "d", "02.wav"},
		{"u7", "d", "おまけ.pdf"},
	}
	// 大小写不敏感 + 自动补点："MP4"→".mp4"，"webm"→".webm"
	m := &EngineManager{Config: &model.Config{}}
	m.Config.Downloader.ExcludeExt = "MP4, webm"

	got := m.filterExcludeExt(urls)
	want := []string{"01 トラック.mp3", "01 トラック.mp3.vtt", "cover.jpg", "02.wav", "おまけ.pdf"}
	if len(got) != len(want) {
		t.Fatalf("filterExcludeExt() kept %d files, want %d: %v", len(got), len(want), names(got))
	}
	for i, f := range got {
		if f[2] != want[i] {
			t.Errorf("kept[%d] = %q, want %q", i, f[2], want[i])
		}
	}

	// 空配置不过滤
	m.Config.Downloader.ExcludeExt = ""
	if got := m.filterExcludeExt(urls); len(got) != len(urls) {
		t.Errorf("empty exclude_ext: kept %d files, want %d", len(got), len(urls))
	}

	// 部分命中：排除 mp3+webm；mp3 的字幕不后缀命中 ".mp3"，因精确匹配被保留
	m.Config.Downloader.ExcludeExt = "mp3,webm"
	if got := m.filterExcludeExt(urls); len(got) != 5 {
		t.Errorf("exclude mp3,webm: kept %d files, want 5: %v", len(got), names(got))
	}
}

// runFilterPipeline 按 DownloadOne 的真实顺序串联全部过滤器
func runFilterPipeline(m *EngineManager, urls [][]string, baseDir string) [][]string {
	urls = m.filterTargetAudioFormat(urls)
	urls = m.filterIncludeExt(urls)
	urls = m.filterExcludeExt(urls)
	urls = m.filterIncludeKeyword(urls, baseDir)
	urls = m.filterExcludeKeyword(urls, baseDir)
	return urls
}

func TestEngineManager_FilterPipelineMatrix(t *testing.T) {
	// 同一音轨的 mp3/wav 双格式各带字幕，外加图片/视频/文档各一个
	urls := [][]string{
		{"u1", "d", "01.mp3"},
		{"u2", "d", "01.mp3.vtt"},
		{"u3", "d", "01.wav"},
		{"u4", "d", "01.wav.vtt"},
		{"u5", "d", "cover.jpg"},
		{"u6", "d", "movie.mp4"},
		{"u7", "d", "おまけ.pdf"},
	}
	// 只有 wav 一套的作品，用于 prefer_media 回退行为
	wavOnly := [][]string{
		{"u1", "d", "01.wav"},
		{"u2", "d", "01.wav.vtt"},
		{"u3", "d", "cover.jpg"},
	}

	cases := []struct {
		name        string
		preferMedia string
		include     string
		exclude     string
		fixture     [][]string
		want        []string
	}{
		{
			name: "无任何配置：全部保留", fixture: urls,
			want: []string{"01.mp3", "01.mp3.vtt", "01.wav", "01.wav.vtt", "cover.jpg", "movie.mp4", "おまけ.pdf"},
		},
		{
			name: "仅白名单 .mp3：精确匹配不带字幕", include: ".mp3", fixture: urls,
			want: []string{"01.mp3"},
		},
		{
			name: "白名单嵌套 .mp3,.mp3.vtt：mp3 一套完整", include: ".mp3,.mp3.vtt", fixture: urls,
			want: []string{"01.mp3", "01.mp3.vtt"},
		},
		{
			name: "白名单全音频+泛字幕：全部音轨和字幕", include: ".mp3,.wav,.vtt", fixture: urls,
			want: []string{"01.mp3", "01.mp3.vtt", "01.wav", "01.wav.vtt"},
		},
		{
			name: "白名单 .mp3,.vtt：泛字幕命中，wav 的字幕也保留", include: ".mp3,.vtt", fixture: urls,
			want: []string{"01.mp3", "01.mp3.vtt", "01.wav.vtt"},
		},
		{
			name: "白名单仅 .vtt：只要字幕，两种 vtt 视为同一种全部保留", include: ".vtt", fixture: urls,
			want: []string{"01.mp3.vtt", "01.wav.vtt"},
		},
		{
			name:    "黑名单穷举四项 .mp3,.wav,.mp3.vtt,.wav.vtt：音频字幕全排除，其余保留",
			exclude: ".mp3,.wav,.mp3.vtt,.wav.vtt", fixture: urls,
			want: []string{"cover.jpg", "movie.mp4", "おまけ.pdf"},
		},
		{
			name: "黑名单 .mp3：本体排除，其字幕不后缀命中故保留", exclude: ".mp3", fixture: urls,
			want: []string{"01.mp3.vtt", "01.wav", "01.wav.vtt", "cover.jpg", "movie.mp4", "おまけ.pdf"},
		},
		{
			name: "黑名单泛字幕 .vtt：保留全部音频，字幕全排除", exclude: ".vtt", fixture: urls,
			want: []string{"01.mp3", "01.wav", "cover.jpg", "movie.mp4", "おまけ.pdf"},
		},
		{
			name: "黑名单嵌套 .mp3.vtt：只排 mp3 字幕，wav 一套完整", exclude: ".mp3.vtt", fixture: urls,
			want: []string{"01.mp3", "01.wav", "01.wav.vtt", "cover.jpg", "movie.mp4", "おまけ.pdf"},
		},
		{
			name:    "白名单 .mp3,.vtt 与黑名单 .mp3.vtt 串联：mp3 本体与 wav 字幕保留",
			include: ".mp3,.vtt", exclude: ".mp3.vtt", fixture: urls,
			want: []string{"01.mp3", "01.wav.vtt"},
		},
		{
			name: "大小写不敏感 MP3, .MP3.VTT", include: "MP3, .MP3.VTT", fixture: urls,
			want: []string{"01.mp3", "01.mp3.vtt"},
		},
		{
			name: "缺点号 mp3.vtt：自动补点，嵌套扩展名只命中 mp3 的字幕", include: "mp3.vtt", fixture: urls,
			want: []string{"01.mp3.vtt"},
		},
		{
			name: "多余空项 .mp3, ,.jpg：空项被跳过", include: ".mp3, ,.jpg", fixture: urls,
			want: []string{"01.mp3", "cover.jpg"},
		},
		{
			name:    "白名单与黑名单重叠 .mp3,.jpg 排除 .mp3：先白后黑均生效",
			include: ".mp3,.jpg", exclude: ".mp3", fixture: urls,
			want: []string{"cover.jpg"},
		},
		{
			name:        "prefer_media=mp3 与白名单 .vtt 叠加：仅剩 mp3 的字幕",
			preferMedia: "mp3", include: ".vtt", fixture: urls,
			want: []string{"01.mp3.vtt"},
		},
		{
			name:        "prefer_media=mp3：选中 mp3 一套，非音频文件不受影响",
			preferMedia: "mp3", fixture: urls,
			want: []string{"01.mp3", "01.mp3.vtt", "cover.jpg", "movie.mp4", "おまけ.pdf"},
		},
		{
			name:        "prefer_media=mp3 但作品只有 wav：音频全部被丢弃，仅剩非音频",
			preferMedia: "mp3", fixture: wavOnly,
			want: []string{"cover.jpg"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &EngineManager{Config: &model.Config{}}
			m.Config.Downloader.PreferMedia = tc.preferMedia
			m.Config.Downloader.IncludeExt = tc.include
			m.Config.Downloader.ExcludeExt = tc.exclude

			got := runFilterPipeline(m, tc.fixture, "work")
			if len(got) != len(tc.want) {
				t.Fatalf("kept %d files, want %d: %v", len(got), len(tc.want), names(got))
			}
			// 关键词过滤是流式保留，顺序与输入一致
			for i, f := range got {
				if f[2] != tc.want[i] {
					t.Errorf("kept[%d] = %q, want %q (all: %v)", i, f[2], tc.want[i], names(got))
				}
			}
		})
	}
}

func TestEngineManager_FilterKeyword(t *testing.T) {
	// 模拟 SEあり / SEなし 双版本目录结构
	urls := [][]string{
		{"u1", filepath.Join("work", "SEあり"), "01.mp3"},
		{"u2", filepath.Join("work", "SEあり", "folder"), "02.wav"},
		{"u3", filepath.Join("work", "SEなし"), "01.mp3"},
		{"u4", "work", "03_SEなし.mp3"},
		{"u5", "work", "04.mp3"},
	}
	base := "work"

	t.Run("黑名单排除 SEなし 目录和文件名", func(t *testing.T) {
		m := &EngineManager{Config: &model.Config{}}
		m.Config.Downloader.ExcludeKeyword = "SEなし, no se"
		got := m.filterExcludeKeyword(urls, base)
		want := []string{"01.mp3", "02.wav", "04.mp3"}
		if len(got) != len(want) {
			t.Fatalf("kept %d files, want %d: %v", len(got), len(want), names(got))
		}
		for i, f := range got {
			if f[2] != want[i] {
				t.Errorf("kept[%d] = %q, want %q", i, f[2], want[i])
			}
		}
	})

	t.Run("白名单只留 SEあり 目录", func(t *testing.T) {
		m := &EngineManager{Config: &model.Config{}}
		m.Config.Downloader.IncludeKeyword = "SEあり"
		got := m.filterIncludeKeyword(urls, base)
		want := []string{"01.mp3", "02.wav"}
		if len(got) != len(want) {
			t.Fatalf("include_keyword kept %d files, want %d: %v", len(got), len(want), names(got))
		}
		for i, f := range got {
			if f[2] != want[i] {
				t.Errorf("kept[%d] = %q, want %q", i, f[2], want[i])
			}
		}
	})
}

// TestEngineManager_PreviewFilterStats 保证 list 命令的过滤预览与真实下载过滤链路一致
func TestEngineManager_PreviewFilterStats(t *testing.T) {
	m := &EngineManager{Config: &model.Config{}}
	m.Config.Downloader.PreferMedia = "mp3"
	m.Config.Downloader.IncludeExt = ".mp3,.mp3.vtt"
	m.Config.Downloader.ExcludeKeyword = "SEなし"

	relPaths := []string{
		"SEあり/01.mp3", "SEあり/01.mp3.vtt",
		"SEあり/01.wav", "SEあり/01.wav.vtt",
		"SEなし/01.mp3", "SEなし/01.mp3.vtt",
		"cover.jpg",
	}
	kept, stats := m.PreviewFilterStats(relPaths)

	want := []string{"SEあり/01.mp3", "SEあり/01.mp3.vtt"}
	if len(kept) != len(want) {
		t.Fatalf("preview kept %v, want %v", kept, want)
	}
	for i, p := range kept {
		if p != want[i] {
			t.Errorf("kept[%d] = %q, want %q", i, p, want[i])
		}
	}

	// prefer_media 丢弃 wav 双件套、include_ext 丢弃 cover.jpg、exclude_keyword 丢弃 SEなし 双件套
	wantStats := []FilterStat{
		{Rule: "prefer_media", Detail: "mp3", Excluded: 2},
		{Rule: "include_ext", Detail: ".mp3,.mp3.vtt", Excluded: 1},
		{Rule: "exclude_keyword", Detail: "seなし", Excluded: 2},
	}
	if len(stats) != len(wantStats) {
		t.Fatalf("stats = %+v, want %+v", stats, wantStats)
	}
	for i, s := range stats {
		if s != wantStats[i] {
			t.Errorf("stats[%d] = %+v, want %+v", i, s, wantStats[i])
		}
	}
}

func names(urls [][]string) []string {
	out := make([]string, 0, len(urls))
	for _, f := range urls {
		out = append(out, f[2])
	}
	return out
}

func TestEngineManager_AuthLogin(t *testing.T) {
	// Use a relative path from the project root
	configDir := filepath.Join(os.Getenv("HOME"), ".asmroner-data")
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Skip("Config directory not found, skipping integration test")
	}
	_, err := model.LoadConfig(configDir)
	if err != nil {
		t.Fatalf("LoadConfig() failed, err: %v", err)
	}
	manager, err := NewEngineManager(0.5, 1, 200, 400)
	if err != nil {
		t.Fatalf("NewEngineManager() failed, err: %v", err)
	}
	if err := manager.AuthLogin(context.TODO()); err != nil {
		t.Fatalf("AuthLogin() failed, err: %v", err)
	}
	if manager.JWTToken == "" {
		t.Error("AuthLogin() failed, JWTToken is empty")
	}
}
