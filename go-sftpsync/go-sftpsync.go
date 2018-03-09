package main

import (
	"time"
	"github.com/BurntSushi/toml"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"github.com/pkg/sftp"
	"os"
	"net"
	"strings"
	"fmt"
	"path/filepath"
	"log"
	"log/syslog"
	"path"
	"io/ioutil"
	"sync"
)

const CONFIGPATH = "/etc/go-sftpsync/go-sftpsync.conf"


type duration struct {
	time.Duration
}

func (d *duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

type destInfo struct {
	user, host, serverPath string
}

func (d *destInfo) UnmarshalText(text []byte) error {
	var err error
	components := strings.Split(string(text),"@")
	d.user = components[0]
	if len(components)< 2{
		return fmt.Errorf("destination info not in format user@host:/path")
	}

	components = strings.Split(components[1],":")
	if len(components)< 2{
		return fmt.Errorf("destination info not in format user@host:/path")
	}
	d.host = components[0]
	d.serverPath = components[1]
	return err
}

type Sync struct {
	Interval duration
	Source string
	Target destInfo
	Password string
}

func sshAgent() ssh.AuthMethod {
	if sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		return ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers)
	}
	return nil
}

func sftpMkdirParents(client *sftp.Client, dir string) (err error) {
	var parents string
	sshFxFailure := uint32(4)

	if path.IsAbs(dir) {
		// Otherwise, an absolute path given below would be turned in to a relative one
		// by splitting on "/"
		parents = "/"
	}

	for _, name := range strings.Split(dir, "/") {
		if name == "" {
			// Paths with double-/ in them should just move along
			// this will also catch the case of the first character being a "/", i.e. an absolute path
			continue
		}
		parents = path.Join(parents, name)
		err = client.Mkdir(parents)
		if status, ok := err.(*sftp.StatusError); ok {
			if status.Code == sshFxFailure {
				var fi os.FileInfo
				fi, err = client.Stat(parents)
				if err == nil {
					if !fi.IsDir() {
						return fmt.Errorf("File exists: %s", parents)
					}
				}
			}
		}
		if err != nil {
			break
		}
	}
	return err
}

func (s *Sync) Run(){
	fmt.Println("Rnuning")
	config := &ssh.ClientConfig{
		User: s.Target.user,
		Auth: []ssh.AuthMethod{
			ssh.Password(s.Password),
			sshAgent(),
		},
	}
	config.RekeyThreshold = 0
	handler,_ := syslog.New(syslog.LOG_NOTICE, "sftpsync")
	errLog := log.New(handler,
		"[ERROR] ",
		log.Ldate|log.Ltime|log.Lshortfile)

	conn, err := ssh.Dial("tcp", s.Target.host+":22", config)
	if err != nil {
		errLog.Println(err)
		fmt.Println(err)
		return
	}
	client, err := sftp.NewClient(conn)
	if err != nil {
		errLog.Println(err)
		fmt.Println(err)
		return
	}
	defer client.Close()
	err = filepath.Walk(s.Source, func(path string, info os.FileInfo, err error)error{
		pathWithoutSource := strings.Replace(path, s.Source, "", 1)
		pathToCreate := filepath.Join(s.Target.serverPath, pathWithoutSource)
		if info.IsDir(){
			sftpMkdirParents(client, pathToCreate)
		} else {
			f, err := client.Create(pathToCreate)
			if err != nil{
				errLog.Println(err)
				fmt.Println(err)
				return nil
			}
			lf, err := ioutil.ReadFile(path)
			if err != nil{
				errLog.Println(err)
				fmt.Println(err)
				return nil
			}
			_, err = f.Write(lf)
			if err != nil{
				errLog.Println(err)
				fmt.Println(err)
				return nil
			}
			stat, err := f.Stat()
			if err != nil{
				errLog.Println(err)
				fmt.Println(err)
				return nil
			}
			if stat.Size() != info.Size(){
				errLog.Println("local file and remote file not the size after transfer")
			}else{
				errLog.Printf("Successfully uploaded %s to %s", path, pathToCreate)
			}

		}
		return nil
	})
	if err != nil{
		errLog.Println(err)
	}
}

type globalConfig struct {
	Syncs map[string]*Sync
}


func main() {
	config := globalConfig{}
	if _, err := toml.DecodeFile(CONFIGPATH, &config); err != nil {
		panic(err)
	}
	var wg sync.WaitGroup

	wg.Add(len(config.Syncs))
	for name,s := range config.Syncs{
		go func(){
			fmt.Printf("running %s", name)
			defer wg.Done()
			s.Run()
		}()
	}
	wg.Wait()
}
