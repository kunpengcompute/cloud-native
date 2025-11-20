/*
This is just a fake kperf interface.
Do not use it directly.
*/
package kperf

// PmuDeviceAttr{} is a high level interface for initializing pmu events for devices,
// such as L3 cache, DDRC, PCIe, and SMMU, to collect metrics like bandwidth, latency, and others.
// This interface is an alternative option for initializing events besides PmuOpen.
type PmuDeviceAttr struct {
	Metric uint32
}

// PmuData{} is a struct containing variables for pmu data.
type PmuData struct {
	fake int
}

// PmuDeviceData is the main struct for kunpeng-perf-monitor to use.
type PmuDeviceData struct {
	Metric uint32
	// The metric value. The meaning of value depends on metric type.
	// Refer to comments of PmuDeviceMetric for detailed info.
	Count  float64
	NumaId uint32 // for pernuma metric
}

// PmuDataVo contains a list of PmuData.
type PmuDataVo struct {
	GoData []PmuData // PmuData list
	fd     int       // fd
}

// PmuDeviceDataVo is a list of PmuDeviceData.
type PmuDeviceDataVo struct {
	GoDeviceData []PmuDeviceData
}

// vars for PmuDeviceAttr to use.
var (
	PMU_HHA_CROSS_NUMA   uint32
	PMU_HHA_CROSS_SOCKET uint32
)

// PmuDeviceOpen() is a high level interface for initializing pmu events for devices,
// such as L3 cache, DDRC, PCIe, and SMMU, to collect metrics like bandwidth, latency, and others.
// This interface is an alternative option for initializing events besides PmuOpen.
func PmuDeviceOpen(attrs []PmuDeviceAttr) (int, error) {
	return 0, nil
}

// PmuClose() closes the pmu device with the given file descriptor.
func PmuClose(fd int) {

}

// PmuEnable() enables counting or sampling of task <pd> with the given file descriptor.
func PmuEnable(fd int) error {
	return nil
}

// PmuDisable() disables counting or sampling of task <pd> with the given file descriptor.
func PmuDisable(fd int) error {
	return nil
}

// PmuRead() collects pmu data starting from the last PmuEnable or PmuRead with the given file descriptor.
func PmuRead(fd int) (PmuDataVo, error) {
	return PmuDataVo{}, nil
}

// PmuDataFree() frees the pmu data pointer.
// param pmuDataVo
func PmuDataFree(dataVo PmuDataVo) {

}

// PmuGetDevMetric() queries device metrics from pmuData and metric array.
func PmuGetDevMetric(dataVo PmuDataVo, attrs []PmuDeviceAttr) (PmuDeviceDataVo, error) {
	return PmuDeviceDataVo{}, nil
}

// DevDataFree() frees the pmu device data pointer.
func DevDataFree(deviceData PmuDeviceDataVo) {

}
