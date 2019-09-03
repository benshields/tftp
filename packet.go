package tftp

import "net"

type Packet struct {
	from net.Addr
	data []byte
	error
}
