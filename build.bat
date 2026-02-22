@echo off
setlocal enabledelayedexpansion

echo === Building Resource Queue Plugin ===

echo.
echo [1/3] Building server binaries...
if not exist server\dist mkdir server\dist

set PLATFORMS=linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64

for %%P in (%PLATFORMS%) do (
    for /f "tokens=1,2 delims=-" %%A in ("%%P") do (
        set GOOS=%%A
        set GOARCH=%%B
        set EXT=
        if "%%A"=="windows" set EXT=.exe
        echo   Building %%A/%%B...
        cd server
        set CGO_ENABLED=0
        go build -o dist\plugin-%%P!EXT! .
        if errorlevel 1 (
            echo ERROR: Failed to build for %%P
            cd ..
            exit /b 1
        )
        cd ..
    )
)

echo.
echo [2/3] Building webapp...
cd webapp
call npm install --legacy-peer-deps
if errorlevel 1 (
    echo ERROR: npm install failed
    cd ..
    exit /b 1
)
call npm run build
if errorlevel 1 (
    echo ERROR: webpack build failed
    cd ..
    exit /b 1
)
cd ..

echo.
echo [3/3] Packing plugin...
go run pack.go
if errorlevel 1 (
    echo ERROR: Pack failed
    exit /b 1
)

echo.
echo === Build complete! ===
echo Upload the .tar.gz file to Mattermost System Console ^> Plugin Management
