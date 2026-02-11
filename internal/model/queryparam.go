package model

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
)

// 查询参数解析
type QueryParams struct {
	//原始查询字符串
	QueryStr   string      `json:"queryStr"`
	PlainTexts []string    `json:"plainTexts"`
	SearchPair *SearchPair `json:"searchPair"`
	PageInfo   *PageInfo   `json:"pageInfo"`
	HasParsed  bool        `json:"hasParsed"`
}

type SearchPair struct {
	// $va:床貓$ $duration:40m$ $sell:100$ 下面的字段都支持反选

	//搜索标签
	Tag string `json:"tag"`
	//搜索社团
	Circle string `json:"circle"`
	//搜索声优
	Va string `json:"va"`
	//筛选作品时长 大于 10M/10H等
	Duration string `json:"duration"`
	//筛选评分 大于
	Rate string `json:"rate"`
	//筛选价格 大于/小于 1000等
	Price string `json:"price"`
	//筛选销量 大于
	Sell string `json:"sell"`
	//筛选年龄 大于/小于 18等
	Age string `json:"age"`
	//筛选语言
	Lang string `json:"lang"`
}

type PageInfo struct {
	//order=dl_count&sort=desc&page=1&pageSize=20&subtitle=0&includeTranslationWorks=true
	//想要的条目
	//排序种类
	Order string `json:"order"`
	//排序方式
	Sort string `json:"sort"`
	//是否包含字幕文件
	Subtitle string `json:"subtitle"`
	//是否包含翻译作品
	IncludeTranslationWorks bool `json:"includeTranslationWorks"`
	Page                    int  `json:"page"`
	PageSize                int  `json:"size"`
	Count                   int  `json:"count"`
}

func NewQueryParams(rawQueryStr string) *QueryParams {
	return &QueryParams{
		QueryStr:   rawQueryStr,
		SearchPair: nil,
		PageInfo: &PageInfo{
			Order: "release",
			//order 包含:
			//release 发售时间倒序
			//dl_count 下载量倒序
			//create_date 创建时间倒序
			//rating 我的评价
			//price 价格
			//rate_average_2dp 评分
			//review_count 评论数
			//id RJid
			//nsfw 全年龄向排序
			Sort:                    "desc",
			Subtitle:                "0",
			IncludeTranslationWorks: true,
			Page:                    1,
			PageSize:                20,
			Count:                   20,
		},
		HasParsed: false,
	}
}

func (p *QueryParams) ParseQueryStr() error {
	if p.QueryStr == "" {
		return errors.New("empty query string")
	}
	queryStr := p.QueryStr

	//  修女,洗脑,-触手
	//  $tag:内射/中出$ $circle:青春×フェティシズム$ $va:陽向葵ゅか$ $duration:1h$ $rate:4.75$ $price:1000$ $sell:700$ $age:adult$ $-lang:JPN$?order=dl_count&sort=desc&page=1&pageSize=20&subtitle=0&includeTranslationWorks=true
	// 	tag:内射/中出,circle:青春×フェティシズム,va:陽向葵ゅか,duration:1h,rate:4.75,-price:1000,sell:700,age:adult,-lang:JPN
	//	order=dl_count&sort=desc&page=1&pageSize=20&subtitle=0&includeTranslationWorks=true

	//完整的查询
	//修女,洗脑,-触手@tag:内射/中出,circle:青春×フェティシズム,va:陽向葵ゅか,duration:1h,rate:4.75,-price:1000,sell:700,age:adult,-lang:JPN?order=dl_count&sort=desc&page=1&pageSize=20&subtitle=0&includeTranslationWorks=true

	if strings.Contains(queryStr, "@") {
		// 解析普通文本
		plainText := queryStr[:strings.Index(queryStr, "@")]
		plainTexts := parsePlainText(plainText)
		p.PlainTexts = plainTexts
		queryStr = strings.ReplaceAll(queryStr, plainText+"@", "")

	}
	if strings.Contains(queryStr, "?") {
		searchPairStr := queryStr[:strings.Index(queryStr, "?")]
		searchPair := parseSearchPair(searchPairStr)
		p.SearchPair = &searchPair
		queryStr = strings.ReplaceAll(queryStr, searchPairStr+"?", "")

	}
	split := strings.Split(queryStr, "?")
	searchPairStr := split[0]
	if searchPairStr != "" {
		searchPair := parseSearchPair(searchPairStr)
		p.SearchPair = &searchPair
	}
	if len(split) > 1 {
		pageInfo := parsePageInfo(split[1])
		p.PageInfo = &pageInfo
	}
	if len(p.PlainTexts) == 0 {
		p.PlainTexts = []string{queryStr}
	}

	// 标记为已解析
	p.HasParsed = true

	return nil
}

func (p *QueryParams) BuildAsmrOneQueryStr() (string, error) {
	if !p.HasParsed {
		return "", errors.New("query params not parsed")
	}
	builder := strings.Builder{}
	// 构建普通文本部分
	if len(p.PlainTexts) > 0 {
		plainText := " " + strings.Join(p.PlainTexts, " ")
		builder.WriteString(plainText)
	}
	if p.SearchPair != nil {
		// 构建搜索参数
		if p.SearchPair.Tag != "" {
			builder.WriteString(" " + "$" + p.SearchPair.Tag + "$")
		}
		if p.SearchPair.Circle != "" {
			builder.WriteString(" " + "$" + p.SearchPair.Circle + "$")
		}
		if p.SearchPair.Va != "" {
			builder.WriteString(" " + "$" + p.SearchPair.Va + "$")
		}
		if p.SearchPair.Duration != "" {
			builder.WriteString(" " + "$" + p.SearchPair.Duration + "$")
		}
		if p.SearchPair.Rate != "" {
			builder.WriteString(" " + "$" + p.SearchPair.Rate + "$")
		}
		if p.SearchPair.Price != "" {
			builder.WriteString(" " + "$" + p.SearchPair.Price + "$")
		}
		if p.SearchPair.Sell != "" {
			builder.WriteString(" " + "$" + p.SearchPair.Sell + "$")
		}
		if p.SearchPair.Age != "" {
			builder.WriteString(" " + "$" + p.SearchPair.Age + "$")
		}
		if p.SearchPair.Lang != "" {
			builder.WriteString(" " + "$" + p.SearchPair.Lang + "$")
		}
	}
	s := builder.String()
	encodedUrl := url.QueryEscape(s)
	encodedUrl = strings.ReplaceAll(encodedUrl, "+", "%20")

	s2 := strings.Builder{}
	if p.PageInfo != nil {
		// 构建分页参数
		s2.WriteString("?order=" + p.PageInfo.Order)
		s2.WriteString("&sort=" + p.PageInfo.Sort)
		s2.WriteString("&page=" + strconv.Itoa(p.PageInfo.Page))
		s2.WriteString("&pageSize=" + strconv.Itoa(p.PageInfo.PageSize))
		s2.WriteString("&subtitle=" + p.PageInfo.Subtitle)
		s2.WriteString("&includeTranslationWorks=" + strconv.FormatBool(p.PageInfo.IncludeTranslationWorks))
	}
	return encodedUrl + s2.String(), nil
}

func parsePlainText(s string) []string {
	// 按逗号分隔
	items := strings.Split(s, ",")
	// 移除首尾空格
	for i, item := range items {
		items[i] = strings.TrimSpace(item)
	}
	return items
}

// ---------------------------------------
// 解析搜索部分（tag/circle/va/...）
// ---------------------------------------
func parseSearchPair(s string) SearchPair {
	pair := SearchPair{}

	if s == "" {
		return pair
	}

	items := strings.Split(s, ",")
	for _, item := range items {
		// 保持原样，不拆 key/value，只识别 key 对应字段
		switch {
		case strings.HasPrefix(item, "tag:") || strings.HasPrefix(item, "-tag:"):
			pair.Tag = item
		case strings.HasPrefix(item, "circle:") || strings.HasPrefix(item, "-circle:"):
			pair.Circle = item
		case strings.HasPrefix(item, "va:") || strings.HasPrefix(item, "-va:"):
			pair.Va = item
		case strings.HasPrefix(item, "duration:") || strings.HasPrefix(item, "-duration:"):
			pair.Duration = item
		case strings.HasPrefix(item, "rate:") || strings.HasPrefix(item, "-rate:"):
			pair.Rate = item
		case strings.HasPrefix(item, "price:") || strings.HasPrefix(item, "-price:"):
			pair.Price = item // 保留原样：price:100 / -price:1000
		case strings.HasPrefix(item, "sell:") || strings.HasPrefix(item, "-sell:"):
			pair.Sell = item
		case strings.HasPrefix(item, "age:") || strings.HasPrefix(item, "-age:"):
			pair.Age = item
		case strings.HasPrefix(item, "lang:") || strings.HasPrefix(item, "-lang:"):
			pair.Lang = item // 保留原样：lang:JPN / -lang:JPN
		}
	}

	return pair
}

// ---------------------------------------
// 解析分页 PageInfo 部分
// ---------------------------------------
func parsePageInfo(q string) PageInfo {
	m, _ := url.ParseQuery(q)
	pi := PageInfo{}

	pi.Order = m.Get("order")
	pi.Sort = m.Get("sort")
	pi.Subtitle = m.Get("subtitle")
	pi.IncludeTranslationWorks = m.Get("includeTranslationWorks") == "true"

	pi.Page, _ = strconv.Atoi(m.Get("page"))
	pi.PageSize, _ = strconv.Atoi(m.Get("pageSize"))

	return pi
}
