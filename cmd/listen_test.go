package cmd

import "testing"

// TestParseFolderName 表驱动测试：验证解析器兼容旧格式与新命名格式
func TestParseFolderName(t *testing.T) {
	legacy := "RJ01037721-20230401-sub-标题"
	rjidOnly := "RJ01037721"

	// 旧格式解析
	mediaId, date, subtitle, hasSub, title, ok := parseFolderName(legacy)
	if !ok {
		t.Fatalf("legacy format should parse ok")
	}
	if mediaId != "RJ01037721" {
		t.Errorf("mediaId = %q, want RJ01037721", mediaId)
	}
	if date != "20230401" {
		t.Errorf("date = %q, want 20230401", date)
	}
	if subtitle != "sub" {
		t.Errorf("subtitle = %q, want sub", subtitle)
	}
	if hasSub != true {
		t.Errorf("hasSub = %v, want true", hasSub)
	}
	if title != "标题" {
		t.Errorf("title = %q, want 标题", title)
	}

	// {rjid}-only 格式（issue #46 的目标场景）
	mediaId, date, subtitle, hasSub, title, ok = parseFolderName(rjidOnly)
	if !ok {
		t.Fatalf("rjid-only format should parse ok")
	}
	if mediaId != "RJ01037721" {
		t.Errorf("mediaId = %q, want RJ01037721", mediaId)
	}
	if date != "" {
		t.Errorf("date = %q, want empty", date)
	}
	if subtitle != "" {
		t.Errorf("subtitle = %q, want empty", subtitle)
	}
	if title != "" {
		t.Errorf("title = %q, want empty", title)
	}

	// 含 - 的标题不再被截断（旧解析器的 bug）
	mediaId, date, subtitle, hasSub, title, ok = parseFolderName("RJ01037721-20230401-nosub-星降る夜の-家出娘")
	if !ok {
		t.Fatalf("dashed title should parse ok")
	}
	if title != "星降る夜の-家出娘" {
		t.Errorf("dashed title = %q, want 星降る夜の-家出娘", title)
	}
	if date != "20230401" {
		t.Errorf("dashed date = %q, want 20230401", date)
	}
	if subtitle != "nosub" {
		t.Errorf("dashed subtitle = %q, want nosub", subtitle)
	}

	// 非 ID 目录应被跳过
	_, _, _, _, _, ok = parseFolderName("SomeOtherDir")
	if ok {
		t.Errorf("non-ID dir should be skipped")
	}
}

// TestFullExt 表驱动测试：验证多级后缀提取（含泛字幕 .vtt 的嵌套展示）
func TestFullExt(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"01.mp3.vtt", ".mp3.vtt"}, // 嵌套字幕后缀完整展示
		{"01.wav.vtt", ".wav.vtt"}, // wav 的字幕同理
		{"01.mp3", ".mp3"},         // 普通后缀
		{"cover.jpg", ".jpg"},      // 图片
		{"Track 1.5.mp3", ".mp3"},  // 文件名含点号时只取最后一段
		{"01.vtt", ".vtt"},         // 纯字幕无前置后缀
		{"おまけ", "(无后缀)"},           // 无点号
		{"01.MP3.VTT", ".mp3.vtt"}, // 大小写不敏感
	}
	for _, c := range cases {
		if got := fullExt(c.name); got != c.want {
			t.Errorf("fullExt(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}
