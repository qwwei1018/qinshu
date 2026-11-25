@echo off
chcp 65001 >nul
cd /d "%~dp0"
echo 正在嵌入图标并编译单文件 Qin书.exe ...
if exist "assets\qinshu.ico" (
    where rsrc >nul 2>nul
    if %errorlevel% neq 0 (
        go install github.com/akavel/rsrc@latest >nul 2>nul
    )
    where rsrc >nul 2>nul
    if %errorlevel%==0 (
        rsrc -arch amd64 -ico "assets\qinshu.ico" -o "cmd\qinshu\rsrc_windows_amd64.syso"
    ) else (
        where windres >nul 2>nul
        if %errorlevel%==0 (
            windres -i "cmd\qinshu\qinshu.rc" -o "cmd\qinshu\rsrc_windows_amd64.syso" -O coff
        )
    )
)
go build -ldflags="-H windowsgui -s -w" -o "Qin书.exe" ./cmd/qinshu
if %errorlevel% neq 0 (
    echo 编译失败，请确认已安装 Go 1.22+。
    pause
    exit /b 1
)
echo 编译完成：%~dp0Qin书.exe
pause
