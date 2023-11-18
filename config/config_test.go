package config

import (
	"fmt"
	"testing"
)

func TestGetRespFastestSiteUrl(t *testing.T) {
	url := GetRespFastestSiteUrl()
	fmt.Println(url)
}
