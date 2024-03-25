package tls_api

import (
	"bufio"
	"crypto/x509"
	"encoding/binary"
)

func parseCertificateTLSRecord(reader *bufio.Reader) CertificateTLSRecord {
	var tlsRecord CertificateTLSRecord

	binary.Read(reader, binary.BigEndian, &tlsRecord.HandshakeType)
	binary.Read(reader, binary.BigEndian, &tlsRecord.CertificateMessageLength)
	binary.Read(reader, binary.BigEndian, &tlsRecord.CertificatesLength)

	var certificatesLength = bytesToInt(tlsRecord.CertificatesLength)
	var lengthCounter = 0
	for certificatesLength-lengthCounter > 0 {
		var certificateLengthBytes = [3]byte{}
		binary.Read(reader, binary.BigEndian, &certificateLengthBytes)
		var certificateLength = bytesToInt(certificateLengthBytes)
		certificateBytes := make([]byte, certificateLength)
		binary.Read(reader, binary.BigEndian, &certificateBytes)
		certificate, _ := x509.ParseCertificate(certificateBytes)
		if certificate != nil {
			tlsRecord.Certificates = append(tlsRecord.Certificates, *certificate)
		}
		lengthCounter = lengthCounter + 3 + certificateLength
	}
	return tlsRecord
}

func bytesToInt(bytes [3]byte) int {
	return int(uint(bytes[2]) | uint(bytes[1])<<8 | uint(bytes[0])<<16)
}
