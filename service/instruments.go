package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/airhandsome/go-ios/pkg/libimobiledevice"
	"log"
)

const (
	deviceInfo              = "com.apple.instruments.server.services.deviceinfo"
	processControl          = "com.apple.instruments.server.services.processcontrol"
	deviceApplictionListing = "com.apple.instruments.server.services.device.applictionListing"
	graphicsOpengl          = "com.apple.instruments.server.services.graphics.opengl"        // 获取FPS
	sysmontap               = "com.apple.instruments.server.services.sysmontap"              // 获取性能数据用
	xcodeNetworkStatistics  = "com.apple.xcode.debug-gauge-data-providers.NetworkStatistics" // 获取单进程网络数据
	xcodeEnergyStatistics   = "com.apple.xcode.debug-gauge-data-providers.Energy"            // 获取功耗数据
	networking              = "com.apple.instruments.server.services.networking"             // 全局网络数据 instruments 用的就是这个
	mobileNotifications     = "com.apple.instruments.server.services.mobilenotifications"    // 监控应用状态
)

var _ Instruments = (*instruments)(nil)

func newInstruments(client *libimobiledevice.InstrumentsClient) *instruments {
	return &instruments{
		client: client,
	}
}

type instruments struct {
	client *libimobiledevice.InstrumentsClient
}

func (i *instruments) notifyOfPublishedCapabilities() (err error) {
	_, err = i.client.NotifyOfPublishedCapabilities()
	return
}

func (i *instruments) requestChannel(channel string) (id uint32, err error) {
	return i.client.RequestChannel(channel)
}

func (i *instruments) AppLaunch(bundleID string, opts ...AppLaunchOption) (pid int, err error) {
	opt := new(appLaunchOption)
	opt.appPath = ""
	opt.options = map[string]interface{}{
		"StartSuspendedKey": uint64(0),
		"KillExisting":      uint64(0),
	}
	if len(opts) != 0 {
		for _, optFunc := range opts {
			optFunc(opt)
		}
	}

	var id uint32
	if id, err = i.requestChannel("com.apple.instruments.server.services.processcontrol"); err != nil {
		return 0, err
	}

	args := libimobiledevice.NewAuxBuffer()
	if err = args.AppendObject(opt.appPath); err != nil {
		return 0, err
	}
	if err = args.AppendObject(bundleID); err != nil {
		return 0, err
	}
	if err = args.AppendObject(opt.environment); err != nil {
		return 0, err
	}
	if err = args.AppendObject(opt.arguments); err != nil {
		return 0, err
	}
	if err = args.AppendObject(opt.options); err != nil {
		return 0, err
	}

	var result *libimobiledevice.DTXMessageResult
	selector := "launchSuspendedProcessWithDevicePath:bundleIdentifier:environment:arguments:options:"
	if result, err = i.client.Invoke(selector, args, id, true); err != nil {
		return 0, err
	}

	if nsErr, ok := result.Obj.(libimobiledevice.NSError); ok {
		return 0, fmt.Errorf("%s", nsErr.NSUserInfo.(map[string]interface{})["NSLocalizedDescription"])
	}

	return int(result.Obj.(uint64)), nil
}

func (i *instruments) appProcess(bundleID string) (err error) {
	var id uint32
	if id, err = i.requestChannel("com.apple.instruments.server.services.processcontrol"); err != nil {
		return err
	}

	args := libimobiledevice.NewAuxBuffer()
	if err = args.AppendObject(bundleID); err != nil {
		return err
	}

	selector := "processIdentifierForBundleIdentifier:"
	if _, err = i.client.Invoke(selector, args, id, true); err != nil {
		return err
	}

	return
}

func (i *instruments) startObserving(pid int) (err error) {
	var id uint32
	if id, err = i.requestChannel("com.apple.instruments.server.services.processcontrol"); err != nil {
		return err
	}

	args := libimobiledevice.NewAuxBuffer()
	if err = args.AppendObject(pid); err != nil {
		return err
	}

	var result *libimobiledevice.DTXMessageResult
	selector := "startObservingPid:"
	if result, err = i.client.Invoke(selector, args, id, true); err != nil {
		return err
	}

	if nsErr, ok := result.Obj.(libimobiledevice.NSError); ok {
		return fmt.Errorf("%s", nsErr.NSUserInfo.(map[string]interface{})["NSLocalizedDescription"])
	}
	return
}

func (i *instruments) AppKill(pid int) (err error) {
	var id uint32
	if id, err = i.requestChannel("com.apple.instruments.server.services.processcontrol"); err != nil {
		return err
	}

	args := libimobiledevice.NewAuxBuffer()
	if err = args.AppendObject(pid); err != nil {
		return err
	}

	selector := "killPid:"
	if _, err = i.client.Invoke(selector, args, id, false); err != nil {
		return err
	}

	return
}

func (i *instruments) AppRunningProcesses() (processes []Process, err error) {
	var id uint32
	if id, err = i.requestChannel(deviceInfo); err != nil {
		return nil, err
	}

	selector := "runningProcesses"

	var result *libimobiledevice.DTXMessageResult
	if result, err = i.client.Invoke(selector, libimobiledevice.NewAuxBuffer(), id, true); err != nil {
		return nil, err
	}

	objs := result.Obj.([]interface{})

	processes = make([]Process, 0, len(objs))

	for _, v := range objs {
		m := v.(map[string]interface{})

		var data []byte
		if data, err = json.Marshal(m); err != nil {
			debugLog(fmt.Sprintf("process marshal: %v\n%v\n", err, m))
			err = nil
			continue
		}

		var tp Process
		if err = json.Unmarshal(data, &tp); err != nil {
			debugLog(fmt.Sprintf("process unmarshal: %v\n%v\n", err, m))
			err = nil
			continue
		}

		processes = append(processes, tp)
	}

	return
}

func (i *instruments) AppList(opts ...AppListOption) (apps []Application, err error) {
	opt := new(appListOption)
	opt.updateToken = ""
	opt.appsMatching = make(map[string]interface{})
	if len(opts) != 0 {
		for _, optFunc := range opts {
			optFunc(opt)
		}
	}

	var id uint32
	if id, err = i.requestChannel("com.apple.instruments.server.services.device.applictionListing"); err != nil {
		return nil, err
	}

	args := libimobiledevice.NewAuxBuffer()
	if err = args.AppendObject(opt.appsMatching); err != nil {
		return nil, err
	}
	if err = args.AppendObject(opt.updateToken); err != nil {
		return nil, err
	}

	selector := "installedApplicationsMatching:registerUpdateToken:"

	var result *libimobiledevice.DTXMessageResult
	if result, err = i.client.Invoke(selector, args, id, true); err != nil {
		return nil, err
	}

	objs := result.Obj.([]interface{})

	for _, v := range objs {
		m := v.(map[string]interface{})

		var data []byte
		if data, err = json.Marshal(m); err != nil {
			debugLog(fmt.Sprintf("application marshal: %v\n%v\n", err, m))
			err = nil
			continue
		}

		var app Application
		if err = json.Unmarshal(data, &app); err != nil {
			debugLog(fmt.Sprintf("application unmarshal: %v\n%v\n", err, m))
			err = nil
			continue
		}
		apps = append(apps, app)
	}

	return
}

func (i *instruments) DeviceInfo() (devInfo *DeviceInfo, err error) {
	var id uint32
	if id, err = i.requestChannel(deviceInfo); err != nil {
		return nil, err
	}

	selector := "systemInformation"

	var result *libimobiledevice.DTXMessageResult
	if result, err = i.client.Invoke(selector, libimobiledevice.NewAuxBuffer(), id, true); err != nil {
		return nil, err
	}

	data, err := json.Marshal(result.Obj)
	if err != nil {
		return nil, err
	}
	devInfo = new(DeviceInfo)
	err = json.Unmarshal(data, devInfo)

	return
}

func (i *instruments) GraphicInfo() (devInfo *GraphicsInfo, err error) {
	/*
		{'CommandBufferRenderCount': 0,
			'CoreAnimationFramesPerSecond': 0,
			'Device Utilization %': 0,
			'IOGLBundleName': 'Built-In',
			'Renderer Utilization %': 0,
			'SplitSceneCount': 0,
			'TiledSceneBytes': 0,
			'Tiler Utilization %': 0,
			'XRVideoCardRunTimeStamp': 448363116,
			'agpTextureCreationBytes': 0,
			'agprefTextureCreationBytes': 0,
			'contextGLCount': 0,
			'finishGLWaitTime': 0,
			'freeToAllocGPUAddressWaitTime': 0,
			'gartMapInBytesPerSample': 0,
			'gartMapOutBytesPerSample': 0,
			'gartUsedBytes': 30965760,
			'hardwareWaitTime': 0,
			'iosurfaceTextureCreationBytes': 0,
			'oolTextureCreationBytes': 0,
			'recoveryCount': 0,
			'stdTextureCreationBytes': 0,
			'textureCount': 1382}
	*/
	var id uint32
	if id, err = i.requestChannel("com.apple.instruments.server.services.graphics.opengl"); err != nil {
		return nil, err
	}
	selector := "systemInformation"

	var result *libimobiledevice.DTXMessageResult
	if result, err = i.client.Invoke(selector, libimobiledevice.NewAuxBuffer(), id, true); err != nil {
		return nil, err
	}

	data, err := json.Marshal(result.Obj)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &devInfo)

	return
}
func (i *instruments) call(channel, selector string, auxiliaries ...interface{}) (
	result *libimobiledevice.DTXMessageResult, err error) {

	chanID, err := i.requestChannel(channel)
	if err != nil {
		return nil, err
	}

	args := libimobiledevice.NewAuxBuffer()
	for _, aux := range auxiliaries {
		if err = args.AppendObject(aux); err != nil {
			return nil, err
		}
	}

	return i.client.Invoke(selector, args, chanID, true)
}

func (i *instruments) registerCallback(obj string, cb func(m libimobiledevice.DTXMessageResult)) {
	i.client.RegisterCallback(obj, cb)
}

type Application struct {
	AppExtensionUUIDs         []string `json:"AppExtensionUUIDs,omitempty"`
	BundlePath                string   `json:"BundlePath"`
	CFBundleIdentifier        string   `json:"CFBundleIdentifier"`
	ContainerBundleIdentifier string   `json:"ContainerBundleIdentifier,omitempty"`
	ContainerBundlePath       string   `json:"ContainerBundlePath,omitempty"`
	DisplayName               string   `json:"DisplayName"`
	ExecutableName            string   `json:"ExecutableName,omitempty"`
	Placeholder               bool     `json:"Placeholder,omitempty"`
	PluginIdentifier          string   `json:"PluginIdentifier,omitempty"`
	PluginUUID                string   `json:"PluginUUID,omitempty"`
	Restricted                int      `json:"Restricted"`
	Type                      string   `json:"Type"`
	Version                   string   `json:"Version"`
}

type DeviceInfo struct {
	Description       string `json:"_deviceDescription"`
	DisplayName       string `json:"_deviceDisplayName"`
	Identifier        string `json:"_deviceIdentifier"`
	Version           string `json:"_deviceVersion"`
	ProductType       string `json:"_productType"`
	ProductVersion    string `json:"_productVersion"`
	XRDeviceClassName string `json:"_xrdeviceClassName"`
}

type GraphicsInfo struct {
	CommandBufferRenderCount      int     `json:"commandBufferRenderCount"`
	CoreAnimationFramesPerSecond  float64 `json:"coreAnimationFramesPerSecond"`
	DeviceUtilizationPercent      float64 `json:"deviceUtilizationPercent"`
	IOGLBundleName                string  `json:"ioglBundleName"`
	RendererUtilizationPercent    float64 `json:"rendererUtilizationPercent"`
	SplitSceneCount               int     `json:"splitSceneCount"`
	TiledSceneBytes               int64   `json:"tiledSceneBytes"`
	TilerUtilizationPercent       float64 `json:"tilerUtilizationPercent"`
	XRVideoCardRunTimeStamp       int64   `json:"xRVideoCardRunTimeStamp"`
	AgpTextureCreationBytes       int64   `json:"agpTextureCreationBytes"`
	AgprefTextureCreationBytes    int64   `json:"agprefTextureCreationBytes"`
	ContextGLCount                int     `json:"contextGLCount"`
	FinishGLWaitTime              int64   `json:"finishGLWaitTime"`
	FreeToAllocGPUAddressWaitTime int64   `json:"freeToAllocGPAAddressWaitTime"`
	GartMapInBytesPerSample       int64   `json:"gartMapInBytesPerSample"`
	GartMapOutBytesPerSample      int64   `json:"gartMapOutBytesPerSample"`
	GartUsedBytes                 int64   `json:"gartUsedBytes"`
	HardwareWaitTime              int64   `json:"hardwareWaitTime"`
	IosurfaceTextureCreationBytes int64   `json:"iosurfaceTextureCreationBytes"`
	OolTextureCreationBytes       int64   `json:"oolTextureCreationBytes"`
	RecoveryCount                 int     `json:"recoveryCount"`
	StdTextureCreationBytes       int64   `json:"stdTextureCreationBytes"`
	TextureCount                  int     `json:"textureCount"`
}

func (i *instruments) SetGraphicSampleRate(rate int) (result *libimobiledevice.DTXMessageResult, err error) {
	return i.call(graphicsOpengl,
		"setSamplingRate:",
		rate/100)
}
func (i *instruments) StartGraphicSample() (result *libimobiledevice.DTXMessageResult, err error) {
	return i.call(
		graphicsOpengl,
		"startSamplingAtTimeInterval:",
		0)
}
func (i *instruments) StopGraphicSample() (result *libimobiledevice.DTXMessageResult, err error) {
	return i.call(graphicsOpengl, "stopSampling")
}

func (i *instruments) getPidByBundleId(bundleId string) (pid int, err error) {
	appList, err := i.AppList()
	if err != nil {
		log.Printf("get app list error: %v\n", err)
		return 0, err
	}

	var appName string
	for _, app := range appList {
		if app.CFBundleIdentifier == bundleId {
			appName = app.ExecutableName
			break
		}
	}
	if appName == "" {
		return -1, errors.New(fmt.Sprintf("Can't find bundleId %s", bundleId))
	}

	processes, err := i.AppRunningProcesses()
	if err != nil {
		log.Printf("get running app processes error: %v\n", err)
		return -1, err
	}
	for _, v := range processes {
		fmt.Println(v.Name)
		if v.Name == appName {
			return v.Pid, nil
		}
	}

	return -1, errors.New(fmt.Sprintf("can't find pid by bundleId: %s", bundleId))
}

func (i *instruments) SetCpuAndMemorySampleConfig(config map[string]interface{}) (result *libimobiledevice.DTXMessageResult, err error) {
	return i.call(
		sysmontap,
		"setConfig:",
		config)
}

func (i *instruments) StartCpuAndMemorySample() (result *libimobiledevice.DTXMessageResult, err error) {
	return i.call(sysmontap,
		"start")
}

func (i *instruments) StopCpuAndMemorySample() (result *libimobiledevice.DTXMessageResult, err error) {
	return i.call(sysmontap, "stop")
}

func (i *instruments) SetNetworkSampleConfig() (result *libimobiledevice.DTXMessageResult, err error) {
	return i.call(
		networking,
		"replayLastRecordedSession",
	)
}

func (i *instruments) StartNetworkSample() (result *libimobiledevice.DTXMessageResult, err error) {

	return i.call(
		networking,
		"startMonitoring",
	)
}
func (i *instruments) StopNetworkSample() (result *libimobiledevice.DTXMessageResult, err error) {
	return i.call(
		networking,
		"stopMonitoring",
	)

}
