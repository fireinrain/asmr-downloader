package utils

import (
	"asmroner/internal/consts"
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// FastFetchResult holds the URL and response time for fastest-site detection.
type FastFetchResult struct {
	URL      string
	Duration string
}

// EnsureDirExist 确保目录存在，不存在则创建
func EnsureDirExist(pathDir string) {
	if _, err := os.Stat(pathDir); os.IsNotExist(err) {
		_ = os.MkdirAll(pathDir, 0755)
	}
}

// FileSize2Byte WantedSize2Byte 将字符串表示的文件大小转换为字节数
func FileSize2Byte(fileSizeStr string) (int64, error) {
	var fileSize int64
	var err error
	switch {
	case strings.HasSuffix(fileSizeStr, "PB"):
		fileSize, err = strconv.ParseInt(fileSizeStr[:len(fileSizeStr)-2], 10, 64)
		fileSize *= 1024 * 1024 * 1024 * 1024 * 1024
	case strings.HasSuffix(fileSizeStr, "TB"):
		fileSize, err = strconv.ParseInt(fileSizeStr[:len(fileSizeStr)-2], 10, 64)
		fileSize *= 1024 * 1024 * 1024 * 1024
	case strings.HasSuffix(fileSizeStr, "GB"):
		fileSize, err = strconv.ParseInt(fileSizeStr[:len(fileSizeStr)-2], 10, 64)
		fileSize *= 1024 * 1024 * 1024
	case strings.HasSuffix(fileSizeStr, "MB"):
		fileSize, err = strconv.ParseInt(fileSizeStr[:len(fileSizeStr)-2], 10, 64)
		fileSize *= 1024 * 1024
	//KB make no sense
	//case strings.HasSuffix(fileSizeStr, "KB"):
	//	fileSize, err = strconv.ParseInt(fileSizeStr[:len(fileSizeStr)-2], 10, 64)
	//	fileSize *= 1024
	default:
		err = errors.New("invalid file size format")
	}
	return fileSize, err
}

func RandomUserAgent(useragents []string) string {
	return useragents[rand.Intn(len(useragents))]
}

// FastFetch fetches a URL and sends it to the channel on success.
// The first URL to arrive in the channel is the fastest.
func FastFetch(url string, wg *sync.WaitGroup, ch chan<- string) {
	defer wg.Done()

	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	ch <- url
}

// IsValidDlsiteID 校验Dlsite ID是否合法
func IsValidDlsiteID(id string) (isValid bool, prefix string, number string, err error) {
	//reg exp
	reg := consts.AsmrOneIDRegex
	//reg := `^RJ[0-9]{6}$`
	match := reg.FindStringSubmatch(id)
	if len(match) == 0 {
		return false, "", "", errors.New("invalid Dlsite ID format")
	}
	prefix = match[0][:2]
	number = match[0][2:]
	return true, prefix, number, nil

}

// NormalDirPathStr 去除可能导致目录创建失败的字符串
func NormalDirPathStr(path string) string {
	for _, str := range []string{"?", "<", ">", ":", "*", "|", " ", "\""} {
		path = strings.ReplaceAll(path, str, "_")
	}
	return strings.TrimSpace(path)
}

func FilterList[T any](list []T, keep func(T) bool) []T {
	result := make([]T, 0, len(list))
	for _, v := range list {
		if keep(v) {
			result = append(result, v)
		}
	}
	return result
}

func ListContains[T comparable](list []T, target T) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}

// RemoveEmptyDirs 递归移除目录中的所有空目录
func RemoveEmptyDirs(name string) {
	// 递归遍历目录
	entries, err := os.ReadDir(name)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// 递归处理子目录
			RemoveEmptyDirs(filepath.Join(name, entry.Name()))
		}
	}

	// 检查目录是否为空
	entries, err = os.ReadDir(name)
	if err != nil {
		return
	}
	if len(entries) == 0 {
		// 移除空目录
		os.Remove(name)
	}
}

// TruncateRune 截断字符串，保留前n个字符并添加省略号
func TruncateRune(s string, n int) string {
	runes := []rune(s)
	if len(runes) > n {
		return string(runes[:n]) + "..."
	}
	return s
}

// NextlineRune 在字符串中插入换行符
func NextlineRune(s string, n int) string {
	runes := []rune(s)
	if len(runes) > n {
		return string(runes[:n]) + "\n" + string(runes[n:])
	}
	return s
}

// ensureDirExists 确保目标文件所在目录存在，不存在则自动创建
func ensureDirExists(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0755)
}

// ExportToJSON 导出结构体切片到 JSON 文件
func ExportToJSON(data any, path string) error {
	if err := ensureDirExists(path); err != nil {
		return err
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, b, 0644)
}

// ExportToCSV 导出结构体切片到 CSV 文件
func ExportToCSV(data any, path string) error {
	if err := ensureDirExists(path); err != nil {
		return err
	}
	v := reflect.ValueOf(data)
	if v.Kind() != reflect.Slice {
		return fmt.Errorf("ExportToCSV: data must be slice")
	}
	if v.Len() == 0 {
		return fmt.Errorf("ExportToCSV: slice empty")
	}

	elemType := v.Index(0).Type()
	if elemType.Kind() != reflect.Struct {
		return fmt.Errorf("ExportToCSV: slice element must be struct")
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)

	// 写表头
	headers := make([]string, elemType.NumField())
	for i := 0; i < elemType.NumField(); i++ {
		f := elemType.Field(i)
		name := f.Tag.Get("json")
		if name == "" {
			name = f.Name
		}
		headers[i] = name
	}
	writer.Write(headers)

	// 写行数据
	for i := 0; i < v.Len(); i++ {
		item := v.Index(i)
		row := make([]string, elemType.NumField())
		for j := 0; j < elemType.NumField(); j++ {
			row[j] = fmt.Sprintf("%v", item.Field(j).Interface())
		}
		writer.Write(row)
	}

	writer.Flush()
	return writer.Error()
}

// PromptConfirm 提示用户确认操作
func PromptConfirm(message string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [y/n]: ", message)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))
	return strings.ToLower(response) == "y"
}

// Byte2FileSize 将字节数转换为人类可读的文件大小字符串
func Byte2FileSize(size int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	f := float64(size)
	for _, unit := range units {
		if f < 1024 {
			return fmt.Sprintf("%.2f %s", f, unit)
		}
		f /= 1024
	}
	return fmt.Sprintf("%.2f %s", f, units[len(units)-1])
}

// GetDirSize 递归计算目录大小（包含子目录）
func GetDirSize(path string) (int64, error) {
	var size int64
	err := filepath.WalkDir(path, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			fileInfo, err := d.Info()
			if err != nil {
				return err
			}
			size += fileInfo.Size()
		}
		return nil
	})
	return size, err
}
