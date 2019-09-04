// Copyright (c) 2019, Benjamin Shields. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tftp implements the Trivial File Transfer Protocol as defined in RFC 1350.
package tftp

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
