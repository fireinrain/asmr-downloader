package utils

import "testing"

// TestSanitizeFileName 表驱动测试：路径分隔符移除、非法字符替换、空白修剪
func TestSanitizeFileName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty input", "", ""},
		{"illegal chars replaced", `a?b*c<d>`, "a_b_c_d_"},
		{"colon replaced", "a:b", "a_b"},
		{"both separators removed", `dir\sub/name`, "dirsubname"},
		{"spaces become underscores", " a b ", "_a_b_"},
		{"only illegal chars", `<>?:*|"`, "_______"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeFileName(tt.in); got != tt.want {
				t.Errorf("SanitizeFileName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestBuildFolderName 表驱动测试：覆盖默认格式兼容、自定义格式与净化逻辑
func TestBuildFolderName(t *testing.T) {
	tests := []struct {
		name        string
		format      string
		workID      string
		release     string
		hasSubtitle bool
		title       string
		want        string
	}{
		{
			// 默认格式必须与历史硬编码 Sprintf 输出逐字节一致
			name:        "empty format uses legacy default",
			format:      "",
			workID:      "RJ01037721",
			release:     "2023-04-01",
			hasSubtitle: false,
			title:       "タイトル Test/Work",
			want:        "RJ01037721-20230401-nosub-タイトル_TestWork",
		},
		{
			name:        "subtitle true",
			format:      "",
			workID:      "RJ01037721",
			release:     "2023-04-01",
			hasSubtitle: true,
			title:       "Work",
			want:        "RJ01037721-20230401-sub-Work",
		},
		{
			name:        "rjid only (issue #46)",
			format:      "{rjid}",
			workID:      "RJ01037721",
			release:     "2023-04-01",
			hasSubtitle: false,
			title:       "Some Long Title",
			want:        "RJ01037721",
		},
		{
			name:        "custom order",
			format:      "{subtitle}_{date}_{title}-{rjid}",
			workID:      "RJ123",
			release:     "2023-04-01",
			hasSubtitle: true,
			title:       "T",
			want:        "sub_20230401_T-RJ123",
		},
		{
			name:        "forbidden char in format sanitized",
			format:      "{rjid}|{title}",
			workID:      "RJ123",
			release:     "2023-04-01",
			hasSubtitle: false,
			title:       "T",
			want:        "RJ123_T",
		},
		{
			name:        "slash in format produces no nested dir",
			format:      "{rjid}/{title}",
			workID:      "RJ123",
			release:     "2023-04-01",
			hasSubtitle: false,
			title:       "T",
			want:        "RJ123T",
		},
		{
			name:        "backslash in format produces no nested dir",
			format:      `{rjid}\{title}`,
			workID:      "RJ123",
			release:     "2023-04-01",
			hasSubtitle: false,
			title:       "T",
			want:        "RJ123T",
		},
		{
			name:        "unknown placeholder preserved",
			format:      "{rjid}-{unknown}",
			workID:      "RJ123",
			release:     "2023-04-01",
			hasSubtitle: false,
			title:       "T",
			want:        "RJ123-{unknown}",
		},
		{
			name:        "empty release keeps legacy double dash",
			format:      "",
			workID:      "RJ123",
			release:     "",
			hasSubtitle: false,
			title:       "T",
			want:        "RJ123--nosub-T",
		},
		{
			name:        "whitespace-only format falls back to default",
			format:      "   ",
			workID:      "RJ123",
			release:     "2023-04-01",
			hasSubtitle: false,
			title:       "T",
			want:        "RJ123-20230401-nosub-T",
		},
		{
			name:        "lowercase id uppercased",
			format:      "{rjid}",
			workID:      "rj01037721",
			release:     "2023-04-01",
			hasSubtitle: false,
			title:       "T",
			want:        "RJ01037721",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildFolderName(tt.format, tt.workID, tt.release, tt.hasSubtitle, tt.title)
			if got != tt.want {
				t.Errorf("BuildFolderName(%q, %q, %q, %v, %q) = %q, want %q",
					tt.format, tt.workID, tt.release, tt.hasSubtitle, tt.title, got, tt.want)
			}
		})
	}
}
