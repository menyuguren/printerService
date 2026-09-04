package spooler

import "context"

type Windows struct{}

func (Windows) Submit(ctx context.Context, printer string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return submitRaw(printer, data)
}
