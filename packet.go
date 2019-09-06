// Copyright (c) 2019, Benjamin Shields. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tftp implements the Trivial File Transfer Protocol as defined in RFC 1350.
package tftp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
)

type Packet struct {
	from net.Addr
	data []byte
	error
}

/*func (packet Packet) readOpCode() (opCode, error) {
	var op opCode
	bytesReader := bytes.NewReader(packet.data)
	if err := binary.Read(bytesReader, binary.BigEndian, &op); err != nil {
		return op, err
	}
	return op, nil
}*/

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// RequestPacket is generated from a RRQ/WRQ packet as defined in RFC 1350.
type RequestPacket struct {
	openFlag            // generated from the opCode
	filename     string // null-terminator is removed
	encodingFlag        // generated from the mode value
}

// parseRequestPacket parses the packet's data into the fields of
// the returned RequestPacket. If the packet is not correctly
// formed, an error is returned explaining why.
func parseRequestPacket(packet Packet) (*RequestPacket, error) {
	openFlag, err := packet.readOpenFlag()
	if err != nil {
		return nil, err
	}
	filename, err := packet.readFilename()
	if err != nil {
		return nil, err
	}
	encodingFlag, err := packet.readEncodingFlag()
	if err != nil {
		return nil, err
	}
	request := &RequestPacket{
		openFlag:     openFlag,
		filename:     filename,
		encodingFlag: encodingFlag,
	}
	return request, nil
}

func (packet Packet) readOpenFlag() (openFlag, error) {
	var flag openFlag
	op, err := packet.readOpCode()
	if err != nil {
		return flag, err
	}
	return opCodeToOpenFlag(op)
}

func opCodeToOpenFlag(op opCode) (openFlag, error) {
	switch op {
	case RRQ:
		return read, nil
	case WRQ:
		return write, nil
	default:
		var flag openFlag
		err := fmt.Errorf("expected opcode matching RRQ(%v) or WRQ(%v), found %v", RRQ, WRQ, op)
		return flag, err
	}
}

func (packet Packet) readFilename() (string, error) {
	if len(packet.data) < minRequestPacketSize {
		return "", errors.New("incorrectly formed request packet")
	}
	buffer := bytes.NewBuffer(packet.data)
	buffer.Next(sizeOfOpCode)
	return readNetasciiString(buffer)
}

func (packet Packet) readEncodingFlag() (encodingFlag, error) {
	var flag encodingFlag
	if len(packet.data) < minRequestPacketSize {
		return flag, errors.New("incorrectly formed request packet")
	}
	buffer := bytes.NewBuffer(packet.data)
	buffer.Next(sizeOfOpCode)
	_, err := readNetasciiString(buffer) // the first string is the filename
	if err != nil {
		return flag, err
	}
	mode, err := readNetasciiString(buffer)
	if err != nil {
		return flag, err
	}
	return modeToEncodingFlag(mode)
}

func modeToEncodingFlag(mode string) (encodingFlag, error) {
	mode = strings.ToLower(mode)
	switch mode {
	case "netascii":
		return netascii, nil
	case "octet":
		return octet, nil
	default:
		var flag encodingFlag
		err := fmt.Errorf("expected mode matching netascii or octet, found %v", mode)
		return flag, err
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// DataPacket is generated from a DATA packet as defined in RFC 1350.
type DataPacket struct {
	blockNumber uint16
	data        []byte
}

// parseDataPacket parses the packet into the fields of
// the returned DataPacket. If the packet is not correctly
// formed, an error is returned explaining why.
func parseDataPacket(packet Packet) (*DataPacket, error) {
	err := packet.readDataOpCode()
	if err != nil {
		return nil, err
	}
	blockNumber, err := packet.readBlockNumber()
	if err != nil {
		return nil, err
	}
	data, err := packet.readData()
	if err != nil {
		return nil, err
	}
	dataPacket := &DataPacket{
		blockNumber: blockNumber,
		data:        data,
	}
	return dataPacket, nil
}

func (packet Packet) readDataOpCode() error {
	op, err := packet.readOpCode()
	if err != nil {
		return err
	}
	if op != DATA {
		return errOperation
	}
	return nil
}

func (packet Packet) readBlockNumber() (uint16, error) {
	bytesReader := bytes.NewReader(packet.data)
	var blockNumber uint16
	offset, err := binarySize(DATA)
	if err != nil {
		return blockNumber, err
	}
	if _, err := bytesReader.Seek(int64(offset), io.SeekStart); err != nil {
		return blockNumber, err
	}
	if err := binary.Read(bytesReader, binary.BigEndian, &blockNumber); err != nil {
		return blockNumber, err
	}
	return blockNumber, nil
}

func (packet Packet) readData() ([]byte, error) {
	bytesReader := bytes.NewReader(packet.data)
	offset, err := binarySize(DATA, uint16(0))
	if err != nil {
		return nil, err
	}
	if _, err := bytesReader.Seek(int64(offset), io.SeekStart); err != nil {
		return nil, err
	}
	data := make([]byte, bytesReader.Len())
	if err := binary.Read(bytesReader, binary.BigEndian, data); err != nil {
		return nil, err
	}
	return data, nil
}

func createDataPacket(blockNumber uint16, data []byte) DataPacket {
	dataPacket := DataPacket{
		blockNumber: blockNumber,
		data:        data,
	}
	return dataPacket
}

func (dataPacket DataPacket) bytes() []byte {
	// FIXME this is where I'm leaving off for today.
	// Write out the fields using binary.write into something for sendPacket()
	return nil
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// ErrorPacket represents an Error Packet as defined in RFC 1350.
type ErrorPacket struct {
	tftpError
	data []byte
}

func createErrorPacket(tftpErr tftpError) (ErrorPacket, error) {
	size, err := errorPacketSize(tftpErr)
	if err != nil {
		return backupError(), err
	}
	data := make([]byte, 0, size)
	buf := bytes.NewBuffer(data)

	write := func(data interface{}) {
		if err == nil {
			err = binary.Write(buf, binary.BigEndian, data)
		}
	}

	write(ERROR)
	write(tftpErr.errorCode)
	write(tftpErr.errorMsg)
	write(0x00)

	// TODO ERROR: looks like an internal server error
	if err != nil {
		return backupError(), err
	}

	pak := ErrorPacket{
		tftpError: tftpErr,
		data:      data,
	}

	return pak, nil
}

func errorPacketSize(err tftpError) (int, error) {
	return binarySize(ERROR, err.errorCode, err.errorMsg, 0x00)
}

func backupError() ErrorPacket {
	msg := []byte("server not implemented")
	backup := append([]byte{0x00, 0x05, 0x00, 0x01}, msg...)
	backup = append(backup, 0x00)
	foo := ErrorPacket{
		tftpError: tftpError{
			errorCode: 0,
			errorMsg:  errors.New("server not implemented"),
		},
		data: backup,
	}
	return foo
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func readNetasciiString(buffer *bytes.Buffer) (string, error) {
	netasciiStr, err := buffer.ReadBytes(0x00)
	if err != nil {
		return "", err
	}
	str := string(bytes.TrimRight(netasciiStr, string(0x00)))
	return str, nil
}

func (packet Packet) readOpCode() (opCode, error) {
	bytesReader := bytes.NewReader(packet.data)
	var op opCode
	if err := binary.Read(bytesReader, binary.BigEndian, &op); err != nil {
		return op, err
	}
	return op, nil
}

func binarySize(a ...interface{}) (int, error) {
	size := 0
	for _, v := range a {
		sz := binary.Size(v)
		if sz == -1 {
			msg := "binarySize: expected a fixed-size value or a slice of fixed-size values, or a pointer to such data"
			err := fmt.Errorf("%v, found %v\n", msg, v)
			return -1, err
		}
		size += sz
	}
	return size, nil
}
