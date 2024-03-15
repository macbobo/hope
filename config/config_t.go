package config

import (
	"net"
	"time"
)

/**
0: TCP
1: UDP
*/
const PROTOCOL_TCP = 0
const PROTOCOL_UDP = 1

type Config_t struct {
	Uid      uint64
	Uid_hash string

	App          string //默认“”
	Protocol_str string //默认为“tcp”
	Protocol     int

	Bindip_str string
	Bindip     net.IP
	Bindport   uint16

	Upstreamip_str string
	Upstreamip     net.IP
	Upstreamport   uint16

	//转换通讯协议
	Appex          string //保留使用
	Protocolex_str string
	Protocolex     int

	Createtime time.Time

	Subext []Config_t //仅在应用层app时使用，如ftp，sip等

}

const LOCALIP_STR = "127.0.0.1"
