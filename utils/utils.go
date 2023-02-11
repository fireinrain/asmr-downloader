package utils

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"github.com/melbahja/got"
	"github.com/xxjwxc/gowp/workpool"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const FailedDownloadFileName = "failed-download.txt"

var FailedDownloadFile *os.File

func init() {
	f, err := os.OpenFile(FailedDownloadFileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("错误日志文件创建失败: ", err.Error())
	}
	FailedDownloadFile = f
}

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
		fileClient := got.New()
		err := fileClient.Download(fileUrl, storePath)

		if err != nil {
			fmt.Println(err)
			//fmt.Printf("文件: %s下载失败: %s\n", fileName, fileUrl)
			fmt.Printf("文件: %s下载失败: %s\n", fileName, err.Error())
			//记录失败文件  时间, 文件路径，文件url
			logStr := GetCurrentDateTime() + "," + storePath + "," + fileUrl + "\n"
			write := bufio.NewWriter(FailedDownloadFile)
			_, _ = write.WriteString(logStr)
			//Flush将缓存的文件真正写入到文件中
			write.Flush()
			//清理下载失败的文件碎片
			err2 := os.Remove(storePath)
			if err2 != nil {
				fmt.Println("删除碎片文件失败文件失败:", err2)
			}
		} else {
			fmt.Println("文件下载成功: ", fileName)
			//fmt.Println("文件下载成功: ", filePathToStore)
		}
		return nil
	}

}

// GetCurrentDateTime
//
//	@Description: 获取当前时间
//	@return string
func GetCurrentDateTime() string {
	now := time.Now()
	// Format the time using the standard format string
	currentTimeStr := now.Format("2006-01-02 15:04:05")
	return currentTimeStr
}

// NewFixFileDownloader
//
//	 下载上一次循环下载出错的文件
//		@Description: 下载
//		@param url
//		@param storePath
//		@param resultLines
//		@return []string
//		@return error
func NewFixFileDownloader(url string, storePath string, resultLines []string) ([]string, error) {
	//确保路径存在
	exists := FileOrDirExists(storePath)
	if !exists {
		dir := filepath.Dir(storePath)
		err := os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			fmt.Printf("自动创建上一次下载失败文件目录失败: %s\n", err)
			return nil, nil
		}
	}
	var fileUrl = url
	fileClient := got.New()
	err := fileClient.Download(fileUrl, storePath)
	//err := errors.New("")

	if err != nil {
		fmt.Println(err)
		//fmt.Printf("文件: %s下载失败: %s\n", fileName, fileUrl)
		fmt.Printf("文件: %s下载失败: %s\n", storePath, err.Error())
		//记录失败文件  时间, 文件路径，文件url
		logStr := GetCurrentDateTime() + "," + storePath + "," + fileUrl
		resultLines = append(resultLines, logStr)
	} else {
		fmt.Println("文件下载成功: ", storePath)
		//fmt.Println("文件下载成功: ", filePathToStore)
	}
	return resultLines, nil

}

// FixBrokenDownloadFile
//
//	@Description: 以最大重试方式修复下载出错的文件
//	@param maxRetry
func FixBrokenDownloadFile(maxRetry int) {
	fmt.Println("正在自动处理下载失败的媒体文件,请稍后...")
	//复制下载出错的日志文件
	var FailedDownloadFileNameTemp = FailedDownloadFileName + ".tmp"
	err := CopyFile(FailedDownloadFileName, FailedDownloadFileName+".tmp")
	if err != nil {
		fmt.Printf("复制文件: %s失败: %s\n", FailedDownloadFileName, err.Error())
		return
	}
	fi, err := os.Open(FailedDownloadFileNameTemp)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}
	defer fi.Close()

	br := bufio.NewReader(fi)
	var resultLine = []string{}
	for {
		line, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}
		if len(strings.Trim(string(line), "\r\n")) > 0 {
			resultLine = append(resultLine, string(line))
		}
	}
	var resultContainer = []string{}
	for i := 0; i < maxRetry; i++ {
		for index, brokenLine := range resultLine {
			fmt.Printf("index: %d,line: %s\n", index, brokenLine)
			fileInfos := strings.Split(brokenLine, ",")
			downloader, _ := NewFixFileDownloader(fileInfos[2], fileInfos[1], resultContainer)
			resultContainer = downloader
		}
		if len(resultContainer) <= 0 {
			break
		}
		fmt.Printf("重试下载文件再次出错,重试中(剩余重试次数: %d)...\n", maxRetry-i-1)
	}
	//删除temp文件
	err2 := os.Remove(FailedDownloadFileNameTemp)
	if err2 != nil {
		fmt.Println("删除临时文件失败:", err2)
		return
	}
	//清理文件
	err = FailedDownloadFile.Truncate(0)
	if err != nil {
		fmt.Println("清空下载失败日志文件失败:", err)
		return
	}
	fmt.Println("重试下载失败媒体文件已处理完成!")

}

// CheckIfNeedFixBrokenDownloadFile
// CheckIfNeedFixBroken
//
//	@Description: 检测是否需要修复下载出错的文件
//	@return bool
func CheckIfNeedFixBrokenDownloadFile() bool {
	file, err := os.OpenFile(FailedDownloadFileName, os.O_RDONLY, 0666)
	defer file.Close()
	if err != nil {
		fmt.Printf("打开文件失败: %s\n", err.Error())
		return false
	}
	br := bufio.NewReader(file)
	var resultLine = []string{}
	for {
		line, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}
		if len(strings.Trim(string(line), "\r\n")) > 0 {
			resultLine = append(resultLine, string(line))
		}
	}
	if len(resultLine) == 0 {
		return false
	}
	return true
}

// CopyFile
//
//	@Description: 复制文件
//	@param src
//	@param dst
//	@return err
func CopyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if e := out.Close(); e != nil {
			err = e
		}
	}()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	err = out.Sync()
	if err != nil {
		return err
	}

	si, err := os.Stat(src)
	if err != nil {
		return err
	}
	err = os.Chmod(dst, si.Mode())
	if err != nil {
		return err
	}
	return err
}
