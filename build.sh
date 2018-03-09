#!/bin/bash

env GOOS=linux GOARCH=arm GOARM=7 go test ./rpi-sshsign
env GOOS=linux GOARCH=arm GOARM=7 go build -o bin/rpi-sshsign ./rpi-sshsign
cp bin/rpi-sshsign ansible/files/rpi-sshsign
env GOOS=linux go test ./openvpn-mongopass
go build -o bin/openvpn-mongopass ./openvpn-mongopass
env GOOS=linux GOARCH=arm GOARM=7 go test .

env GOOS=linux GOARCH=arm GOARM=7 go build -a -o bin/go-eyepi .
cp bin/go-eyepi ansible/files/go-eyepi
