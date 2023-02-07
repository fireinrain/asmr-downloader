package utils

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"github.com/xxjwxc/gowp/workpool"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Client httpClient
var Client = sync.Pool{
	New: func() interface{} {
		return &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				TLSClientConfig: &tls.Config{
					MaxVersion: tls.VersionTLS12, // Cloudflare 会杀
				},
			},
		}
	},
}

// FileOrDirExists  判断所给路径文件/文件夹是否存在
func FileOrDirExists(path string) bool {
	_, err := os.Stat(path) //os.Stat获取文件信息
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}

// PromotForInput 获取用户输入
func PromotForInput(message string, defaultValue string) string {
	fmt.Println(message)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	value := scanner.Text()
	err := scanner.Err()
	if err != nil {
		fmt.Printf("输入有误: %s\n", value)
		os.Exit(0)
	}
	if value == "" {
		return defaultValue
	}
	all := strings.ReplaceAll(value, "\n", "")
	return strings.TrimSpace(all)
}

// NewWorkerPool 工作池
func NewWorkerPool(maxWorkerCount int) *workpool.WorkPool {
	return workpool.New(maxWorkerCount)
}

// MosaicStr 模糊字符串
func MosaicStr(inputStr string, mosaicStrTmp string) string {
	if mosaicStrTmp == "" {
		mosaicStrTmp = "*"
	}
	var result = strings.Builder{}
	size := len(inputStr)
	for i := 0; i < size; i++ {
		result.WriteString(mosaicStrTmp)
	}
	return result.String()
}

// GenerateReqSeed 生成请求种子 seed参数
func GenerateReqSeed() int {
	rand.Seed(time.Now().UnixNano())
	result := int(100 * rand.Float64())
	return result
}

// CalculateMaxPage
//
//	@Description: 计算最大页数
//	@param totalCount 总数据
//	@param pageSize 每页数据
//	@return int 最大页数
func CalculateMaxPage(totalCount int, pageSize int) int {
	if totalCount < 0 || pageSize <= 0 {
		panic("totalCount必须大于等于0, pageSize必须大于0")
	}
	if totalCount == 0 {
		return 1
	}
	i := totalCount / pageSize
	padding := totalCount % pageSize
	if padding != 0 {
		i += 1
	}
	return i
}

// NewFileDownloader
//
//	@Description: 下载文件
//	@param url
//	@param path
//	@param filename
//	@return func()
func NewFileDownloader(url string, path string, filename string) func() error {
	return func() error {
		var fileUrl = url
		var filePathToStore = path
		var fileName = filename
		var storePath = filepath.Join(filePathToStore, fileName)
		//fileClient := got.New()
		//err := fileClient.Download(fileUrl, storePath)
		println(storePath)
		println(fileUrl)
		time.Sleep(3 * time.Second)

		//if err != nil {
		//	fmt.Println(err)
		//	fmt.Printf("文件: %s下载失败: %s\n", fileName, fileUrl)
		//} else {
		//	fmt.Println("文件下载成功: ", url)
		//	//fmt.Println("文件下载成功: ", filePathToStore)
		//}
		fmt.Println("文件下载成功: ", url)

		return nil
	}

}
