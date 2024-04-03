package test

import (
	"fmt"
	"github.com/airhandsome/go-ios/service"
	"os"
	"testing"
)

var crashReportMoverSrv service.CrashReportMover

func setupCrashReportMoverSrv(t *testing.T) {
	setupLockdownSrv(t)

	var err error
	if lockdownSrv, err = dev.LockdownService(); err != nil {
		t.Fatal(err)
	}

	if crashReportMoverSrv, err = lockdownSrv.CrashReportMoverService(); err != nil {
		t.Fatal(err)
	}
}

func Test_crashReportMover_Move(t *testing.T) {
	setupCrashReportMoverSrv(t)

	service.SetDebug(true)
	userHomeDir, _ := os.UserHomeDir()
	// err := crashReportMoverSrv.Move(userHomeDir + "/Documents/temp/2021-04/out_gidevice")
	// err := crashReportMoverSrv.Move(userHomeDir+"/Documents/temp/2021-04/out_gidevice",
	err := crashReportMoverSrv.Move(userHomeDir+"/Documents/temp/2021-04/out_gidevice_extract",
		service.WithKeepCrashReport(true),
		service.WithExtractRawCrashReport(true),
		service.WithWhenMoveIsDone(func(filename string) {
			fmt.Println("Copy:", filename)
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
}
