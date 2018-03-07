#!/bin/bash
mkdir bin
go build rpi-sshsign -o bin/rpi-sshsign
cp bin/rpi-sshsign ansible/files/rpi-sshsign
go build openvpn-mongopass -o bin/openvpn-mongopass
env GOOS=linux GOARCH=arm GOARM=7 go build -a -o bin/go-eyepi
cp bin/go-eyepi ansible/files/go-eyepi
