package slcan

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestControllerOpenSendReceiveClose(t *testing.T) {
	t.Parallel()
	var writes [][]byte
	controller, err := NewController(500000,
		func(_ context.Context, data []byte) error {
			writes = append(writes, append([]byte(nil), data...))
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
	if err := controller.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	wantWrites := [][]byte{[]byte("C\r"), []byte("S6\r"), []byte("O\r"), []byte("t12320102\r"), []byte("C\r")}
	if !reflect.DeepEqual(writes, wantWrites) {
		t.Fatalf("writes = %q, want %q", writes, wantWrites)
	}
}

func TestControllerValidationAndFailureBoundaries(t *testing.T) {
	t.Parallel()
	if _, err := NewController(333333, func(context.Context, []byte) error { return nil }); !errors.Is(err, ErrUnsupportedBitRate) {
		t.Fatalf("unsupported bitrate error = %v", err)
	}
	if _, err := NewController(500000, nil); err == nil {
		t.Fatal("nil writer accepted")
	}

	want := errors.New("write failed")
	writes := 0
	controller, err := NewController(500000, func(context.Context, []byte) error {
		writes++
		if writes == 2 {
			return want
		}
		return nil
	})
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
