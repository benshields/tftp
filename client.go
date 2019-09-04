// Copyright (c) 2019, Benjamin Shields. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tftp implements the Trivial File Transfer Protocol as defined in RFC 1350.
package tftp

import (
	"fmt"
	"net"
)

// client defines parameters for maintaining a TFTP client connection.
type Client struct {
	// lastPacket is the last TFTP packet received from the client, used in case of re-transmission.
	lastPacket Packet

	// remoteAddr is the address at which this client can be reached.
	remoteAddr net.Addr

	// handler interfaces with the file that the client is reading from or writing to.
	fileHandler readWriteCloser
}

func NewClient(request Packet) *Client {
	c := &Client{lastPacket: request,
		remoteAddr: request.from}
	return c
}

func (client *Client) setup() error { // setup() is an instance of Sequential coupling...
	// TODO: setup the fileHandler

	// TODO: figure out if it's a RRQ or WRQ
	requestType, err := client.lastPacket.readOpCode()
	if err != nil {
		return err
	}
	switch requestType {
	case RRQ:
		// setup a new filehandler
	case WRQ:
		// setup a new filehandler
	default:
		// TODO send an error packet back
		badRequestError := fmt.Errorf("%v: expected opcode matching RRQ(%v) or WRQ(%v), found %v",
			errOperation.errorMsg.Error(), RRQ, WRQ, requestType)
		return badRequestError
	}

	return nil
}
