#!/usr/bin/env bash

#unix 平台执行脚本
cd ..
git pull
platform=$(uname -a)

if [[ $platform =~ "Darwin" ]]
then
    go build -o asmr-downloader-darwin-amd64 main.go
elif [[ $platform =~ "x86_64" ]];then
    go build -o asmr-downloader-unix-amd64 main.go
else
    go build -o asmr-downloader-linux-amd64 main.go
fi