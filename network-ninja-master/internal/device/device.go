// Package device provides read-only local inventory. Values are collected from
// the running system; unavailable attributes are omitted instead of guessed.
package device

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
)

type Host struct {
	Hostname      string      `json:"hostname"`
	OS            string      `json:"os"`
	Architecture  string      `json:"architecture"`
	Kernel        string      `json:"kernel,omitempty"`
	CPUCount      int         `json:"cpu_count"`
	MemoryTotalKB uint64      `json:"memory_total_kb,omitempty"`
	Interfaces    []Interface `json:"interfaces"`
}

type Interface struct {
	Name         string   `json:"name"`
	HardwareAddr string   `json:"hardware_addr,omitempty"`
	Flags        string   `json:"flags"`
	Addresses    []string `json:"addresses"`
}

type Device struct {
	Path         string `json:"path"`
	Kind         string `json:"kind"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Product      string `json:"product,omitempty"`
	VendorID     string `json:"vendor_id,omitempty"`
	ProductID    string `json:"product_id,omitempty"`
	Serial       string `json:"serial,omitempty"`
	Driver       string `json:"driver,omitempty"`
}

func InspectHost() (Host, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return Host{}, fmt.Errorf("read hostname: %w", err)
	}
	result := Host{Hostname: hostname, OS: runtime.GOOS, Architecture: runtime.GOARCH, CPUCount: runtime.NumCPU()}
	if bytes, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		result.Kernel = strings.TrimSpace(string(bytes))
	}
	result.MemoryTotalKB = memoryTotalKB()
	interfaces, err := net.Interfaces()
	if err != nil {
		return Host{}, fmt.Errorf("enumerate interfaces: %w", err)
	}
	for _, item := range interfaces {
		addresses, _ := item.Addrs()
		entry := Interface{Name: item.Name, HardwareAddr: item.HardwareAddr.String(), Flags: item.Flags.String()}
		for _, address := range addresses {
			entry.Addresses = append(entry.Addresses, address.String())
		}
		sort.Strings(entry.Addresses)
		result.Interfaces = append(result.Interfaces, entry)
	}
	sort.Slice(result.Interfaces, func(i, j int) bool { return result.Interfaces[i].Name < result.Interfaces[j].Name })
	return result, nil
}

func memoryTotalKB() uint64 {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == "MemTotal:" {
			var value uint64
			fmt.Sscan(fields[1], &value)
			return value
		}
	}
	return 0
}

func Inventory() ([]Device, error) {
	var devices []Device
	usbDevices, err := usbInventory("/sys/bus/usb/devices")
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	devices = append(devices, usbDevices...)
	serialDevices, err := serialInventory()
	if err != nil {
		return nil, err
	}
	devices = append(devices, serialDevices...)
	sort.Slice(devices, func(i, j int) bool { return devices[i].Path < devices[j].Path })
	return devices, nil
}

func usbInventory(root string) ([]Device, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var devices []Device
	for _, entry := range entries {
		base := filepath.Join(root, entry.Name())
		vendor := readTrimmed(filepath.Join(base, "idVendor"))
		productID := readTrimmed(filepath.Join(base, "idProduct"))
		if vendor == "" || productID == "" {
			continue
		}
		device := Device{Path: base, Kind: "usb", VendorID: vendor, ProductID: productID, Manufacturer: readTrimmed(filepath.Join(base, "manufacturer")), Product: readTrimmed(filepath.Join(base, "product")), Serial: readTrimmed(filepath.Join(base, "serial"))}
		if link, err := os.Readlink(filepath.Join(base, "driver")); err == nil {
			device.Driver = filepath.Base(link)
		}
		devices = append(devices, device)
	}
	return devices, nil
}

func serialInventory() ([]Device, error) {
	matches, err := filepath.Glob("/dev/tty[A-Za-z]*")
	if err != nil {
		return nil, err
	}
	var devices []Device
	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil || info.Mode()&os.ModeDevice == 0 {
			continue
		}
		devices = append(devices, Device{Path: path, Kind: "serial"})
	}
	return devices, nil
}

func readTrimmed(path string) string {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(bytes))
}

// IsPrivileged reports effective root on Unix-like live environments. It is
// informational; no operation silently elevates itself.
func IsPrivileged() bool { return syscall.Geteuid() == 0 }
