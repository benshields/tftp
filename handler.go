package tftp

import (
	"context"
	"time"
)

type Handler struct {
	// requestReader listens for new packets from a client.
	packetReader *Conn
}

func HandleRequest(ctx context.Context, request Packet, done chan<- error) {
	handler := NewHandler(request)
	handlerFinished := handler.Start(ctx)
	select {
	case err := <-handlerFinished:
		done <- err
	case <-ctx.Done():
		done <- ctx.Err()
	}
}

func NewHandler(request Packet) *Handler {
	handler := &Handler{}
	return handler
}

func (handler *Handler) setup() error {
	err := handler.setupPacketReader()
	if err != nil {
		return err
	}

	return nil
}

func (handler *Handler) setupPacketReader() error {
	conn, err := NewConn("127.0.0.1:0") // :0 tells the OS to assign an ephemeral port
	if err != nil {
		return err
	}
	handler.packetReader = conn
	return nil
}

func (handler *Handler) Start(ctx context.Context) <-chan error {
	out := make(chan error)
	go func() {
		err := handler.setup()
		if err != nil {
			out <- err
			// TODO call some handler cleanup func?
			return
		}
		for {
			ctxTimeout, _ := context.WithTimeout(ctx, 5*time.Second)
			in := handler.packetReader.Read(ctxTimeout)
			select {
			case packet := <-in:
				go handler.Handle(packet)
			case <-ctx.Done():
				// TODO should I close packetReader here? Send an error packet to client?
				// TODO call some handler cleanup func?
				/*// this needs to be deferred
				err = clientReader.rwc.Close()
				if err != nil {
					out <- err
				}*/
				return
			}
		}
	}()
	return out
}

func (handler *Handler) Handle(request Packet) {
	/* TODO This is a dummy function */
}
