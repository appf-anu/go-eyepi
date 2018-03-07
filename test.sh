#!/bin/bash
env GOOS=linux go test ./rpi-sshsign
env GOOS=linux go test ./openvpn-mongopass
env GOOS=linux GOARCH=arm GOARM=7 go test .
