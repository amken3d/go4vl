package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"os"

	"github.com/vladimirvivien/go4vl/device"
	"github.com/vladimirvivien/go4vl/v4l2"
)

func main() {
	devName := "/dev/video0"
	totalFrames := 3
	width := 640
	height := 480
	flag.StringVar(&devName, "d", devName, "device name (path)")
	flag.IntVar(&totalFrames, "c", totalFrames, "number of frames to caputure")
	flag.IntVar(&width, "w", width, "picture width")
	flag.IntVar(&height, "h", height, "picture height")
	flag.Parse()

	// open device
	device, err := device.Open(devName)
	if err != nil {
		log.Fatalf("failed to open device: %s", err)
	}
	defer device.Close()

	fps, err := device.GetFrameRate()
	if err != nil {
		log.Fatalf("failed to get framerate: %s", err)
	}

	allFormats, err := device.GetAllFormatDescriptions()
	if err != nil {
		log.Fatal("failed to get format descriptions: %s", err)
	}

	// helper function to search for format descriptions
	findPreferredFmt := func(pixEncoding v4l2.FourCCType) (v4l2.FormatDescription, error) {
		for _, desc := range allFormats {
			if desc.PixelFormat == pixEncoding {
				return desc, nil
			}
		}
		return v4l2.FormatDescription{}, fmt.Errorf("preferred format not found")
	}

	// search for preferred formats
	preferredPixFmts := []v4l2.FourCCType{v4l2.PixelFmtMPEG, v4l2.PixelFmtMJPEG, v4l2.PixelFmtJPEG, v4l2.PixelFmtYUYV}

	var sizes []v4l2.FrameSizeEnum
	var preferredFmt v4l2.FormatDescription
	for _, pixFmt := range preferredPixFmts {
		preferredFmt, err = findPreferredFmt(pixFmt)
		if err != nil {
			continue
		}
		sizes, err = device.GetFormatFrameSizes(preferredFmt.PixelFormat)
		if err != nil {
			continue
		}
		if sizes != nil && len(sizes) > 0 {
			break
		}
	}

	if sizes == nil || len(sizes) == 0 {
		log.Fatal("no appropriate sizes found for specified format: %s", preferredFmt)
	}

	// select size
	prefSize := sizes[0]
	prefSizeFound := false
	for _, size := range sizes {
		if size.Size.MinWidth == uint32(width) && size.Size.MinHeight == uint32(height) {
			prefSizeFound = true
			break
		}
	}

	if !prefSizeFound {
		log.Printf("Specified size %dx%d not supported, setting to %dx%d: %s", width, height, prefSize.Size.MinWidth, prefSize.Size.MinHeight)
		width = int(prefSize.Size.MinWidth)
		height = int(prefSize.Size.MinHeight)
	}

	log.Printf("Found preferred size: %#v", prefSize)

	// configure device with preferred fmt
	if err := device.SetPixFormat(v4l2.PixFormat{
		Width:       prefSize.Size.MinWidth,
		Height:      prefSize.Size.MinHeight,
		PixelFormat: preferredFmt.PixelFormat,
		Field:       v4l2.FieldNone,
	}); err != nil {
		log.Fatalf("failed to set format: %s", err)
	}

	pixFmt, err := device.GetPixFormat()
	if err != nil {
		log.Fatalf("failed to get format: %s", err)
	}
	log.Printf("Pixel format set to [%s]", pixFmt)

	// start stream
	ctx, cancel := context.WithCancel(context.TODO())
	if err := device.Start(ctx); err != nil {
		log.Fatalf("failed to stream: %s", err)
	}

	// process frames from capture channel
	count := 0
	log.Printf("Capturing %d frames (buffers: %d, %d fps)...", totalFrames, device.BufferCount(), fps)
	for frame := range device.GetOutput() {
		if count >= totalFrames {
			break
		}
		count++

		if len(frame) == 0 {
			log.Println("received frame size 0")
			continue
		}

		log.Printf("captured %d bytes", len(frame))
		img, fmtName, err := image.Decode(bytes.NewReader(frame))
		if err != nil {
			log.Printf("failed to decode jpeg: %s", err)
			continue
		}
		log.Printf("decoded image format: %s", fmtName)

		var imgBuf bytes.Buffer
		if err := jpeg.Encode(&imgBuf, img, nil); err != nil {
			log.Printf("failed to encode jpeg: %s", err)
			continue
		}

		fileName := fmt.Sprintf("capture_%d.jpg", count)
		file, err := os.Create(fileName)
		if err != nil {
			log.Printf("failed to create file %s: %s", fileName, err)
			continue
		}

		if _, err := file.Write(frame); err != nil {
			log.Printf("failed to write file %s: %s", fileName, err)
			file.Close()
			continue
		}
		log.Printf("Saved file: %s", fileName)
		if err := file.Close(); err != nil {
			log.Printf("failed to close file %s: %s", fileName, err)
		}
	}

	cancel() // stop capture
	if err := device.Stop(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("Done.")

}
