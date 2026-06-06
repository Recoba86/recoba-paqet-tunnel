package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"paqet/internal/conf"
	"paqet/internal/tnet"
)

type PType = byte

const (
	PPING PType = 0x01
	PPONG PType = 0x02
	PTCPF PType = 0x03
	PTCP  PType = 0x04
	PUDP  PType = 0x05
)

type Proto struct {
	Type PType
	Addr *tnet.Addr
	TCPF []conf.TCPF
}

func (p *Proto) Read(r io.Reader) error {
	var length uint32
	err := binary.Read(r, binary.BigEndian, &length)
	if err != nil {
		return err
	}
	if length > 1024*1024 {
		return fmt.Errorf("protocol payload too large: %d", length)
	}

	data := make([]byte, length)
	_, err = io.ReadFull(r, data)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, p)
}

func (p *Proto) Write(w io.Writer) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}

	err = binary.Write(w, binary.BigEndian, uint32(len(data)))
	if err != nil {
		return err
	}

	_, err = w.Write(data)
	return err
}
