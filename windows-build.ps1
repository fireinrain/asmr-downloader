$env:CGO_ENABLED = "1"
$env:CC = "gcc"
$env:CXX = "g++"
go build -x -v -ldflags "-s -w" -o asmr-downloader.exe