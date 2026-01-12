package test

import (
	"encoding/base64"
	"strings"
	"github.com/airhandsome/go-ios/service"
	"testing"
)

var imageMounterSrv service.ImageMounter

func setupImageMounterSrv(t *testing.T) {
	setupLockdownSrv(t)

	var err error
	if lockdownSrv, err = dev.LockdownService(); err != nil {
		t.Fatal(err)
	}

	// Once
	// dev.Images()
	if imageMounterSrv, err = lockdownSrv.ImageMounterService(); err != nil {
		t.Fatal(err)
	}
}

func Test_imageMounter_Images(t *testing.T) {
	setupImageMounterSrv(t)

	// Check iOS version first via lockdown
	version, err := dev.GetValue("", "ProductVersion")
	if err == nil && version != nil {
		if versionStr, ok := version.(string); ok {
			if strings.Index(versionStr, "17.") >= 0 || strings.Index(versionStr, "18.") >= 0 {
				t.Logf("📱 iOS %s detected - DeveloperDiskImage not needed", versionStr)
				t.Log("✅ iOS 17+ uses Developer Mode instead of mounting images")
				t.Log("💡 Enable: Settings → Privacy & Security → Developer Mode")
				t.Skip("Skipping image mount test for iOS 17+")
				return
			}
		}
	}

	// imageSignatures, err := dev.Images()
	imageSignatures, err := imageMounterSrv.Images("Developer")
	if err != nil {
		t.Fatal(err)
	}

	for i, imgSign := range imageSignatures {
		t.Logf("%2d, %s", i+1, base64.StdEncoding.EncodeToString(imgSign))
	}
}

func Test_imageMounter_UploadImageAndMount(t *testing.T) {
	setupImageMounterSrv(t)

	// Check iOS version first via lockdown
	version, err := dev.GetValue("", "ProductVersion")
	if err == nil && version != nil {
		if versionStr, ok := version.(string); ok {
			if strings.Index(versionStr, "17.") >= 0 || strings.Index(versionStr, "18.") >= 0 {
				t.Logf("📱 iOS %s detected - DeveloperDiskImage not needed", versionStr)
				t.Log("✅ iOS 17+ uses Developer Mode instead of mounting images")
				t.Log("💡 Enable: Settings → Privacy & Security → Developer Mode")
				t.Skip("Skipping image mount test for iOS 17+")
				return
			}
			t.Logf("iOS %s detected - will attempt to mount DeveloperDiskImage", versionStr)
		}
	}

	devImgPath := "/private/var/mobile/Media/PublicStaging/staging.dimage"
	dmgPath := "/Applications/Xcode.app/Contents/Developer/Platforms/iPhoneOS.platform/DeviceSupport/14.4/DeveloperDiskImage.dmg"
	signaturePath := "/Applications/Xcode.app/Contents/Developer/Platforms/iPhoneOS.platform/DeviceSupport/14.4/DeveloperDiskImage.dmg.signature"

	if err := imageMounterSrv.UploadImageAndMount("Developer", devImgPath, dmgPath, signaturePath); err != nil {
		t.Fatal(err)
	}
}
