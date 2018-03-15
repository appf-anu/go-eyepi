#!/bin/bash

VERSION=`git describe --tags`
BUILT=`date +%FT%T%z`
echo "$VERSION"
env GOOS=linux GOARCH=arm GOARM=7 go test ./rpi-sshsign
env GOOS=linux GOARCH=arm GOARM=7 go build -o bin/rpi-sshsign ./rpi-sshsign
cp bin/rpi-sshsign ansible/files/rpi-sshsign
env GOOS=linux go test ./openvpn-mongopass
go build -o bin/openvpn-mongopass ./openvpn-mongopass
env GOOS=linux go test .
env GOOS=linux GOARCH=arm GOARM=7 go build -a -o bin/go-eyepi -ldflags "-X main.Version=$VERSION -X main.Built=$BUILT" .
cp bin/go-eyepi ansible/files/go-eyepi
