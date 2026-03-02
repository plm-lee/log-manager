package sysstats

import (
	"os"
	"runtime"

	"github.com/shirou/gopsutil/v3/process"
)

// ProcessStats 进程资源占用
type ProcessStats struct {
	MemAllocMB  float64 `json:"mem_alloc_mb"`
	MemSysMB    float64 `json:"mem_sys_mb"`
	CPUPercent  float64 `json:"cpu_percent"`
}

// GetProcessStats 获取当前进程内存与 CPU 占用
func GetProcessStats() (*ProcessStats, error) {
	s := &ProcessStats{}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	s.MemAllocMB = float64(ms.Alloc) / (1024 * 1024)
	s.MemSysMB = float64(ms.Sys) / (1024 * 1024)

	p, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		return s, nil
	}
	if cpu, err := p.CPUPercent(); err == nil {
		s.CPUPercent = cpu
	}
	return s, nil
}
