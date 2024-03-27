package main

import (
	"fmt"
	"github.com/spf13/cobra"
)

var appName string
var appListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed applications",
	Long:  `List all installed applications on the device`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("App list command run\n")
	},
}

var appLaunchCmd = &cobra.Command{
	Use:   "launch",
	Short: "Launch an application",
	Long:  `Launch an application by name`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("App launch command run %s\n", appName)
	},
}

var appUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Listen for device events",
	Long:  `Listen for device events such as USB connection`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Devices listen command run\n")
	},
}

func InitAppCmd() {
	// 为 "launch" 命令绑定参数
	appLaunchCmd.Flags().StringVarP(&appName, "name", "n", "", "Name of the application to launch")
}
