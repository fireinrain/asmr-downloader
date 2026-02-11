package model

import (
	"testing"
)

func TestQueryParams_ParseQueryStr(t *testing.T) {
	q := "修女,洗脑,-触手@tag:内射/中出,circle:青春×フェティシズム,va:陽向葵ゅか,duration:1h,rate:4.75,-price:1000,sell:700,age:adult,-lang:JPN?order=dl_count&sort=desc&page=1&pageSize=20&subtitle=0&includeTranslationWorks=true"
	queryParams := NewQueryParams(q)
	err := queryParams.ParseQueryStr()
	if err != nil {
		t.Fatalf("ParseQueryStr() unexpected error: %v", err)
	}
	str, err := queryParams.BuildAsmrOneQueryStr()
	if err != nil {
		t.Fatalf("BuildAsmrOneQueryStr() unexpected error: %v", err)
	}
	if str == "" {
		t.Error("BuildAsmrOneQueryStr() returned empty string")
	}
	t.Logf("BuildAsmrOneQueryStr result: %s", str)
}
