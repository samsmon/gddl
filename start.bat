@echo off
title Google Drive Downloader
cd /d "%~dp0backend"

echo ===================================================
echo   Google Drive Downloader (Go + Svelte)
echo ===================================================
echo   Starting server at http://localhost:8080 ...
echo   Close this window to stop the server.
echo ===================================================

start "" http://localhost:8080
gdrive-downloader.exe
pause
