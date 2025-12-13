package main

var (
	version   = "v1.0.0"           // 程序版本，可在 build 时通过 ldflags 替换
	buildTime = "2025-12-08 20:00" // 构建时间，可在 build 时替换
	author    = "fireinrain"       // 开发者
)

//编译时设置变量
//go build -ldflags "-X 'main.version=v1.2.2' -X 'main.buildTime=$(date +%Y-%m-%d_%H:%M)'" -o asmroner
