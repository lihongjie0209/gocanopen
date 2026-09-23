package slcan

import (
	"errors"
	"testing"

	canopen "github.com/samsamfire/gocanopen/v2"
)

func TestCodecGoldenFrames(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		frame canopen.Frame
		wire  string
	}{
		{name: "standard data", frame: frame(0x123, 0, []byte{0xDE, 0xAD}), wire: "t1232DEAD\r"},
		{name: "extended data", frame: frame(canopen.CanEffFlag|0x01ABCDE0, 0, []byte{0x01}), wire: "T01ABCDE0101\r"},
		{name: "standard remote", frame: frame(canopen.CanRtrFlag|0x321, 7, nil), wire: "r3217\r"},
		{name: "extended remote", frame: frame(canopen.CanEffFlag|canopen.CanRtrFlag|0x1ABCDEFF, 8, nil), wire: "R1ABCDEFF8\r"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := Encode(test.frame)
			if err != nil {
				t.Fatal(err)
			}
			if string(encoded) != test.wire {
				t.Fatalf("Encode() = %q, want %q", encoded, test.wire)
			}
			decoded, err := Decode(encoded)
			if err != nil {
				t.Fatal(err)
			}
			if decoded != test.frame {
				t.Fatalf("Decode() = %+v, want %+v", decoded, test.frame)
			}
		})
	}
}

func TestDecodeTimestampAndControl(t *testing.T) {
	t.Parallel()
	decoded, err := Decode([]byte("t1231AA1F2E\r"))
	if err != nil || decoded.ID != 0x123 || decoded.DLC != 1 || decoded.Data[0] != 0xAA {
		t.Fatalf("Decode timestamp = %+v, %v", decoded, err)
	}
	for _, wire := range []string{"\r", "\a\r", "F00\r", "V0101\r", "N1234\r", "z\r", "Z\r"} {
		if _, err := Decode([]byte(wire)); !errors.Is(err, ErrControl) {
			t.Fatalf("Decode(%q) error = %v, want ErrControl", wire, err)
		}
	}
}

func TestCodecRejectsMalformedFrames(t *testing.T) {
	t.Parallel()
	for _, wire := range []string{
		"x1230\r", "t12\r", "t8000\r", "T200000000\r", "t1239\r",
		"t1231\r", "t1231GG\r", "r1231AA\r", "t1230ABC\r", "t1231AAZZZZ\r",
	} {
		if _, err := Decode([]byte(wire)); !errors.Is(err, ErrInvalidFrame) {
			t.Fatalf("Decode(%q) error = %v, want ErrInvalidFrame", wire, err)
		}
	}
	invalid := []canopen.Frame{
		frame(0x800, 0, nil),
		frame(canopen.CanEffFlag|0x20000000, 0, nil),
		frame(0x123, 9, nil),
	}
	for _, value := range invalid {
		if _, err := Encode(value); !errors.Is(err, ErrInvalidFrame) {
			t.Fatalf("Encode(%+v) error = %v, want ErrInvalidFrame", value, err)
		}
	}
}

func frame(id uint32, dlc uint8, data []byte) canopen.Frame {
	result := canopen.Frame{ID: id, DLC: dlc}
	copy(result.Data[:], data)
	if data != nil {
		result.DLC = uint8(len(data))
	}
	return result
}
