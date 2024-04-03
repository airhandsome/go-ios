package controller

import (
	"fmt"
	"github.com/airhandsome/go-ios/service"
	"time"
)

func ProfilerStart(udid, bundleId string, options []service.DataType) {
	setupDevice(udid)

	data, err := dev.ProfilerStart(options, bundleId)
	if err != nil {
		return
	}
	timer := time.NewTimer(time.Second * 20)
	for {
		select {
		case <-timer.C:
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
