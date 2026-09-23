package master

import (
	"errors"
	"reflect"
	"testing"

	canopen "github.com/samsamfire/gocanopen/v2"
	"github.com/samsamfire/gocanopen/v2/pkg/nmt"
)

func TestPDOFrame(t *testing.T) {
	t.Parallel()
	frame, err := PDOFrame(0x187, []byte{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if frame.ID != 0x187 || frame.DLC != 3 || !reflect.DeepEqual(frame.Data[:3], []byte{1, 2, 3}) {
		t.Fatalf("frame = %+v", frame)
	}
	for _, cobID := range []uint16{0, 0x800} {
		if _, err := PDOFrame(cobID, nil); !errors.Is(err, ErrInvalidCOBID) {
			t.Fatalf("PDOFrame(%#x) error = %v", cobID, err)
		}
	}
	if _, err := PDOFrame(1, make([]byte, 9)); !errors.Is(err, ErrInvalidPDO) {
		t.Fatalf("oversize error = %v", err)
	}
}

func TestNMTFrame(t *testing.T) {
	t.Parallel()
	commands := []nmt.Command{
		nmt.CommandEnterOperational, nmt.CommandEnterStopped,
		nmt.CommandEnterPreOperational, nmt.CommandResetNode,
		nmt.CommandResetCommunication,
	}
	for _, command := range commands {
		frame, err := NMTFrame(command, 7)
		if err != nil {
			t.Fatal(err)
		}
		if frame.ID != 0 || frame.DLC != 2 || frame.Data[0] != byte(command) || frame.Data[1] != 7 {
			t.Fatalf("command %#x frame = %+v", command, frame)
		}
	}
	if _, err := NMTFrame(nmt.Command(3), 1); !errors.Is(err, ErrInvalidNMT) {
		t.Fatalf("invalid command error = %v", err)
	}
	if _, err := NMTFrame(nmt.CommandEnterOperational, 128); !errors.Is(err, ErrInvalidNodeID) {
		t.Fatalf("invalid node error = %v", err)
	}
	if _, err := NMTFrame(nmt.CommandEnterOperational, 0); err != nil {
		t.Fatalf("broadcast error = %v", err)
	}
}

func TestHeartbeatFrame(t *testing.T) {
	t.Parallel()
	states := []struct {
		value uint8
		name  string
	}{
		{value: nmt.StateInitializing, name: "bootup"},
		{value: nmt.StateStopped, name: "stopped"},
		{value: nmt.StateOperational, name: "operational"},
		{value: nmt.StatePreOperational, name: "preOperational"},
		{value: 42, name: "unknown"},
	}
	for _, state := range states {
		frame := canopen.NewFrame(0x707, 0, 1)
		frame.Data[0] = state.value
		heartbeat, err := ParseHeartbeat(frame)
		if err != nil {
			t.Fatal(err)
		}
		if heartbeat.NodeID != 7 || heartbeat.State != state.value || heartbeat.StateName != state.name || heartbeat.Bootup != (state.value == 0) {
			t.Fatalf("heartbeat = %+v", heartbeat)
		}
	}
	if cobID, err := HeartbeatCOBID(127); err != nil || cobID != 0x77f {
		t.Fatalf("HeartbeatCOBID() = %#x, %v", cobID, err)
	}
	for _, frame := range []canopen.Frame{
		canopen.NewFrame(0x700, 0, 1),
		canopen.NewFrame(0x780, 0, 1),
		canopen.NewFrame(0x701, 0, 0),
		canopen.NewFrame(canopen.CanRtrFlag|0x701, 0, 1),
	} {
		if _, err := ParseHeartbeat(frame); !errors.Is(err, ErrInvalidHeartbeat) {
			t.Fatalf("ParseHeartbeat(%+v) error = %v", frame, err)
		}
	}
}
