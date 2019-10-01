// Copyright (c) 2019, Benjamin Shields. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tftp implements the Trivial File Transfer Protocol as defined in RFC 1350.
package tftp

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

type RequestHandler interface {
	Start(ctx context.Context) <-chan error
	ResponseWriter
}

func HandleRequest(ctx context.Context, request Packet, done chan<- error) {
	handler := NewHandlerObject(request)
	handlerFinished := handler.Start(ctx)
	select {
	case err := <-handlerFinished:
		done <- err
	case <-ctx.Done():
		done <- ctx.Err()
	}
}

type HandlerObject struct {
	ResponseWriter

	// packetReader listens for new packets from a client.
	packetReader *Conn

	// lastPacket is the last TFTP packet received from the client, used in case of re-transmission.
	lastPacket Packet

	// remoteAddr is the address at which this client can be reached.
	remoteAddr net.Addr

	// ErrorLog specifies an optional logger for errors setting
	// connections, unexpected behavior from handlers, and
	// underlying FileSystem errors.
	// If nil, logging is done via the log package's standard logger.
	ErrorLog *log.Logger // Go 1.3
}

func NewHandlerObject(request Packet) *HandlerObject {
	handlerObject := &HandlerObject{
		lastPacket: request,
		remoteAddr: request.from,
	}
	return handlerObject
}

func (handlerObject *HandlerObject) Start(ctx context.Context) <-chan error {
	done := make(chan error)
	go func() {
		err := handlerObject.setup(ctx)
		if err != nil {
			handlerObject.sendDefaultErrorPacket()
			done <- err
			return
		}

		go handlerObject.Handle(handlerObject.lastPacket)

		for {
			ctxTimeout, _ := context.WithTimeout(ctx, 5*time.Second)
			in := handlerObject.packetReader.Read(ctxTimeout)
			select {
			case packet := <-in:
				log.Printf("tftp: new packet received:\n\tfrom: %v\n\tdata: %v\n", packet.from, packet.data)
				handlerObject.lastPacket = packet
				go handlerObject.Handle(packet)
			case <-ctx.Done(): // THE SERVER IS CLOSING
				handlerObject.sendDefaultErrorPacket()
				done <- ctx.Err()
				return
			case <-ctxTimeout.Done(): // THE CONNECTION IS TERMINATED (SHOULD BE DALLYING)
				handlerObject.sendDefaultErrorPacket()
				done <- ctx.Err()
				return
			}
		}
	}()
	return done
}

func (handlerObject *HandlerObject) setup(ctx context.Context) error { // setup() is an instance of Sequential coupling...
	handlerObject.setupLogger(ctx)

	err := handlerObject.setupPacketReader()
	if err != nil {
		return err // TODO make some error packet to return, internal server error, and log
	}

	err = handlerObject.setupPacketHandler()
	if err != nil {
		return err // TODO make some error packet to return, incorrectly formed packet or fail to open file?, and log
	}

	return nil
}

func (handlerObject *HandlerObject) setupLogger(ctx context.Context) {
	logger, ok := ctx.Value(LoggerContextKey).(*log.Logger)
	if ok {
		handlerObject.ErrorLog = logger
	}
}

func (handlerObject *HandlerObject) setupPacketReader() error {
	//conn, err := NewConn("127.0.0.1:0") // :0 tells the OS to assign an ephemeral port, this doesn't seem to like a mix of IPv4 & IPv6
	conn, err := NewConn(":0") // TODO this is a test, but it seems to be working!
	if err != nil {
		return fmt.Errorf("func (handlerObject *HandlerObject) setupPacketReader() error:: %v", err)
	}
	handlerObject.packetReader = conn
	return nil
}

func (handlerObject *HandlerObject) setupPacketHandler() error {
	req, err := parseRequestPacket(handlerObject.lastPacket)
	if err != nil {
		badRequestError := fmt.Errorf("%v: error occurred while reading opcode in Request packet from %v - %v",
			errNotDef.errorMsg.Error(), handlerObject.lastPacket.from, err)
		return badRequestError
	}

	handler, err := newPacketHandler(req)
	if err != nil {
		if os.IsExist(err) {
			return errFileExists
		} else if os.IsNotExist(err) {
			return errNoFile
		} else {
			fileError := fmt.Errorf("%v: error occurred while opening file in Request packet from %v - %v",
				errNotDef.errorMsg.Error(), handlerObject.lastPacket.from, err)
			return fileError // TODO FINDME This is where I'm leaving off, working on correct errors for opening files
		}
	}
	handlerObject.ResponseWriter = handler
	return nil
}

func (handlerObject *HandlerObject) Handle(packet Packet) {
	response := handlerObject.ResponseWriter.WriteResponse(packet)
	err := handlerObject.sendPacket(response)
	if err != nil {
		handlerObject.sendDefaultErrorPacket()
		handlerObject.logf("tftp: failed to send:\n\tresponse: %v\n\tdue to error: %v", response, err)
	}
}

func (handlerObject *HandlerObject) sendDefaultErrorPacket() {
	// TODO should I close packetReader here? Send an error packet to client?
	// TODO call some handlerObject cleanup func? Then dally. Any cleanup I'm forgetting? logging?
	// TODO send error packet
	_ = handlerObject.sendPacket(backupError().raw) // TODO unhandled error
	_ = handlerObject.packetReader.rwc.Close()
	_ = handlerObject.ResponseWriter.Close()
	// TODO call some handlerObject cleanup func? Then dally. Any cleanup I'm forgetting? logging?
}

// TODO this is a horrible placeholder
func (handlerObject *HandlerObject) sendPacket(pak []byte) error {
	_, err := handlerObject.packetReader.rwc.WriteTo(pak, handlerObject.remoteAddr)
	if err != nil {
		return fmt.Errorf("func (handlerObject *HandlerObject) sendPacket(pak []byte) error:: %v", err)
	}
	log.Printf("sendPacket():\n\tto:   %v\n\tdata: %v\n", handlerObject.remoteAddr, pak)
	return nil
}

func (handlerObject *HandlerObject) logf(format string, args ...interface{}) {
	if handlerObject.ErrorLog != nil {
		handlerObject.ErrorLog.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}
