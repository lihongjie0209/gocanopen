package sdo

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
)

const MaxMasterTransferBytes = 16 << 20

var (
	ErrMasterProtocol  = errors.New("CANopen SDO master protocol error")
	ErrMasterSizeLimit = errors.New("CANopen SDO master size limit exceeded")
)

// MasterExchange sends exactly one eight-byte SDO request and returns its
// correlated eight-byte response. Transport subscription, deadlines, and
// serialization are owned by the caller.
type MasterExchange func(context.Context, [8]byte) ([8]byte, error)

type MasterAbortError struct {
	Index    uint16
	SubIndex uint8
	Code     uint32
}

func (e *MasterAbortError) Error() string {
	return fmt.Sprintf("CANopen SDO abort %#08x at %#04x:%d", e.Code, e.Index, e.SubIndex)
}

func MasterUpload(ctx context.Context, index uint16, subIndex uint8, maxBytes int, exchange MasterExchange) ([]byte, error) {
	if err := validateMasterArgs(ctx, maxBytes, exchange); err != nil {
		return nil, err
	}
	request := [8]byte{0x40}
	binary.LittleEndian.PutUint16(request[1:3], index)
	request[3] = subIndex
	response, err := masterExchange(ctx, request, index, subIndex, exchange)
	if err != nil {
		return nil, err
	}
	command := response[0]
	if command>>5 != 2 || binary.LittleEndian.Uint16(response[1:3]) != index || response[3] != subIndex {
		return nil, fmt.Errorf("%w: invalid upload initiation response", ErrMasterProtocol)
	}
	if command&0x02 != 0 {
		length := 4
		if command&0x01 != 0 {
			length -= int(command>>2) & 0x03
		}
		if length > maxBytes {
			return nil, ErrMasterSizeLimit
		}
		return append([]byte(nil), response[4:4+length]...), nil
	}
	declaredSize := -1
	if command&0x01 != 0 {
		declared := binary.LittleEndian.Uint32(response[4:8])
		if declared > uint32(maxBytes) {
			return nil, ErrMasterSizeLimit
		}
		declaredSize = int(declared)
	}
	data := make([]byte, 0, max(0, declaredSize))
	toggle := byte(0)
	for {
		request = [8]byte{0x60 | toggle}
		response, err = masterExchange(ctx, request, index, subIndex, exchange)
		if err != nil {
			return nil, err
		}
		if response[0]>>5 != 0 || response[0]&0x10 != toggle {
			return nil, fmt.Errorf("%w: invalid upload segment response", ErrMasterProtocol)
		}
		unused := int(response[0]>>1) & 0x07
		last := response[0]&0x01 != 0
		if !last && unused != 0 {
			return nil, fmt.Errorf("%w: non-final upload segment declares unused bytes", ErrMasterProtocol)
		}
		count := 7 - unused
		if len(data)+count > maxBytes {
			return nil, ErrMasterSizeLimit
		}
		data = append(data, response[1:1+count]...)
		if last {
			if declaredSize >= 0 && len(data) != declaredSize {
				return nil, fmt.Errorf("%w: upload size mismatch", ErrMasterProtocol)
			}
			return data, nil
		}
		toggle ^= 0x10
	}
}

func MasterDownload(ctx context.Context, index uint16, subIndex uint8, data []byte, exchange MasterExchange) error {
	if err := validateMasterArgs(ctx, max(1, len(data)), exchange); err != nil {
		return err
	}
	if len(data) > MaxMasterTransferBytes {
		return ErrMasterSizeLimit
	}
	request := [8]byte{}
	binary.LittleEndian.PutUint16(request[1:3], index)
	request[3] = subIndex
	expedited := len(data) >= 1 && len(data) <= 4
	if expedited {
		request[0] = 0x23 | byte(4-len(data))<<2
		copy(request[4:], data)
	} else {
		request[0] = 0x21
		binary.LittleEndian.PutUint32(request[4:], uint32(len(data)))
	}
	response, err := masterExchange(ctx, request, index, subIndex, exchange)
	if err != nil {
		return err
	}
	if response[0] != 0x60 || binary.LittleEndian.Uint16(response[1:3]) != index || response[3] != subIndex {
		return fmt.Errorf("%w: invalid download initiation response", ErrMasterProtocol)
	}
	if expedited {
		return nil
	}
	toggle := byte(0)
	for offset := 0; ; {
		remaining := len(data) - offset
		count := min(7, remaining)
		last := count == remaining
		request = [8]byte{}
		request[0] = toggle | byte(7-count)<<1
		if last {
			request[0] |= 0x01
		}
		copy(request[1:], data[offset:offset+count])
		response, err = masterExchange(ctx, request, index, subIndex, exchange)
		if err != nil {
			return err
		}
		if response[0]>>5 != 1 || response[0]&0x10 != toggle {
			return fmt.Errorf("%w: invalid download segment response", ErrMasterProtocol)
		}
		offset += count
		if last {
			return nil
		}
		toggle ^= 0x10
	}
}

func validateMasterArgs(ctx context.Context, maxBytes int, exchange MasterExchange) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if exchange == nil {
		return fmt.Errorf("%w: exchange is required", ErrMasterProtocol)
	}
	if maxBytes <= 0 || maxBytes > MaxMasterTransferBytes {
		return ErrMasterSizeLimit
	}
	return nil
}

func masterExchange(ctx context.Context, request [8]byte, index uint16, subIndex uint8, exchange MasterExchange) ([8]byte, error) {
	if err := ctx.Err(); err != nil {
		return [8]byte{}, err
	}
	response, err := exchange(ctx, request)
	if err != nil {
		return [8]byte{}, err
	}
	if response[0] == 0x80 {
		responseIndex := binary.LittleEndian.Uint16(response[1:3])
		responseSubIndex := response[3]
		if responseIndex != index || responseSubIndex != subIndex {
			return [8]byte{}, fmt.Errorf("%w: abort object mismatch", ErrMasterProtocol)
		}
		return [8]byte{}, &MasterAbortError{Index: responseIndex, SubIndex: responseSubIndex, Code: binary.LittleEndian.Uint32(response[4:8])}
	}
	return response, nil
}
