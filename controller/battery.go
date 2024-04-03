package controller

func GetBatteryInfo(udid string) (map[string]interface{}, error) {
	setupDevice(udid)
	return dev.GetBatteryInfo()
}
