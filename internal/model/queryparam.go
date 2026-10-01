package model

import (
	"errors"
	"fmt"
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
			//order 可选值:
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

// 严格语法: 关键词,排除词@过滤条件?分页参数
//
//	修女,-触手@tag:内射/中出,va:陽向葵ゅか,duration:1h,-price:1000?order=dl_count&sort=desc
//
// 规则:
//   - 三段用 @ 和 ? 显式分隔，段内多值一律逗号分隔
//   - 过滤条件必须写在 @ 之后；- 前缀表示排除/反选
//   - 同一过滤条件/分页参数出现多次、出现未知 key，均直接报错
func (p *QueryParams) ParseQueryStr() error {
	if strings.TrimSpace(p.QueryStr) == "" {
		return errors.New("查询字符串为空")
	}
	queryStr := p.QueryStr

	// 先拆分页参数: ...?order=...&sort=...
	pagePart := ""
	if i := strings.Index(queryStr, "?"); i >= 0 {
		pagePart = queryStr[i+1:]
		queryStr = queryStr[:i]
	}

	// 再拆普通文本与搜索对: 普通文本@搜索对
	plainPart := queryStr
	pairPart := ""
	if i := strings.Index(queryStr, "@"); i >= 0 {
		plainPart = queryStr[:i]
		pairPart = queryStr[i+1:]
	}

	// 关键词部分: 全部作为普通文本，但识别为过滤条件的 token 要求写在 @ 之后
	if plainTokens := splitTokens(plainPart); len(plainTokens) > 0 {
		for _, token := range plainTokens {
			if hasSearchPairPrefix(token) {
				return fmt.Errorf("过滤条件 %q 必须写在 @ 之后，例如: 关键词@%s", token, token)
			}
		}
		p.PlainTexts = plainTokens
	}

	// 过滤条件部分: 每个 token 必须是已识别的 key 前缀，同一条件不允许重复
	if strings.Contains(pairPart, "@") {
		return errors.New("过滤条件中存在多余的 @，只允许一个 @ 分隔关键词与过滤条件")
	}
	if pairTokens := splitTokens(pairPart); len(pairTokens) > 0 {
		searchPair, err := parseSearchPair(pairTokens)
		if err != nil {
			return err
		}
		p.SearchPair = searchPair
	}

	if pagePart != "" {
		// 未指定的字段保留 NewQueryParams 的默认值
		if err := parsePageInfo(pagePart, p.PageInfo); err != nil {
			return err
		}
	}

	// 关键词、过滤条件均未识别出任何内容
	if len(p.PlainTexts) == 0 && p.SearchPair == nil {
		return errors.New("未解析出任何关键词或过滤条件，语法: 关键词,排除词@过滤条件?分页参数")
	}

	// 标记为已解析
	p.HasParsed = true

	return nil
}

func (p *QueryParams) BuildAsmrOneQueryStr() (string, error) {
	if !p.HasParsed {
		return "", errors.New("query params not parsed")
	}
	// 组装搜索关键字: 普通文本 + $搜索对$，以空格连接，不再引入前导空格
	var parts []string
	if len(p.PlainTexts) > 0 {
		parts = append(parts, strings.Join(p.PlainTexts, " "))
	}
	if p.SearchPair != nil {
		// 构建搜索参数
		for _, item := range []string{
			p.SearchPair.Tag, p.SearchPair.Circle, p.SearchPair.Va, p.SearchPair.Duration,
			p.SearchPair.Rate, p.SearchPair.Price, p.SearchPair.Sell, p.SearchPair.Age, p.SearchPair.Lang,
		} {
			if item != "" {
				parts = append(parts, "$"+item+"$")
			}
		}
	}
	encodedUrl := url.QueryEscape(strings.Join(parts, " "))
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

// splitTokens 按逗号分隔并移除首尾空格，丢弃空 token
func splitTokens(s string) []string {
	var tokens []string
	for _, item := range strings.Split(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			tokens = append(tokens, item)
		}
	}
	return tokens
}

// ---------------------------------------
// 解析搜索部分（tag/circle/va/...）
// ---------------------------------------

// searchPairPrefixes 支持的全部 key 前缀（配合反选 "-" 使用）
var searchPairPrefixes = []string{
	"tag:", "circle:", "va:", "duration:", "rate:", "price:", "sell:", "age:", "lang:",
}

// hasSearchPairPrefix 判断 token 是否为已识别的搜索对前缀（含反选 "-" 变体）
func hasSearchPairPrefix(item string) bool {
	s := strings.TrimPrefix(item, "-")
	for _, prefix := range searchPairPrefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

// parseSearchPair 逐个解析过滤条件 token，要求 key 已识别且不重复
func parseSearchPair(tokens []string) (*SearchPair, error) {
	pair := &SearchPair{}

	for _, item := range tokens {
		// 保持原样，不拆 value，只识别 key 对应字段
		key := strings.TrimPrefix(item, "-")
		if i := strings.Index(key, ":"); i >= 0 {
			key = key[:i]
		}
		var field *string
		switch key {
		case "tag":
			field = &pair.Tag
		case "circle":
			field = &pair.Circle
		case "va":
			field = &pair.Va
		case "duration":
			field = &pair.Duration
		case "rate":
			field = &pair.Rate
		case "price":
			field = &pair.Price
		case "sell":
			field = &pair.Sell
		case "age":
			field = &pair.Age
		case "lang":
			field = &pair.Lang
		default:
			return nil, fmt.Errorf("未识别的过滤条件 %q，支持: %s", item, strings.Join(searchPairPrefixes, " "))
		}
		if *field != "" {
			return nil, fmt.Errorf("过滤条件 %q 重复出现，同一条件只允许写一次", key)
		}
		*field = item
	}

	return pair, nil
}

// ---------------------------------------
// 解析分页 PageInfo 部分
// ---------------------------------------

// pageKeys 分页参数支持的全部 key
var pageKeys = []string{"order", "sort", "subtitle", "page", "pageSize", "includeTranslationWorks"}

// orderValues order 参数支持的全部取值
var orderValues = map[string]bool{
	"release": true, "dl_count": true, "create_date": true, "rating": true, "price": true,
	"rate_average_2dp": true, "review_count": true, "id": true, "nsfw": true,
}

// parsePageInfo 解析分页参数并合并进现有 PageInfo，未指定的字段保留默认值；
// 未知 key、重复 key、非法取值均直接报错
func parsePageInfo(q string, pi *PageInfo) error {
	m, err := url.ParseQuery(q)
	if err != nil {
		return fmt.Errorf("分页参数解析失败: %w", err)
	}

	for k, vs := range m {
		if len(vs) > 1 {
			return fmt.Errorf("分页参数 %q 重复出现，同一参数只允许写一次", k)
		}
		if !contains(pageKeys, k) {
			return fmt.Errorf("未知分页参数 %q，支持: %s", k, strings.Join(pageKeys, "/"))
		}
	}

	if v := m.Get("order"); v != "" {
		if !orderValues[v] {
			return fmt.Errorf("order 取值 %q 无效，支持: release/dl_count/create_date/rating/price/rate_average_2dp/review_count/id/nsfw", v)
		}
		pi.Order = v
	}
	if v := m.Get("sort"); v != "" {
		if v != "desc" && v != "asc" {
			return fmt.Errorf("sort 取值 %q 无效，支持: desc/asc", v)
		}
		pi.Sort = v
	}
	if v := m.Get("subtitle"); v != "" {
		if v != "0" && v != "1" {
			return fmt.Errorf("subtitle 取值 %q 无效，支持: 0(全部)/1(仅含字幕)", v)
		}
		pi.Subtitle = v
	}
	if v := m.Get("includeTranslationWorks"); v != "" {
		if v != "true" && v != "false" {
			return fmt.Errorf("includeTranslationWorks 取值 %q 无效，支持: true/false", v)
		}
		pi.IncludeTranslationWorks = v == "true"
	}
	if v := m.Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return fmt.Errorf("page 取值 %q 无效，须为正整数", v)
		}
		pi.Page = n
	}
	if v := m.Get("pageSize"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return fmt.Errorf("pageSize 取值 %q 无效，须为正整数", v)
		}
		pi.PageSize = n
	}

	return nil
}

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}
