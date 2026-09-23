// Package master provides strict, transport-neutral CANopen master frames.
package master

import (
	"errors"
	"fmt"

	canopen "github.com/samsamfire/gocanopen/v2"
	"github.com/samsamfire/gocanopen/v2/pkg/heartbeat"
	"github.com/samsamfire/gocanopen/v2/pkg/nmt"
)

var (
	ErrInvalidNodeID    = errors.New("invalid CANopen node ID")
	ErrInvalidCOBID     = errors.New("invalid CANopen COB-ID")
	ErrInvalidPDO       = errors.New("invalid CANopen PDO")
	ErrInvalidNMT       = errors.New("invalid CANopen NMT command")
	ErrInvalidHeartbeat = errors.New("invalid CANopen heartbeat")
)

type Heartbeat struct {
	NodeID    uint8
	State     uint8
	StateName string
	Bootup    bool
}

func PDOFrame(cobID uint16, data []byte) (canopen.Frame, error) {
	if cobID == 0 || uint32(cobID) > canopen.CanSffMask {
		return canopen.Frame{}, fmt.Errorf("%w: %#x", ErrInvalidCOBID, cobID)
	}
	if len(data) > 8 {
		return canopen.Frame{}, fmt.Errorf("%w: payload has %d bytes", ErrInvalidPDO, len(data))
	}
	frame := canopen.NewFrame(uint32(cobID), 0, uint8(len(data)))
	copy(frame.Data[:], data)
	return frame, nil
}

func NMTFrame(command nmt.Command, nodeID uint8) (canopen.Frame, error) {
	if nodeID > 127 {
		return canopen.Frame{}, fmt.Errorf("%w: %d", ErrInvalidNodeID, nodeID)
	}
	switch command {
	case nmt.CommandEnterOperational, nmt.CommandEnterStopped,
		nmt.CommandEnterPreOperational, nmt.CommandResetNode,
		nmt.CommandResetCommunication:
	default:
		return canopen.Frame{}, fmt.Errorf("%w: %#x", ErrInvalidNMT, command)
	}
	frame := canopen.NewFrame(nmt.ServiceId, 0, 2)
	frame.Data[0] = byte(command)
	frame.Data[1] = nodeID
	return frame, nil
}

func HeartbeatCOBID(nodeID uint8) (uint16, error) {
	if nodeID == 0 || nodeID > 127 {
		return 0, fmt.Errorf("%w: %d", ErrInvalidNodeID, nodeID)
	}
	return heartbeat.ServiceId + uint16(nodeID), nil
}

func ParseHeartbeat(frame canopen.Frame) (Heartbeat, error) {
	if frame.ID&^canopen.CanSffMask != 0 || frame.ID <= heartbeat.ServiceId || frame.ID > heartbeat.ServiceId+127 || frame.DLC != 1 {
		return Heartbeat{}, ErrInvalidHeartbeat
	}
	nodeID := uint8(frame.ID - heartbeat.ServiceId)
	state := frame.Data[0]
	return Heartbeat{NodeID: nodeID, State: state, StateName: heartbeatStateName(state), Bootup: state == nmt.StateInitializing}, nil
}

func heartbeatStateName(state uint8) string {
	switch state {
	case nmt.StateInitializing:
		return "bootup"
	case nmt.StateStopped:
		return "stopped"
	case nmt.StateOperational:
		return "operational"
	case nmt.StatePreOperational:
		return "preOperational"
	default:
		return "unknown"
	}
}
