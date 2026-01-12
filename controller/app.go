package controller

import (
	"fmt"
	"strings"
	"github.com/airhandsome/go-ios/service"
)

func Launch(udid, bundleId string) (int, bool) {
	setupDevice(udid)
	
	// Check iOS version and provide helpful hints
	info, _ := dev.DeviceInfo()
	if info != nil && (strings.Index(info.ProductVersion, "17.") >= 0 || strings.Index(info.ProductVersion, "18.") >= 0) {
		fmt.Println("📱 iOS 17+ detected")
		fmt.Println("💡 Make sure 'Developer Mode' is enabled on your device")
	}
	
	pid, err := dev.AppLaunch(bundleId)
	if err != nil {
		fmt.Printf("❌ Launch error: %v\n", err)
		if info != nil && strings.Index(info.ProductVersion, "17.") >= 0 {
			fmt.Println("")
			fmt.Println("⚠️  If you see permission errors, please ensure:")
			fmt.Println("   1. Developer Mode is enabled: Settings → Privacy & Security → Developer Mode")
			fmt.Println("   2. Your device has been restarted after enabling Developer Mode")
			fmt.Println("   3. You've trusted this computer on your device")
			fmt.Println("   4. Your device is unlocked")
		}
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
