package machineinfo

import "testing"

func TestFromCPUInfoMapsInfraMachineFields(t *testing.T) {
	got, err := fromCPUInfo("amd64", []cpuInfo{{
		Family: "6", Model: "165", ModelName: "Intel Test CPU", Flags: []string{"sse4_2", "avx2"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got.CPUArchitecture != "amd64" || got.CPUFamily != "6" || got.CPUModel != "165" || got.CPUModelName != "Intel Test CPU" {
		t.Fatalf("machine info = %#v", got)
	}
	if len(got.CPUFlags) != 2 || got.CPUFlags[1] != "avx2" {
		t.Fatalf("CPUFlags = %#v", got.CPUFlags)
	}
}

func TestFromCPUInfoUsesExplicitARM64Fallback(t *testing.T) {
	got, err := fromCPUInfo("arm64", []cpuInfo{{ModelName: "ARM Test CPU", Flags: []string{"asimd"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got.CPUArchitecture != "arm64" || got.CPUFamily != "arm64" || got.CPUModel != "0" {
		t.Fatalf("ARM64 machine info = %#v", got)
	}
}

func TestFromCPUInfoRejectsMissingCPU(t *testing.T) {
	if _, err := fromCPUInfo("amd64", nil); err == nil {
		t.Fatal("fromCPUInfo() error = nil, want missing CPU error")
	}
}

func TestFromCPUInfoKeepsEmptyFlagsAsJSONArray(t *testing.T) {
	got, err := fromCPUInfo("amd64", []cpuInfo{{Family: "6", Model: "1", Flags: []string{}}})
	if err != nil {
		t.Fatal(err)
	}
	if got.CPUFlags == nil {
		t.Fatal("CPUFlags = nil, want non-nil empty slice")
	}
}
