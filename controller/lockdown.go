package controller

import (
	"fmt"
	"github.com/airhandsome/go-ios/service"
	"log"
	"os"
	"os/signal"
	"path"
	"time"
)

var lockdownSrv service.Lockdown

func setupLockdownSrv() {
	setupDevice("")

	var err error
	if lockdownSrv, err = dev.LockdownService(); err != nil {
		log.Fatal(err)
	}
}

func Test_lockdown_QueryType() {
	setupLockdownSrv()

	lockdownType, err := lockdownSrv.QueryType()
	if err != nil {
		log.Fatal(err)
	}

	log.Println(lockdownType.Type)
}

func Test_lockdown_GetValue() {
	setupLockdownSrv()

	// v, err := dev.GetValue("com.apple.mobile.iTunes", "")
	// v, err := dev.GetValue("com.apple.mobile.internal", "")
	v, err := dev.GetValue("com.apple.mobile.battery", "")

	// v, err := lockdownSrv.GetValue("", "ProductVersion")
	// v, err := lockdownSrv.GetValue("", "DeviceName")
	// v, err := lockdownSrv.GetValue("com.apple.mobile.iTunes", "")
	// v, err := lockdownSrv.GetValue("com.apple.mobile.battery", "")
	// v, err := lockdownSrv.GetValue("com.apple.disk_usage", "")
	if err != nil {
		log.Fatal(err)
	}

	log.Println(v)
}

func Test_lockdown_SyslogRelayService() {
	setupLockdownSrv()

	syslogRelaySrv, err := lockdownSrv.SyslogRelayService()
	if err != nil {
		log.Fatal(err)
	}
	syslogRelaySrv.Stop()

	lines := syslogRelaySrv.Lines()

	done := make(chan os.Signal, 1)

	go func() {
		for line := range lines {
			fmt.Println(line)
		}
		done <- os.Interrupt
		fmt.Println("DONE!!!")
	}()

	signal.Notify(done, os.Interrupt, os.Kill)

	<-done
	syslogRelaySrv.Stop()
	time.Sleep(time.Second)
}

func Test_lockdown_CrashReportMoverService() {
	setupLockdownSrv()

	crashReportMoverSrv, err := lockdownSrv.CrashReportMoverService()
	if err != nil {
		log.Fatal(err)
	}

	filenames := make([]string, 0, 36)
	fn := func(cwd string, info *service.AfcFileInfo) {
		if cwd == "." {
			cwd = ""
		}
		filenames = append(filenames, path.Join(cwd, info.Name()))
		// fmlog.Println(path.Join(cwd, name))
	}
	err = crashReportMoverSrv.WalkDir(".", fn)
	if err != nil {
		log.Fatal(err)
	}

	for _, n := range filenames {
		log.Println(n)
	}

	log.Println(len(filenames))
}
