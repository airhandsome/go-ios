package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var udid string
var bundleId string

func main() {

	var rootCmd = &cobra.Command{
		Use:   "gos",
		Short: "GOS (Go ios system) commands",
		Long:  `A collection of commands for managing and interacting with devices`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Root command run\n")
		},
	}

	var mountCmd = &cobra.Command{
		Use:   "mount",
		Short: "Mount a device",
		Long:  `Mount a device to the filesystem`,
		Run: func(cmd *cobra.Command, args []string) {
			if bundleId != "" {
				fmt.Printf("Current bundleId is %s \n", bundleId)
			}
			fmt.Printf("Mount command run\n")
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
			fmt.Printf("\"Get battery data\"\n")
		},
	}

	var perfCmd = &cobra.Command{
		Use:   "perf",
		Short: "Get perf data",
		Long:  "Get perf data",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("\"Get battery data\"\n")
		},
	}

	InitAppCmd()

	//add udid flag
	rootCmd.Flags().StringVarP(&udid, "udid", "u", "", "your device udid")
	appCmd.Flags().StringVarP(&bundleId, "bundleId", "b", "", "your bundleId")
	appCmd.AddCommand(appListCmd, appLaunchCmd, appUninstallCmd, batteryCmd, perfCmd)
	rootCmd.AddCommand(appCmd, mountCmd)
	//useless
	rootCmd.AddCommand(runWdacmd, runXctestCmd, devicesListenCmd, remoteConnectCmd, remoteShareCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
