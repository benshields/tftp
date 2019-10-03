package tftp

import (
	"io"
	"os"
)

type ResponseWriter interface {
	WriteResponse(pak Packet) (response []byte)
	Close() error
}

func newPacketHandler(req *RequestPacket) (ResponseWriter, *tftpError) {
	fileHandler := newBlockStreamer(req.filename, req.openFlag, req.encodingFlag)
	err := fileHandler.Open()
	if err != nil {
		return nil, ftpOpenFileError(err)
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

func ftpOpenFileError(err error) *tftpError {
	if os.IsExist(err) {
		return &errFileExists
	} else if os.IsNotExist(err) {
		return &errNoFile
	} else if os.IsPermission(err) {
		return &errAccess
	} else {
		msg := "error occurred while opening file - %v"
		fileError := errNotDef.fmt(msg, err)
		return &fileError
	}
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
	n, err := rrqResponseWriter.fileHandler.Read(data) // TODO#01 I can't just call Read, it needs to be based on the correct block number
	/* TODO I bet I can do a scheme where handler deals with block num
	if RRQ
		bn == 1 and call read
	else if bn == last.bn+1
		call read
	else if bn == last.bn
		resend last
	else
		send error because it's out of order
	*/
	data = data[:n]
	if err != nil && err != io.EOF {
		return internalErrorPacket().raw
	}

	blockNumber, err := rrqResponseWriter.nextBlockNumber(pak)
	if err != nil { // TODO the error I received is actually useful, it should be returned
		return internalErrorPacket().raw
	}

	dataPacket := createDataPacket(blockNumber, data)

	raw, err := dataPacket.bytes()
	if err != nil {
		return internalErrorPacket().raw
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
		return internalErrorPacket().raw
	}

	if blockNumber != 0 {
		// TODO: so right here, if the packet data is 0-511 bytes, I need to dally (keep sending final ACK in response to final DATA)
		// TODO: In general, I just want to make sure I don't write the same data twice. So I need some block number error checking.
		data, err := wrqResponseWriter.parsePacket(pak)
		if err != nil {
			return internalErrorPacket().raw
		}

		n, err := wrqResponseWriter.fileHandler.Write(data) // TODO I can't just call Write, it needs to be based on the correct block number
		if err != nil || n != len(data) {                   // TODO do I really have to be measuring len here? It's probably included in the error
			return internalErrorPacket().raw
		}
	}

	ackPacket := createAckPacket(blockNumber)

	raw, err := ackPacket.bytes()
	if err != nil {
		return internalErrorPacket().raw
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

func (wrqResponseWriter *WrqResponseWriter) parsePacket(pak Packet) ([]byte, error) {
	dataPacket, err := parseDataPacket(pak)
	if err != nil {
		return nil, err
	}
	return dataPacket.data, nil
}
