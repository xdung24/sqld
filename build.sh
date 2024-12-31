#//bin/bash

GO_OS=$(go env GOOS)
GO_ARCH=$(go env GOARCH)
GIT_HASH=$(git rev-parse --short HEAD)
BUILD_TIME="`date +%Y/%m/%d-%H:%M:%S%z`"

echo "GO_OS: $GO_OS"
echo "GO_ARCH: $GO_ARCH"
echo "GIT_HASH: $GIT_HASH"
echo "BUILD_TIME: $BUILD_TIME"

ldflags="-X main.GoOs=$GO_OS -X main.GoArch=$GO_ARCH -X main.GitHash=$GIT_HASH -X main.BuildTime=$BUILD_TIME"
echo $ldflags
echo "building sqld"
go mod download
CGO_ENABLED=1 go build -o ./tmp/sqld.exe --ldflags="$ldflags"