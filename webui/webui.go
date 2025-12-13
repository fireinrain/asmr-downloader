package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
)

// 内嵌整个public目录
//
//go:embed public/*
//go:embed public/css/*
//go:embed public/js/*
var publicAssets embed.FS

// GetFileSystem 返回一个可以被Gin使用的文件系统
func GetFileSystem() http.FileSystem {
	// 创建一个子文件系统，将public目录作为根目录
	f, err := fs.Sub(publicAssets, "public")
	if err != nil {
		panic(err)
	}
	return http.FS(f)
}

// GetFileContent 直接获取文件内容
func GetFileContent(filePath string) ([]byte, error) {
	// 在路径前添加public前缀，因为我们的内嵌根目录是public
	fullPath := path.Join("public", filePath)
	return publicAssets.ReadFile(fullPath)
}
