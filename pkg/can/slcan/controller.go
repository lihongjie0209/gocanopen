package slcan

import (
	"context"
	"errors"
	"fmt"

	canopen "github.com/samsamfire/gocanopen/v2"
)

var ErrUnsupportedBitRate = errors.New("unsupported SLCAN bit rate")

type WriteFunc func(context.Context, []byte) error

type FrameHandler func(context.Context, canopen.Frame) error

// Controller owns Lawicel channel commands and frame encoding/dispatch while
// leaving serial-port resource ownership to its caller.
type Controller struct {
	bitRateCode byte
	write       WriteFunc
	handle      FrameHandler
}

func NewController(bitRate int, write WriteFunc, handle FrameHandler) (*Controller, error) {
	code, ok := BitRateCode(bitRate)
	if !ok {
		return nil, fmt.Errorf("%w: %d", ErrUnsupportedBitRate, bitRate)
	}
	if write == nil {
		return nil, errors.New("SLCAN writer is required")
	}
	if handle == nil {
		return nil, errors.New("SLCAN frame handler is required")
	}
	return &Controller{bitRateCode: code, write: write, handle: handle}, nil
}

func BitRateCode(bitRate int) (byte, bool) {
	code, ok := map[int]byte{
		10000: '0', 20000: '1', 50000: '2', 100000: '3', 125000: '4',
		250000: '5', 500000: '6', 800000: '7', 1000000: '8',
	}[bitRate]
	return code, ok
}

func (c *Controller) Open(ctx context.Context) error {
	commands := [][]byte{{'C', '\r'}, {'S', c.bitRateCode, '\r'}, {'O', '\r'}}
	for index, command := range commands {
		if err := c.write(ctx, command); err != nil {
			return fmt.Errorf("writing SLCAN open command %d: %w", index+1, err)
		}
	}
	return nil
}

func (c *Controller) Send(ctx context.Context, frame canopen.Frame) error {
	encoded, err := Encode(frame)
	if err != nil {
		return err
	}
	if err := c.write(ctx, encoded); err != nil {
		return fmt.Errorf("writing SLCAN frame: %w", err)
	}
	return nil
}

func (c *Controller) Receive(ctx context.Context, wire []byte) error {
	frame, err := Decode(wire)
	if err != nil {
		return err
	}
	if err := c.handle(ctx, frame); err != nil {
		return fmt.Errorf("handling SLCAN frame: %w", err)
	}
	return nil
}

func (c *Controller) Close(ctx context.Context) error {
	if err := c.write(ctx, []byte{'C', '\r'}); err != nil {
		return fmt.Errorf("closing SLCAN channel: %w", err)
	}
	return nil
}
