// Copyright (c) 2019, Benjamin Shields. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tftp implements the Trivial File Transfer Protocol as defined in RFC 1350.
package tftp

import (
	"errors"
	"fmt"
)

// bufferSize defines the size of buffer used to listen for TFTP read and write requests. This accommodates the
// standard Ethernet MTU blocksize (1500 bytes) minus headers of TFTP (4 bytes), UDP (8 bytes) and IP (20 bytes).
const bufferSize = 1468

const sizeOfOpCode = 2

// bufferSize defines the minimum size of a TFTP Read Request or Write Request packet. This accommodates the
// opCode (2 bytes) plus filename (2 bytes) plus mode (2 bytes). The filename and mode are at least 1 byte and
// are also terminated by a null byte.
const minRequestPacketSize = 6

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// opCode specifies one of the five types of packets supported by TFTP. OpCodes are two bytes with values from 1 to 5.
type opCode uint16

const (
	_            = iota
	RRQ   opCode = iota // Read request 	1
	WRQ                 // Write request 	2
	DATA                // Data				3
	ACK                 // Acknowledgment	4
	ERROR               // Error			5
)

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var (
	ErrServerClosed = errors.New("the server is closed")
)

type tftpError struct {
	errorCode uint16
	errorMsg  error // TODO should this just be a string?
}

func (e tftpError) fmt(format string, a ...interface{}) tftpError {
	fmtMsg := fmt.Sprintf(format, a...)
	fmtErr := errors.New(e.errorMsg.Error() + ": " + fmtMsg)
	formattedError := tftpError{
		errorCode: e.errorCode,
		errorMsg:  fmtErr,
	}
	return formattedError
}

func (e tftpError) Error() string {
	return fmt.Sprintf("TFTP error %v occurred: %v", e.errorCode, e.errorMsg)
}

var (
	errNotDef     = tftpError{0, errors.New("undefined")}
	errNoFile     = tftpError{1, errors.New("file not found")}
	errAccess     = tftpError{3, errors.New("access violation")}
	errMemory     = tftpError{4, errors.New("disk full or allocation exceeded")}
	errOperation  = tftpError{5, errors.New("illegal TFTP operation")}
	errTID        = tftpError{6, errors.New("unknown transfer ID")}
	errFileExists = tftpError{7, errors.New("file already exists")}
	errNoUser     = tftpError{8, errors.New("no such user")}
)
