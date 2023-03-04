#!/bin/bash

export GO111MODULE="on"
go install github.com/mitchellh/gox@latest # cross compile
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X github.com/aerokube/cm/cmd.buildStamp=`date -u '+%Y-%m-%d_%I:%M:%S%p'` -X github.com/aerokube/cm/cmd.gitRevision=`git describe --tags || git rev-parse HEAD` -s -w"
CGO_ENABLED=0 gox -os "linux darwin windows" -arch "amd64"  -osarch="darwin/arm64" -osarch="windows/386" -output "dist/{{.Dir}}_{{.OS}}_{{.Arch}}" -ldflags "-X github.com/aerokube/cm/cmd.buildStamp=`date -u '+%Y-%m-%d_%I:%M:%S%p'` -X github.com/aerokube/cm/cmd.gitRevision=`git describe --tags || git rev-parse HEAD` -s -w"
