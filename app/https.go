package app

import (
	"fmt"
	"github.com/macbobo/gope/app/tls_api"
	"github.com/panjf2000/gnet"
)

type Https struct {
	Http
	tlsversion string
	tlscert    string //证书配置
	tlskey     string
	mitmstate  map[string]int //会话状态
}

func (a *Https) ParserRequ(packet []byte, c gnet.Conn, p interface{}) (interface{}, []byte, error) {
	tlsi := tls_api.ParseTLSPayload(packet)
	fmt.Println("client", tlsi)

	//todo MITM劫持
	k := c.Context()
	switch a.mitmstate[k.(string)] {
	case 0:
		if tls_api.GetTLSVersion(tlsi.ClientHelloTLSRecord.HandshakeProtocol.TLSVersion) != "" {
			//cert, err := tls.LoadX509KeyPair(a.tlscert, a.tlskey)
			//if err == nil {
			//	cfg := &tls.Config{Certificates: []tls.Certificate{cert}}
			//}
		}
	case 1:
	case 2:
	default:

	}

	return nil, nil, nil
}

func (a *Https) ParserResp(packet []byte, c gnet.Conn, p interface{}) (interface{}, []byte, error) {
	tls := tls_api.ParseTLSPayload(packet)
	fmt.Println("server", tls)

	return nil, nil, nil
}

func (a *Https) Reset(c gnet.Conn) {
	a.Http.Reset(c)
}

func (a *Https) Tick(parent interface{}) {
	a.Http.Tick(parent)
}
