package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/airhandsome/go-ios/pkg/ipa"
	"github.com/airhandsome/go-ios/pkg/libimobiledevice"
	"github.com/airhandsome/go-ios/pkg/nskeyedarchiver"
	uuid "github.com/satori/go.uuid"
	"howett.net/plist"
)

const LockdownPort = 62078

var _ Device = (*device)(nil)

func newDevice(client *libimobiledevice.UsbmuxClient, properties DeviceProperties) *device {
	return &device{
		umClient:   client,
		properties: &properties,
		profiler:   map[DataType]Profiler{},
	}
}

type device struct {
	umClient       *libimobiledevice.UsbmuxClient
	lockdownClient *libimobiledevice.LockdownClient

	properties *DeviceProperties

	lockdown          *lockdown
	imageMounter      ImageMounter
	screenshot        Screenshot
	simulateLocation  SimulateLocation
	installationProxy InstallationProxy
	instruments       Instruments
	afc               Afc
	houseArrest       HouseArrest
	syslogRelay       SyslogRelay
	diagnosticsRelay  DiagnosticsRelay
	springBoard       SpringBoard
	crashReportMover  CrashReportMover
	pcapd             Pcapd
	profiler          map[DataType]Profiler
}

func (d *device) Properties() DeviceProperties {
	return *d.properties
}

func (d *device) NewConnect(port int, timeout ...time.Duration) (InnerConn, error) {
	newClient, err := libimobiledevice.NewUsbmuxClient(timeout...)
	if err != nil {
		return nil, err
	}

	var pkt libimobiledevice.Packet
	if pkt, err = newClient.NewPlistPacket(
		newClient.NewConnectRequest(d.properties.DeviceID, port),
	); err != nil {
		newClient.Close()
		return nil, err
	}

	if err = newClient.SendPacket(pkt); err != nil {
		newClient.Close()
		return nil, err
	}

	if _, err = newClient.ReceivePacket(); err != nil {
		newClient.Close()
		return nil, err
	}

	return newClient.InnerConn(), err
}

func (d *device) ReadPairRecord() (pairRecord *PairRecord, err error) {
	var pkt libimobiledevice.Packet
	if pkt, err = d.umClient.NewPlistPacket(
		d.umClient.NewReadPairRecordRequest(d.properties.SerialNumber),
	); err != nil {
		return nil, err
	}

	if err = d.umClient.SendPacket(pkt); err != nil {
		return nil, err
	}

	var respPkt libimobiledevice.Packet
	if respPkt, err = d.umClient.ReceivePacket(); err != nil {
		return nil, err
	}

	var reply = struct {
		Data []byte `plist:"PairRecordData"`
	}{}
	if err = respPkt.Unmarshal(&reply); err != nil {
		return nil, err
	}

	var record PairRecord
	if _, err = plist.Unmarshal(reply.Data, &record); err != nil {
		return nil, err
	}

	pairRecord = &record
	return
}

func (d *device) SavePairRecord(pairRecord *PairRecord) (err error) {
	var data []byte
	if data, err = plist.Marshal(pairRecord, plist.XMLFormat); err != nil {
		return err
	}

	var pkt libimobiledevice.Packet
	if pkt, err = d.umClient.NewPlistPacket(
		d.umClient.NewSavePairRecordRequest(d.properties.SerialNumber, d.properties.DeviceID, data),
	); err != nil {
		return err
	}

	if err = d.umClient.SendPacket(pkt); err != nil {
		return err
	}

	if _, err = d.umClient.ReceivePacket(); err != nil {
		return err
	}

	return
}

func (d *device) DeletePairRecord() (err error) {
	var pkt libimobiledevice.Packet
	if pkt, err = d.umClient.NewPlistPacket(
		d.umClient.NewDeletePairRecordRequest(d.properties.SerialNumber),
	); err != nil {
		return err
	}

	if err = d.umClient.SendPacket(pkt); err != nil {
		return err
	}

	if _, err = d.umClient.ReceivePacket(); err != nil {
		return err
	}

	return
}

func (d *device) LockdownService() (lockdown Lockdown, err error) {
	// if d.lockdown != nil {
	// 	return d.lockdown, nil
	// }

	var innerConn InnerConn
	if innerConn, err = d.NewConnect(LockdownPort, 0); err != nil {
		return nil, err
	}
	d.lockdownClient = libimobiledevice.NewLockdownClient(innerConn)
	d.lockdown = newLockdown(d)
	_, err = d.lockdown._getProductVersion()
	lockdown = d.lockdown
	return
}

func (d *device) QueryType() (LockdownType, error) {
	if _, err := d.LockdownService(); err != nil {
		return LockdownType{}, err
	}
	return d.lockdown.QueryType()
}

func (d *device) GetValue(domain, key string) (v interface{}, err error) {
	if _, err = d.LockdownService(); err != nil {
		return nil, err
	}
	if d.lockdown.pairRecord == nil {
		if err = d.lockdown.handshake(); err != nil {
			return nil, err
		}
	}
	if err = d.lockdown.startSession(d.lockdown.pairRecord); err != nil {
		return nil, err
	}
	if v, err = d.lockdown.GetValue(domain, key); err != nil {
		return nil, err
	}
	err = d.lockdown.stopSession()
	return
}

func (d *device) Pair() (pairRecord *PairRecord, err error) {
	if _, err = d.LockdownService(); err != nil {
		return nil, err
	}
	return d.lockdown.Pair()
}

func (d *device) imageMounterService() (imageMounter ImageMounter, err error) {
	if d.imageMounter != nil {
		return d.imageMounter, nil
	}
	if _, err = d.LockdownService(); err != nil {
		return nil, err
	}
	if d.imageMounter, err = d.lockdown.ImageMounterService(); err != nil {
		return nil, err
	}
	imageMounter = d.imageMounter
	return
}

func (d *device) Images(imgType ...string) (imageSignatures [][]byte, err error) {
	if _, err = d.imageMounterService(); err != nil {
		return nil, err
	}
	if len(imgType) == 0 {
		imgType = []string{"Developer"}
	}
	return d.imageMounter.Images(imgType[0])
}

func (d *device) MountDeveloperDiskImage(dmgPath string, signaturePath string) (err error) {
	if _, err = d.imageMounterService(); err != nil {
		return err
	}
	devImgPath := "/private/var/mobile/Media/PublicStaging/staging.dimage"
	return d.imageMounter.UploadImageAndMount("Developer", devImgPath, dmgPath, signaturePath)
}

func (d *device) screenshotService() (screenshot Screenshot, err error) {
	if d.screenshot != nil {
		return d.screenshot, nil
	}

	if _, err = d.LockdownService(); err != nil {
		return nil, err
	}
	if d.screenshot, err = d.lockdown.ScreenshotService(); err != nil {
		return nil, err
	}
	screenshot = d.screenshot
	return
}

func (d *device) Screenshot() (raw *bytes.Buffer, err error) {
	if _, err = d.screenshotService(); err != nil {
		return nil, err
	}
	return d.screenshot.Take()
}

func (d *device) simulateLocationService() (simulateLocation SimulateLocation, err error) {
	if d.simulateLocation != nil {
		return d.simulateLocation, nil
	}
	if _, err = d.LockdownService(); err != nil {
		return nil, err
	}
	if d.simulateLocation, err = d.lockdown.SimulateLocationService(); err != nil {
		return nil, err
	}
	simulateLocation = d.simulateLocation
	return
}

func (d *device) SimulateLocationUpdate(longitude float64, latitude float64, coordinateSystem ...CoordinateSystem) (err error) {
	if _, err = d.simulateLocationService(); err != nil {
		return err
	}
	return d.simulateLocation.Update(longitude, latitude, coordinateSystem...)
}

func (d *device) SimulateLocationRecover() (err error) {
	if _, err = d.simulateLocationService(); err != nil {
		return err
	}
	return d.simulateLocation.Recover()
}

func (d *device) installationProxyService() (installationProxy InstallationProxy, err error) {
	if d.installationProxy != nil {
		return d.installationProxy, nil
	}
	if _, err = d.LockdownService(); err != nil {
		return nil, err
	}
	if d.installationProxy, err = d.lockdown.InstallationProxyService(); err != nil {
		return nil, err
	}
	installationProxy = d.installationProxy
	return
}

func (d *device) InstallationProxyBrowse(opts ...InstallationProxyOption) (currentList []interface{}, err error) {
	if _, err = d.installationProxyService(); err != nil {
		return nil, err
	}
	return d.installationProxy.Browse(opts...)
}

func (d *device) InstallationProxyLookup(opts ...InstallationProxyOption) (lookupResult interface{}, err error) {
	if _, err = d.installationProxyService(); err != nil {
		return nil, err
	}
	return d.installationProxy.Lookup(opts...)
}

func (d *device) instrumentsService() (instruments Instruments, err error) {
	if d.instruments != nil {
		return d.instruments, nil
	}
	if _, err = d.LockdownService(); err != nil {
		return nil, err
	}
	if d.instruments, err = d.lockdown.InstrumentsService(); err != nil {
		return nil, err
	}
	instruments = d.instruments
	return
}

func (d *device) AppLaunch(bundleID string, opts ...AppLaunchOption) (pid int, err error) {
	if _, err = d.instrumentsService(); err != nil {
		return 0, err
	}
	return d.instruments.AppLaunch(bundleID, opts...)
}

func (d *device) AppKill(pid int) (err error) {
	if _, err = d.instrumentsService(); err != nil {
		return err
	}
	return d.instruments.AppKill(pid)
}

func (d *device) AppRunningProcesses() (processes []Process, err error) {
	if _, err = d.instrumentsService(); err != nil {
		return nil, err
	}
	return d.instruments.AppRunningProcesses()
}

func (d *device) AppList(opts ...AppListOption) (apps []Application, err error) {
	if _, err = d.instrumentsService(); err != nil {
		return nil, err
	}
	return d.instruments.AppList(opts...)
}

func (d *device) DeviceInfo() (devInfo *DeviceInfo, err error) {
	if _, err = d.instrumentsService(); err != nil {
		return nil, err
	}
	return d.instruments.DeviceInfo()
}

func (d *device) GraphicInfo() (graphicInfo *GraphicsInfo, err error) {
	if _, err = d.instrumentsService(); err != nil {
		return nil, err
	}
	return d.instruments.GraphicInfo()
}

func (d *device) testmanagerdService() (testmanagerd Testmanagerd, err error) {
	if _, err = d.LockdownService(); err != nil {
		return nil, err
	}
	if testmanagerd, err = d.lockdown.TestmanagerdService(); err != nil {
		return nil, err
	}
	return
}

func (d *device) AfcService() (afc Afc, err error) {
	if d.afc != nil {
		return d.afc, nil
	}
	if _, err = d.LockdownService(); err != nil {
		return nil, err
	}
	if d.afc, err = d.lockdown.AfcService(); err != nil {
		return nil, err
	}
	afc = d.afc
	return
}

func (d *device) AppInstall(ipaPath string) (err error) {
	if _, err = d.AfcService(); err != nil {
		return err
	}

	stagingPath := "PublicStaging"
	if _, err = d.afc.Stat(stagingPath); err != nil {
		if err != ErrAfcStatNotExist {
			return err
		}
		if err = d.afc.Mkdir(stagingPath); err != nil {
			return fmt.Errorf("app install: %w", err)
		}
	}

	var info map[string]interface{}
	if info, err = ipa.Info(ipaPath); err != nil {
		return err
	}
	bundleID, ok := info["CFBundleIdentifier"]
	if !ok {
		return errors.New("can't find 'CFBundleIdentifier'")
	}

	installationPath := path.Join(stagingPath, fmt.Sprintf("%s.ipa", bundleID))

	var data []byte
	if data, err = os.ReadFile(ipaPath); err != nil {
		return err
	}
	if err = d.afc.WriteFile(installationPath, data, AfcFileModeWr); err != nil {
		return err
	}

	if _, err = d.installationProxyService(); err != nil {
		return err
	}

	return d.installationProxy.Install(fmt.Sprintf("%s", bundleID), installationPath)
}

func (d *device) AppUninstall(bundleID string) (err error) {
	if _, err = d.installationProxyService(); err != nil {
		return err
	}

	return d.installationProxy.Uninstall(bundleID)
}

func (d *device) HouseArrestService() (houseArrest HouseArrest, err error) {
	if d.houseArrest != nil {
		return d.houseArrest, nil
	}
	if _, err = d.LockdownService(); err != nil {
		return nil, err
	}
	if d.houseArrest, err = d.lockdown.HouseArrestService(); err != nil {
		return nil, err
	}
	houseArrest = d.houseArrest
	return
}

func (d *device) syslogRelayService() (syslogRelay SyslogRelay, err error) {
	if d.syslogRelay != nil {
		return d.syslogRelay, nil
	}
	if _, err = d.LockdownService(); err != nil {
		return nil, err
	}
	if d.syslogRelay, err = d.lockdown.SyslogRelayService(); err != nil {
		return nil, err
	}
	syslogRelay = d.syslogRelay
	return
}

func (d *device) Syslog() (lines <-chan string, err error) {
	if _, err = d.syslogRelayService(); err != nil {
		return nil, err
	}
	return d.syslogRelay.Lines(), nil
}

func (d *device) SyslogStop() {
	if d.syslogRelay == nil {
		return
	}
	d.syslogRelay.Stop()
}

func (d *device) Reboot() (err error) {
	if _, err = d.LockdownService(); err != nil {
		return
	}
	if d.diagnosticsRelay, err = d.lockdown.DiagnosticsRelayService(); err != nil {
		return
	}
	if err = d.diagnosticsRelay.Reboot(); err != nil {
		return
	}
	return
}

func (d *device) Shutdown() (err error) {
	if _, err = d.LockdownService(); err != nil {
		return
	}
	if d.diagnosticsRelay, err = d.lockdown.DiagnosticsRelayService(); err != nil {
		return
	}
	if err = d.diagnosticsRelay.Shutdown(); err != nil {
		return
	}
	return
}

func (d *device) springBoardService() (springBoard SpringBoard, err error) {
	if d.springBoard != nil {
		return d.springBoard, nil
	}
	if _, err = d.LockdownService(); err != nil {
		return nil, err
	}
	if d.springBoard, err = d.lockdown.SpringBoardService(); err != nil {
		return nil, err
	}
	springBoard = d.springBoard
	return
}

func (d *device) GetIconPNGData(bundleId string) (raw *bytes.Buffer, err error) {
	if _, err = d.LockdownService(); err != nil {
		return
	}
	if d.springBoard, err = d.lockdown.SpringBoardService(); err != nil {
		return
	}
	if raw, err = d.springBoard.GetIconPNGData(bundleId); err != nil {
		return
	}
	return
}

func (d *device) GetInterfaceOrientation() (orientation libimobiledevice.OrientationState, err error) {
	if _, err = d.springBoardService(); err != nil {
		return
	}
	if orientation, err = d.springBoard.GetInterfaceOrientation(); err != nil {
		return
	}
	return
}

func (d *device) PcapdService() (pcapd Pcapd, err error) {
	// if d.pcapd != nil {
	// 	return d.pcapd, nil
	// }
	if _, err = d.LockdownService(); err != nil {
		return nil, err
	}
	if d.pcapd, err = d.lockdown.PcapdService(); err != nil {
		return nil, err
	}
	pcapd = d.pcapd
	return
}

func (d *device) Pcap() (lines <-chan []byte, err error) {
	if _, err = d.PcapdService(); err != nil {
		return nil, err
	}
	return d.pcapd.Packet(), nil
}

func (d *device) PcapStop() {
	if d.pcapd == nil {
		return
	}
	d.pcapd.Stop()
}

func (d *device) crashReportMoverService() (crashReportMover CrashReportMover, err error) {
	if d.crashReportMover != nil {
		return d.crashReportMover, nil
	}
	if _, err = d.LockdownService(); err != nil {
		return nil, err
	}
	if d.crashReportMover, err = d.lockdown.CrashReportMoverService(); err != nil {
		return nil, err
	}
	crashReportMover = d.crashReportMover
	return
}

func (d *device) MoveCrashReport(hostDir string, opts ...CrashReportMoverOption) (err error) {
	if _, err = d.crashReportMoverService(); err != nil {
		return err
	}
	return d.crashReportMover.Move(hostDir, opts...)
}

func (d *device) XCTest(bundleID string, opts ...XCTestOption) (out <-chan string, cancel context.CancelFunc, err error) {
	xcTestOpt := defaultXCTestOption()
	for _, fn := range opts {
		fn(xcTestOpt)
	}

	ctx, cancelFunc := context.WithCancel(context.TODO())
	_out := make(chan string)

	xcodeVersion := uint64(30)

	var tmSrv1 Testmanagerd
	if tmSrv1, err = d.testmanagerdService(); err != nil {
		return _out, cancelFunc, err
	}

	var xcTestManager1 XCTestManagerDaemon
	if xcTestManager1, err = tmSrv1.newXCTestManagerDaemon(); err != nil {
		return _out, cancelFunc, err
	}

	var version []int
	if version, err = d.lockdown._getProductVersion(); err != nil {
		return _out, cancelFunc, err
	}

	if DeviceVersion(version...) >= DeviceVersion(11, 0, 0) {
		if err = xcTestManager1.initiateControlSession(xcodeVersion); err != nil {
			return _out, cancelFunc, err
		}
	}

	var tmSrv2 Testmanagerd
	if tmSrv2, err = d.testmanagerdService(); err != nil {
		return _out, cancelFunc, err
	}

	var xcTestManager2 XCTestManagerDaemon
	if xcTestManager2, err = tmSrv2.newXCTestManagerDaemon(); err != nil {
		return _out, cancelFunc, err
	}

	xcTestManager2.registerCallback("_XCT_logDebugMessage:", func(m libimobiledevice.DTXMessageResult) {
		// more information ( each operation )
		// fmt.Println("###### xcTestManager2 ### -->", m)
		if strings.Contains(fmt.Sprintf("%s", m), "Received test runner ready reply with error: (null)") {
			// fmt.Println("###### xcTestManager2 ### -->", fmt.Sprintf("%v", m.Aux[0]))
			time.Sleep(time.Second)
			if err = xcTestManager2.startExecutingTestPlan(xcodeVersion); err != nil {
				debugLog(fmt.Sprintf("startExecutingTestPlan %d: %s", xcodeVersion, err))
				return
			}
		}
	})
	xcTestManager2.registerCallback("_Golang-iDevice_Unregistered", func(m libimobiledevice.DTXMessageResult) {
		// more information
		//  _XCT_testRunnerReadyWithCapabilities:
		//  _XCT_didBeginExecutingTestPlan
		//  _XCT_didBeginInitializingForUITesting
		//  _XCT_testSuite:didStartAt:
		//  _XCT_testCase:method:willStartActivity:
		//  _XCT_testCase:method:didFinishActivity:
		//  _XCT_testCaseDidStartForTestClass:method:
		// fmt.Println("###### xcTestManager2 ### _Unregistered -->", m)
	})

	sessionId := uuid.NewV4()
	if err = xcTestManager2.initiateSession(xcodeVersion, nskeyedarchiver.NewNSUUID(sessionId.Bytes())); err != nil {
		return _out, cancelFunc, err
	}

	if _, err = d.installationProxyService(); err != nil {
		return _out, cancelFunc, err
	}

	var vResult interface{}
	if vResult, err = d.installationProxy.Lookup(WithBundleIDs(bundleID)); err != nil {
		return _out, cancelFunc, err
	}

	lookupResult := vResult.(map[string]interface{})
	lookupResult = lookupResult[bundleID].(map[string]interface{})
	appContainer := lookupResult["Container"].(string)
	appPath := lookupResult["Path"].(string)

	var pathXCTestCfg string
	if pathXCTestCfg, err = d._uploadXCTestConfiguration(bundleID, sessionId, lookupResult); err != nil {
		return _out, cancelFunc, err
	}

	if _, err = d.instrumentsService(); err != nil {
		return _out, cancelFunc, err
	}

	if err = d.instruments.appProcess(bundleID); err != nil {
		return _out, cancelFunc, err
	}

	pathXCTestConfiguration := appContainer + pathXCTestCfg

	appEnv := map[string]interface {
	}{
		"CA_ASSERT_MAIN_THREAD_TRANSACTIONS": "0",
		"CA_DEBUG_TRANSACTIONS":              "0",
		"DYLD_FRAMEWORK_PATH":                appPath + "/Frameworks:",
		"DYLD_LIBRARY_PATH":                  appPath + "/Frameworks",
		"NSUnbufferedIO":                     "YES",
		"SQLITE_ENABLE_THREAD_ASSERTIONS":    "1",
		"WDA_PRODUCT_BUNDLE_IDENTIFIER":      "",
		"XCTestConfigurationFilePath":        pathXCTestConfiguration, // Running tests with active test configuration:
		// "XCTestBundlePath":        fmt.Sprintf("%s/PlugIns/%s.xctest", appPath, name), // !!! ERROR
		// "XCTestSessionIdentifier": sessionId.String(), // !!! ERROR
		// "XCTestSessionIdentifier":  "",
		"XCODE_DBG_XPC_EXCLUSIONS": "com.apple.dt.xctestSymbolicator",
		"MJPEG_SERVER_PORT":        "",
		"USE_PORT":                 "",
		"LLVM_PROFILE_FILE":        appContainer + "/tmp/%p.profraw",
	}
	if DeviceVersion(version...) >= DeviceVersion(11, 0, 0) {
		appEnv["DYLD_INSERT_LIBRARIES"] = "/Developer/usr/lib/libMainThreadChecker.dylib"
		appEnv["OS_ACTIVITY_DT_MODE"] = "YES"
	}
	appArgs := []interface{}{
		"-NSTreatUnknownArgumentsAsOpen", "NO",
		"-ApplePersistenceIgnoreState", "YES",
	}
	appOpt := map[string]interface{}{
		"StartSuspendedKey": uint64(0),
	}
	if DeviceVersion(version...) >= DeviceVersion(12, 0, 0) {
		appOpt["ActivateSuspended"] = uint64(1)
	}

	if len(xcTestOpt.appEnv) != 0 {
		for k, v := range xcTestOpt.appEnv {
			appEnv[k] = v
		}
	}

	if len(xcTestOpt.appOpt) != 0 {
		for k, v := range xcTestOpt.appEnv {
			appOpt[k] = v
		}
	}

	d.instruments.registerCallback("outputReceived:fromProcess:atTime:", func(m libimobiledevice.DTXMessageResult) {
		// fmt.Println("###### instruments ### -->", m.Aux[0])
		_out <- fmt.Sprintf("%s", m.Aux[0])
	})

	var pid int
	if pid, err = d.instruments.AppLaunch(bundleID,
		WithAppPath(appPath),
		WithEnvironment(appEnv),
		WithArguments(appArgs),
		WithOptions(appOpt),
		WithKillExisting(true),
	); err != nil {
		return _out, cancelFunc, err
	}

	// see https://github.com/electricbubble/gidevice/issues/31
	// if err = d.instruments.startObserving(pid); err != nil {
	// 	return _out, cancelFunc, err
	// }

	if DeviceVersion(version...) >= DeviceVersion(12, 0, 0) {
		err = xcTestManager1.authorizeTestSession(pid)
	} else if DeviceVersion(version...) <= DeviceVersion(9, 0, 0) {
		err = xcTestManager1.initiateControlSessionForTestProcessID(pid)
	} else {
		err = xcTestManager1.initiateControlSessionForTestProcessIDProtocolVersion(pid, xcodeVersion)
	}
	if err != nil {
		return _out, cancelFunc, err
	}

	go func() {
		d.instruments.registerCallback("_Golang-iDevice_Over", func(_ libimobiledevice.DTXMessageResult) {
			cancelFunc()
		})

		<-ctx.Done()
		tmSrv1.close()
		tmSrv2.close()
		xcTestManager1.close()
		xcTestManager2.close()
		if _err := d.AppKill(pid); _err != nil {
			debugLog(fmt.Sprintf("xctest kill: %d", pid))
		}
		// time.Sleep(time.Second)
		close(_out)
		return
	}()

	return _out, cancelFunc, err
}

func (d *device) _uploadXCTestConfiguration(bundleID string, sessionId uuid.UUID, lookupResult map[string]interface{}) (pathXCTestCfg string, err error) {
	if _, err = d.HouseArrestService(); err != nil {
		return "", err
	}

	var appAfc Afc
	if appAfc, err = d.houseArrest.Container(bundleID); err != nil {
		return "", err
	}

	appTmpFilenames, err := appAfc.ReadDir("/tmp")
	if err != nil {
		return "", err
	}

	for _, tName := range appTmpFilenames {
		if strings.HasSuffix(tName, ".xctestconfiguration") {
			if _err := appAfc.Remove(fmt.Sprintf("/tmp/%s", tName)); _err != nil {
				debugLog(fmt.Sprintf("remove /tmp/%s: %s", tName, err))
				continue
			}
		}
	}

	nameExec := lookupResult["CFBundleExecutable"].(string)
	name := nameExec[:len(nameExec)-len("-Runner")]
	appPath := lookupResult["Path"].(string)

	pathXCTestCfg = fmt.Sprintf("/tmp/%s-%s.xctestconfiguration", name, strings.ToUpper(sessionId.String()))

	var content []byte
	if content, err = nskeyedarchiver.Marshal(
		nskeyedarchiver.NewXCTestConfiguration(
			nskeyedarchiver.NewNSUUID(sessionId.Bytes()),
			nskeyedarchiver.NewNSURL(fmt.Sprintf("%s/PlugIns/%s.xctest", appPath, name)),
			bundleID,
			appPath,
		),
	); err != nil {
		return "", err
	}

	if err = appAfc.WriteFile(pathXCTestCfg, content, AfcFileModeWr); err != nil {
		return "", err
	}

	return
}

func (d *device) GetBatteryInfo() (map[string]interface{}, error) {
	v, err := d.GetValue("com.apple.mobile.battery", "")
	if err != nil {
		return nil, err
	}
	var info map[string]interface{}
	var ok bool
	if info, ok = v.(map[string]interface{}); !ok {
		info = map[string]interface{}{}
	}

	if _, err = d.LockdownService(); err != nil {
		return info, err
	}
	if d.diagnosticsRelay, err = d.lockdown.DiagnosticsRelayService(); err != nil {
		return info, err
	}
	if powerInfo, err := d.diagnosticsRelay.PowerSource(); err != nil {
		return info, err
	} else {
		if powerInfo["Status"] == "Success" {
			if diagnostics, ok := powerInfo["Diagnostics"].(map[string]interface{}); ok {
				if ioRegistry, ok := diagnostics["IORegistry"].(map[string]interface{}); ok {
					battery, err := parseBatteryData(ioRegistry)
					if err != nil {
						log.Println("parse battery data error")
						return info, err
					}
					value := reflect.ValueOf(*battery)
					typeOfObj := value.Type()
					for i := 0; i < value.NumField(); i++ {
						fieldName := typeOfObj.Field(i).Name
						fieldValue := value.Field(i).Interface()
						info[fieldName] = fieldValue
					}
				}
			}

		}
		return info, nil
	}

	return nil, err
}

func (d *device) ProfilerStart(perfs []DataType, bundleId string) (<-chan string, error) {
	outData := make(chan string, 10)
	var pid = -1
	if bundleId != "" {
		for pid >= 0 {
			pid, err := d.instruments.getPidByBundleId(bundleId)
			if err != nil {
				log.Printf("get pid by bundleId %s error \n", bundleId)
				return nil, err
			}
			//process has not started yet, call start to get pid
			if pid == -1 {
				pid, err = d.instruments.AppLaunch(bundleId)
				if err != nil {
					log.Printf("start app by bundleId %s error \n", bundleId)
					return nil, err
				}
			}
		}
	}

	typeMap := map[DataType]bool{}
	for _, perf := range perfs {
		typeMap[perf] = true
	}

	if typeMap[FPS] {
		profiler, err := d.NewGraphicProfiler()
		if err != nil {
			return nil, err
		}
		fpsData, err := profiler.Start()
		go func() {
			for {
				data := <-fpsData
				outData <- data
				fmt.Println("Fps" + data)
			}
		}()
		d.profiler[FPS] = profiler
	}

	//seems it is useless, get the whole device network
	if typeMap[NETWORK] {
		profiler, err := d.NewNetworkProfiler()
		if err != nil {
			return nil, err
		}
		networkData, err := profiler.Start()
		go func() {
			for {
				data := <-networkData
				outData <- data
				fmt.Println("network" + data)
			}
		}()
		d.profiler[NETWORK] = profiler
	}

	if typeMap[CPU] || typeMap[MEMORY] || typeMap[NETWORK] || typeMap[DISK] {
		profiler, err := d.NewCpuAndMemoryProfiler()
		var param []string
		for k := range typeMap {
			param = append(param, string(k))
		}
		profiler.config = profiler.GenerateDefaultConfig(pid, param...)

		if err != nil {
			return nil, err
		}
		cpuData, err := profiler.Start()
		go func() {
			for {
				data := <-cpuData
				outData <- data
				fmt.Println("cpu" + data)
			}
		}()
		d.profiler[CPU] = profiler

	}

	return outData, nil
}
func (d *device) ProfilerStop() {
	for _, v := range d.profiler {
		v.Stop()
	}
}

func (d *device) GetPidByBundleId(bundleId string) (int, error) {
	var err error
	d.instruments, err = d.instrumentsService()
	if err != nil {
		return -1, err
	}
	return d.instruments.getPidByBundleId(bundleId)
}

type BatteryInfo struct {
	Serial                string  `json:"Serial,omitempty"`
	CurrentCapacity       float64 `json:"CurrentCapacity,omitempty"`
	CycleCount            int     `json:"CycleCount"`
	AbsoluteCapacity      float64 `json:"AbsoluteCapacity"`
	NominalChargeCapacity float64 `json:"NominalChargeCapacity"`
	DesignCapacity        float64 `json:"DesignCapacity"`
	Voltage               float64 `json:"Voltage"`
	BootVoltage           float64 `json:"BootVoltage"`
	AdapterDetailsVoltage uint64  `json:"AdapterDetailsVoltage,omitempty"`
	AdapterDetailsWatts   uint64  `json:"AdapterDetailsWatts,omitempty"`
	InstantAmperage       float64 `json:"InstantAmperage"`
	Temperature           float64 `json:"Temperature"`
}

func parseBatteryData(batteryData map[string]interface{}) (*BatteryInfo, error) {
	adapterDetailsData, ok := batteryData["AdapterDetails"].(map[string]interface{})
	if !ok {
		return nil, errors.New("failed to cast AdapterDetails data to map")
	}

	battery := &BatteryInfo{}
	battery.AdapterDetailsVoltage = adapterDetailsData["Voltage"].(uint64)
	battery.AdapterDetailsWatts = adapterDetailsData["Watts"].(uint64)

	// Use a defer statement to handle any unmarshalling errors.
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("failed to unmarshal IORegistry data: %v", r)
			fmt.Println(err)
		}
	}()

	// Unmarshal the IORegistry data directly into the Battery struct.
	registryDataBytes, err := json.Marshal(batteryData)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(registryDataBytes, battery); err != nil {
		return nil, err
	}

	return battery, nil
}

func (c *CpuAndMemoryProfiler) parseProcessData(dataArray []interface{}) {
	/**
	dataArray example:
	[
	  map[
	    CPUCount:2
	    EnabledCPUs:2
	    PerCPUUsage:[
	      map[CPU_NiceLoad:0 CPU_SystemLoad:-1 CPU_TotalLoad:3.6363636363636402 CPU_UserLoad:-1]
	      map[CPU_NiceLoad:0 CPU_SystemLoad:-1 CPU_TotalLoad:2.7272727272727195 CPU_UserLoad:-1]
	    ]
	    System:[36408520704 6897049600 3031160 773697 15596 61940 1297 26942 588 17020 127346 1835008 119718056 107009899 174046 103548]
	    SystemCPUUsage:map[CPU_NiceLoad:0 CPU_SystemLoad:-1 CPU_TotalLoad:6.36363636363636 CPU_UserLoad:-1]
	    StartMachAbsTime:5896602132889
	    EndMachAbsTime:5896628486761
	    Type:41
	 ]
	 map[
	   Processes:map[
	     0:[1.3582834340402803 0]
	     124:[0.011456702068519481 124]
	     136:[0.05468332721703649 136]
	   ]
	   StartMachAbsTime:5896602295095
	   EndMachAbsTime:5896628780514
	   Type:5
	  ]
	]
	*/

	processData := make(map[string]interface{})
	processData["type"] = "process"
	processData["timestamp"] = time.Now().Unix()
	processData["pid"] = c.pid

	defer func() {
		processBytes, _ := json.Marshal(processData)
		c.chanProcess <- processBytes
	}()

	systemInfo := dataArray[0].(map[string]interface{})
	processInfo := dataArray[1].(map[string]interface{})
	if _, ok := systemInfo["System"]; !ok {
		systemInfo, processInfo = processInfo, systemInfo
	}

	var targetProcessValue []interface{}
	processList := processInfo["Processes"].(map[string]interface{})
	for pid, v := range processList {
		if pid != strconv.Itoa(c.pid) {
			continue
		}
		targetProcessValue = v.([]interface{})
	}

	if targetProcessValue == nil {
		processData["msg"] = fmt.Sprintf("process %d not found", c.pid)
		return
	}

	processAttributesMap := make(map[string]interface{})
	for idx, value := range c.options.ProcessAttributes {
		processAttributesMap[value] = targetProcessValue[idx]
	}
	processData["proc_perf"] = processAttributesMap
	//
	//systemAttributesValue := systemInfo["System"].([]interface{})
	//systemAttributesMap := make(map[string]int64)
	//for idx, value := range c.options.SystemAttributes {
	//	systemAttributesMap[value] = convert2Int64(systemAttributesValue[idx])
	//}
	//processData["sys_perf"] = systemAttributesMap
}

func (c *CpuAndMemoryProfiler) parseSystemData(dataArray []interface{}) {
	timestamp := time.Now().Unix()
	var systemInfo map[string]interface{}

	var dataTime uint64 = 0
	for _, value := range dataArray {
		t, ok := value.(map[string]interface{})
		if ok && t["SystemCPUUsage"] != nil && t["EndMachAbsTime"] != nil && t["EndMachAbsTime"].(uint64) > dataTime {
			systemInfo = t
			dataTime = t["EndMachAbsTime"].(uint64)
		}
	}

	/**
	systemInfo example:
	map[
	  CPUCount:2
	  EnabledCPUs:2
	  PerCPUUsage:[
	    map[CPU_NiceLoad:0 CPU_SystemLoad:-1 CPU_TotalLoad:3.9215686274509807 CPU_UserLoad:-1]
	    map[CPU_NiceLoad:0 CPU_SystemLoad:-1 CPU_TotalLoad:11.650485436893206 CPU_UserLoad:-1]]
	  ]
	  System:[704211 35486281728 6303789056 3001119 1001 11033 52668 1740 40022 2114 17310 126903 1835008 160323 107909856 95067 95808179]
	  SystemCPUUsage:map[
	    CPU_NiceLoad:0
	    CPU_SystemLoad:-1
	    CPU_TotalLoad:15.572054064344186
	    CPU_UserLoad:-1
	  ]
	  StartMachAbsTime:5339240248449
	  EndMachAbsTime:5339264441260
	  Type:41
	]
	*/

	if c.options.SysCPU {
		sysCPUUsage := systemInfo["SystemCPUUsage"].(map[string]interface{})
		sysCPUInfo := SystemCPUData{
			PerfDataBase: PerfDataBase{
				Type:      "sys_cpu",
				TimeStamp: timestamp,
			},
			NiceLoad:   sysCPUUsage["CPU_NiceLoad"].(float64),
			SystemLoad: sysCPUUsage["CPU_SystemLoad"].(float64),
			TotalLoad:  sysCPUUsage["CPU_TotalLoad"].(float64),
			UserLoad:   sysCPUUsage["CPU_UserLoad"].(float64),
		}
		cpuBytes, _ := json.Marshal(sysCPUInfo)
		c.chanSysCPU <- cpuBytes
	}

	systemAttributesValue := systemInfo["System"].([]interface{})
	systemAttributesMap := make(map[string]int64)
	for idx, value := range c.options.SystemAttributes {
		systemAttributesMap[value] = convert2Int64(systemAttributesValue[idx])
	}

	if c.options.SysMem {
		kernelPageSize := int64(16384) // core_profile_session_tap get kernel_page_size
		// kernelPageSize := int64(1) // why 16384 ?
		appMemory := (systemAttributesMap["vmIntPageCount"] - systemAttributesMap["vmPurgeableCount"]) * kernelPageSize
		cachedFiles := (systemAttributesMap["vmExtPageCount"] + systemAttributesMap["vmPurgeableCount"]) * kernelPageSize
		compressed := systemAttributesMap["vmCompressorPageCount"] * kernelPageSize
		usedMemory := (systemAttributesMap["vmUsedCount"] - systemAttributesMap["vmExtPageCount"]) * kernelPageSize
		wiredMemory := systemAttributesMap["vmWireCount"] * kernelPageSize
		swapUsed := systemAttributesMap["__vmSwapUsage"]
		freeMemory := systemAttributesMap["vmFreeCount"] * kernelPageSize

		sysMemInfo := SystemMemData{
			PerfDataBase: PerfDataBase{
				Type:      "sys_mem",
				TimeStamp: timestamp,
			},
			AppMemory:   appMemory,
			UsedMemory:  usedMemory,
			WiredMemory: wiredMemory,
			FreeMemory:  freeMemory,
			CachedFiles: cachedFiles,
			Compressed:  compressed,
			SwapUsed:    swapUsed,
		}
		memBytes, _ := json.Marshal(sysMemInfo)
		c.chanSysMem <- memBytes
	}

	if c.options.SysDisk {
		diskBytesRead := systemAttributesMap["diskBytesRead"]
		diskBytesWritten := systemAttributesMap["diskBytesWritten"]
		diskReadOps := systemAttributesMap["diskReadOps"]
		diskWriteOps := systemAttributesMap["diskWriteOps"]

		sysDiskInfo := SystemDiskData{
			PerfDataBase: PerfDataBase{
				Type:      "sys_disk",
				TimeStamp: timestamp,
			},
			DataRead:    diskBytesRead,
			DataWritten: diskBytesWritten,
			ReadOps:     diskReadOps,
			WriteOps:    diskWriteOps,
		}
		diskBytes, _ := json.Marshal(sysDiskInfo)
		c.chanSysDisk <- diskBytes
	}

	if c.options.SysNetwork {
		netBytesIn := systemAttributesMap["netBytesIn"]
		netBytesOut := systemAttributesMap["netBytesOut"]
		netPacketsIn := systemAttributesMap["netPacketsIn"]
		netPacketsOut := systemAttributesMap["netPacketsOut"]

		sysNetworkInfo := SystemNetworkData{
			PerfDataBase: PerfDataBase{
				Type:      "sys_network",
				TimeStamp: timestamp,
			},
			BytesIn:    netBytesIn,
			BytesOut:   netBytesOut,
			PacketsIn:  netPacketsIn,
			PacketsOut: netPacketsOut,
		}
		networkBytes, _ := json.Marshal(sysNetworkInfo)
		c.chanSysNetwork <- networkBytes
	}
}

type SystemCPUData struct {
	PerfDataBase         // system cpu
	NiceLoad     float64 `json:"nice_load"`
	SystemLoad   float64 `json:"system_load"`
	TotalLoad    float64 `json:"total_load"`
	UserLoad     float64 `json:"user_load"`
}

type SystemMemData struct {
	PerfDataBase       // mem
	AppMemory    int64 `json:"app_memory"`
	FreeMemory   int64 `json:"free_memory"`
	UsedMemory   int64 `json:"used_memory"`
	WiredMemory  int64 `json:"wired_memory"`
	CachedFiles  int64 `json:"cached_files"`
	Compressed   int64 `json:"compressed"`
	SwapUsed     int64 `json:"swap_used"`
}

type SystemDiskData struct {
	PerfDataBase       // disk
	DataRead     int64 `json:"data_read"`
	DataWritten  int64 `json:"data_written"`
	ReadOps      int64 `json:"reads_in"`
	WriteOps     int64 `json:"writes_out"`
}

type SystemNetworkData struct {
	PerfDataBase       // network
	BytesIn      int64 `json:"bytes_in"`
	BytesOut     int64 `json:"bytes_out"`
	PacketsIn    int64 `json:"packets_in"`
	PacketsOut   int64 `json:"packets_out"`
}
