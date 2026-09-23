package slcan

import (
	"context"
	"errors"
	"reflect"
	"testing"

	canopen "github.com/samsamfire/gocanopen/v2"
)

func TestControllerOpenSendReceiveClose(t *testing.T) {
	t.Parallel()
	var writes [][]byte
	var received []canopen.Frame
	controller, err := NewController(500000,
		func(_ context.Context, data []byte) error {
			writes = append(writes, append([]byte(nil), data...))
			return nil
		},
		func(_ context.Context, frame canopen.Frame) error {
			received = append(received, frame)
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := controller.Open(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := controller.Send(t.Context(), frame(0x123, 0, []byte{1, 2})); err != nil {
		t.Fatal(err)
	}
	if err := controller.Receive(t.Context(), []byte("T01ABCDE01AA\r")); err != nil {
		t.Fatal(err)
	}
	if err := controller.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	wantWrites := [][]byte{[]byte("C\r"), []byte("S6\r"), []byte("O\r"), []byte("t12320102\r"), []byte("C\r")}
	if !reflect.DeepEqual(writes, wantWrites) {
		t.Fatalf("writes = %q, want %q", writes, wantWrites)
	}
	if len(received) != 1 || received[0].ID != canopen.CanEffFlag|0x01ABCDE0 || received[0].DLC != 1 || received[0].Data[0] != 0xAA {
		t.Fatalf("received = %+v", received)
	}
}

func TestControllerValidationAndFailureBoundaries(t *testing.T) {
	t.Parallel()
	if _, err := NewController(333333, func(context.Context, []byte) error { return nil }, func(context.Context, canopen.Frame) error { return nil }); !errors.Is(err, ErrUnsupportedBitRate) {
		t.Fatalf("unsupported bitrate error = %v", err)
	}
	if _, err := NewController(500000, nil, func(context.Context, canopen.Frame) error { return nil }); err == nil {
		t.Fatal("nil writer accepted")
	}
	if _, err := NewController(500000, func(context.Context, []byte) error { return nil }, nil); err == nil {
		t.Fatal("nil handler accepted")
	}

	want := errors.New("write failed")
	writes := 0
	controller, err := NewController(500000, func(context.Context, []byte) error {
		writes++
		if writes == 2 {
			return want
		}
		return nil
	}, func(context.Context, canopen.Frame) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := controller.Open(t.Context()); !errors.Is(err, want) {
		t.Fatalf("Open error = %v, want %v", err, want)
	}
	if writes != 2 {
		t.Fatalf("writes = %d, want 2", writes)
	}
}

func TestControllerReceivePreservesProtocolErrors(t *testing.T) {
	t.Parallel()
	controller, err := NewController(500000, func(context.Context, []byte) error { return nil }, func(context.Context, canopen.Frame) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := controller.Receive(t.Context(), []byte("F00\r")); !errors.Is(err, ErrControl) {
		t.Fatalf("control error = %v", err)
	}
	if err := controller.Receive(t.Context(), []byte("t1239\r")); !errors.Is(err, ErrInvalidFrame) {
		t.Fatalf("invalid error = %v", err)
	}
}
