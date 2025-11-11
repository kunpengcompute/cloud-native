#!/usr/bin/env python3
import os
import sys

UACCE_PATH = "/sys/class/uacce"

# 设备ID映射表
DEVICE_MAP = {
    "hpre": "0xa258",
    "sec": "0xa255",
    "zip": "0xa250",
}

def read_file(path):
    try:
        with open(path, "r") as f:
            return f.read().strip()
    except Exception as e:
        print(f"[WARN] Cannot read {path}: {e}")
        return None

def write_file(path, value):
    try:
        with open(path, "w") as f:
            f.write(str(value))
        print(f"[OK] Wrote {value} to {path}")
    except Exception as e:
        print(f"[ERROR] Failed to write {path}: {e}")

def scan_devices():
    """扫描 /sys/class/uacce/ 下的设备，找出匹配的目录"""
    matched_devices = []

    if not os.path.isdir(UACCE_PATH):
        print(f"Error: {UACCE_PATH} does not exist.")
        return matched_devices

    for dev in os.listdir(UACCE_PATH):
        if not dev.startswith("hisi_"):
            continue

        dev_path = os.path.join(UACCE_PATH, dev)
        device_dir = os.path.join(dev_path, "device")
        device_file = os.path.join(device_dir, "device")

        dev_id = read_file(device_file)
        if not dev_id:
            continue

        for name, devid in DEVICE_MAP.items():
            if dev_id.lower() == devid.lower():
                matched_devices.append((dev, name, device_dir))
                print(f"[MATCH] {dev} ({dev_id}) -> {name}")
                break

    return matched_devices

def main():
    if len(sys.argv) != 2:
        print(f"Usage: {sys.argv[0]} <num_vfs>")
        sys.exit(1)

    num_vfs = sys.argv[1]

    if not num_vfs.isdigit():
        print("Error: num_vfs must be a number.")
        sys.exit(1)

    # 第一阶段：扫描匹配的设备
    matched_devices = scan_devices()

    if not matched_devices:
        print("[INFO] No matching devices found.")
        sys.exit(0)

    print(f"\n[INFO] Found {len(matched_devices)} matching devices. Writing VF={num_vfs}...\n")

    # 第二阶段：仅对匹配的目录写入
    for dev, name, device_dir in matched_devices:
        sriov_path = os.path.join(device_dir, "sriov_numvfs")
        if not os.path.exists(sriov_path):
            print(f"[WARN] {sriov_path} not found for {dev}, skipping.")
            continue
        write_file(sriov_path, num_vfs)

if __name__ == "__main__":
    main()