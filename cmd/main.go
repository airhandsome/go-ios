package main

import (
	"fmt"
	"github.com/airhandsome/go-ios/controller"
	"github.com/airhandsome/go-ios/service"
	"github.com/spf13/cobra"
	"os"
)

var udid string
var bundleId string
var picName string

func main() {
	controller.ProfilerStart(udid, bundleId, []service.DataType{service.CPU})
	var rootCmd = &cobra.Command{
		Use:   "gos",
		Short: "GOS (Go ios system) commands",
		Long:  `A collection of commands for managing and interacting with devices`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Root command run\n")
		},
	}

	var devicesCmd = &cobra.Command{
		Use:   "devices",
		Short: "List all available device",
		Long:  `List all available device`,
		Run: func(cmd *cobra.Command, args []string) {
			list := controller.GetDeviceList("")
			for _, v := range list {
				fmt.Printf(v.Properties().UDID)
			}
		},
	}

	var deviceInfoCmd = &cobra.Command{
		Use:   "info",
		Short: "List device info",
		Long:  `List device info`,
		Run: func(cmd *cobra.Command, args []string) {
			info := controller.GetDeviceInfo(udid)
			if info != nil {
				fmt.Printf("%+v", info)
			}
		},
	}

	var mountCmd = &cobra.Command{
		Use:   "mount",
		Short: "Mount a device",
		Long:  `Mount a device to the filesystem`,
		Run: func(cmd *cobra.Command, args []string) {
			err := controller.Mount(udid)
			if err != nil {
				fmt.Printf("Mount error %s\n", err)
			}
			fmt.Println("Mount success!")
		},
	}

	var appCmd = &cobra.Command{
		Use:   "app",
		Short: "Command related to app",
		Long:  `Command related to app`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("App list command run\n")
		},
	}

	var batteryCmd = &cobra.Command{
		Use:   "battery",
		Short: "Get battery data",
		Long:  "Get battery data",
		Run: func(cmd *cobra.Command, args []string) {
			val, err := controller.GetBatteryInfo(udid)
			if err != nil {
				fmt.Printf("Get battery data error %s\n", err)
			} else {
				for k, v := range val {
					fmt.Printf("%s:%v\n", k, v)
				}
			}
		},
	}

	var perfCmd = &cobra.Command{
		Use:   "perf",
		Short: "Get perf data",
		Long:  "Get perf data",
		Run: func(cmd *cobra.Command, args []string) {
			//todo
			fmt.Printf("\"Get battery data\"\n")
		},
	}

	var screenshotCmd = &cobra.Command{
		Use:   "screenshot",
		Short: "Get ios screenshot",
		Long:  "Get ios screenshot",
		Run: func(cmd *cobra.Command, args []string) {
			if !controller.Screenshot(picName) {
				fmt.Println("Get screenshot error")
			}
		},
	}

	InitAppCmd()

	//add udid flag
	rootCmd.Flags().StringVarP(&udid, "udid", "u", "", "your device udid")
	appCmd.Flags().StringVarP(&bundleId, "bundleId", "b", "", "your bundleId")
	appCmd.AddCommand(appListCmd, appLaunchCmd, appUninstallCmd, batteryCmd)
	screenshotCmd.Flags().StringVarP(&picName, "path", "p", "", "screenshot name")
	rootCmd.AddCommand(appCmd, mountCmd, screenshotCmd, perfCmd, devicesCmd, deviceInfoCmd, batteryCmd)
	//useless
	rootCmd.AddCommand(runWdacmd, runXctestCmd, devicesListenCmd, remoteConnectCmd, remoteShareCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
