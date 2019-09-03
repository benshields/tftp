package tftp

import "context"

func HandleRequest(ctx context.Context, request Packet, done chan<- error) {
	clientReader, err := NewConn("127.0.0.1:0") // :0 tells the OS to assign an ephemeral port
	if err != nil {
		done <- err
		return
	}
	clientPackets := clientReader.ReadContinuously(ctx)
	clientDone := make(chan error)
	clientHandler := NewHandler(ctx, request, clientDone)
	for {
		select {
		case packet := <-clientPackets:
			clientHandler <- packet
		case err := <-clientDone:
			done <- err
			return
		case <-ctx.Done():
			done <- ctx.Err()
			return
		}
	}
}

// TODO: I think this should become its own type and file
func NewHandler(ctx context.Context, request Packet, out chan<- error) chan<- Packet {
	in := make(chan Packet)
	go func() {

		for {
			select {
			case packet := <-in:
				go handle(packet)
			case <-ctx.Done():
				return
			}
		}
	}()
	return in
}

func handle(request Packet) {
	/* TODO This is a dummy function */
}
