// Copyright (c) 2019, Benjamin Shields. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tftp implements the Trivial File Transfer Protocol as defined in RFC 1350.
package tftp

import (
	"context"
	"fmt"
	"log"
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
		return err // TODO make some error packet to return, internal server error, and log
	}
	err = handler.Client.setupFileHandler()
	if err != nil {
		return err // TODO make some error packet to return, incorrectly formed packet or fail to open file?, and log
	}
	return nil
}

func (handler *Handler) setupPacketReader() error {
	//conn, err := NewConn("127.0.0.1:0") // :0 tells the OS to assign an ephemeral port
	conn, err := NewConn(":0") // TODO this is a test
	if err != nil {
		return fmt.Errorf("func (handler *Handler) setupPacketReader() error:: %v", err)
	}
	handler.packetReader = conn
	return nil
}

func (handler *Handler) Start(ctx context.Context) <-chan error {
	done := make(chan error)
	go func() {
		err := handler.setup()
		if err != nil {
			// TODO send error packet
			_ = handler.sendPacket(backupError().data) // TODO unhandled error
			_ = handler.packetReader.rwc.Close()
			_ = handler.Client.fileHandler.Close()
			done <- err
			// TODO call some handler cleanup func? Then dally. Any cleanup I'm forgetting? logging?
			return
		}
		// TODO send first response
		go handler.Handle(handler.lastPacket)

		for {
			ctxTimeout, _ := context.WithTimeout(ctx, 5*time.Second)
			in := handler.packetReader.Read(ctxTimeout)
			select {
			case packet := <-in:
				go handler.Handle(packet) // TODO this is a dummy func
			case <-ctx.Done(): // THE SERVER IS CLOSING
				// TODO should I close packetReader here? Send an error packet to client?
				// TODO call some handler cleanup func? Then dally. Any cleanup I'm forgetting? logging?
				// TODO send error packet
				_ = handler.sendPacket(backupError().data) // TODO unhandled error
				_ = handler.packetReader.rwc.Close()
				_ = handler.Client.fileHandler.Close()
				done <- err
				// TODO call some handler cleanup func? Then dally. Any cleanup I'm forgetting? logging?
				return
			case <-ctxTimeout.Done(): // THE CONNECTION IS TERMINATED (SHOULD BE DALLYING)
				// TODO should I close packetReader here? Send an error packet to client?
				// TODO call some handler cleanup func? Then dally. Any cleanup I'm forgetting? logging?
				// TODO send error packet
				_ = handler.sendPacket(backupError().data) // TODO unhandled error
				_ = handler.packetReader.rwc.Close()
				_ = handler.Client.fileHandler.Close()
				done <- err
				// TODO call some handler cleanup func? Then dally. Any cleanup I'm forgetting? logging?
				return
			}
		}
	}()
	return done
}

func (handler *Handler) Handle(request Packet) {
	/* TODO This is a dummy function */
	data := make([]byte, 512)
	n, err := handler.Client.fileHandler.Read(data)
	if err != nil {
		_ = handler.sendPacket(backupError().data) // TODO unhandled error
	}
	// FIXME this is where I'm leaving off for today
	// Implement the happy path of creating a data packet to shoot back to the client.
	n++ // this is just to get rid of "n is unused" error
}

// TODO this is a horrible placeholder
func (handler *Handler) sendPacket(pak []byte) error {
	_, err := handler.packetReader.rwc.WriteTo(pak, handler.Client.remoteAddr)
	if err != nil {
		return fmt.Errorf("func (handler *Handler) sendPacket(pak []byte) error:: %v", err)
	}
	log.Printf("sendPacket():\n\tto:   %v\n\tdata: %v\n", handler.Client.remoteAddr, pak)
	return nil
}
