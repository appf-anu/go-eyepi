package main

import (
	"gopkg.in/mgo.v2"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/ssh"
	"strings"
	"fmt"
	"gopkg.in/mgo.v2/bson"
	"errors"
	"os"
	"encoding/base64"
)

var (
	session    *mgo.Session
	rpiCollection *mgo.Collection
	userCollection *mgo.Collection
)

type RaspberryPi struct{
	Ssh_private_key string
	Ssh_public_key string
	Machine string
	Name string
}

func (dbRpi RaspberryPi) ValidateMessage(messageandsig string) error {
	split := strings.SplitN(messageandsig, "|", 2)
	message, sig := split[0], split[1]
	decodedSig, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		fmt.Println("[auth] decode error:", err)
	}
	sigbytes := []byte(decodedSig)
	pub, _, _, _, keyError := ssh.ParseAuthorizedKey([]byte(dbRpi.Ssh_public_key))
	if keyError != nil{
		fmt.Printf("[auth]	KEYERROR	%s\n", dbRpi.Name)
		return keyError
	}

	signature := ssh.Signature{Format: pub.Type(), Blob: sigbytes}
	signError := pub.Verify([]byte(message), &signature)
	if signError != nil {
		fmt.Printf("[auth]	SIGNERROR	%s\n", dbRpi.Name)
		return signError
	}
	fmt.Printf("[auth]	SUCCESS	%s\n", dbRpi.Name)
	return nil
}


type User struct{
	Email string
	Password string
}

func (dbUser User) ValidatePassword(inputPasswordString string) error {
	inputPassword := []byte(inputPasswordString)
	userPassword := []byte(dbUser.Password)

	err := bcrypt.CompareHashAndPassword(userPassword, inputPassword)
	if err == nil {
		fmt.Printf("[auth]	SUCCESS	%s\n", dbUser.Email)
		return nil
	}
	return err
}


func Authenticate(id string, inputPasswordString string) error {
	dbUser := User{}
	dbRpi := RaspberryPi{}

	if userCollection.Find(bson.M{"email": id}).One(&dbUser) == nil {
		// user exists and we're going to find em...
		return dbUser.ValidatePassword(inputPasswordString)
	}

	if rpiCollection.Find(bson.M{"name": id}).One(&dbRpi) == nil {
		// raspberry pi exists
		return dbRpi.ValidateMessage(inputPasswordString)
	}

	return errors.New("[auth]	FAIL	No User or Pi.\n")
}

func main() {
	var err error
	session, err = mgo.Dial("localhost")

	if err != nil {
		fmt.Printf("[db]	FAIL	%s", err)
	}

	rpiCollection = session.DB("supersite_database").C("raspberry_pi")
	userCollection = session.DB("supersite_database").C("user")
	username := os.Getenv("username")
	password := os.Getenv("password")

	err = Authenticate(username, password)
	session.Close()
	if err == nil{
		os.Exit(0)
	}
	fmt.Printf("%s", err)
	os.Exit(1)

}

