//go:build windows

package installpath

import (
	"fmt"
	"syscall"
	"unsafe"
)

var registryDLL = syscall.NewLazyDLL("advapi32.dll")
var openKey = registryDLL.NewProc("RegOpenKeyExW")
var queryValue = registryDLL.NewProc("RegQueryValueExW")
var closeKey = registryDLL.NewProc("RegCloseKey")
var expandEnvironment = syscall.NewLazyDLL("kernel32.dll").NewProc("ExpandEnvironmentStringsW")

func readRegistry(key, name string, view uint32) (string, error) {
	keyW, err := syscall.UTF16PtrFromString(key)
	if err != nil {
		return "", err
	}
	nameW, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return "", err
	}
	var handle syscall.Handle
	code, _, _ := openKey.Call(uintptr(syscall.HKEY_LOCAL_MACHINE), uintptr(unsafe.Pointer(keyW)), 0, uintptr(1|view), uintptr(unsafe.Pointer(&handle)))
	if code != 0 {
		return "", syscall.Errno(code)
	}
	defer closeKey.Call(uintptr(handle))
	var kind, size uint32
	code, _, _ = queryValue.Call(uintptr(handle), uintptr(unsafe.Pointer(nameW)), 0, uintptr(unsafe.Pointer(&kind)), 0, uintptr(unsafe.Pointer(&size)))
	if code != 0 {
		return "", syscall.Errno(code)
	}
	if (kind != 1 && kind != 2) || size == 0 || size > 65536 || size%2 != 0 {
		return "", fmt.Errorf("invalid registry install path value")
	}
	value := make([]uint16, size/2+1)
	code, _, _ = queryValue.Call(uintptr(handle), uintptr(unsafe.Pointer(nameW)), 0, uintptr(unsafe.Pointer(&kind)), uintptr(unsafe.Pointer(&value[0])), uintptr(unsafe.Pointer(&size)))
	if code != 0 {
		return "", syscall.Errno(code)
	}
	if kind == 2 {
		expanded := make([]uint16, 32768)
		n, _, err := expandEnvironment.Call(uintptr(unsafe.Pointer(&value[0])), uintptr(unsafe.Pointer(&expanded[0])), uintptr(len(expanded)))
		if n == 0 {
			return "", err
		}
		if n > uintptr(len(expanded)) {
			return "", fmt.Errorf("expanded install path too long")
		}
		value = expanded
	}
	return syscall.UTF16ToString(value), nil
}
