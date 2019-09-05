// Copyright (c) 2019, Benjamin Shields. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tftp implements the Trivial File Transfer Protocol as defined in RFC 1350.
package tftp

import (
	"context"
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
		return err // TODO make some error packet to return, internal server error, and log
	}
	err = handler.Client.setupFileHandler()
	if err != nil {
		return err // TODO make some error packet to return, incorrectly formed packet or fail to open file?, and log
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
		_ = handler.sendPacket(backupError().data) // TODO unhandled error
		// JUST BECAUSE I WANT TO TEST OUT SENDING ONE ERROR REPLY AND BREAKING THE CONNECTION
		done <- err
		// TODO call some handler cleanup func? Then dally. Any cleanup I'm forgetting? logging?
		return
		// JUST BECAUSE I WANT TO TEST OUT SENDING ONE ERROR REPLY AND BREAKING THE CONNECTION
		/*
			if err != nil {
				// TODO send error packet
				_ = handler.sendPacket(backupError().data) // TODO unhandled error
				done <- err
				// TODO call some handler cleanup func? Then dally. Any cleanup I'm forgetting? logging?
				return
			}
			// TODO send first response
			for {
				ctxTimeout, _ := context.WithTimeout(ctx, 5*time.Second)
				in := handler.packetReader.Read(ctxTimeout)
				select {
				case packet := <-in:
					go handler.Handle(packet) // TODO this is a dummy func
				case <-ctx.Done():
					// TODO should I close packetReader here? Send an error packet to client?
					// TODO call some handler cleanup func? Then dally. Any cleanup I'm forgetting? logging?
					//// this needs to be deferred
					//err = clientReader.rwc.Close()
					//if err != nil {
					//	done <- err
					//}
					return
				}
			}*/
	}()
	return done
}

func (handler *Handler) Handle(request Packet) {
	/* TODO This is a dummy function */
}

// TODO this is a horrible placeholder
func (handler *Handler) sendPacket(pak []byte) error {
	_, err := handler.packetReader.rwc.WriteTo(pak, handler.Client.remoteAddr)
	return err
}
