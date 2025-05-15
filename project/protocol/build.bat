@echo off

echo protoc running...

setlocal enabledelayedexpansion
set PROTO_FILES=

for %%F in (./protobuf/*.proto) do (
    set PROTO_FILES=!PROTO_FILES! %%F
)

cd protoc-30.0-rc-2-win64/bin
protoc --proto_path=../../protobuf/ --go_out=../../generate --go-grpc_out=../../generate  !PROTO_FILES!

if %errorlevel% neq 0 (
    echo [ERROR]
    pause
) else (
    echo protoc finnish...
	timeout /t 1 >nul
    exit
)
