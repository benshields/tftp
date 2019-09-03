package tftp

import "net"

// client defines parameters for maintaining a TFTP client connection.
type Client struct {
	// lastPacket is the last TFTP packet received from the client, used in case of re-transmission.
	lastPacket []byte

	// remoteAddr is the address at which this client can be reached.
	remoteAddr net.Addr

	// handler interfaces with the file that the client is reading from or writing to.
	fileHandler readWriteCloser
}

func NewClient(request Packet) *Client {
	c := &Client{lastPacket: request.data,
		remoteAddr: request.from}
	return c
}

func (client *Client) setup() error {
	// TODO: setup the fileHandler
	return nil
}
