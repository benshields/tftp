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
		setupErr := handlerObject.setup(ctx)
		if setupErr != nil {
			handlerObject.sendErrorAndClose(*setupErr)
			done <- setupErr
			return
		}

		go handlerObject.Handle(handlerObject.lastPacket)

		for {
			ctxTimeout, _ := context.WithTimeout(ctx, 5*time.Second)
			in := handlerObject.packetReader.Read(ctxTimeout)
			select {
			case packet := <-in:
				log.Printf("tftp: new packet received:\n\tfrom: %v\n\tdata: %v\n", packet.from, packet.data) // TODO delete
				handlerObject.lastPacket = packet
				go handlerObject.Handle(packet)
			case <-ctx.Done(): // THE SERVER IS CLOSING
				tftpErr := errNotDef.fmt("server is shutting down")
				handlerObject.sendErrorAndClose(tftpErr)
				connectionErr := fmt.Errorf("connection's context closed with: %v", ctx.Err())
				done <- connectionErr
				return
			case <-ctxTimeout.Done(): // THE CONNECTION IS TERMINATED (TODO SHOULD BE DALLYING?)
				tftpErr := errNotDef.fmt("connection timeout")
				handlerObject.sendErrorAndClose(tftpErr)
				done <- ctx.Err()
				return
			}
		}
	}()
	return done
}

func (handlerObject *HandlerObject) setup(ctx context.Context) *tftpError { // setup() is an instance of Sequential coupling...
	handlerObject.setupLogger(ctx)

	err := handlerObject.setupPacketReader()
	if err != nil {
		return err
	}

	err = handlerObject.setupPacketHandler()
	if err != nil {
		return err
	}

	return nil
}

func (handlerObject *HandlerObject) setupLogger(ctx context.Context) {
	logger, ok := ctx.Value(LoggerContextKey).(*log.Logger)
	if ok {
		handlerObject.ErrorLog = logger
	}
}

func (handlerObject *HandlerObject) setupPacketReader() *tftpError {
	conn, err := NewConn(":0") // :0 tells the OS to assign an ephemeral port
	if err != nil {
		internalServerError := errNotDef.fmt("failed to assign TID for connection")
		return &internalServerError
	}
	handlerObject.packetReader = conn
	return nil
}

func (handlerObject *HandlerObject) setupPacketHandler() *tftpError {
	req, err := parseRequestPacket(handlerObject.lastPacket)
	if err != nil {
		msg := "error occurred while reading opcode in Request packet from %v - %v"
		badRequestError := errNotDef.fmt(msg, handlerObject.lastPacket.from, err)
		return &badRequestError
	}

	handler, openFileError := newPacketHandler(req)
	if openFileError != nil {
		return openFileError
	}
	handlerObject.ResponseWriter = handler
	return nil
}

func (handlerObject *HandlerObject) Handle(packet Packet) {
	response := handlerObject.ResponseWriter.WriteResponse(packet)
	err := handlerObject.sendPacket(response)
	if err != nil {
		handlerObject.logf("tftp: failed to send:\n\tresponse: %v\n\tdue to error: %v", response, err)
		handlerObject.sendDefaultErrorAndClose()
	}
}

func (handlerObject *HandlerObject) sendDefaultErrorAndClose() {
	handlerObject.sendErrorAndClose(internalErrorPacket().tftpError)
}

func (handlerObject *HandlerObject) sendErrorAndClose(tftpErr tftpError) {
	rawErrorData := handlerObject.getRawErrorData(tftpErr)
	err := handlerObject.sendPacket(rawErrorData)
	if err != nil {
		handlerObject.logf("tftp: error sending error packet to client - %v", err)
	}

	err = handlerObject.close()
	if err != nil {
		handlerObject.logf("tftp: error closing client handler - %v", err)
	}
}

func (handlerObject *HandlerObject) getRawErrorData(tftpErr tftpError) []byte {
	pak, err := createErrorPacket(tftpErr)
	if err != nil {
		handlerObject.logf("tftp: error creating error packet - %v", err)
		return internalErrorPacket().raw
	} else {
		return pak.raw
	}
}

func (handlerObject *HandlerObject) close() error {
	packetReaderErr := handlerObject.packetReader.rwc.Close()
	var responseWriterErr error
	if handlerObject.ResponseWriter != nil {
		responseWriterErr = handlerObject.ResponseWriter.Close()
	}

	if packetReaderErr != nil {
		return packetReaderErr
	}
	return responseWriterErr
}

func (handlerObject *HandlerObject) sendPacket(pak []byte) error {
	_, err := handlerObject.packetReader.rwc.WriteTo(pak, handlerObject.remoteAddr)
	if err != nil {
		return err
	}
	log.Printf("sendPacket():\n\tto:   %v\n\tdata: %v\n", handlerObject.remoteAddr, pak) // TODO delete
	return nil
}

func (handlerObject *HandlerObject) logf(format string, args ...interface{}) {
	if handlerObject.ErrorLog != nil {
		handlerObject.ErrorLog.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}
