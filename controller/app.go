package controller

import (
	"fmt"
	"github.com/airhandsome/go-ios/service"
)

func Launch(udid, bundleId string) (int, bool) {
	setupDevice(udid)
	pid, err := dev.AppLaunch(bundleId)
	if err != nil {
		fmt.Println(err)
		return -1, false
	}
	return pid, true
}

func List(udid string) ([]service.Application, error) {
	setupDevice(udid)
	apps, err := dev.AppList()
	if err != nil {
		return nil, err
	}
	return apps, nil
}
