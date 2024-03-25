/**
k8spacket@v1.2.1/tls-api
*/

package tls_api

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"reflect"
)

func ParseTLSPayload(payload []byte) TLSWrapper {
	reader := bufio.NewReader(bytes.NewReader(payload))
	var tlsWrapper = TLSWrapper{}
	tlsWrapper = parseTLSRecord(reader, tlsWrapper)
	return tlsWrapper
}

func parseTLSRecord(reader *bufio.Reader, tlsWrapper TLSWrapper) TLSWrapper {
	var b, _ = reader.Peek(1)
	var recordLayer = RecordLayer{}
	if b[0] == TLSRecord {
		binary.Read(reader, binary.BigEndian, &recordLayer)
	}
	b, _ = reader.Peek(1)
	if b[0] == ClientHelloTLS {
		tlsWrapper.ClientHelloTLSRecord = parseClientHelloTLSRecord(reader)
	} else if b[0] == ServerHelloTLS {
		tlsWrapper.ServerHelloTLSRecord = parseServerHelloTLSRecord(reader)
	} else if b[0] == CertificateTLS {
		tlsWrapper.CertificateTLSRecord = parseCertificateTLSRecord(reader)
	} else {
		if !reflect.DeepEqual(recordLayer, RecordLayer{}) {
			parseAnotherTLSRecord(reader, recordLayer.Length)
		} else {
			parseAnotherTLSHandshakeProtocol(reader)
		}
	}
	var _, err = reader.Peek(1)
	if err == nil {
		tlsWrapper = parseTLSRecord(reader, tlsWrapper)
	}
	return tlsWrapper
}

func parseAnotherTLSRecord(reader *bufio.Reader, length uint16) {
	bytes := make([]byte, int(length))
	binary.Read(reader, binary.BigEndian, &bytes)
}

func parseAnotherTLSHandshakeProtocol(reader *bufio.Reader) {
	var handshakeType uint8
	var length [3]byte
	binary.Read(reader, binary.BigEndian, &handshakeType)
	binary.Read(reader, binary.BigEndian, &length)
	bytes := make([]byte, bytesToInt(length))
	binary.Read(reader, binary.BigEndian, &bytes)
}
