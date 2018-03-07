package main

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
	"reflect"
	"fmt"
	"sort"
	//"github.com/kylelemons/godebug/pretty"
)

const (
	BASE_DEVPATH = "/sys/devices"
)

type Device struct {
	KObj string
	Env  map[string]string
}

func RunWaitUdev(changed chan<- bool) {
	devices, err := ExistingDevices("usb")
	if err != nil {
		fmt.Println(err)
	}
	for {
		time.Sleep(time.Second * 5)
		ndevices, err := ExistingDevices("usb")
		if err != nil {
			fmt.Println(err)
		}
		if !reflect.DeepEqual(devices, ndevices) {
			devchanged := len(devices) - len(ndevices)
			if devchanged > 0 {
				Warning.Printf("%d devices removed\n", devchanged)
			} else {
				Warning.Printf("%d devices added\n", devchanged * -1)
			}

			select {
			case changed <- true:
			default:
			}
			devices = ndevices
		}
	}
}

// ExistingDevices return all plugged devices matched by the matcher
// All uevent files inside /sys/devices is crawled to match right env values
func ExistingDevices(subsystem string) ([]Device, error) {
	devices := make([]Device, 0)
	err := filepath.Walk(BASE_DEVPATH, func(path string, info os.FileInfo, err error) error {

		if err != nil {
			return err
		}

		if info.IsDir() || info.Name() != "uevent" {
			return nil
		}

		env, err := getEventFromUEventFile(path)
		if err != nil {
			return err
		}

		kObj := filepath.Dir(path)

		// Append to env subsystem if existing
		subsys := filepath.Join(kObj, "subsystem")
		if link, err := os.Readlink(subsys); err == nil {
			if ss := filepath.Base(link); strings.Contains(ss, subsystem) {
				env["SUBSYSTEM"] = ss
			} else {
				return nil
			}

		}

		// get human readable product name.
		product := filepath.Join(kObj, "product")
		if _, err := os.Stat(product); err == nil {
			b, err := ioutil.ReadFile(product)
			if err != nil {
				return err
			}
			env["PRODUCTNAME"] = string(b)
		}

		if len(env) < 1 {
			return nil
		}

		if env["DRIVER"] != "usb" {
			return nil
		}
		devices = append(devices,
			Device{
				KObj: kObj,
				Env:  env,
			})

		return nil
	})

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].KObj > devices[j].KObj
	})
	if err != nil {
		return devices, err
	}

	return devices, nil
}

// getEventFromUEventFile return all env var define in file
// syntax: name=value for each line
// Function use for /sys/.../uevent files
func getEventFromUEventFile(path string) (rv map[string]string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	rv = make(map[string]string, 0)
	buf := bufio.NewScanner(bytes.NewBuffer(data))

	var line string
	for buf.Scan() {
		line = buf.Text()
		field := strings.SplitN(line, "=", 2)
		if len(field) != 2 {
			return
		}
		rv[field[0]] = field[1]
	}

	return
}
