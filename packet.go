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

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// RequestPacket is generated from a RRQ/WRQ packet as defined in RFC 1350.
type RequestPacket struct {
	openFlag            // generated from the opCode
	filename     string // null-terminator is removed
	encodingFlag        // generated from the mode value
}

// parseRequestPacket parses the packet's raw into the fields of
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

// AckPacket is generated from a ACK packet as defined in RFC 1350.
type AckPacket struct {
	blockNumber uint16
}

func createAckPacket(blockNumber uint16) AckPacket {
	ackPacket := AckPacket{
		blockNumber: blockNumber,
	}
	return ackPacket
}

func (ackPacket AckPacket) bytes() ([]byte, error) {
	return binaryWrite(ACK, ackPacket.blockNumber)
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

func (dataPacket DataPacket) bytes() ([]byte, error) {
	return binaryWrite(DATA, dataPacket.blockNumber, dataPacket.data)
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// ErrorPacket represents an Error Packet as defined in RFC 1350.
type ErrorPacket struct {
	tftpError
	raw []byte
}

func createErrorPacket(tftpErr tftpError) (ErrorPacket, error) {
	size, err := errorPacketSize(tftpErr)
	if err != nil {
		return internalErrorPacket(), err
	}
	data := make([]byte, 0, size)
	buffer := bytes.NewBuffer(data)

	write := func(d interface{}) {
		if err == nil {
			err = binary.Write(buffer, binary.BigEndian, d)
		}
	}

	write(ERROR)
	write(tftpErr.errorCode)
	fixedLengthData := []byte(tftpErr.errorMsg.Error())
	write(fixedLengthData)
	write(byte(0x00))

	// TODO ERROR: looks like an internal server error
	if err != nil {
		return internalErrorPacket(), err
	}

	pak := ErrorPacket{
		tftpError: tftpErr,
		raw:       buffer.Bytes(),
	}

	return pak, nil
}

func errorPacketSize(err tftpError) (int, error) {
	fixedLengthData := []byte(err.errorMsg.Error())
	return binarySize(ERROR, err.errorCode, fixedLengthData, byte(0x00))
}

func internalErrorPacket() ErrorPacket {
	errStr := "internal server error"
	opCode := []byte{0x00, 0x05}
	errorCode := []byte{0x00, 0x00}
	errMsg := []byte(errStr)
	nullTerminator := []byte{0x00}

	raw, _ := binaryWrite(opCode, errorCode, errMsg, nullTerminator)

	errPak := ErrorPacket{
		tftpError: tftpError{
			errorCode: 0,
			errorMsg:  errors.New(errStr),
		},
		raw: raw,
	}
	return errPak
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func readNetasciiString(buffer *bytes.Buffer) (string, error) {
	netasciiStr, err := buffer.ReadBytes(0x00)
	if err != nil {
		return "", err
	}
	str := string(bytes.TrimRight(netasciiStr, string(byte(0x00))))
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

func binaryWrite(elements ...interface{}) ([]byte, error) {
	size, err := binarySize(elements...)
	if err != nil {
		return nil, err
	}
	raw := make([]byte, 0, size)
	buffer := bytes.NewBuffer(raw)

	write := func(d interface{}) {
		err = binary.Write(buffer, binary.BigEndian, d)
	}
	for _, e := range elements {
		write(e)
	}

	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func binarySize(elements ...interface{}) (int, error) {
	size := 0
	for _, v := range elements {
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
