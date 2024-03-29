// Copyright (c) 2019, Benjamin Shields. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tftp implements the Trivial File Transfer Protocol as defined in RFC 1350.
package tftp

import (
	"context"
	"net"
)

type Conn struct {
	// rwc is the underlying network connection.
	// This is never wrapped by other types and is the value given out
	// to CloseNotifier callers.
	rwc net.PacketConn

	buffer []byte // buffer holds the raw last read from rwc

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

func (c *Conn) Read(ctx context.Context) <-chan Packet {
	out := make(chan Packet)
	go func() {
		n, addr, err := c.rwc.ReadFrom(c.buffer)
		data := make([]byte, n)
		copy(data, c.buffer[:n])
		if err != nil {
			select {
			case <-ctx.Done():
				out <- Packet{addr, data, ctx.Err()}
			default:
				out <- Packet{addr, data, err}
			}

		}
		out <- Packet{addr, data, nil}
	}()
	return out
}

func (c *Conn) ReadContinuously(ctx context.Context) <-chan Packet {
	out := make(chan Packet)
	go func() {
		for {
			in := c.Read(ctx)
			select {
			case request := <-in:
				if request.error == context.Canceled {
					return
				}
				out <- request
			case <-ctx.Done():
				err := c.rwc.Close()
				if err != nil {
					out <- Packet{nil, nil, err} // TODO refactor to handle this error gracefully!
				}
			}
		}
	}()
	return out
}
