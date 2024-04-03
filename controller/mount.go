package controller

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const PROGRAM_NAME = "GOS"

var alias = map[string]string{
	"12.5": "12.4",
	"15.8": "15.5",
}

func Mount(udid string) error {
	setupDevice(udid)
	info, err := dev.DeviceInfo()
	if err != nil {
		log.Println("Get device info error")
		return err
	}
	if strings.Index(info.ProductVersion, "17.") > 0 {
		return errors.New("iOS 17.x is not supported yet")
	} else {
		if signatures, err := dev.Images(); err == nil && len(signatures) > 0 {
			fmt.Println("DeveloperImage already mounted")
			return nil
		}

		version := info.ProductVersion
		if alias[version] != "" {
			version = alias[version]
		}
		imagePath := getAppDir("device-support/" + version)
		dmgPath := ""
		signaturePath := ""
		_, err := os.Stat(path.Join(imagePath, "DeveloperDiskImage.dmg"))
		if err != nil {
			// can't get local image
			//download img by version
			zipPath := downloadImage(version)
			if zipPath == "" {
				return errors.New("can't find properties image")
			}
			r, err := zip.OpenReader(zipPath)
			if err != nil {
				return errors.New("can't open zip file " + zipPath)
			}
			defer r.Close()
			//decompress to find the image

			for _, f := range r.File {
				fpath := filepath.Join(getAppDir("device-support/"), f.Name)
				if f.FileInfo().IsDir() {
					// 如果是目录，则创建目录
					os.MkdirAll(fpath, os.ModePerm)
				} else {
					// 如果是文件，则解压
					if f.Name == version+"/DeveloperDiskImage.dmg" || f.Name == version+"/DeveloperDiskImage.dmg.signature" {
						// 打开zip包中的文件
						inFile, err := f.Open()
						if err != nil {
							fmt.Println("Error opening file:", err)
							continue
						}

						// 创建目标文件
						outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
						if err != nil {
							fmt.Println("Error creating file:", err)
							inFile.Close()
							continue
						}

						// 复制文件内容
						_, err = io.Copy(outFile, inFile)
						if err != nil {
							fmt.Println("Error copying file:", err)
						}
						if f.Name == version+"/DeveloperDiskImage.dmg" {
							dmgPath = fpath
						} else if f.Name == version+"/DeveloperDiskImage.dmg.signature" {
							signaturePath = fpath
						}
						// 关闭文件
						inFile.Close()
						outFile.Close()
					}
				}
			}
		} else {
			dmgPath = path.Join(imagePath, "DeveloperDiskImage.dmg")
			signaturePath = path.Join(imagePath, "DeveloperDiskImage.dmg.signature")
		}
		if dmgPath != "" && signaturePath != "" {
			err = dev.MountDeveloperDiskImage(dmgPath, signaturePath)
			if err != nil {
				log.Println("mount image error")
				return err
			}
		}
	}
	return nil
}

func decompress(zipFile string) {
	r, err := zip.OpenReader(zipFile)
	if err != nil {
		panic(err)
	}
	defer r.Close()
}

func checkZipFile(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		return false

	} else {
		// 文件存在，检查是否是ZIP文件
		r, err := zip.OpenReader(path)
		defer r.Close()
		if err != nil {
			return false
		}
	}
	return true
}

func getImageUrlByVersion(version string) []string {
	// origin https://github.com/JinjunHan/iOSDeviceSupport
	// alternative repo: https://github.com/iGhibli/iOS-DeviceSupport
	githubRepo := "JinjunHan/iOSDeviceSupport"

	zipName := fmt.Sprintf("%s.zip", version)
	originUrl := fmt.Sprintf("https://github.com/%s/raw/master/iOSDeviceSupport/%s", githubRepo, zipName)
	mirrorUrl := strings.Replace(originUrl, "https://github.com", "https://tool.appetizer.io", 1)
	return []string{originUrl, mirrorUrl}
}

func getAppDir(paths string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	appdir := filepath.Join(home, "."+PROGRAM_NAME)
	if len(paths) > 0 {
		appdir = filepath.Join(appdir, paths)
	}

	return appdir
}

func urlRetrive(zipURL, localFile string) error {
	log.Printf("Download %s -> %s", zipURL, localFile)

	// Create a new file for the downloaded ZIP
	outputFile, err := os.Create(localFile)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	// Perform the HTTP GET request
	resp, err := http.Get(zipURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check if the HTTP response was successful
	if resp.StatusCode != http.StatusOK {
		return err
	}

	// Write the response body to the file
	_, err = io.Copy(outputFile, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

func downloadImage(version string) string {
	urls := getImageUrlByVersion(version)
	localDeviceSupport := getAppDir("device-support")
	imageZipPath := path.Join(localDeviceSupport, version+".zip")
	if !checkZipFile(imageZipPath) {
		for _, url := range urls {
			if urlRetrive(url, imageZipPath) == nil {
				if checkZipFile(imageZipPath) {
					return imageZipPath
				}
				log.Println("image file not zip")
			}
		}
	}
	return ""
}
