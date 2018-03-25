package main

import (
	"github.com/BurntSushi/toml"
	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"github.com/mdaffin/go-telegraf"
	"golang.org/x/image/font/gofont/goregular"
	_ "golang.org/x/image/bmp" // import for TimestampLast
	_ "golang.org/x/image/tiff"
	"image/jpeg"
	"github.com/fsnotify/fsnotify"
	"io"
	"log"
	"log/syslog"
	"os"
	"path/filepath"
	"time"
	"sync"
	//"github.com/pkg/profile"
)

//CONFIGPATH system path to configuration file
const CONFIGPATH = "/etc/go-eyepi/go-eyepi.conf"

var (
	infoLog *log.Logger
	warnLog *log.Logger
	errLog  *log.Logger
	config  *GlobalConfig
	mutex *sync.Mutex
	// Version and Built are both informational
	Version string
	// Built see above
	Built string
)

//GlobalConfig type to support the configuration of all cameras managed
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

//CopyFile copies files from src to dest returns any error
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
	return err
}

//TimestampLast takes a jpeg image path, adds a timestamp to the image and writes it out to outputPath
func TimestampLast(path, outputPathJpeg string) (err error) {
	timestamp := time.Now().Format(time.UnixDate)
	img, err := gg.LoadImage(path)
	if err != nil {
		return
	}
	b := img.Bounds()
	dc := gg.NewContext(b.Dx(), b.Dy())

	dc.SetRGB(1, 1, 1)
	dc.Clear()
	dc.SetRGB(1, 0, 0)
	font, err := truetype.Parse(goregular.TTF)
	if err != nil {
		panic("")
	}
	face := truetype.NewFace(font, &truetype.Options{
		Size: 150,
	})
	dc.SetFontFace(face)

	dc.DrawImage(img, 0, 0)
	dc.DrawStringAnchored(timestamp, 50.0, float64(b.Dy()-50), 0.0, 0.0)

	out, err := os.Create(outputPathJpeg)
	if err != nil {
		return
	}
	defer out.Close()

	jpeg.Encode(out, dc.Image(), &jpeg.Options{jpeg.DefaultQuality})

	return
}

func printCameras(cam interface{}) {
	switch c := cam.(type) {
	case *GphotoCamera:
		infoLog.Printf("Camera %s \n\t%t\n\t%s\n\t%s\n\t%s\n-------\n", c.FilenamePrefix, c.Enable, c.Interval, c.OutputDir, c.USBPort)
	case *RaspberryPiCamera:
		infoLog.Printf("Camera %s \n\t%t\n\t%s\n\t%s\n-------\n", c.FilenamePrefix, c.Enable, c.Interval, c.OutputDir)
	default:
		infoLog.Println("Idk")
	}
}

func reloadCameraConfig() {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	config = &GlobalConfig{
		"2006_01_02_15_04_05",
		&RaspberryPiCamera{
			Enable:         true,
			Interval:       duration{time.Duration(time.Minute * 5)},
			FilenamePrefix: "",
			OutputDir:      "",
		},
		make(map[string]*GphotoCamera),
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
		port, err := cam.resetUsb()
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

	infoLog = log.New(infoHandle,
		"[INFO] ",
		log.Ldate|log.Ltime|log.Lshortfile)

	warnLog = log.New(warningHandle,
		"[WARNING] ",
		log.Ldate|log.Ltime|log.Lshortfile)

	errLog = log.New(errorHandle,
		"[ERROR] ",
		log.Ldate|log.Ltime|log.Lshortfile)
}
func init() {
	infoLogger, _ := syslog.New(syslog.LOG_NOTICE, "eyepi")
	warningLogger, _ := syslog.New(syslog.LOG_NOTICE, "eyepi")
	errLogger, _ := syslog.New(syslog.LOG_NOTICE, "eyepi")
	initLogging(infoLogger, warningLogger, errLogger)
	infoLog.Printf("\n\tgo-eyepi v%s\n\tbuilt on %s", Version, Built)
	mutex = &sync.Mutex{}
	reloadCameraConfig()
}

func main() {
	//defer profile.Start(profile.MemProfile).Stop()

	telegrafClient, telegrafClientErr := telegraf.NewUnix("/tmp/telegraf.sock")
	if telegrafClientErr != nil {
		errLog.Println("Cannot create telegraf client QWTF!!!?: ", telegrafClientErr)
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
		errLog.Fatal(err)
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
			reloadCameraConfig()
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
		case event := <-watcher.Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				for range config.Gphoto {
					stopChan <- true
				}
				stopChan <- true
				reloadCameraConfig()
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
