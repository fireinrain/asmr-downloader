package model

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

// TestLoadConfig_FolderNameFormat 验证 config.toml 中的 folder_name_format
// 能通过 viper.Unmarshal 正确映射到 Downloader.FolderNameFormat 字段。
// 防止 viper.Set 写入的 key 与 mapstructure tag 不一致导致的静默失效。
func TestLoadConfig_FolderNameFormat(t *testing.T) {
	// viper 是全局单例，每个用例前必须 Reset 清空搜索路径与缓存
	viper.Reset()
	dir := t.TempDir()
	content := `user = { account = "guest", password = "guest" }

[downloader]
folder_name_format = '{rjid}-{subtitle}'
prefer_media = "all"

[limit]
sync_qps = 2.0
`
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	config, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if config.Downloader.FolderNameFormat != "{rjid}-{subtitle}" {
		t.Errorf("FolderNameFormat = %q, want {rjid}-{subtitle}", config.Downloader.FolderNameFormat)
	}
	if config.Downloader.PreferMedia != "all" {
		t.Errorf("PreferMedia = %q, want all (相邻字段回归检查)", config.Downloader.PreferMedia)
	}

	// 旧用户配置没有该 key 时应为零值，由 BuildFolderName 回退默认格式
	viper.Reset()
	dir2 := t.TempDir()
	content2 := "[downloader]\nprefer_media = \"all\"\n"
	if err := os.WriteFile(filepath.Join(dir2, "config.toml"), []byte(content2), 0644); err != nil {
		t.Fatal(err)
	}
	config2, err := LoadConfig(dir2)
	if err != nil {
		t.Fatalf("LoadConfig(minimal) failed: %v", err)
	}
	if config2.Downloader.FolderNameFormat != "" {
		t.Errorf("missing key should yield empty string, got %q", config2.Downloader.FolderNameFormat)
	}
}
