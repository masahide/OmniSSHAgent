package pageant

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"syscall"
	"unsafe"

	"github.com/cwchiu/go-winapi"
)

var (
	lock sync.Mutex

	msg = mkMsg([]byte{11})

	ErrPageantNotFound = errors.New("pageant process not found")
	ErrSendMessage     = errors.New("error sending message")

	winGetCurrentThreadID = winAPI("kernel32.dll", "GetCurrentThreadId")
)

const (
	MaxMessageLen = 8192
	wmCopydata    = 74
)

func winAPI(dllName, funcName string) func(...uintptr) (uintptr, uintptr, error) {
	proc := syscall.MustLoadDLL(dllName).MustFindProc(funcName)
	return func(a ...uintptr) (uintptr, uintptr, error) { return proc.Call(a...) }
}

func mkMsg(req []byte) []byte {
	m := make([]byte, 4+len(req))
	binary.BigEndian.PutUint32(m, uint32(len(req)))
	copy(m[4:], req)
	return m
}

func pageantWindow() winapi.HWND {
	nameP := "Pageant"
	h := winapi.FindWindow(
		syscall.StringToUTF16Ptr(nameP),
		syscall.StringToUTF16Ptr(nameP),
	)
	return h
}

func AlreadyRunning() ([]byte, error) {
	lock.Lock()
	defer lock.Unlock()

	paWin := pageantWindow()

	if paWin == 0 {
		return nil, ErrPageantNotFound
	}

	thID, _, _ := winGetCurrentThreadID()
	mapName := fmt.Sprintf("PageantRequest%08x", thID)
	pMapName, _ := syscall.UTF16PtrFromString(mapName)

	mmap, err := syscall.CreateFileMapping(syscall.InvalidHandle, nil, syscall.PAGE_READWRITE, 0, MaxMessageLen+4, pMapName)
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(mmap)
	ptr, err := syscall.MapViewOfFile(mmap, syscall.FILE_MAP_WRITE, 0, 0, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.UnmapViewOfFile(ptr)
	mmSlice := (*(*[MaxMessageLen]byte)(unsafe.Pointer(ptr)))[:]
	copy(mmSlice, msg)
	mapNameBytesZ := append([]byte(mapName), 0)
	cds := copyDataStruct{
		dwData: checkID,
		cbData: uint32(len(mapNameBytesZ)),
		lpData: uintptr(unsafe.Pointer(&(mapNameBytesZ[0]))),
	}
	resp := winapi.SendMessage(paWin, wmCopydata, 0, uintptr(unsafe.Pointer(&cds)))
	if resp == 0 {
		return nil, ErrSendMessage
	}

	respLen := binary.BigEndian.Uint32(mmSlice[:4])
	if respLen > MaxMessageLen-4 {
		return nil, ErrSendMessage
	}

	respData := make([]byte, respLen+4)
	copy(respData, mmSlice)

	return respData, nil
}
