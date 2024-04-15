package service

import (
	"context"
	"encoding/json"
	"github.com/airhandsome/go-ios/pkg/libimobiledevice"
	"log"
	"time"
)

type DataType string

const (
	SCREENSHOT DataType = "screenshot"
	CPU        DataType = "cpu"
	MEMORY     DataType = "memory"
	NETWORK    DataType = "network" // 流量
	FPS        DataType = "fps"
	DISK       DataType = "disk"
	PAGE       DataType = "page"
	GPU        DataType = "gpu"
)

type CpuAndMemoryProfiler struct {
	ins    Instruments
	ctx    context.Context
	config map[string]interface{}
	cancel context.CancelFunc
	pid    int
}

type NetworkProfiler struct {
	ins    Instruments
	ctx    context.Context
	cancel context.CancelFunc
}

type GraphicProfiler struct {
	ins    Instruments
	ctx    context.Context
	cancel context.CancelFunc
}

func (d *device) NewNetworkProfiler() (*NetworkProfiler, error) {
	ins, err := d.instrumentsService()
	if err != nil {
		log.Println("Can't get device instrument")
		return nil, err
	}
	return &NetworkProfiler{
		ins: ins,
	}, nil
}

func (p *NetworkProfiler) Start() (<-chan string, error) {
	_, err := p.ins.SetNetworkSampleConfig()
	if err != nil {
		return nil, err
	}

	_, err = p.ins.StartNetworkSample()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	outCh := make(chan string, 10)

	p.ins.registerCallback("", func(m libimobiledevice.DTXMessageResult) {
		select {
		case <-ctx.Done():
			p.ins.StopNetworkSample()
			return
		default:
			data, ok := m.Obj.(map[string]interface{})
			if ok {
				outData, err := json.Marshal(data)
				if err != nil {
					log.Println("marshal map error " + err.Error())

				} else {
					outCh <- string(outData)
				}
			}
		}
	})
	p.cancel = cancel
	return outCh, nil
}

func (p *NetworkProfiler) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
}

func (d *device) NewGraphicProfiler() (*GraphicProfiler, error) {
	ins, err := d.instrumentsService()
	if err != nil {
		log.Println("Can't get device instrument")
		return nil, err
	}
	return &GraphicProfiler{
		ins: ins,
	}, nil
}

func (p *GraphicProfiler) Start() (<-chan string, error) {
	_, err := p.ins.SetGraphicSampleRate(1000)
	if err != nil {
		return nil, err
	}

	_, err = p.ins.StartGraphicSample()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	outCh := make(chan string, 10)

	p.ins.registerCallback("", func(m libimobiledevice.DTXMessageResult) {
		select {
		case <-ctx.Done():
			p.ins.StopGraphicSample()
			return
		default:
			data, ok := m.Obj.(map[string]interface{})
			if ok {
				outData, err := json.Marshal(data)
				if err != nil {
					log.Println("marshal map error " + err.Error())

				} else {
					outCh <- string(outData)
				}
			}
		}
	})
	p.cancel = cancel
	return outCh, nil
}

func (p *GraphicProfiler) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
}

func (d *device) NewCpuAndMemoryProfiler() (*CpuAndMemoryProfiler, error) {
	ins, err := d.instrumentsService()
	if err != nil {
		log.Println("Can't get device instrument")
		return nil, err
	}
	return &CpuAndMemoryProfiler{
		ins: ins,
	}, nil
}

func (p *CpuAndMemoryProfiler) GenerateDefaultConfig(pid int, options ...string) map[string]interface{} {
	/*
	 config = {
	            "bm": 0,
	            "cpuUsage": True,
	            "procAttrs": [
	                "memVirtualSize", "cpuUsage", "ctxSwitch", "intWakeups",
	                "physFootprint", "memResidentSize", "memAnon", "pid"
	            ],
	            "sampleInterval": 1000000000, # 1e9 ns == 1s
	            "sysAttrs": [
	                "vmExtPageCount", "vmFreeCount", "vmPurgeableCount",
	                "vmSpeculativeCount", "physMemSize"
	            ],
	            "ur": 1000
	        }
	*/
	config := map[string]interface{}{
		"bm":             0,
		"cpuUsage":       true,
		"sampleInterval": time.Second, // 1e9 ns == 1s sample frequency, default 1 second
		"ur":             1000,        // output frequency
	}
	var sysAttrs []string
	var procAttrs []string
	procAttrs = append(procAttrs, "pid")
	for _, option := range options {
		if option == "disk" {
			sysAttrs = append(sysAttrs, p.GetDiskConfig()...)
		} else if option == "network" {
			sysAttrs = append(sysAttrs, p.GetNetworkConfig()...)
		} else if option == "cpu" {

			procAttrs = append(procAttrs, p.GetCpuConfig()...)
		} else if option == "memory" {
			sysAttrs = append(sysAttrs, p.GetMemoryConfig()...)
			procAttrs = append(procAttrs, p.GetMemoryProcessConfig()...)
		}
	}
	config["procAttrs"] = procAttrs
	config["sysAttrs"] = sysAttrs
	return config
}

func (p *CpuAndMemoryProfiler) GetNetworkConfig() []string {
	return []string{ // network
		"netBytesIn",
		"netBytesOut",
		"netPacketsIn",
		"netPacketsOut"}
}

func (p *CpuAndMemoryProfiler) GetDiskConfig() []string {
	return []string{ // disk
		"diskBytesRead",
		"diskBytesWritten",
		"diskReadOps",
		"diskWriteOps"}
}
func (p *CpuAndMemoryProfiler) GetCpuConfig() []string {
	return []string{
		"cpuUsage",
		"ctxSwitch",
		"intWakeups",
		"physFootprint",
	}
}

func (p *CpuAndMemoryProfiler) GetMemoryConfig() []string {
	return []string{
		"vmCompressorPageCount",
		"vmExtPageCount",
		"vmFreeCount",
		"vmIntPageCount",
		"vmPurgeableCount",
		"vmWireCount",
		"vmUsedCount",
		"vmSpeculativeCount",
		"__vmSwapUsage",
		"physMemSize",
	}
}

func (p *CpuAndMemoryProfiler) GetMemoryProcessConfig() []string {
	return []string{
		"memVirtualSize",
		"memResidentSize",
		"memAnon",
	}
}

func (p *CpuAndMemoryProfiler) Start() (<-chan string, error) {

	_, err := p.ins.SetCpuAndMemorySampleConfig(p.config)
	if err != nil {
		return nil, err
	}

	_, err = p.ins.StartCpuAndMemorySample()
	if err != nil {
		return nil, err
	}
	outCh := make(chan string, 100)

	// register listener
	ctx, cancel := context.WithCancel(context.TODO())
	p.ins.registerCallback("", func(m libimobiledevice.DTXMessageResult) {
		select {
		case <-ctx.Done():
			p.ins.StopCpuAndMemorySample()
			return
		default:
			dataArray, ok := m.Obj.([]interface{})
			if !ok || len(dataArray) < 2 {
				return
			}
			if p.pid != 0 {
				p.parseProcessData(dataArray, outCh)
				p.parseSystemData(dataArray, outCh)
			} else {
				p.parseSystemData(dataArray, outCh)
			}
		}
	})
	p.cancel = cancel

	return outCh, nil
}

func (p *CpuAndMemoryProfiler) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
}

type PerfDataBase struct {
	Type      string `json:"type"`
	TimeStamp int64  `json:"timestamp"`
	Msg       string `json:"msg,omitempty"` // message for invalid data
}

func ParseSystemData(array []interface{}) {

}
