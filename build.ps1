# Builds the program for Windows and for a Linux server.
# Run it from this folder in PowerShell:  .\build.ps1

New-Item -ItemType Directory -Force -Path dist | Out-Null

go build -o dist\binmonitor.exe .

$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -o dist\binmonitor_linux .
Remove-Item Env:GOOS
Remove-Item Env:GOARCH

Write-Host "Done. The programs are in the dist folder."
