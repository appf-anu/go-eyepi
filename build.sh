#!/bin/bash

VERSION=`git describe --tags`
BUILT=`date +%FT%T%z`
env GOOS=linux GOARCH=arm GOARM=7 go test ./rpi-sshsign
echo "Building rpi-sshsign"
env GOOS=linux GOARCH=arm GOARM=7 go build -o bin/rpi-sshsign ./rpi-sshsign
echo "Testing openvpn-mongopass"
env GOOS=linux go test ./openvpn-mongopass
echo "Building openvpn-mongopass"
env GOOS=linux go build -o bin/openvpn-mongopass ./openvpn-mongopass
echo "Testing go-eyepi"
env GOOS=linux go test .
echo "Building go-eyepi"
env CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -i -a -o bin/go-eyepi -ldflags "-X main.Version=$VERSION -X main.Built=$BUILT" .
