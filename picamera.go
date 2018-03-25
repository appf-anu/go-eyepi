package main

import (
	//"os"
	// "github.com/garyhouston/exif44"
	// "github.com/garyhouston/tiff66"
	"bufio"
	"bytes"
	"fmt"
	"github.com/mdaffin/go-telegraf"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

//RaspberryPiCamera type to support the raspberry pi camera through the cli
type RaspberryPiCamera struct {
	Enable         bool
	Interval       duration
	FilenamePrefix string
	OutputDir      string
	ImageTypes     []string
	args           *RaspiStillArgs
}

//RunWait start the camera on an interval capture
func (cam *RaspberryPiCamera) RunWait(stop <-chan bool, captureTime chan<- telegraf.Measurement) {

	waitForNextTimepoint := time.After(time.Until(time.Now().Add(cam.Interval.Duration).Truncate(cam.Interval.Duration)))

	select {
	case <-stop:
		return
	case <-waitForNextTimepoint:
		break
	}

	ticker := time.NewTicker(cam.Interval.Duration)
	start := time.Now()
	timestamp := time.Now().Truncate(cam.Interval.Duration).Format(config.TimestampFormat)
	err := cam.capture(timestamp)

	if err != nil {
		errLog.Println("error capturing: ", err)
	} else {
		m := telegraf.MeasureFloat64("camera", "timing_capture_s", time.Since(start).Seconds())
		m.AddTag("camera_name", cam.FilenamePrefix)
		captureTime <- m
		infoLog.Printf("capture took %s\n", time.Since(start))
	}
	for {
		select {
		case t := <-ticker.C:
			if cam.Enable{
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
					infoLog.Printf("capture took %s\n", time.Since(start))
				}
			}
		case <-stop:
			return
		}
	}
}

// is this function whats causing memory errors?
func (cam *RaspberryPiCamera) getImage() ([]byte, error) {
	if cam.args == nil {
		cam.args = NewRaspistillArgs()
	}
	cmd := createCommand(cam.args)
	return cmd.Output()
}

// does this work the same way?
//func (cam *RaspberryPiCamera) getImage() ([]byte, error) {
//	if cam.args == nil {
//		cam.args = NewRaspistillArgs()
//	}
//	cmd := createCommand(cam.args)
//	stdout, err := cmd.StdoutPipe()
//
//	if err := cmd.Start(); err != nil {
//		return []byte{}, err
//	}
//
//	var buf bytes.Buffer
//	defer buf.Reset()
//	_, err = buf.ReadFrom(stdout)
//	if err != nil{
//		return []byte{}, err
//	}
//
//	output := buf.Bytes()
//
//	if err := cmd.Wait(); err != nil {
//		return []byte{}, err
//	}
//
//	return output, err
//}


func (cam *RaspberryPiCamera) capture(timestamp string) error {
	if len(cam.ImageTypes) == 0 {
		cam.ImageTypes = []string{"jpg", "tiff"}
	}
	for _, fileType := range cam.ImageTypes {
		var image []byte
		var err error
		filePath := filepath.Join(cam.OutputDir, fmt.Sprintf("%s_%s.%s", cam.FilenamePrefix, timestamp, fileType))
		filePathLast := filepath.Join(cam.OutputDir, fmt.Sprintf("last_image.%s", fileType))
		if stringInSlice(fileType ,[]string{"jpeg","jpg"}) {
			cam.args = &RaspiStillArgs{Encoding: "jpg", Quality: 100, Brightness: defBrightness}
			image, err = cam.getImage()
			if err != nil {
				return err
			}
		} else if stringInSlice(fileType, []string{"tif", "tiff"}) {
			cam.args = &RaspiStillArgs{Encoding: "bmp", Brightness: defBrightness}
			imageBMP, err := cam.getImage()
			if err != nil {
				return err
			}
			// convert bmp to tiff
			bmpreader := bytes.NewReader(imageBMP)
			bmpimage, err := bmp.Decode(bmpreader)
			if err != nil {
				return err
			}

			var tiffbytes bytes.Buffer
			tiffwriter := bufio.NewWriter(&tiffbytes)
			err = tiff.Encode(tiffwriter, bmpimage, &tiff.Options{Compression: tiff.Deflate})
			if err != nil {
				return err
			}
			tiffwriter.Flush()
			image = tiffbytes.Bytes()

		} else if stringInSlice(fileType, []string{"bmp", "png", "gif"}) {
			cam.args = &RaspiStillArgs{Encoding: fileType, Brightness: defBrightness}
			image, err = cam.getImage()
			if err != nil {
				return err
			}
		}

		if err = ioutil.WriteFile(filePath, image, 0665); err != nil {
			return err
		}

		if fileType == "jpg" {
			// we actually dont want to fail here or anywhere
			TimestampLast(filePath, filePathLast)
		} else {
			CopyFile(filePath, filePathLast)
		}

	}
	return nil
}


//func (cam *RaspberryPiCamera) capture(timestamp string) error {
//	// the filepath must resolve with %C for cameras that return multiple images (like canons jpg+raw)
//	filePath := filepath.Join(cam.OutputDir, fmt.Sprintf("%s_%s.jpg", cam.FilenamePrefix, timestamp))
//	lastJpegPath := filepath.Join(cam.OutputDir, fmt.Sprintf("last_image.jpg"))
//
//	cmd := cam.createCaptureCommand(filePath)
//
//	err := cmd.Run()
//
//	if err != nil {
//		return err
//	}
//
//	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
//		if err = TimestampLast(filePath, lastJpegPath); err != nil {
//			return err
//		}
//	}
//
//	return nil
//}
//
//func (cam *RaspberryPiCamera) createCaptureCommand(targetFilename string) *exec.Cmd {
//	filenameArg := fmt.Sprintf("--output=%s", targetFilename)
//	command := exec.Command("/opt/vc/bin/raspistill",
//		"-o ", filenameArg)
//
//	return command
//}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

//this is all ripped straight from https://github.com/technomancers/piCamera, modified for raspistill
const (
	defBrightness = 50
	defMode       = 0
	defEncoding   = "jpg"
	defQuality    = 75
)

//RaspiStillArgs are arguments used to set camera settings for the desired output
//https://www.raspberrypi.org/documentation/raspbian/applications/camera.md
type RaspiStillArgs struct {
	Encoding      string // Encoding to use for output file (jpg, bmp, gif, png)
	HorizFlip     bool   // flip the image horizontally
	VertFlip      bool   // flip the camera vertically
	Width         int    // width of the image
	Height        int    // height of the image
	Sharpness     int    // change the sharpness of the camera (-100 , 100 DEF 0)
	Contrast      int    // change the contrast of the camera (-100 , 100 DEF 0)
	Brightness    int    // change the brightness of the camera (0 , 100 DEF 50)
	Saturation    int    // change the saturation of the camera (-100 , 100 DEF 0)
	ISO           int    // change the sensitivity the camera is to light (100 , 800 DEF 100)
	EV            int    // Slightly under or over expose the camera (-10 , 10 DEF 0)
	Bitrate       int    // set the bitrate in bits per second. Max is 25000000
	Quantization  int    // set Quantization parameter
	Quality       int    // set jpeg quality <0 to 100>
	Mode          int    // set the mode of the camera by checking the documentation
	ShutterSpeed  int    // set the shutter speed in microseconds (Max 6000000)
	Rotation      int    // set the rotation of the image. (0, 90, 180, 270)
	Annotate      string // annotate the image according to the documentation
	AnnotateExtra string // annotate the image according to the documentation
}

//NewRaspistillArgs returns a RaspividArgs with the default settings
func NewRaspistillArgs() *RaspiStillArgs {
	return &RaspiStillArgs{
		Brightness: defBrightness,
		Mode:       defMode,
	}
}

func createCommand(args *RaspiStillArgs) *exec.Cmd {
	command := exec.Command("/opt/vc/bin/raspistill", "-t", "5")
	var final []string
	if args.Width != 0 {
		final = append(final, "-w", strconv.Itoa(args.Width))
	}
	if args.Height != 0 {
		final = append(final, "-h", strconv.Itoa(args.Height))
	}
	if args.HorizFlip {
		final = append(final, "-hf")
	}
	if args.VertFlip {
		final = append(final, "-vf")
	}
	if args.Sharpness != 0 {
		final = append(final, "-sh", strconv.Itoa(args.Sharpness))
	}
	if args.Contrast != 0 {
		final = append(final, "-co", strconv.Itoa(args.Contrast))
	}
	if args.Brightness != defBrightness {
		final = append(final, "-br", strconv.Itoa(args.Brightness))
	}
	if args.Encoding != defEncoding {
		final = append(final, "-e", args.Encoding)
	}
	if args.Saturation != 0 {
		final = append(final, "-sa", strconv.Itoa(args.Saturation))
	}
	if args.ISO != 0 {
		final = append(final, "-ISO", strconv.Itoa(args.ISO))
	}
	if args.EV != 0 {
		final = append(final, "-ev", strconv.Itoa(args.EV))
	}
	if args.Rotation != 0 {
		final = append(final, "-rot", strconv.Itoa(args.Rotation))
	}
	if args.ShutterSpeed != 0 {
		final = append(final, "-ss", strconv.Itoa(args.ShutterSpeed))
	}
	if args.Mode != defMode {
		final = append(final, "-md", strconv.Itoa(args.Mode))
	}
	if args.AnnotateExtra != "" {
		final = append(final, "-ae", args.AnnotateExtra)
	}
	if args.Annotate != "" {
		final = append(final, "-a", args.Annotate)
	}
	if args.Quality != defQuality && args.Encoding == "jpg" {
		final = append(final, "-q", strconv.Itoa(args.Quality))
	}
	final = append(final, "-o", "-")
	command.Args = append(command.Args, final...)
	return command
}

// EXIF

// type readExifHandle struct {
// 	tiffbytes *bytes.Buffer
// }
// //ReadExif Exif handler, needs to be exported
// func (readexif readExifHandle) ReadExif(format exif44.FileFormat, imageIdx uint32, exif exif44.Exif, err error) error {
// 	if err != nil {
// 		return err
// 	}
//
// 	buf := readexif.tiffbytes.Bytes()
// 	valid, order, ifdPos := tiff66.GetHeader(buf)
// 	if !valid {
// 		fmt.Fprintln(os.Stderr, "Not a valid tiff file wtf")
// 		fmt.Fprintln(os.Stderr, err)
// 		return nil
// 	}
// 	root, err := tiff66.GetIFDTree(buf, order, ifdPos, tiff66.TIFFSpace)
// 	if err != nil {
// 		return err
// 	}
//
// 	fmt.Println("-----------")
// 	for _, field := range exif.TIFF.Fields {
// 		fmt.Println(field)
// 	}
// 	fmt.Println("-----------")
// 	root.AddFields(exif.TIFF.Fields)
// 	//root.DeleteEmptyIFDs()
// 	root.Fix()
// 	for _, node := range root.SubIFDs {
// 		fmt.Println(node.Tag)
// 		for _, field := range node.Node.Fields {
// 			field.Print(root.Order, tiff66.TagNames, 0)
// 		}
// 		fmt.Println("-----------")
// 	}
//
// 	fileSize := tiff66.HeaderSize + root.TreeSize()
// 	out := make([]byte, fileSize)
// 	tiff66.PutHeader(out, order, tiff66.HeaderSize)
// 	_, err = root.PutIFDTree(out, tiff66.HeaderSize)
// 	if err != nil {
// 		return err
// 	}
// 	var tiffbytes bytes.Buffer
// 	tiffwriter := bufio.NewWriter(&tiffbytes)
// 	tiffwriter.Write(out[:])
// 	tiffwriter.Flush()
//
// 	return nil
// }
