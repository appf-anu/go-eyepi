package main

import (
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"fmt"
	"encoding/base64"
	"os"
	"time"
)

const KEY = "/root/.ssh/id_rsa"

func main() {
	keybytes, err := ioutil.ReadFile(KEY)
	if err != nil {
		panic(err)
	}

	s, err := ssh.ParseRawPrivateKey(keybytes)
	if err != nil {
		panic(err)
	}
	signer,err := ssh.NewSignerFromKey(s)
	hostname, err := os.Hostname()
	if err != nil{
		panic(err)
	}
	signTime := time.Now().Format("2006_01_02_15_04_05")

	signature, err := signer.Sign(nil, []byte(signTime))

	encodedSig := base64.StdEncoding.EncodeToString(signature.Blob)
	signed := fmt.Sprintf("%s|%s", signTime, encodedSig)

	fdata := fmt.Sprintf("%s\n%s",hostname, signed)
	err = ioutil.WriteFile("/etc/openvpn/client/login.conf", []byte(fdata), 0700)
	if err != nil{
		panic(err)
	}
}
