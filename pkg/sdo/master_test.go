package sdo

import (
	"context"
	"encoding/binary"
	"errors"
	"reflect"
	"testing"
)

func TestMasterUploadExpeditedAndSegmented(t *testing.T) {
	t.Parallel()
	t.Run("expedited", func(t *testing.T) {
		exchange := scriptedExchange(t, [][8]byte{{0x4B, 0x00, 0x20, 0x01, 0x34, 0x12}})
		data, err := MasterUpload(t.Context(), 0x2000, 1, 16, exchange)
		if err != nil || !reflect.DeepEqual(data, []byte{0x34, 0x12}) {
			t.Fatalf("MasterUpload() = %x, %v", data, err)
		}
	})
	t.Run("segmented", func(t *testing.T) {
		init := [8]byte{0x41, 0x00, 0x20, 0x01}
		binary.LittleEndian.PutUint32(init[4:], 9)
		exchange := scriptedExchange(t, [][8]byte{
			init,
			{0x00, 1, 2, 3, 4, 5, 6, 7},
			{0x1B, 8, 9},
		})
		data, err := MasterUpload(t.Context(), 0x2000, 1, 16, exchange)
		if err != nil || !reflect.DeepEqual(data, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}) {
			t.Fatalf("MasterUpload() = %x, %v", data, err)
		}
	})
}

func TestMasterDownloadExpeditedSegmentedAndEmpty(t *testing.T) {
	t.Parallel()
	t.Run("expedited", func(t *testing.T) {
		exchange := scriptedExchange(t, [][8]byte{{0x60, 0x00, 0x20, 0x01}})
		if err := MasterDownload(t.Context(), 0x2000, 1, []byte{1, 2, 3}, exchange); err != nil {
			t.Fatal(err)
		}
	})
	for _, test := range []struct {
		name string
		data []byte
	}{
		{name: "segmented", data: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}},
		{name: "empty", data: []byte{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			responses := [][8]byte{{0x60, 0x00, 0x20, 0x01}}
			if len(test.data) > 0 {
				responses = append(responses, [8]byte{0x20}, [8]byte{0x30})
			} else {
				responses = append(responses, [8]byte{0x20})
			}
			exchange := scriptedExchange(t, responses)
			if err := MasterDownload(t.Context(), 0x2000, 1, test.data, exchange); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMasterTransferRejectsAbortToggleSizeAndContext(t *testing.T) {
	t.Parallel()
	abort := [8]byte{0x80, 0x00, 0x20, 0x01}
	binary.LittleEndian.PutUint32(abort[4:], 0x06010000)
	_, err := MasterUpload(t.Context(), 0x2000, 1, 8, scriptedExchange(t, [][8]byte{abort}))
	var abortErr *MasterAbortError
	if !errors.As(err, &abortErr) || abortErr.Code != 0x06010000 || abortErr.Index != 0x2000 || abortErr.SubIndex != 1 {
		t.Fatalf("abort error = %#v", err)
	}

	init := [8]byte{0x41, 0x00, 0x20, 0x01}
	binary.LittleEndian.PutUint32(init[4:], 9)
	if _, err := MasterUpload(t.Context(), 0x2000, 1, 8, scriptedExchange(t, [][8]byte{init})); !errors.Is(err, ErrMasterSizeLimit) {
		t.Fatalf("size error = %v", err)
	}
	if _, err := MasterUpload(t.Context(), 0x2000, 1, 16, scriptedExchange(t, [][8]byte{{0x40, 0x00, 0x20, 0x01}, {0x10}})); !errors.Is(err, ErrMasterProtocol) {
		t.Fatalf("toggle error = %v", err)
	}

	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	called := false
	_, err = MasterUpload(canceled, 0x2000, 1, 8, func(context.Context, [8]byte) ([8]byte, error) {
		called = true
		return [8]byte{}, nil
	})
	if !errors.Is(err, context.Canceled) || called {
		t.Fatalf("canceled upload error=%v called=%v", err, called)
	}
}

func scriptedExchange(t *testing.T, responses [][8]byte) MasterExchange {
	t.Helper()
	index := 0
	return func(_ context.Context, _ [8]byte) ([8]byte, error) {
		if index >= len(responses) {
			t.Fatalf("unexpected exchange %d", index)
		}
		response := responses[index]
		index++
		return response, nil
	}
}
