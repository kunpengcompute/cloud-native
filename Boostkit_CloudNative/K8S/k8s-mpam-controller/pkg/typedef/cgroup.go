package typedef

import (
	"fmt"
	"k8s-mpam-controller/pkg/util"
	"path/filepath"
	"strings"
)

const (
	RootDir string = "/sys/fs/cgroup"
)

// Hierarchy is used to represent a cgroup path
type Hierarchy struct {
	MountPoint string `json:"mountPoint,omitempty"`
	Path       string `json:"cgroupPath"`
}

type (
	// Key uniquely determines the cgroup value of the container or pod
	Key struct {
		// SubSys refers to the subsystem of the cgroup
		SubSys string
		// FileName represents the cgroup file name
		FileName string
	}
	// Attr represents a single cgroup attribute, and Err represents whether the Value is available
	Attr struct {
		Value string
		Err   error
	}
	// SetterAndGetter is used for set and get value to/from cgroup file
	SetterAndGetter interface {
		SetCgroupAttr(*Key, string) error
		GetCgroupAttr(*Key) *Attr
	}
)

// AbsoluteCgroupPath returns the absolute path of the cgroup
func AbsoluteCgroupPath(elem ...string) string {
	elem = append([]string{RootDir}, elem...)
	return filepath.Join(elem...)
}

// ReadCgroupFile reads data from cgroup files
func ReadCgroupFile(elem ...string) ([]byte, error) {
	return readCgroupFile(AbsoluteCgroupPath(elem...))
}

// WriteCgroupFile writes data to cgroup file
func WriteCgroupFile(content string, elem ...string) error {
	return writeCgroupFile(AbsoluteCgroupPath(elem...), content)
}

func readCgroupFile(cgPath string) ([]byte, error) {
	if !util.PathExist(cgPath) {
		return nil, fmt.Errorf("%v: no such file or directory", cgPath)
	}
	return util.ReadFile(cgPath)
}

func writeCgroupFile(cgPath, content string) error {
	if !util.PathExist(cgPath) {
		return fmt.Errorf("%v: no such file or directory", cgPath)
	}
	return util.WriteFile(cgPath, content)
}

// GetMountDir returns the mount point path of the cgroup
func GetMountDir() string {
	return RootDir
}

// Int64 parses CgroupAttr as int64 type
func (attr *Attr) Int64() (int64, error) {
	if attr.Err != nil {
		return 0, attr.Err
	}
	return util.ParseInt64(attr.Value)
}

// NewHierarchy creates a Hierarchy instance
func NewHierarchy(mountPoint, path string) *Hierarchy {
	return &Hierarchy{
		MountPoint: mountPoint,
		Path:       path,
	}
}

// SetCgroupAttr sets value to the cgroup file
func (h *Hierarchy) SetCgroupAttr(key *Key, value string) error {
	if err := validateCgroupKey(key); err != nil {
		return err
	}
	var mountPoint = RootDir
	if len(h.MountPoint) > 0 {
		mountPoint = h.MountPoint
	}
	return writeCgroupFile(filepath.Join(mountPoint, key.SubSys, h.Path, key.FileName), value)
}

// GetCgroupAttr gets cgroup file content
func (h *Hierarchy) GetCgroupAttr(key *Key) *Attr {
	if err := validateCgroupKey(key); err != nil {
		return &Attr{Err: err}
	}
	var mountPoint = RootDir
	if len(h.MountPoint) > 0 {
		mountPoint = h.MountPoint
	}
	data, err := readCgroupFile(filepath.Join(mountPoint, key.SubSys, h.Path, key.FileName))
	if err != nil {
		return &Attr{Err: err}
	}
	return &Attr{Value: strings.TrimSpace(string(data)), Err: nil}
}

// validateCgroupKey is used to verify the validity of the cgroup key
func validateCgroupKey(key *Key) error {
	if key == nil {
		return fmt.Errorf("key cannot be empty")
	}
	if len(key.SubSys) == 0 || len(key.FileName) == 0 {
		return fmt.Errorf("invalid key")
	}
	return nil
}
