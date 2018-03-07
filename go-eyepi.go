package main

import (
	"time"
	"github.com/BurntSushi/toml"
	"os"
	"sync"
	"github.com/mdaffin/go-telegraf"
	"path/filepath"
	"log/syslog"
	"log"
	"io"
	"gopkg.in/fsnotify.v1"
)

const CONFIGPATH = "/etc/go-eyepi/go-eyepi.conf"
//const CONFIGPATH = "./go-eyepi.conf"

var (
	Trace          *log.Logger
	Info           *log.Logger
	Warning        *log.Logger
	Error          *log.Logger
	config         *GlobalConfig
	mutex          *sync.Mutex
)

type GlobalConfig struct {
	TimestampFormat string
	RpiCamera       *RaspberryPiCamera
	Gphoto          map[string]*GphotoCamera
}

type duration struct {
	time.Duration
}

func (d *duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

func CopyFile(src, dest string) error {
	from, err := os.Open(src)
	if err != nil {
		return err
	}
	defer from.Close()

	to, err := os.OpenFile(dest, os.O_RDWR|os.O_CREATE, 0665)
	if err != nil {
		return err
	}
	defer to.Close()

	_, err = io.Copy(to, from)
	if err != nil {
		return err
	}
	return nil
}

func printCameras(cam interface{}) {

	switch c := cam.(type) {
	case *GphotoCamera:
		Info.Printf("Camera %s \n\t%t\n\t%s\n\t%s\n\t%s\n-------\n", c.FilenamePrefix, c.Enable, c.Interval, c.OutputDir, c.USBPort)
	case *RaspberryPiCamera:
		Info.Printf("Camera %s \n\t%t\n\t%s\n\t%s\n-------\n", c.FilenamePrefix, c.Enable, c.Interval, c.OutputDir)
	default:
		Info.Println("Idk")
	}

}

func CreateCameras() {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	config = &GlobalConfig{
		"2006_01_02_15_04_05",
		&RaspberryPiCamera{
			Enable: true,
			Interval: duration{time.Duration(time.Minute * 5)},
			FilenamePrefix: "",
			OutputDir: "",
		},
		make(map[string]*GphotoCamera, 0),
	}

	if _, err := toml.DecodeFile(CONFIGPATH, &config); err != nil {
		panic(err)
	}

	if config.RpiCamera.FilenamePrefix == "" {
		config.RpiCamera.FilenamePrefix = hostname + "-" + "Picam"
	}
	if config.RpiCamera.OutputDir == "" {
		config.RpiCamera.OutputDir = filepath.Join("/var/lib/go-eyepi/", config.RpiCamera.FilenamePrefix)
	}
	if config.RpiCamera.Interval.Duration <= time.Duration(time.Second) {
		config.RpiCamera.Interval.Duration = time.Duration(time.Minute * 10)
	}
	os.MkdirAll(config.RpiCamera.OutputDir, 0777)

	for name, cam := range config.Gphoto {
		//fmt.Println(name, cam.FilenamePrefix)
		if cam.FilenamePrefix == "" {
			config.Gphoto[name].FilenamePrefix = hostname + "-" + name
		}
		if cam.OutputDir == "" {
			config.Gphoto[name].OutputDir = filepath.Join("/var/lib/go-eyepi/", config.Gphoto[name].FilenamePrefix)
		}
		if cam.Interval.Duration <= time.Duration(time.Second) {
			config.Gphoto[name].Interval.Duration = time.Duration(time.Minute * 10)
		}
		config.Gphoto[name].mutex = mutex
		port, err := cam.ResetUsb()
		if err != nil {
			cam.Enable = false
		}
		config.Gphoto[name].USBPort = port
		os.MkdirAll(cam.OutputDir, 0777)
	}

	printCameras(config.RpiCamera)
	for _, cam := range config.Gphoto {
		printCameras(cam)
	}
}

func initLogging(
	infoHandle io.Writer,
	warningHandle io.Writer,
	errorHandle io.Writer) {

	Info = log.New(infoHandle,
		"[INFO] ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Warning = log.New(warningHandle,
		"[WARNING] ",
		log.Ldate|log.Ltime|log.Lshortfile)

	Error = log.New(errorHandle,
		"[ERROR] ",
		log.Ldate|log.Ltime|log.Lshortfile)
}
func init() {
	infoLogger, _ := syslog.New(syslog.LOG_NOTICE, "eyepi")
	warningLogger, _ := syslog.New(syslog.LOG_NOTICE, "eyepi")
	errLogger, _ := syslog.New(syslog.LOG_NOTICE, "eyepi")
	initLogging(infoLogger, warningLogger, errLogger)

	mutex = &sync.Mutex{}
	CreateCameras()
}

func main() {
	telegrafClient, telegrafClientErr := telegraf.NewUnix("/tmp/telegraf.sock")
	if telegrafClientErr != nil {
		Error.Println("Cannot create telegraf client QWTF!!!?: ", telegrafClientErr)
	}

	stopChan := make(chan bool)
	timingChan := make(chan telegraf.Measurement)
	for _, cam := range config.Gphoto {
		go cam.RunWait(stopChan, timingChan)
	}
	go config.RpiCamera.RunWait(stopChan, timingChan)
	usbChan := make(chan bool, 1)

	go RunWaitUdev(usbChan)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		Error.Fatal(err)
	}
	defer watcher.Close()
	watcher.Add(CONFIGPATH)

	for {
		select {
		case measurement := <-timingChan:
			if telegrafClientErr == nil {
				telegrafClient.Write(measurement)
			}

		case <-usbChan:
			for range config.Gphoto {
				stopChan <- true
			}
			stopChan <- true
			CreateCameras()
			for len(stopChan) > 0 {
				<-stopChan
			}
			for len(timingChan) > 0 {
				<-timingChan
			}
			for len(usbChan) > 0 {
				<-usbChan
			}
			for _, cam := range config.Gphoto {
				go cam.RunWait(stopChan, timingChan)
			}
			go config.RpiCamera.RunWait(stopChan, timingChan)
		case event := <-watcher.Events:
			if event.Op&fsnotify.Write == fsnotify.Write{
				for range config.Gphoto {
					stopChan <- true
				}
				stopChan <- true
				CreateCameras()
				for len(stopChan) > 0 {
					<-stopChan
				}
				for len(timingChan) > 0 {
					<-timingChan
				}
				for len(usbChan) > 0 {
					<-usbChan
				}
				for _, cam := range config.Gphoto {
					go cam.RunWait(stopChan, timingChan)
				}
				go config.RpiCamera.RunWait(stopChan, timingChan)
			}
		}
	}

}
