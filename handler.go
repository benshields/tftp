// Copyright (c) 2019, Benjamin Shields. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tftp implements the Trivial File Transfer Protocol as defined in RFC 1350.
package tftp

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

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
		return err
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

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type RequestHandler interface {
	Start(ctx context.Context) <-chan error
	ResponseWriter
}

type ResponseWriter interface {
	WriteResponse(pak Packet) (response []byte)
	Close() error
}

func newPacketHandler(req *RequestPacket) (ResponseWriter, error) {
	fileHandler := newBlockStreamer(req.filename, req.openFlag, req.encodingFlag)
	err := fileHandler.Open()
	if err != nil {
		return nil, err
	}

	var handler ResponseWriter
	switch req.openFlag {
	case read:
		handler = newRrqResponseWriter(fileHandler)
	case write:
		handler = newWrqResponseWriter(fileHandler)
	default:
		panic(req.openFlag)
	}
	return handler, nil
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type RrqResponseWriter struct {
	// handler interfaces with the file that the client is reading from or writing to.
	fileHandler
}

func newRrqResponseWriter(fh fileHandler) *RrqResponseWriter {
	rrqResponseWriter := &RrqResponseWriter{
		fileHandler: fh,
	}
	return rrqResponseWriter
}

func (rrqResponseWriter *RrqResponseWriter) WriteResponse(pak Packet) (response []byte) {
	data := make([]byte, 512)
	n, err := rrqResponseWriter.fileHandler.Read(data) // TODO I can't just call Read, it needs to be based on the correct block number
	data = data[:n]
	if err != nil && err != io.EOF {
		return backupError().raw
	}

	blockNumber, err := rrqResponseWriter.nextBlockNumber(pak)
	if err != nil { // TODO the error I received is actually useful, it should be returned
		return backupError().raw
	}

	dataPacket := createDataPacket(blockNumber, data)

	raw, err := dataPacket.bytes()
	if err != nil {
		return backupError().raw
	}
	return raw
}

func (rrqResponseWriter *RrqResponseWriter) Close() error {
	return rrqResponseWriter.fileHandler.Close()
}

func (rrqResponseWriter *RrqResponseWriter) nextBlockNumber(pak Packet) (uint16, error) {
	var blockNumber uint16
	op, err := pak.readOpCode()
	if err != nil {
		return 0, err
	}

	switch op {
	case RRQ:
		blockNumber = 1
	case ACK:
		currentBlockNumber, err := pak.readBlockNumber()
		blockNumber = currentBlockNumber + 1
		if err != nil {
			return 0, err
		}
	default:
		unexpectedPacketTypeErr := errOperation.fmt("expected packet of type RRQ (Read Request) or ACK (Acknowledgement), found %v", op)
		return 0, unexpectedPacketTypeErr
	}

	return blockNumber, nil
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type WrqResponseWriter struct {
	// handler interfaces with the file that the client is reading from or writing to.
	fileHandler
}

func newWrqResponseWriter(fh fileHandler) *WrqResponseWriter {
	wrqResponseWriter := &WrqResponseWriter{
		fileHandler: fh,
	}
	return wrqResponseWriter
}

func (wrqResponseWriter *WrqResponseWriter) WriteResponse(pak Packet) (response []byte) {
	blockNumber, err := wrqResponseWriter.nextBlockNumber(pak)
	if err != nil {
		return backupError().raw
	}

	if blockNumber != 0 {
		// TODO: so right here, if the packet data is 0-511 bytes, I need to dally (keep sending final ACK in response to final DATA)
		// TODO: In general, I just want to make sure I don't write the same data twice. So I need some block number error checking.
		n, err := wrqResponseWriter.fileHandler.Write(pak.data) // TODO I can't just call Write, it needs to be based on the correct block number
		if err != nil || n != len(pak.data) {                   // TODO do I really have to be measuring len here? It's probably included in the error
			return backupError().raw
		}
	}

	ackPacket := createAckPacket(blockNumber)

	raw, err := ackPacket.bytes()
	if err != nil {
		return backupError().raw
	}
	return raw
}

func (wrqResponseWriter *WrqResponseWriter) Close() error {
	return wrqResponseWriter.fileHandler.Close()
}

func (wrqResponseWriter *WrqResponseWriter) nextBlockNumber(pak Packet) (uint16, error) {
	var blockNumber uint16
	op, err := pak.readOpCode()

	switch op {
	case WRQ:
		blockNumber = 0
	case DATA:
		blockNumber, err = pak.readBlockNumber()
		if err != nil {
			return 0, err
		}
	default:
		unexpectedPacketTypeErr := errOperation.fmt("expected packet of type WRQ (Write Request) or DATA (Data), found %v", op)
		return 0, unexpectedPacketTypeErr
	}

	return blockNumber, nil
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
