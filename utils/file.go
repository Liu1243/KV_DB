package utils

import (
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

// DirSize 获取一个目录的大小
func DirSize(dirPath string) (int64, error) {
	var size int64
	err := filepath.Walk(dirPath, func(_ string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}

// AvailableDiskSize 获取磁盘剩余可用空间大小（兼容Windows和Unix-like系统）
func AvailableDiskSize() (uint64, error) {
	// 获取当前工作目录
	wd, err := syscall.Getwd()
	if err != nil {
		return 0, err
	}

	// 根据操作系统选择不同的实现
	//if windowsAvailable, err := getWindowsAvailableDiskSize(wd); err == nil {
	//	return windowsAvailable, nil
	//}
	return getWindowsAvailableDiskSize(wd)

	// 如果Windows实现失败或非Windows系统，尝试Unix-like实现
	//return getUnixAvailableDiskSize(wd)
}

// Windows实现
func getWindowsAvailableDiskSize(path string) (uint64, error) {
	// 将路径转换为UTF16指针
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}

	var freeBytesAvailable, totalNumberOfBytes, totalNumberOfFreeBytes uint64

	// 调用Windows API GetDiskFreeSpaceExW
	ret, _, err := syscall.NewLazyDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW").Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalNumberOfBytes)),
		uintptr(unsafe.Pointer(&totalNumberOfFreeBytes)),
	)

	if ret == 0 {
		return 0, err
	}

	return freeBytesAvailable, nil
}

// Unix-like实现
//func getUnixAvailableDiskSize(path string) (uint64, error) {
//	var stat syscall.Statfs_t
//	if err := syscall.Statfs(path, &stat); err != nil {
//		return 0, err
//	}
//	return stat.Bavail * uint64(stat.Bsize), nil
//}
