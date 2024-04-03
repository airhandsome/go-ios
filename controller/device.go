package controller

import (
	"github.com/airhandsome/go-ios/service"
	"log"
)

var dev service.Device

func GetDeviceList(udid string) []service.Device {
	setupUsbmux()
	devices, err := um.Devices()
	if err != nil {
		log.Fatal(err)
	}

	if len(devices) == 0 {
		log.Fatal("No Device")
	}

	if udid == "" {
		dev = devices[0]
	} else {
		for _, v := range devices {
			if v.Properties().UDID == udid {
				dev = v
				break
			}
		}

	}
	return devices

}

func setupDevice(udid string) {
	if dev == nil || udid != dev.Properties().UDID {
		GetDeviceList(udid)
	}
}

func GetDeviceInfo(udid string) *service.DeviceInfo {
	setupDevice(udid)
	info, err := dev.DeviceInfo()
	if err != nil {
		log.Println("get device info error")
		return nil
	}
	return info
}
