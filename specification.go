// Copyright (c) 2019, Benjamin Shields. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tftp implements the Trivial File Transfer Protocol as defined in RFC 1350.
package tftp

import (
	"errors"
	"fmt"
)

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
	errorMsg  error
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
