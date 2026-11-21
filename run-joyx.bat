@echo off
title JoyX - WhatsApp Bot
cd /d D:\DEVELOPMENT\PROJECT\JoyXv1

echo ==========================================
echo   JoyX Launcher (Joy + WazzapAgent-Go)
echo ==========================================

if not exist bin\joyx.exe (
    echo [INFO] Build pertama kali...
    call go build -o bin\joyx.exe .\cmd\wazzapagent
    if errorlevel 1 (
        echo [ERROR] Build gagal. Cek Go terpasang.
        pause
        exit /b 1
    )
)

echo.
echo [INFO] Menyalakan JoyX...
echo   QR pairing : http://localhost:3001  (buka di Chrome, lalu scan dari HP)
echo   Kalau QR tidak muncul, lihat juga di terminal ini.
echo.
echo Tekan Ctrl+C untuk berhenti.
echo.

call bin\joyx.exe
pause