package utils

import (
	"bufio"
	"os"
	"testing"
)

func TestCalculatePage(t *testing.T) {
	CalculateMaxPage(10, 23)
}

func TestWriteErrorFile(t *testing.T) {
	f, err := os.OpenFile("test.txt", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	defer f.Close()
	if err != nil {
		println(err)
	}
	f.WriteString("success")

	write := bufio.NewWriter(f)
	for i := 0; i < 5; i++ {
		write.WriteString("http://c.biancheng.net/golang/ \n")
	}
	//Flush将缓存的文件真正写入到文件中
	write.Flush()

}

func TestFixBrokenDownloadFile(t *testing.T) {
	FixBrokenDownloadFile(3)
}
