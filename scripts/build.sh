#!/bin/bash
#编译说明
# linux 平台编译脚本
if [ "${1}" == "" ]; then
  echo "------user guide------"
  echo "------this script is portable for linux------"
  echo "./build.sh windows (build for windows)"
  echo "./build.sh linux (build for linux)"
  echo "./build.sh osx (build for macos)"
  exit 0
fi


CGO_ENABLED=0
GOARCH=amd64

#判断是否是在根目录
file=main.go
buildsDir=./builds
confFile=./builds/config.json


if [ ! -f "$file" ]; then
  cd ..
fi
if [ ! -f "$buildsDir" ]; then
  mkdir -p $buildsDir
fi
# 判断builds文件夹下是否存在配置文件，不存在则从根目录的配置文件复制进去
#if [ ! -f "$confFile" ]; then
#  cp ./config.json ./builds
#fi

if [ "${1}" == "windows" ]
then
    GOOS=windows
    go build -v -ldflags "-s -w" -o builds/asmr-downloader-${1}-amd64.exe
elif [ "${1}" == "osx" ]
then
    GOOS=darwin
    go build -v -ldflags "-s -w" -o builds/asmr-downloader-darwin-amd64
else
    GOOS=linux
    go build -v -ldflags "-s -w" -o builds/asmr-downloader-${1}-amd64
fi

#for file in builds/*
#do
#    sha256sum ${file} > ${file}.sha256
#done

echo "build done!"
ls builds