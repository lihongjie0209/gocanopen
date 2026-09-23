// Package slcan implements strict Lawicel SLCAN classical-frame encoding.
package slcan

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	canopen "github.com/samsamfire/gocanopen/v2"
)

var (
	// ErrControl identifies a valid non-frame adapter response.
	ErrControl = errors.New("SLCAN control response")
	// ErrInvalidFrame identifies malformed or unsupported classical CAN data.
	ErrInvalidFrame = errors.New("invalid SLCAN frame")
)

const canFlagMask = canopen.CanEffFlag | canopen.CanRtrFlag

// Encode returns one carriage-return terminated Lawicel frame.
func Encode(frame canopen.Frame) ([]byte, error) {
	extended := frame.ID&canopen.CanEffFlag != 0
	remote := frame.ID&canopen.CanRtrFlag != 0
	if frame.ID & ^(canFlagMask|canopen.CanEffMask) != 0 || frame.DLC > 8 {
		return nil, ErrInvalidFrame
	}
	id := frame.ID & canopen.CanEffMask
	if (!extended && id > canopen.CanSffMask) || (extended && id > canopen.CanEffMask) {
		return nil, ErrInvalidFrame
	}
	prefix := byte('t')
	idWidth := 3
	if extended {
		prefix, idWidth = 'T', 8
	}
	if remote {
		if extended {
			prefix = 'R'
		} else {
			prefix = 'r'
		}
	}
	text := fmt.Sprintf("%c%0*X%X", prefix, idWidth, id, frame.DLC)
	if !remote {
		text += strings.ToUpper(hex.EncodeToString(frame.Data[:frame.DLC]))
	}
	return append([]byte(text), '\r'), nil
}

// Decode parses one Lawicel classical CAN frame. A trailing four-hex-digit
// adapter timestamp is accepted and deliberately discarded.
func Decode(wire []byte) (canopen.Frame, error) {
	line := strings.TrimSuffix(string(wire), "\r")
	if line == "" || line == "\a" || strings.ContainsRune("FVNzZ", rune(line[0])) {
		return canopen.Frame{}, ErrControl
	}
	prefix := line[0]
	extended := prefix == 'T' || prefix == 'R'
	remote := prefix == 'r' || prefix == 'R'
	if prefix != 't' && prefix != 'T' && prefix != 'r' && prefix != 'R' {
		return canopen.Frame{}, ErrInvalidFrame
	}
	idWidth := 3
	if extended {
		idWidth = 8
	}
	if len(line) < 1+idWidth+1 {
		return canopen.Frame{}, ErrInvalidFrame
	}
	id, err := strconv.ParseUint(line[1:1+idWidth], 16, 32)
	if err != nil || (!extended && id > uint64(canopen.CanSffMask)) || (extended && id > uint64(canopen.CanEffMask)) {
		return canopen.Frame{}, ErrInvalidFrame
	}
	dlc, err := strconv.ParseUint(line[1+idWidth:2+idWidth], 16, 4)
	if err != nil || dlc > 8 {
		return canopen.Frame{}, ErrInvalidFrame
	}
	payload := line[2+idWidth:]
	expected := int(dlc) * 2
	if remote {
		expected = 0
	}
	if len(payload) != expected && len(payload) != expected+4 {
		return canopen.Frame{}, ErrInvalidFrame
	}
	if len(payload) == expected+4 {
		if _, err := hex.DecodeString(payload[expected:]); err != nil {
			return canopen.Frame{}, ErrInvalidFrame
		}
	}
	data, err := hex.DecodeString(payload[:expected])
	if err != nil {
		return canopen.Frame{}, ErrInvalidFrame
	}
	result := canopen.Frame{ID: uint32(id), DLC: uint8(dlc)}
	if extended {
		result.ID |= canopen.CanEffFlag
	}
	if remote {
		result.ID |= canopen.CanRtrFlag
	}
	copy(result.Data[:], data)
	return result, nil
}
