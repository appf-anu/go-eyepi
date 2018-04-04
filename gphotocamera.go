package main

import (
	"bytes"
	"fmt"
	"github.com/mdaffin/go-telegraf"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const getSerialNumberRe = "Current: (\\w+)"
const getInUseUsbRe = "usb:(\\d+),(\\d+)"

var /* const */ snRegexp = regexp.MustCompile(getSerialNumberRe)
var /* const */ usbRegexp = regexp.MustCompile(getInUseUsbRe)

//GphotoCamera type to support gphoto2 cameras through cli interaction
type GphotoCamera struct {
	Enable                      bool
	Interval                    duration
	FilenamePrefix, OutputDir   string
	GphotoSerialNumber, USBPort string
}

//RunWait start the camera on an interval capture
func (cam *GphotoCamera) RunWait(stop <-chan bool, captureTime chan<- telegraf.Measurement) {
	waitForNextTimepoint := time.After(time.Until(time.Now().Add(cam.Interval.Duration).Truncate(cam.Interval.Duration)))

	select {
	case <-stop:
		return
	case <-waitForNextTimepoint:
		break
	}

	ticker := time.NewTicker(cam.Interval.Duration)
	defer ticker.Stop()

	start := time.Now()
	timestamp := time.Now().Truncate(cam.Interval.Duration).Format(config.TimestampFormat)
	err := cam.capture(timestamp)

	if err != nil {
		errLog.Println("error capturing: ", err)
	} else {
		m := telegraf.MeasureFloat64("camera", "timing_capture_s", time.Since(start).Seconds())
		m.AddTag("camera_name", cam.FilenamePrefix)
		captureTime <- m
		infoLog.Printf("%s capture took %s\n",cam.FilenamePrefix, time.Since(start))
	}
	for {
		select {
		case t := <-ticker.C:
			if cam.Enable {
				start := time.Now()
				// Truncate the current time to the interval duration
				timestamp := t.Truncate(cam.Interval.Duration).Format(config.TimestampFormat)
				err := cam.capture(timestamp)
				if err != nil {
					errLog.Println("error capturing: ", err)
				} else {

					m := telegraf.MeasureFloat64("camera", "timing_capture_s", time.Since(start).Seconds())
					m.AddTag("camera_name", cam.FilenamePrefix)
					captureTime <- m
					infoLog.Printf("%s capture took %s\n", cam.FilenamePrefix, time.Since(start))
				}
			}
		case <-stop:
			return
		}
	}
}

func (cam *GphotoCamera) capture(timestamp string) error {

	// the filepath must resolve with %C for cameras that return multiple images (like canons jpg+raw)
	filePath := filepath.Join(cam.OutputDir, fmt.Sprintf("%s_%s.%%C", cam.FilenamePrefix, timestamp))
	filePathJpeg := filepath.Join(cam.OutputDir, fmt.Sprintf("%s_%s.jpg", cam.FilenamePrefix, timestamp))
	lastJpegPath := filepath.Join(cam.OutputDir, fmt.Sprintf("last_image.jpg"))

	_, err := cam.resetUsb()
	if err != nil {
		return err
	}

	infoLog.Printf("capturing %s on %s\n to %s\n",
		cam.FilenamePrefix,
		cam.USBPort,
		filePath)

	command := cam.createCaptureCommand(filePath)

	//var outb, errb bytes.Buffer
	//defer outb.Reset()
	//defer errb.Reset()
	//command.Stdout = &outb
	//command.Stderr = &errb

	mutex.Lock()
	defer mutex.Unlock()

	err = command.Start()
	if err != nil {
		//errLog.Println(errb.String())
		return err
	}

	if err = command.Wait(); err != nil {
		//errLog.Println(errb.String())
		return err
	}

	if _, err := os.Stat(filePathJpeg); !os.IsNotExist(err) {
		if err = TimestampLast(filePathJpeg, lastJpegPath); err != nil {
			return err
		}
	}

	return nil
}

func (cam *GphotoCamera) checkUSBPort(port string) (bool, error) {
	usbPortArg := fmt.Sprintf("--port=%s", port)
	command := exec.Command("gphoto2", "--debug-loglevel=error",
		usbPortArg,
		"--get-config=serialnumber")
	mutex.Lock()
	defer mutex.Unlock()

	stdout, err := command.StdoutPipe()
	if err != nil {
		errLog.Println("error listing usb ports")
		return false, err
	}

	err = command.Start()

	if err != nil {
		errLog.Println("error listing usb ports")
		return false, err
	}

	var buf bytes.Buffer
	defer buf.Reset()

	if _, err = buf.ReadFrom(stdout); err != nil {
		return false, err
	}

	output := buf.Bytes()

	if err := command.Wait(); err != nil {
		errLog.Println("error checking usb port: ", string(output))
		return false, err
	}

	regexReturn := snRegexp.Find(output)
	if regexReturn == nil {
		return false, nil
	}
	if strings.Contains(string(regexReturn), cam.GphotoSerialNumber) {
		cam.USBPort = port
		return true, nil
	}
	return false, nil
}

func (cam *GphotoCamera) getAllUsbPorts() ([]string, error) {
	command := exec.Command("gphoto2", "--auto-detect")

	mutex.Lock()
	defer mutex.Unlock()

	stdout, err := command.StdoutPipe()
	if err != nil {
		errLog.Println("error listing usb ports")
		return []string{}, err
	}
	err = command.Start()

	if err != nil {
		errLog.Println("error listing usb ports")
		return []string{}, err
	}

	var buf bytes.Buffer
	defer buf.Reset()

	if _, err = buf.ReadFrom(stdout); err != nil {
		return []string{}, err
	}
	output := buf.Bytes()

	if err := command.Wait(); err != nil {
		errLog.Println("error listing usb ports: ", string(output))
		return []string{}, err
	}

	regexReturn := usbRegexp.FindAll(output, -1)
	if regexReturn == nil {
		return []string{}, nil
	}

	rstrings := make([]string, len(regexReturn))
	for i, usbBytes := range regexReturn {
		rstrings[i] = string(usbBytes)
	}

	return rstrings, nil
}

func (cam *GphotoCamera) resetUsb() (string, error) {
	usbPorts, err := cam.getAllUsbPorts()

	if err != nil {
		return "", err
	}

	for _, port := range usbPorts {
		valid, err := cam.checkUSBPort(port)
		if err != nil {
			errLog.Println("error getting usb port", err)
		}

		if valid {
			cam.USBPort = port
			return port, nil
		}
	}
	return "", fmt.Errorf("Gphoto2 camera with serialnumber %s not detected", cam.GphotoSerialNumber)
}

func (cam *GphotoCamera) createCaptureCommand(targetFilename string) *exec.Cmd {
	filenameArg := fmt.Sprintf("--filename=%s", targetFilename)
	command := exec.Command("gphoto2",
		"--port", cam.USBPort,
		"--set-config=capturetarget=0",
		"--force-overwrite",
		"--capture-image-and-download",
		filenameArg)

	return command
}

//RunGphoto2Command allows runnning of arbitrary gphoto2 commands
func (cam *GphotoCamera) RunGphoto2Command(args ...string) (string, error) {
	valid, err := cam.checkUSBPort(cam.USBPort)
	if valid && err == nil {
		usbPort, err := cam.resetUsb()
		if err != nil {
			return "", err
		}
		cam.USBPort = usbPort
	}
	if err != nil {
		return "", err
	}

	args = append([]string{"--debug-loglevel=error", "--port", cam.USBPort}, args...)
	mutex.Lock()
	command := exec.Command("gphoto2", args...)
	mutex.Unlock()
	output, err := command.Output()
	if err != nil {
		return string(output), err
	}
	return string(output), err

}
