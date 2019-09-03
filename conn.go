package tftp

import (
	"context"
	"net"
)

// bufferSize defines the size of buffer used to listen for TFTP read and write requests. This accommodates the
// standard Ethernet MTU blocksize (1500 bytes) minus headers of TFTP (4 bytes), UDP (8 bytes) and IP (20 bytes).
const bufferSize = 1468

type Conn struct {
	// rwc is the underlying network connection.
	// This is never wrapped by other types and is the value given out
	// to CloseNotifier callers.
	rwc net.PacketConn

	buffer []byte // buffer holds the data last read from rwc

	localAddr net.Addr // address from which the handler is serving the connection
}

func NewConn(addr string) (*Conn, error) {
	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, err
	}
	c := &Conn{
		rwc:       pc,
		buffer:    make([]byte, bufferSize),
		localAddr: pc.LocalAddr(),
	}
	return c, nil
}

func (c *Conn) Read(ctx context.Context) <-chan Request {
	out := make(chan Request)
	go func() {
		n, addr, err := c.rwc.ReadFrom(c.buffer) // this call blocks
		data := make([]byte, n)
		copy(data, c.buffer[:n])
		if err != nil {
			select {
			case <-ctx.Done():
				out <- Request{addr, data, ctx.Err()} // read cancelled
			default:
				out <- Request{addr, data, err} // read failure
			}

		}
		out <- Request{addr, data, nil} // read success
	}()
	return out
}

func (c *Conn) ReadContinuously(ctx context.Context) <-chan Request {
	out := make(chan Request)
	go func() {
		for {
			select {
			case request := <-c.Read(ctx):
				if request.error == context.Canceled {
					return
				}
				out <- request
			case <-ctx.Done():
				err := c.rwc.Close()
				if err != nil {
					out <- Request{nil, nil, err} // refactor to handle this error gracefully!
				}
			default:
			}
		}
	}()
	return out
}
