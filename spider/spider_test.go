package spider

import (
	"fmt"
	"testing"
)

func TestGetIndexPageInfo(t *testing.T) {
	var auth = "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJodHRwczovL2FzbXIub25lIiwic3ViIjoicGV0ZXJsaXUiLCJhdWQiOiJodHRwczovL2FzbXIub25lL2FwaSIsIm5hbWUiOiJwZXRlcmxpdSIsImdyb3VwIjoidXNlciIsImlhdCI6MTY3NTYxOTc4MiwiZXhwIjoxNzA3MTU1NzgyfQ.OF5PIjC9G024-_00ujujj8-y1NXfSWOtkOGWOln_XRA"
	pageInfo, _ := GetIndexPageInfo(auth)
	fmt.Printf("%v", pageInfo)
}
