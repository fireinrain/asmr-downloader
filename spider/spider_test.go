package spider

import (
	"asmr-downloader/config"
	"asmr-downloader/storage"
	"fmt"
	"testing"
)

func TestGetIndexPageInfo(t *testing.T) {
	var auth = "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJodHRwczovL2FzbXIub25lIiwic3ViIjoicGV0ZXJsaXUiLCJhdWQiOiJodHRwczovL2FzbXIub25lL2FwaSIsIm5hbWUiOiJwZXRlcmxpdSIsImdyb3VwIjoidXNlciIsImlhdCI6MTY3NTYxOTc4MiwiZXhwIjoxNzA3MTU1NzgyfQ.OF5PIjC9G024-_00ujujj8-y1NXfSWOtkOGWOln_XRA"
	pageInfo, _ := GetIndexPageInfo(auth, 0)
	fmt.Printf("%v", pageInfo)
}

func TestASMRClient_GetVoiceTracks(t *testing.T) {
	_ = storage.GetDbInstance()
	getConfig := config.GetConfig()
	asmrClient := NewASMRClient(2, getConfig)
	err := asmrClient.Login()
	if err != nil {
		fmt.Println("登录失败:", err)
		return
	}
	fmt.Println("账号登录成功!")
	tracks, err := asmrClient.GetVoiceTracks("1004107")
	println(tracks)

}
