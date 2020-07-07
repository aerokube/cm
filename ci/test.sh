#!/bin/bash

export GO111MODULE="on"
go test -race -v github.com/aerokube/cm/selenoid -coverprofile=coverage.txt -covermode=atomic -coverpkg github.com/aerokube/cm/selenoid
