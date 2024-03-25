package tls_api

import (
	"bufio"
	"bytes"
	"encoding/binary"
)

func parseClientHelloTLSRecord(reader *bufio.Reader) ClientHelloTLSRecord {
	var tlsRecord ClientHelloTLSRecord

	binary.Read(reader, binary.BigEndian, &tlsRecord.HandshakeProtocol)

	binary.Read(reader, binary.BigEndian, &tlsRecord.Session.Length)
	sessionId := make([]byte, tlsRecord.Session.Length)
	binary.Read(reader, binary.BigEndian, &sessionId)
	tlsRecord.Session.Id = sessionId

	binary.Read(reader, binary.BigEndian, &tlsRecord.Ciphers.Length)
	ciphersValue := make([]byte, tlsRecord.Ciphers.Length)
	binary.Read(reader, binary.BigEndian, &ciphersValue)
	tlsRecord.Ciphers.Value = ciphersValue

	binary.Read(reader, binary.BigEndian, &tlsRecord.CompressionMethods.Length)
	compressionMethodsValue := make([]byte, tlsRecord.CompressionMethods.Length)
	binary.Read(reader, binary.BigEndian, &compressionMethodsValue)
	tlsRecord.CompressionMethods.Value = compressionMethodsValue

	binary.Read(reader, binary.BigEndian, &tlsRecord.Extensions.Length)

	tlsRecord.Extensions.Extensions = make(map[uint16]Extension)

	var lengthCounter = 0
	for int(tlsRecord.Extensions.Length)-lengthCounter > 0 {
		var extension Extension
		binary.Read(reader, binary.BigEndian, &extension.Type)
		binary.Read(reader, binary.BigEndian, &extension.Length)
		extensionValue := make([]byte, extension.Length)
		binary.Read(reader, binary.BigEndian, &extensionValue)
		extension.Value = extensionValue
		tlsRecord.Extensions.Extensions[extension.Type] = extension
		lengthCounter += int(extension.Length) + 4
	}

	tlsRecord.ResolvedClientFields.ServerName = getServerName(tlsRecord.Extensions).Value
	tlsRecord.ResolvedClientFields.SupportedVersions = getSupportedVersions(tlsRecord).Value
	tlsRecord.ResolvedClientFields.Ciphers = getCiphers(tlsRecord.Ciphers)

	return tlsRecord
}

func getServerName(record Extensions) ServerNameExtension {
	extension := record.Extensions[ServerNameExt]

	var serverNameExtension ServerNameExtension

	reader := bytes.NewReader(extension.Value)
	binary.Read(reader, binary.BigEndian, &serverNameExtension.ListLength)
	binary.Read(reader, binary.BigEndian, &serverNameExtension.Type)
	binary.Read(reader, binary.BigEndian, &serverNameExtension.Length)
	serverNameValue := make([]byte, serverNameExtension.Length)
	binary.Read(reader, binary.BigEndian, &serverNameValue)
	serverNameExtension.Value = string(serverNameValue)

	return serverNameExtension
}

func getSupportedVersions(tlsRecord ClientHelloTLSRecord) SupportedVersionsExtension {
	extension := tlsRecord.Extensions.Extensions[SupportedVersionsExt]

	var supportedVersionsExtension SupportedVersionsExtension

	reader := bytes.NewReader(extension.Value)
	binary.Read(reader, binary.BigEndian, &supportedVersionsExtension.SupportedVersionLength)
	if supportedVersionsExtension.SupportedVersionLength > 0 {
		supportedVersionValue := make([]byte, 2)
		for i := 0; i < int(supportedVersionsExtension.SupportedVersionLength/2); i++ {
			binary.Read(reader, binary.BigEndian, &supportedVersionValue)
			supportedVersionsExtension.Value = append(supportedVersionsExtension.Value, GetTLSVersion(binary.BigEndian.Uint16(supportedVersionValue)))
		}
	} else {
		supportedVersionsExtension.Value = append(supportedVersionsExtension.Value, GetTLSVersion(tlsRecord.HandshakeProtocol.TLSVersion))
	}

	return supportedVersionsExtension
}

func getCiphers(ciphers Ciphers) []string {
	reader := bytes.NewReader(ciphers.Value)
	cipherValue := make([]byte, 2)
	var result []string
	for i := 0; i < int(ciphers.Length/2); i++ {
		binary.Read(reader, binary.BigEndian, cipherValue)
		result = append(result, GetCipherSuite(binary.BigEndian.Uint16(cipherValue)))
	}
	return result
}

func parseServerHelloTLSRecord(reader *bufio.Reader) ServerHelloTLSRecord {
	var tlsRecord ServerHelloTLSRecord

	binary.Read(reader, binary.BigEndian, &tlsRecord.HandshakeProtocol)

	binary.Read(reader, binary.BigEndian, &tlsRecord.Session.Length)
	sessionId := make([]byte, tlsRecord.Session.Length)
	binary.Read(reader, binary.BigEndian, &sessionId)
	tlsRecord.Session.Id = sessionId

	binary.Read(reader, binary.BigEndian, &tlsRecord.CipherSuite.Value)

	binary.Read(reader, binary.BigEndian, &tlsRecord.CompressionMethods.Length)
	compressionMethodsValue := make([]byte, tlsRecord.CompressionMethods.Length)
	binary.Read(reader, binary.BigEndian, &compressionMethodsValue)
	tlsRecord.CompressionMethods.Value = compressionMethodsValue

	binary.Read(reader, binary.BigEndian, &tlsRecord.Extensions.Length)

	tlsRecord.Extensions.Extensions = make(map[uint16]Extension)
	var lengthCounter = 0
	for int(tlsRecord.Extensions.Length)-lengthCounter > 0 {
		var extension Extension
		binary.Read(reader, binary.BigEndian, &extension.Type)
		binary.Read(reader, binary.BigEndian, &extension.Length)
		extensionValue := make([]byte, extension.Length)
		binary.Read(reader, binary.BigEndian, &extensionValue)
		extension.Value = extensionValue
		tlsRecord.Extensions.Extensions[extension.Type] = extension
		lengthCounter += int(extension.Length) + 4
	}

	tlsRecord.ResolvedServerFields.SupportedVersion = getSupportedVersion(tlsRecord)
	tlsRecord.ResolvedServerFields.Cipher = getCipher(tlsRecord.CipherSuite)

	return tlsRecord
}

func getSupportedVersion(record ServerHelloTLSRecord) string {
	var version = GetTLSVersion(record.HandshakeProtocol.TLSVersion)
	extension := record.Extensions.Extensions[TLSVersionExt]
	if extension.Value != nil {
		version = GetTLSVersion(binary.BigEndian.Uint16(extension.Value))
	}
	return version
}

func getCipher(cipher CipherSuite) string {
	return GetCipherSuite(cipher.Value)
}
