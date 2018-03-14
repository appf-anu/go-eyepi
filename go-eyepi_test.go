package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

var /* const */ testFiles = []string{
	//"test-data/cr2/06.cr2", // not supported in x/image/tiff yet.
	//"test-data/cr2/07.cr2",
	//"test-data/cr2/550d.cr2",
	//"test-data/generated/bmptest.bmp", // not really supported.
	"test-data/generated/jpegtest.jpg",
	"test-data/generated/pngtest.png",
	"test-data/generated/tifftest.tif",
	//"test-data/generated/tifftest_jpeg.tif", // not currently supported in x/image/tiff
	"./test-data/generated/tifftest_lzw.tif",
	"test-data/generated/tifftest_zip.tif",
	"test-data/jpeg/0.jpg",
	"test-data/jpeg/1.jpg",
}

func TestTimestampLast(t *testing.T) {
	output_path_parent := "test-data/output"

	for _, path := range testFiles {
		output_path := filepath.Join(output_path_parent, path)
		os.MkdirAll(filepath.Dir(output_path), 0755)
		err := TimestampLast(path, output_path)
		if err != nil {
			t.Error(err)
		}
		if _, err := os.Stat(output_path); os.IsNotExist(err) {
			t.Error(err)
		}
	}
	os.RemoveAll(output_path_parent)
}

type reTest struct {
	data, expected string
}
type reMultiTest struct {
	data     string
	expected [][]byte
}

var snRegexData = []reTest{
	{
		`Label: Serial Number
		Readonly: 0
		Type: TEXT
		Current: cd6acfa090894f9bbe7b21037a49389b
		END
		`,
		"Current: cd6acfa090894f9bbe7b21037a49389b",
	},
	{
		`\asdfoaksddccc
			Readonly: 0
			Type: TEXT
			Current: cd6acfa090894f9bbe7b21037a49389b
		`,
		"Current: cd6acfa090894f9bbe7b21037a49389b",
	},
	{
		`Current: cd6acfa090894f9bbe7b21037a49389b,`,
		"Current: cd6acfa090894f9bbe7b21037a49389b",
	},
	{
		`\asdfoaksddccc
		Current: cd6acfa090894f9bbe7b21037a49389b`,
		"Current: cd6acfa090894f9bbe7b21037a49389b",
	},
}

var failsnRegexData = []reTest{
	{`*** Error ***
	An error occurred in the io-library ('I/O problem'): No error description available

	*** Error ***
	An error occurred in the io-library ('I/O problem'): No error description available
	*** Error (-7: 'I/O problem') ***

	For debugging messages, please use the --debug option.
	Debugging messages may help finding a solution to your problem.
	If you intend to send any error or debug messages to the gphoto
	developer mailing list <gphoto-devel@lists.sourceforge.net>, please run
	gphoto2 as follows:

		env LANG=C gphoto2 --debug --debug-logfile=my-logfile.txt --get-config serialnumber --port=usb:001,006

	Please make sure there is sufficient quoting around the arguments.
	`,
		""},
}

var usbRegexData = []reMultiTest{
	{`----------------------------------------------------------
	Canon EOS 650D                 usb:001,006
	Canon EOS 650D                 usb:001,007
	`, [][]byte{[]byte("usb:001,006"), []byte("usb:001,007")}},
	{`usb:001,006
	usb:001,007
	`, [][]byte{[]byte("usb:001,006"), []byte("usb:001,007")}},
	{`
	Canon EOS 650D                 usb:001,6
	Canon EOS 650D                 usb:001,007 `,
		[][]byte{[]byte("usb:001,6"), []byte("usb:001,007")}},
}

var failUsbRegexData = []reMultiTest{
	{
		`*** Error ***
		An error occurred in the io-library ('I/O problem'): No error description available

		*** Error ***
		An error occurred in the io-library ('I/O problem'): No error description available
		*** Error (-7: 'I/O problem') ***

		For debugging messages, please use the --debug option.
		Debugging messages may help finding a solution to your problem.
		If you intend to send any error or debug messages to the gphoto
		developer mailing list <gphoto-devel@lists.sourceforge.net>, please run
		gphoto2 as follows:

			env LANG=C gphoto2 --debug --debug-logfile=my-logfile.txt --get-config serialnumber --port=usb:001,006

		Please make sure there is sufficient quoting around the arguments.
		`, [][]byte{},
	},
}

func TestRegexes(t *testing.T) {
	for _, regexData := range snRegexData {
		regexReturn := snRegexp.Find([]byte(regexData.data))

		if string(regexReturn) != regexData.expected {
			t.Errorf("regex (%s): expected %s, actual %s", regexData.data, regexData.expected, string(regexReturn))
		}
	}

	for _, regexData := range usbRegexData {
		regexReturn := usbRegexp.FindAll([]byte(regexData.data), -1)

		if !reflect.DeepEqual(regexReturn, regexData.expected) {
			t.Errorf("regex (%s): expected %s, actual %s", regexData.data, regexData.expected, regexReturn)
		}
	}
}

var udevTestExpected = map[string]string{
	"MAJOR":   "189",
	"MINOR":   "258",
	"DEVNAME": "bus/usb/003/003",
	"DEVTYPE": "usb_device",
	"DRIVER":  "usb",
	"PRODUCT": "4d9/169/110",
	"TYPE":    "0/0/0",
	"BUSNUM":  "003",
	"DEVNUM":  "003",
}

func TestgetEventFromUEventFile(t *testing.T) {
	stuff, err := getEventFromUEventFile("test-data/uevent")
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(stuff, udevTestExpected) {
		t.Errorf("uevent unexpected. %s", stuff)
	}
}
