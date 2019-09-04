// Copyright (c) 2019, Benjamin Shields. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tftp implements the Trivial File Transfer Protocol as defined in RFC 1350.
package tftp

import (
	"bytes"
	"encoding/binary"
	"net"
)

type Packet struct {
	from net.Addr
	data []byte
	error
}

func (packet Packet) readOpCode() (opCode, error) {
	var op opCode
	bytesReader := bytes.NewReader(packet.data)

	// TODO ERROR send an ERROR packet (0): couldn't find opCode
	if err := binary.Read(bytesReader, binary.BigEndian, &op); err != nil {
		return op, err
	}

	return op, nil
}
