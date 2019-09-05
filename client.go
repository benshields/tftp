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
	fileHandler
}

func NewClient(request Packet) *Client {
	c := &Client{lastPacket: request,
		remoteAddr: request.from}
	return c
}

func (client *Client) setupFileHandler() error { // setupFileHandler() is an instance of Sequential coupling...
	req, err := parseRequestPacket(client.lastPacket)
	if err != nil {
		badRequestError := fmt.Errorf("%v: error occurred while reading opcode in Request packet from %v - %v",
			errNotDef.errorMsg.Error(), client.lastPacket.from, err)
		return badRequestError
	}
	client.fileHandler = newBlockStreamer(req.filename, req.openFlag, req.encodingFlag)
	return client.fileHandler.Open()
}
