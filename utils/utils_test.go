package utils

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
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

func TestGetRapidRespSiteUrl(t *testing.T) {
	//访问asmr.one 最新域名发布页
	// official : https://as.mr
	// cf worker proxy: https://as.131433.xyz
	var officialPublishSite = "https://as.mr"
	var cfProxyPublishSite = "https://as.131433.xyz"
	var latestPublishSite = ""
	client := Client.Get().(*http.Client)
	req, _ := http.NewRequest("GET", "https://as.mr", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.198 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		println("尝试访问asmr.one最新站点发布页as.mr失败: ", err.Error())
		println("当前使用as.131433.xyz访问")
		latestPublishSite = cfProxyPublishSite
	} else {
		println("当前使用as.131433.xyz访问...")
		latestPublishSite = officialPublishSite
	}
	Client.Put(client)
	defer resp.Body.Close()

	client = Client.Get().(*http.Client)
	req, _ = http.NewRequest("GET", latestPublishSite, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.198 Safari/537.36")
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		println("访问asmr.one最新域名发布页出现错误: ", err.Error())
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}

	// Convert the response body to a string and print it
	bodyText := string(body)
	fmt.Println("Response Text:", bodyText)
	Client.Put(client)

	pattern := `<script type="module" crossorigin src="(/assets/index\.[a-f0-9]+\.js)"></script>`
	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(bodyText)

	var jsFilePath = ""
	if len(match) > 1 {
		jsFilePath = match[1]
		fmt.Println("JavaScript file path:", jsFilePath)
	} else {
		fmt.Println("JavaScript file path not found.")
	}

	jsContentUrl := latestPublishSite + jsFilePath
	client = Client.Get().(*http.Client)
	req, _ = http.NewRequest("GET", jsContentUrl, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/86.0.4240.198 Safari/537.36")
	resp, err = client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		println("访问asmr.one最新域名发布页js resource出现错误: ", err.Error())
		return
	}
	defer resp.Body.Close()
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}
	jsText := string(body)
	fmt.Println("Response Text:", jsText)

	//extract rapid resp site url from js file text
	sitePattern := `link:\s*"([^"]+)"`
	re = regexp.MustCompile(sitePattern)
	matches := re.FindAllStringSubmatch(jsText, -1)
	result := []string{}
	for _, match := range matches {
		if len(match) > 1 {
			link := match[1]
			if strings.HasPrefix("https://", link) {
				result = append(result, link)
			}
			//fmt.Println("Link:", link)
		}
	}
}
