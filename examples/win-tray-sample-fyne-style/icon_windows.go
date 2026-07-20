//go:build windows

package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
	"unsafe"
)

const iconResourceVersion = 0x00030000

type icoEntry struct {
	width       int32
	height      int32
	bitCount    uint16
	bytesInRes  uint32
	imageOffset uint32
}

func createIconFromICO(ico []byte, desiredWidth, desiredHeight int32) (uintptr, error) {
	entry, err := selectICOEntry(ico, desiredWidth, desiredHeight)
	if err != nil {
		return 0, err
	}

	start := uint64(entry.imageOffset)
	end := start + uint64(entry.bytesInRes)
	if start >= uint64(len(ico)) || end > uint64(len(ico)) || start == end {
		return 0, errors.New("ICO image data is outside the file")
	}

	image := ico[start:end]
	r1, _, callErr := procCreateIconFromResourceEx.Call(
		uintptr(unsafe.Pointer(&image[0])),
		uintptr(len(image)),
		1,
		iconResourceVersion,
		uintptr(uint32(desiredWidth)),
		uintptr(uint32(desiredHeight)),
		lrDefaultColor,
	)
	runtime.KeepAlive(ico)

	if r1 == 0 {
		return 0, windowsCallError(callErr)
	}
	return r1, nil
}

func selectICOEntry(ico []byte, desiredWidth, desiredHeight int32) (icoEntry, error) {
	if len(ico) < 6 {
		return icoEntry{}, errors.New("ICO header is truncated")
	}
	if binary.LittleEndian.Uint16(ico[0:2]) != 0 {
		return icoEntry{}, errors.New("ICO reserved field is not zero")
	}
	if binary.LittleEndian.Uint16(ico[2:4]) != 1 {
		return icoEntry{}, errors.New("embedded file is not an icon")
	}

	count := int(binary.LittleEndian.Uint16(ico[4:6]))
	if count == 0 {
		return icoEntry{}, errors.New("ICO contains no images")
	}
	if 6+count*16 > len(ico) {
		return icoEntry{}, errors.New("ICO directory is truncated")
	}

	bestScore := int64(1<<62 - 1)
	var best icoEntry

	for i := 0; i < count; i++ {
		offset := 6 + i*16
		width := int32(ico[offset])
		height := int32(ico[offset+1])
		if width == 0 {
			width = 256
		}
		if height == 0 {
			height = 256
		}

		entry := icoEntry{
			width:       width,
			height:      height,
			bitCount:    binary.LittleEndian.Uint16(ico[offset+6 : offset+8]),
			bytesInRes:  binary.LittleEndian.Uint32(ico[offset+8 : offset+12]),
			imageOffset: binary.LittleEndian.Uint32(ico[offset+12 : offset+16]),
		}

		score := abs64(int64(width-desiredWidth))*1000 + abs64(int64(height-desiredHeight))*1000
		// Prefer higher color depth when dimensions are equal.
		score -= int64(entry.bitCount)
		if score < bestScore {
			bestScore = score
			best = entry
		}
	}

	if best.bytesInRes == 0 {
		return icoEntry{}, fmt.Errorf("ICO has no usable image for %dx%d", desiredWidth, desiredHeight)
	}
	return best, nil
}

func abs64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
