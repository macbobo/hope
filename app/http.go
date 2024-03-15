package app

import (
	"fmt"
	"github.com/panjf2000/gnet"
)

type Http struct {
}

func (a *Http) ParserRequ(packet []byte, c gnet.Conn, p interface{}) (interface{}, []byte, error) {
	fmt.Printf("%s\n", string(packet))
	return nil, nil, nil
}

func (a *Http) ParserResp(packet []byte, c gnet.Conn, p interface{}) (interface{}, []byte, error) {
	fmt.Printf("%s\n", string(packet))
	return nil, nil, nil
}

func (a *Http) Reset(c gnet.Conn) {
}

func (a *Http) Tick(parent interface{}) {
}
