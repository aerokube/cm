# Configuration Manager (aka Centimeter)
[![Build Status](https://travis-ci.org/aerokube/cm.svg?branch=master)](https://travis-ci.org/aerokube/cm)
[![Coverage](https://codecov.io/github/aerokube/cm/coverage.svg)](https://codecov.io/gh/aerokube/cm)
[![Release](https://img.shields.io/github/release/aerokube/cm.svg)](https://github.com/aerokube/cm/releases/latest)

Configuration manager is used to generate configuration for Aerokube products.

## Development
To build cm:

1) Install [Golang](https://golang.org/doc/install)
2) Setup `$GOPATH` [properly](https://github.com/golang/go/wiki/GOPATH)
3) Install [govendor](https://github.com/kardianos/govendor): 
```
$ go get -u github.com/kardianos/govendor
```
4) Get cm source:
```
$ go get -d github.com/aerokube/cm
```
5) Go to project directory:
```
$ cd $GOPATH/src/github.com/aerokube/cm
```
6) Checkout dependencies:
```
$ govendor sync
```
7) Build source:
```
$ go build
```
8) Run cm:
```
$ ./cm --help
```
9) To build [Docker](http://docker.com/) container type:
```
$ GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build
$ docker build -t cm:latest .
```
