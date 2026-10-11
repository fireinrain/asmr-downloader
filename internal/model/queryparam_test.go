package model

import (
	"strings"
	"testing"
)

// build 构造并解析查询参数，失败直接终止测试
func build(t *testing.T, q string) *QueryParams {
	t.Helper()
	p := NewQueryParams(q)
	if err := p.ParseQueryStr(); err != nil {
		t.Fatalf("ParseQueryStr(%q) unexpected error: %v", q, err)
	}
	return p
}

// buildErr 断言解析必须失败
func buildErr(t *testing.T, q string) {
	t.Helper()
	p := NewQueryParams(q)
	if err := p.ParseQueryStr(); err == nil {
		t.Fatalf("ParseQueryStr(%q) expected error, got nil", q)
	}
}

// TestQueryParams_ParseQueryStr 完整语法冒烟测试
func TestQueryParams_ParseQueryStr(t *testing.T) {
	q := "修女,洗脑,-触手@tag:内射/中出,circle:青春×フェティシズム,va:陽向葵ゅか,duration:1h,rate:4.75,-price:1000,sell:700,age:adult,-lang:JPN?order=dl_count&sort=desc&page=1&pageSize=20&subtitle=0&includeTranslationWorks=true"
	p := build(t, q)

	if p.SearchPair == nil {
		t.Fatal("SearchPair should not be nil")
	}
	for field, want := range map[string]string{
		"Tag":      p.SearchPair.Tag,
		"Circle":   p.SearchPair.Circle,
		"Va":       p.SearchPair.Va,
		"Duration": p.SearchPair.Duration,
		"Rate":     p.SearchPair.Rate,
		"Price":    p.SearchPair.Price,
		"Sell":     p.SearchPair.Sell,
		"Age":      p.SearchPair.Age,
		"Lang":     p.SearchPair.Lang,
	} {
		if want == "" {
			t.Errorf("SearchPair.%s should not be empty", field)
		}
	}
	pi := p.PageInfo
	if pi.Order != "dl_count" || pi.Sort != "desc" || pi.Page != 1 || pi.PageSize != 20 ||
		pi.Subtitle != "0" || !pi.IncludeTranslationWorks {
		t.Errorf("PageInfo not parsed correctly: %+v", pi)
	}
}

// TestQueryParams_ReadmeExamples README 样例回归测试
func TestQueryParams_ReadmeExamples(t *testing.T) {
	t.Run("普通文本+筛选", func(t *testing.T) {
		p := build(t, "护士,-中出@duration:1h")
		if want := []string{"护士", "-中出"}; strings.Join(p.PlainTexts, ",") != strings.Join(want, ",") {
			t.Errorf("PlainTexts = %v, want %v", p.PlainTexts, want)
		}
		if p.SearchPair == nil || p.SearchPair.Duration != "duration:1h" {
			t.Errorf("Duration not parsed: %+v", p.SearchPair)
		}
		// 分页参数未指定时保留默认值
		if p.PageInfo.Order != "release" || p.PageInfo.PageSize != 20 {
			t.Errorf("defaults clobbered: %+v", p.PageInfo)
		}
	})

	t.Run("筛选+分页参数共存", func(t *testing.T) {
		p := build(t, "护士,-中出@duration:1h?order=dl_count&sort=desc")
		if p.SearchPair == nil || p.SearchPair.Duration != "duration:1h" {
			t.Errorf("Duration lost: %+v", p.SearchPair)
		}
		if p.PageInfo.Order != "dl_count" || p.PageInfo.Sort != "desc" {
			t.Errorf("PageInfo ignored: %+v", p.PageInfo)
		}
	})

	t.Run("va筛选+subtitle", func(t *testing.T) {
		p := build(t, "@va:陽向葵ゅか,age:adult?subtitle=1")
		if p.SearchPair == nil || p.SearchPair.Va != "va:陽向葵ゅか" || p.SearchPair.Age != "age:adult" {
			t.Errorf("Va/Age lost: %+v", p.SearchPair)
		}
		if p.PageInfo.Subtitle != "1" {
			t.Errorf("Subtitle ignored: %+v", p.PageInfo)
		}
	})

	t.Run("纯分页参数不混入关键字", func(t *testing.T) {
		p := build(t, "护士?page=2&pageSize=50&subtitle=1")
		if strings.Join(p.PlainTexts, ",") != "护士" {
			t.Errorf("PlainTexts = %v, want [护士]", p.PlainTexts)
		}
		if p.PageInfo.Page != 2 || p.PageInfo.PageSize != 50 || p.PageInfo.Subtitle != "1" {
			t.Errorf("PageInfo not parsed: %+v", p.PageInfo)
		}
	})

	t.Run("空段与多余逗号容忍", func(t *testing.T) {
		p := build(t, "护士,@tag:内射,")
		if strings.Join(p.PlainTexts, ",") != "护士" {
			t.Errorf("PlainTexts = %v, want [护士]", p.PlainTexts)
		}
		if p.SearchPair == nil || p.SearchPair.Tag != "tag:内射" {
			t.Errorf("Tag not parsed: %+v", p.SearchPair)
		}
	})

	t.Run("构建结果无前导空格", func(t *testing.T) {
		p := build(t, "护士,-中出@duration:1h")
		out, err := p.BuildAsmrOneQueryStr()
		if err != nil {
			t.Fatalf("BuildAsmrOneQueryStr() unexpected error: %v", err)
		}
		if strings.HasPrefix(out, "%20") {
			t.Errorf("query has leading space: %s", out)
		}
		if !strings.Contains(out, "%24duration%3A1h%24") {
			t.Errorf("duration filter missing: %s", out)
		}
	})
}

// TestQueryParams_StrictErrors 严格语法: 非法输入一律报错
func TestQueryParams_StrictErrors(t *testing.T) {
	cases := []struct {
		name  string
		query string
	}{
		{"空查询", "  "},
		{"过滤条件缺@", "护士,tag:内射"},
		{"反选过滤条件缺@", "护士,-lang:JPN"},
		{"未识别的过滤条件", "@foo:bar"},
		{"过滤条件重复", "@tag:内射,tag:耳かき"},
		{"多余的@", "护士@tag:内射@va:陽向葵ゅか"},
		{"未知分页参数", "护士?foo=1"},
		{"分页参数重复", "护士?page=1&page=2"},
		{"order 非法取值", "护士?order=bad"},
		{"sort 非法取值", "护士?sort=bad"},
		{"subtitle 非法取值", "护士?subtitle=2"},
		{"page 非法取值", "护士?page=0"},
		{"pageSize 非取值", "护士?pageSize=abc"},
		{"includeTranslationWorks 非法取值", "护士?includeTranslationWorks=yes"},
		{"只有分隔符无内容", "@"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			buildErr(t, c.query)
		})
	}
}
