@echo off
echo Building Syslog2Bot Web Server...

cd /d "%~dp0"

echo Building frontend...
cd frontend
call npm run build
cd ..

echo Building web server binary...
go build -tags web -o build/bin/syslog2bot-web.exe

echo Copying templates...
if exist templates (
    xcopy /E /I /Y templates build\bin\templates
)

echo Done! Output: build/bin/
echo.
echo Files:
echo   - syslog2bot-web.exe (binary)
echo   - templates/ (parse_templates.json, filter_policies.json)
echo.
echo Usage: syslog2bot-web.exe [port]
echo Default port: 8080

pause
