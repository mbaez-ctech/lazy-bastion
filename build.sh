#!/bin/bash
set -e

APP=lazy-bastion
SHORT=lzb

mkdir -p dist

echo "Building for Mac (ARM)..."
GOOS=darwin GOARCH=arm64 go build -o dist/${APP}-mac-arm64 .
cp dist/${APP}-mac-arm64 dist/${SHORT}-mac-arm64

echo "Building for Mac (Intel)..."
GOOS=darwin GOARCH=amd64 go build -o dist/${APP}-mac-amd64 .
cp dist/${APP}-mac-amd64 dist/${SHORT}-mac-amd64

echo "Building for Linux..."
GOOS=linux GOARCH=amd64 go build -o dist/${APP}-linux-amd64 .
cp dist/${APP}-linux-amd64 dist/${SHORT}-linux-amd64

echo "Building for Windows..."
GOOS=windows GOARCH=amd64 go build -o dist/${APP}.exe .
cp dist/${APP}.exe dist/${SHORT}.exe

echo "Done. Binaries in ./dist/"
