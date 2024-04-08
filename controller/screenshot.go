package controller

import (
	"fmt"
	"github.com/airhandsome/go-ios/service"
	"image"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"path"
)

var screenshotSrv service.Screenshot

func setupScreenshotSrv() {
	setupLockdownSrv()

	var err error
	if lockdownSrv, err = dev.LockdownService(); err != nil {
		log.Fatal(err)
	}

	if screenshotSrv, err = lockdownSrv.ScreenshotService(); err != nil {
		log.Fatal(err)
	}
}

func Screenshot(name string) bool {
	setupScreenshotSrv()
	raw, _ := dev.Screenshot()

	// raw, err := dev.Screenshot()
	//raw, err := screenshotSrv.Take()
	//if err != nil {
	//	log.Fatal(err)
	//	return false
	//}
	//_ = raw

	img, format, err := image.Decode(raw)
	if err != nil {
		log.Fatal(err)
		return false
	}

	CurrentDir, _ := os.Getwd()
	ImageDir := path.Join(CurrentDir, "Image")
	_, err = os.Stat(ImageDir)
	if err != nil {
		if os.IsNotExist(err) {
			// 路径不存在，创建目录
			err := os.Mkdir(ImageDir, 0755)
			if err != nil {
				fmt.Printf("创建目录失败： %s\n", err)
				return false
			}
		} else {
			// 路径存在，但不是一个目录
			fmt.Printf("路径 %s 存在，但不是一个目录\n", ImageDir)
		}
	}

	if name == "" {
		name = dev.Properties().UDID + "." + format
	}
	file, err := os.Create(path.Join(ImageDir, name))
	if err != nil {
		log.Fatal(err)
		return false
	}
	defer func() { _ = file.Close() }()
	switch format {
	case "png":
		err = png.Encode(file, img)
	case "jpeg":
		err = jpeg.Encode(file, img, nil)
	}
	if err != nil {
		log.Fatal(err)
		return false
	}
	log.Println(file.Name())
	return true
}
