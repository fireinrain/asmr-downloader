%windows 平台编译打包脚本%
%关闭打印代码执行过程%
@echo off
cd ..
git pull
%判断构建目录是否存在配置文件%
SET SourceFile=./builds/config.json

if not exist %SourceFile% (
   echo copy config.json to builds dir
   xcopy .\config.json .\builds\
)

IF EXIST "%PROGRAMFILES(X86)%" (GOTO 64BIT) ELSE (GOTO 32BIT)
:64BIT
set platform="%1%"
if %platform% == "windows" GOTO windows

if %platform% == "linux" GOTO linux

if %platform% == "osx" GOTO darwin

if %platform% == "" GOTO explain
:explain
echo "------user guide------"
echo "------this script is portable for linux------"
echo "./build.sh windows (build for windows)"
echo "./build.sh linux (build for linux)"
echo "./build.sh osx (build for macos)"
exit

:windows
echo ">>>>>>: build %1 executable file on windows"
echo arch 64-bit...
set GOARCH=amd64
set GOOS=windows
set CGO_ENABLED=0
go build -o ./builds/asmr-downloader-windows-amd64.exe
echo "build success"
exit

:linux
echo ">>>>>>: build %1 executable file on windows"
echo arch 64-bit...
set GOARCH=amd64
set GOOS=linux
set CGO_ENABLED=0
go build -o ./builds/asmr-downloader-linux-amd64
echo "build success"
exit

:osx
echo ">>>>>>: build %1 executable file on windows"

echo arch 64-bit...
set GOARCH=amd64
set GOOS=darwin
set CGO_ENABLED=0
go build -o ./builds/asmr-downloader-darwin-amd64
echo "bui;d success"
exit

%编译32位，基本不使用%
:32BIT
echo 32-bit...
go build -o asmr-downloader-windows-i386.exe
exit

pause