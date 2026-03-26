#!/bin/bash

echo "Building Syslog2Bot Web Server..."

cd "$(dirname "$0")"

echo "Building frontend..."
cd frontend
npm run build
cd ..

echo "Building web server binary for Linux amd64..."
GOOS=linux GOARCH=amd64 go build -tags web -o build/bin/syslog2bot-web

echo "Copying templates..."
cp -r templates build/bin/templates

echo "Done! Output: build/bin/"
echo ""
echo "Files:"
echo "  - syslog2bot-web (binary)"
echo "  - templates/ (parse_templates.json, filter_policies.json)"
echo ""
echo "Usage: ./syslog2bot-web [port]"
echo "Default port: 8080"
