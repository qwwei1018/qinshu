@echo off
chcp 65001 >nul

net session >nul 2>&1
if %errorlevel% neq 0 (
    powershell -NoProfile -Command "Start-Process '%~f0' -Verb RunAs"
    exit /b 0
)

set "INSTALL_DIR=C:\Program Files\QinShu"
echo ============================================================
echo   Qin书输入法 (Win11 原生 TSF 版) 卸载程序
echo ============================================================

taskkill /f /im "Qin书.exe" >nul 2>&1

if exist "%INSTALL_DIR%\QinShuTSF_x64.dll" (
    "%SystemRoot%\System32\regsvr32.exe" /u /s "%INSTALL_DIR%\QinShuTSF_x64.dll"
)
if exist "%INSTALL_DIR%\QinShuTSF_x86.dll" (
    "%SystemRoot%\SysWOW64\regsvr32.exe" /u /s "%INSTALL_DIR%\QinShuTSF_x86.dll"
)

powershell -NoProfile -Command "Remove-Item -Force -ErrorAction SilentlyContinue ([Environment]::GetFolderPath('Startup') + '\Qin书输入法.lnk')"
rmdir /s /q "%INSTALL_DIR%" >nul 2>&1

echo 卸载完成！如仍有残留 DLL 被系统占用，注销或重启后将自动释放。
pause
