package controller

import (
	"github.com/airhandsome/go-ios/pkg/libimobiledevice"
	"github.com/airhandsome/go-ios/service"
	"log"
	"testing"
	"time"
)

var um service.Usbmux

func setupUsbmux() {
	var err error
	um, err = service.NewUsbmux()
	if err != nil {
		log.Fatal(err)
	}
}

func usbmux_Devices() {
	setupUsbmux()

	devices, err := um.Devices()
	if err != nil {
		log.Fatal(err)
	}

	for _, dev := range devices {
		log.Println(dev.Properties().SerialNumber, dev.Properties().ProductID, dev.Properties().DeviceID)
	}
}

func usbmux_ReadBUID(t *testing.T) {
	setupUsbmux()

	buid, err := um.ReadBUID()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf(buid)
}

func usbmux_Listen() {
	setupUsbmux()

	devNotifier := make(chan service.Device)
	cancelFunc, err := um.Listen(devNotifier)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		time.Sleep(20 * time.Second)
		cancelFunc()
	}()

	for dev := range devNotifier {
		if dev.Properties().ConnectionType != "" {
			log.Println(dev.Properties().SerialNumber, dev.Properties().ProductID, dev.Properties().DeviceID)
		} else {
			log.Println(libimobiledevice.MessageTypeDeviceRemove, dev.Properties().DeviceID)
		}
	}

	time.Sleep(5 * time.Second)
	log.Println("Done")
}
