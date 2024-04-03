package main

import (
	"fmt"
	"github.com/airhandsome/go-ios/controller"
	"github.com/spf13/cobra"
	"strings"
)

var appName string
var appListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed applications",
	Long:  `List all installed applications on the device`,
	Run: func(cmd *cobra.Command, args []string) {
		list, err := controller.List(udid)
		if err != nil {
			fmt.Println("Get app list error %s", udid)
		} else {
			for _, app := range list {
				if app.CFBundleIdentifier != "" && strings.Index(app.CFBundleIdentifier, "com.apple") < 0 {
					fmt.Printf("%s %s %s %s\n", app.Type, app.DisplayName, app.CFBundleIdentifier, app.Version)
				}
			}
		}
	},
}

var appLaunchCmd = &cobra.Command{
	Use:   "launch",
	Short: "Launch an application",
	Long:  `Launch an application by name`,
	Run: func(cmd *cobra.Command, args []string) {
		if appName == "" {
			fmt.Println("appName should not be null, please use -n or --name for argument")
		} else {
			if pid, ok := controller.Launch(udid, appName); ok {
				fmt.Printf("App launch %s with pid %d\n", appName, pid)
			} else {
				fmt.Printf("App launch %s error", appName)
			}
		}
	},
}

var appUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Listen for device events",
	Long:  `Listen for device events such as USB connection`,
	Run: func(cmd *cobra.Command, args []string) {
		//todo
		fmt.Printf("Devices listen command run\n")
	},
}

func InitAppCmd() {
	// 为 "launch" 命令绑定参数
	appLaunchCmd.Flags().StringVarP(&appName, "name", "n", "", "Name of the application to launch")
}
