@echo off
rem Builds SimplePlayerPresence.exe (needs Go: https://go.dev/dl/).
rem The extension folder is embedded into the exe, so copy it in first.
cd /d "%~dp0"
if exist extension rmdir /s /q extension
xcopy /e /i /q ..\extension extension >nul
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -trimpath -ldflags "-s -w -H windowsgui" -o ..\SimplePlayerPresence.exe .
rmdir /s /q extension
