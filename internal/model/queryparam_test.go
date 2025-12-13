package model

import (
	"fmt"
	"testing"
)

func TestQueryParams_ParseQueryStr(t *testing.T) {
	q := "修女,洗脑,-触手@tag:内射/中出,circle:青春×フェティシズム,va:陽向葵ゅか,duration:1h,rate:4.75,-price:1000,sell:700,age:adult,-lang:JPN?order=dl_count&sort=desc&page=1&pageSize=20&subtitle=0&includeTranslationWorks=true"
	queryParams := NewQueryParams(q)
	err := queryParams.ParseQueryStr()
	if err == nil {
		t.Errorf("ParseQueryStr() error = %v, wantErr %v", err, "empty query string")
	}
	str, err := queryParams.BuildAsmrOneQueryStr()
	if err != nil {
		t.Errorf("BuildAsmrOneQueryStr() error = %v, wantErr %v", err, "")
	}
	fmt.Println(str)
	//fmt.Println(queryParams)
}
