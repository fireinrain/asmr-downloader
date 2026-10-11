package model

// 音轨
type Track struct {
	Type             string  `json:"type"`
	Title            string  `json:"title"`
	Children         []Track `json:"children,omitempty"`
	Hash             string  `json:"hash,omitempty"`
	WorkTitle        string  `json:"workTitle,omitempty"`
	MediaStreamURL   string  `json:"mediaStreamUrl,omitempty"`
	MediaDownloadURL string  `json:"mediaDownloadUrl,omitempty"`
	// 文件大小（字节数），由 tracks 接口返回，用于完整性与断点续传判断
	Size int64 `json:"size,omitempty"`
}
