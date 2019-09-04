// Copyright (c) 2019, Benjamin Shields. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tftp implements the Trivial File Transfer Protocol as defined in RFC 1350.
package tftp

import (
	"context"
	"time"
)

type Handler struct {
	// packetReader listens for new packets from a client.
	packetReader *Conn

	// Client is a pointer to the client being served by this handler.
	*Client
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
	c := NewClient(request)
	handler := &Handler{Client: c}
	return handler
}

func (handler *Handler) setup() error { // setup() is an instance of Sequential coupling...
	err := handler.setupPacketReader()
	if err != nil {
		return err
	}
	err = handler.Client.setup()
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
	done := make(chan error)
	go func() {
		err := handler.setup()
		if err != nil {
			done <- err
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
					done <- err
				}*/
				return
			}
		}
	}()
	return done
}

func (handler *Handler) Handle(request Packet) {
	/* TODO This is a dummy function */
}
