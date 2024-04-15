package controller

import (
	"fmt"
	"github.com/airhandsome/go-ios/service"
	"os"
	"os/signal"
	"syscall"
)

func ProfilerStart(udid, bundleId string, options []service.DataType) {
	setupDevice(udid)

	data, err := dev.ProfilerStart(options, bundleId)
	if err != nil {
		return
	}
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGKILL)
	for {
		select {
		case <-signalChan:
			ProfilerStop()
			break
		case d := <-data:
			fmt.Println(d)
		}
	}
}

func ProfilerStop() {
	dev.ProfilerStop()
}

func GraphicInfo() {
	setupDevice("")
	dev.GraphicInfo()
}
