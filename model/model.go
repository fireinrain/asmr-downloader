package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// PageResult 每页请求响应结构体
type PageResult struct {
	Works      []Works    `json:"works"`
	Pagination Pagination `json:"pagination"`
}
type Pagination struct {
	CurrentPage int `json:"currentPage"`
	PageSize    int `json:"pageSize"`
	TotalCount  int `json:"totalCount"`
}

type RateCountDetail struct {
	ReviewPoint int `json:"review_point"`
	Count       int `json:"count"`
	Ratio       int `json:"ratio"`
}
type Rank struct {
	Term     string `json:"term"`
	Category string `json:"category"`
	Rank     int    `json:"rank"`
	RankDate string `json:"rank_date"`
}
type Vas struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type EnUs struct {
	Name string `json:"name"`
}
type JaJp struct {
	Name string `json:"name"`
}
type ZhCn struct {
	Name string `json:"name"`
}
type I18N struct {
	EnUs EnUs `json:"en-us"`
	JaJp JaJp `json:"ja-jp"`
	ZhCn ZhCn `json:"zh-cn"`
}
type Tags struct {
	ID   int    `json:"id"`
	I18N I18N   `json:"i18n"`
	Name string `json:"name"`
}
type Circle struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
type Works struct {
	ID                int               `json:"id"`
	Title             string            `json:"title"`
	CircleID          int               `json:"circle_id"`
	Name              string            `json:"name"`
	Nsfw              bool              `json:"nsfw"`
	Release           string            `json:"release"`
	DlCount           int               `json:"dl_count"`
	Price             int               `json:"price"`
	SourceType        string            `json:"source_type"`
	SourceID          string            `json:"source_id"`
	SourceURL         string            `json:"source_url"`
	ReviewCount       int               `json:"review_count"`
	RateCount         int               `json:"rate_count"`
	RateAverage2Dp    float64           `json:"rate_average_2dp"`
	RateCountDetail   []RateCountDetail `json:"rate_count_detail"`
	Rank              []Rank            `json:"rank"`
	HasSubtitle       bool              `json:"has_subtitle"`
	CreateDate        string            `json:"create_date"`
	Vas               []Vas             `json:"vas"`
	Tags              []Tags            `json:"tags"`
	UserRating        interface{}       `json:"userRating"`
	Circle            Circle            `json:"circle"`
	SamCoverURL       string            `json:"samCoverUrl"`
	ThumbnailCoverURL string            `json:"thumbnailCoverUrl"`
	MainCoverURL      string            `json:"mainCoverUrl"`
}

// //////////////////////////////////////// 每页请求响应END ////////////////////////////////////////

// MetaDataStatics
//
//	@Description: 下载统计
type MetaDataStatics struct {
	//总数量
	TotalCount int `json:"total_count"`
	// 含字幕数量
	SubTitleCount int `json:"sub_title_count"`
	// 无字幕数量
	NoSubTitleCount int `json:"no_sub_title_count"`

	// 已下载字幕数量
	SubTitleDownloaded int `json:"sub_title_downloaded"`
	// 无字幕下载数量
	NoSubTitleDownloaded int `json:"no_sub_title_downloaded"`

	// 未下载字幕数量
	SubTitleUnDownloaded int `json:"sub_title_un_downloaded"`

	//无字幕未下载数量
	NoSubTitleUnDownloaded int `json:"no_sub_title_un_downloaded"`

	//已下载作品数量
	HavenDownTotal int `json:"haven_down_total"`

	// 未下载作品数量
	UnDownTotal int `json:"un_down_total"`
}

// DownloadInfoStatics
//
//	DownloadInfoStatics
//	@Description: 下载作品统计信息
type DownloadInfoStatics struct {
	TimeNow                   string `json:"time_now"`
	TotalCount                int    `json:"total_count"`
	SubTitleCount             int    `json:"sub_title_count"`
	SubTitleDownloaded        int    `json:"sub_title_downloaded"`
	NoSubTitleCount           int    `json:"no_sub_title_count"`
	NoSubTitleDownloaded      int    `json:"no_sub_title_downloaded"`
	SubTitleDownloadPercent   string `json:"sub_title_download_percent"`
	NoSubTitleDownloadPercent string `json:"no_sub_title_download_percent"`
	TotalDownPercent          string `json:"total_down_percent"`
}

// GetStaticsInfo
//
//	@Description: 生成统计信息
//	@receiver s
//	@return DownloadInfoStatics
func (s *MetaDataStatics) GetStaticsInfo() *DownloadInfoStatics {
	now := time.Now()
	// Format the time using the standard format string
	currentTime := now.Format("2006-01-02 15:04:05")
	subtitledownloadpercent, _ := strconv.ParseFloat(fmt.Sprintf("%.4f", float64(s.SubTitleDownloaded)/float64(s.SubTitleCount)), 64)
	nosubtitledownloadpercent, _ := strconv.ParseFloat(fmt.Sprintf("%.4f", float64(s.NoSubTitleDownloaded)/float64(s.NoSubTitleCount)), 64)
	totaldownpercent, _ := strconv.ParseFloat(fmt.Sprintf("%.4f", float64(s.SubTitleDownloaded+s.NoSubTitleDownloaded)/float64(s.TotalCount)), 64)

	return &DownloadInfoStatics{
		TimeNow:    currentTime,
		TotalCount: s.TotalCount,

		SubTitleCount:      s.SubTitleCount,
		SubTitleDownloaded: s.SubTitleDownloaded,

		NoSubTitleCount:      s.NoSubTitleCount,
		NoSubTitleDownloaded: s.NoSubTitleDownloaded,

		SubTitleDownloadPercent:   strconv.FormatFloat(subtitledownloadpercent*100, 'f', 2, 64),
		NoSubTitleDownloadPercent: strconv.FormatFloat(nosubtitledownloadpercent*100, 'f', 2, 64),
		TotalDownPercent:          strconv.FormatFloat(totaldownpercent*100, 'f', 2, 64),
	}
}

// PrettyInfoStr
//
//	@Description: 下载统计打印
//	@receiver statics
//	@return string
func (statics *DownloadInfoStatics) PrettyInfoStr() string {
	builder := strings.Builder{}
	builder.WriteString("\n")
	builder.WriteString("----------------------\n")
	builder.WriteString(fmt.Sprintf("当前时间: %s\n", statics.TimeNow))
	builder.WriteString(fmt.Sprintf("作品总数: %d部\n", statics.TotalCount))

	builder.WriteString(fmt.Sprintf("含字幕作品数: %d部\n", statics.SubTitleCount))
	builder.WriteString(fmt.Sprintf("含字幕作品已下载数: %d部\n", statics.SubTitleDownloaded))

	builder.WriteString(fmt.Sprintf("无字幕作品数: %d部\n", statics.NoSubTitleCount))
	builder.WriteString(fmt.Sprintf("无字幕作品已下载数: %d部\n", statics.NoSubTitleDownloaded))

	builder.WriteString(fmt.Sprintf("含字幕作品下载进度: %s%%\n", statics.SubTitleDownloadPercent))
	builder.WriteString(fmt.Sprintf("无字幕作品下载进度: %s%%\n", statics.NoSubTitleDownloadPercent))
	builder.WriteString(fmt.Sprintf("总下载进度: %s%%", statics.TotalDownPercent))
	return builder.String()
}
