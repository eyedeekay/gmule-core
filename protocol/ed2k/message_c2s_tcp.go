package ed2k

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
)

// LoginMessage is the first message send by the client to the server after TCP connection establishment.
type LoginMessage struct {
	message
	UID      UID
	ClientID uint32
	// The TCP port used by the client, configurable.
	Port uint16
	Tags []Tag
}

// Encode encodes the message to binary data.
func (m *LoginMessage) Encode() (data []byte, err error) {
	if m == nil {
		return
	}
	buf := new(bytes.Buffer)
	if _, err = m.Header.WriteTo(buf); err != nil {
		return
	}
	buf.WriteByte(MessageLoginRequest)
	buf.Write(m.UID.Bytes())

	if err = binary.Write(buf, binary.LittleEndian, m.ClientID); err != nil {
		return
	}
	if err = binary.Write(buf, binary.LittleEndian, m.Port); err != nil {
		return
	}

	if err = binary.Write(buf, binary.LittleEndian, uint32(len(m.Tags))); err != nil {
		return
	}

	for _, tag := range m.Tags {
		if _, err = tag.WriteTo(buf); err != nil {
			return
		}
	}

	data = buf.Bytes()

	size := len(data) - HeaderLength
	binary.LittleEndian.PutUint32(data[1:5], uint32(size)) // message size

	return
}

// Decode decodes the message from binary data.
func (m *LoginMessage) Decode(data []byte) (err error) {
	header := Header{}
	err = header.Decode(data)
	if err != nil {
		return
	}
	pos := HeaderLength
	if len(data) < pos+int(header.Size) ||
		len(data) < pos+1+16+4+2+4 {
		return ErrShortBuffer
	}
	if data[5] != MessageLoginRequest {
		return ErrWrongMessageType
	}
	m.Header = header
	pos++
	copy(m.UID[:], data[pos:pos+16])

	pos += 16
	m.ClientID = binary.LittleEndian.Uint32(data[pos : pos+4])

	pos += 4
	m.Port = binary.LittleEndian.Uint16(data[pos : pos+2])

	pos += 2
	tagCount := binary.LittleEndian.Uint32(data[pos : pos+4])

	pos += 4
	r := bytes.NewReader(data[pos:])
	for i := 0; i < int(tagCount); i++ {
		tag, err := ReadTag(r)
		if err != nil {
			return err
		}
		m.Tags = append(m.Tags, tag)
	}
	return
}

// Type is the message type
func (m LoginMessage) Type() uint8 {
	return MessageLoginRequest
}

func (m LoginMessage) String() string {
	b := bytes.Buffer{}
	b.WriteString("[login]\n")
	b.WriteString(m.Header.String())
	b.WriteString("\n")
	fmt.Fprintf(&b, "uid: %s, clientID: %#x(%s), port: %d\n", m.UID, m.ClientID, ClientID(m.ClientID).String(), m.Port)
	for i, tag := range m.Tags {
		fmt.Fprintf(&b, "tag%d - %v: %v\n", i, tag.Name(), tag.Value())
	}
	return b.String()
}

// ServerMessage is variable length message that is sent from the server to client.
// A single server-message may contain several messages separated by new line characters ('\r','\n' or both).
// Messages that start with "server version", "warning", "error" and "emDynIP" have special meaning for the client.
type ServerMessage struct {
	message
	// A list of server messages separated by new lines.
	Messages string
}

// Encode encodes the message to binary data.
func (m *ServerMessage) Encode() (data []byte, err error) {
	if m == nil {
		return
	}
	buf := new(bytes.Buffer)

	if _, err = m.Header.WriteTo(buf); err != nil {
		return
	}
	buf.WriteByte(MessageServerMessage)

	size := len(m.Messages)
	if err = binary.Write(buf, binary.LittleEndian, uint16(size)); err != nil {
		return
	}
	if _, err = buf.WriteString(m.Messages); err != nil {
		return
	}

	data = buf.Bytes()
	size = len(data) - HeaderLength
	binary.LittleEndian.PutUint32(data[1:5], uint32(size)) // message size

	return
}

// Decode decodes the message from binary data.
func (m *ServerMessage) Decode(data []byte) (err error) {
	header := Header{}
	err = header.Decode(data)
	if err != nil {
		return
	}
	pos := HeaderLength
	if len(data) < pos+int(header.Size) ||
		len(data) < pos+3 {
		return ErrShortBuffer
	}
	if data[5] != MessageServerMessage {
		return ErrWrongMessageType
	}
	m.Header = header
	pos++
	size := binary.LittleEndian.Uint16(data[pos : pos+2])
	pos += 2
	if len(data) < pos+int(size) {
		return ErrShortBuffer
	}
	m.Messages = string(data[pos : pos+int(size)])
	return
}

// Type is the message type
func (m ServerMessage) Type() uint8 {
	return MessageServerMessage
}

func (m ServerMessage) String() string {
	b := bytes.Buffer{}
	b.WriteString("[server-message]\n")
	b.WriteString(m.Header.String())
	b.WriteString("\n")
	b.WriteString(m.Messages)
	return b.String()
}

// IDChangeMessage is the message sent by the server as a response to the login request message and
// signifies that the server has accepted the client connection.
type IDChangeMessage struct {
	message
	ClientID uint32
	// Currently only 1 bit (the LSB) has meaning, setting it to 1 signals that the server supports compression.
	Bitmap uint32
}

// Encode encodes the message to binary data.
func (m *IDChangeMessage) Encode() (data []byte, err error) {
	if m == nil {
		return
	}
	buf := new(bytes.Buffer)
	if _, err = m.Header.WriteTo(buf); err != nil {
		return
	}
	buf.WriteByte(MessageIDChange)

	if err = binary.Write(buf, binary.LittleEndian, m.ClientID); err != nil {
		return
	}
	if err = binary.Write(buf, binary.LittleEndian, m.Bitmap); err != nil {
		return
	}

	data = buf.Bytes()
	size := len(data) - HeaderLength
	binary.LittleEndian.PutUint32(data[1:5], uint32(size)) // message size

	return
}

// Decode decodes the message from binary data.
func (m *IDChangeMessage) Decode(data []byte) (err error) {
	header := Header{}
	err = header.Decode(data)
	if err != nil {
		return
	}
	pos := HeaderLength
	if len(data) < pos+int(header.Size) ||
		len(data) < pos+9 {
		return ErrShortBuffer
	}
	if data[5] != MessageIDChange {
		return ErrWrongMessageType
	}
	m.Header = header
	pos++
	m.ClientID = binary.LittleEndian.Uint32(data[pos : pos+4])
	pos += 4
	m.Bitmap = binary.LittleEndian.Uint32(data[pos : pos+4])

	return
}

// Type is the message type
func (m IDChangeMessage) Type() uint8 {
	return MessageIDChange
}

func (m IDChangeMessage) String() string {
	b := bytes.Buffer{}
	b.WriteString("[id-change]\n")
	b.WriteString(m.Header.String())
	b.WriteString("\n")
	fmt.Fprintf(&b, "clientID: %#x(%s), bitmap: %#x", m.ClientID, ClientID(m.ClientID).String(), m.Bitmap)
	return b.String()
}

// OfferFilesMessage is used by the client to describe local files available for other clients to download.
// In case the client has files to offer, the offer-files message is sent immediately after the
// connection establishment. The message is also transmitted when the client’s shared file list changes.
type OfferFilesMessage struct {
	message
	// An optional list of files, in any case no more than 200.
	// The Server can also set a lower limit to this number.
	Files []File
}

// Encode encodes the message to binary data.
func (m *OfferFilesMessage) Encode() (data []byte, err error) {
	if m == nil {
		return
	}
	buf := new(bytes.Buffer)
	if _, err = m.Header.WriteTo(buf); err != nil {
		return
	}
	buf.WriteByte(MessageOfferFiles)

	if err = binary.Write(buf, binary.LittleEndian, uint32(len(m.Files))); err != nil {
		return
	}
	for _, file := range m.Files {
		if _, err = file.WriteTo(buf); err != nil {
			return
		}
	}

	data = buf.Bytes()
	size := len(data) - HeaderLength
	binary.LittleEndian.PutUint32(data[1:5], uint32(size)) // message size

	return
}

// Decode decodes the message from binary data.
func (m *OfferFilesMessage) Decode(data []byte) (err error) {
	header := Header{}
	err = header.Decode(data)
	if err != nil {
		return
	}
	pos := HeaderLength
	if len(data) < pos+int(header.Size) ||
		len(data) < pos+5 {
		return ErrShortBuffer
	}
	if data[5] != MessageOfferFiles {
		return ErrWrongMessageType
	}
	m.Header = header
	pos++
	fileCount := binary.LittleEndian.Uint32(data[pos : pos+4])
	pos += 4
	r := bytes.NewReader(data[pos:])
	for i := 0; i < int(fileCount); i++ {
		file, err := ReadFile(r)
		if err != nil {
			return err
		}
		m.Files = append(m.Files, *file)
	}
	return
}

// Type is the message type
func (m OfferFilesMessage) Type() uint8 {
	return MessageOfferFiles
}

func (m OfferFilesMessage) String() string {
	b := bytes.Buffer{}
	b.WriteString("[offer-files]\n")
	b.WriteString(m.Header.String())
	b.WriteString("\nfiles:\n")
	for i, file := range m.Files {
		fmt.Fprintf(&b, "file%d - %X %s:%d\n", i, file.Hash, ClientID(file.ClientID).String(), file.Port)
		for j, tag := range file.Tags {
			fmt.Fprintf(&b, "tag%d - %v: %v\n", j, tag.Name(), tag.Value())
		}

	}
	return b.String()
}

// GetServerListMessage is sent when the client is configured to expand its list of eMule servers by querying its current server.
// This message may be sent from the client to the server immediately after a successful handshake completion.
type GetServerListMessage struct {
	message
}

// Encode encodes the message to binary data.
func (m *GetServerListMessage) Encode() (data []byte, err error) {
	if m == nil {
		return
	}
	buf := new(bytes.Buffer)
	if _, err = m.Header.WriteTo(buf); err != nil {
		return
	}
	buf.WriteByte(MessageGetServerList)

	data = buf.Bytes()
	size := len(data) - HeaderLength
	binary.LittleEndian.PutUint32(data[1:5], uint32(size)) // message size

	return
}

// Decode decodes the message from binary data.
func (m *GetServerListMessage) Decode(data []byte) (err error) {
	header := Header{}
	err = header.Decode(data)
	if err != nil {
		return
	}
	pos := HeaderLength
	if len(data) < pos+int(header.Size) ||
		len(data) < pos+1 {
		return ErrShortBuffer
	}
	if data[5] != MessageGetServerList {
		return ErrWrongMessageType
	}
	m.Header = header

	return
}

// Type is the message type
func (m GetServerListMessage) Type() uint8 {
	return MessageGetServerList
}

func (m GetServerListMessage) String() string {
	b := bytes.Buffer{}
	b.WriteString("[get-server-list]\n")
	b.WriteString(m.Header.String())
	return b.String()
}

// ServerListMessage is sent from the server to the client.
// The message contains information about additional eMule servers to be used to expand the client’s server list.
type ServerListMessage struct {
	message
	// Server descriptor entries, each entry size is 6 bytes and contains 4 bytes IP address and then 2 byte TCP port.
	Servers []*net.TCPAddr
}

// Encode encodes the message to binary data.
func (m *ServerListMessage) Encode() (data []byte, err error) {
	if m == nil {
		return
	}
	buf := new(bytes.Buffer)
	if _, err = m.Header.WriteTo(buf); err != nil {
		return
	}
	buf.WriteByte(MessageServerList)
	buf.WriteByte(byte(len(m.Servers))) // entry count

	for _, addr := range m.Servers {
		if addr == nil {
			addr = &net.TCPAddr{
				IP:   net.IPv4zero,
				Port: 0,
			}
		}
		buf.Write(addr.IP.To4())
		binary.Write(buf, binary.LittleEndian, uint16(addr.Port))
	}

	data = buf.Bytes()
	size := len(data) - HeaderLength
	binary.LittleEndian.PutUint32(data[1:5], uint32(size)) // message size

	return
}

// Decode decodes the message from binary data.
func (m *ServerListMessage) Decode(data []byte) (err error) {
	header := Header{}
	err = header.Decode(data)
	if err != nil {
		return
	}
	pos := HeaderLength
	if len(data) < pos+int(header.Size) ||
		len(data) < pos+2 {
		return ErrShortBuffer
	}
	if data[5] != MessageServerList {
		return ErrWrongMessageType
	}
	m.Header = header
	pos++

	count := int(data[pos])
	pos++
	if len(data) < pos+count*6 {
		return ErrShortBuffer
	}

	for i := 0; i < count; i++ {
		m.Servers = append(m.Servers,
			&net.TCPAddr{
				IP:   net.IP(data[pos : pos+4]),
				Port: int(binary.LittleEndian.Uint16(data[pos+4 : pos+6])),
			})
		pos += 6
	}
	return
}

// Type is the message type
func (m ServerListMessage) Type() uint8 {
	return MessageServerList
}

func (m ServerListMessage) String() string {
	b := bytes.Buffer{}
	b.WriteString("[server-list]\n")
	b.WriteString(m.Header.String())
	b.WriteString("\n")
	b.WriteString("servers:\n")
	var ss []string
	for _, addr := range m.Servers {
		ss = append(ss, addr.String())
	}
	b.WriteString(strings.Join(ss, ","))
	return b.String()
}

// ServerStatusMessage is sent from the server to the client.
// The message contains information on the current number of users and files on the server.
// The information in this message is both stored by the client and also displayed to the user.
type ServerStatusMessage struct {
	message
	// The number of users currently logged in to the server.
	UserCount uint32
	// The number of files that this server is informed about.
	FileCount uint32
}

// Encode encodes the message to binary data.
func (m *ServerStatusMessage) Encode() (data []byte, err error) {
	if m == nil {
		return
	}
	buf := new(bytes.Buffer)
	if _, err = m.Header.WriteTo(buf); err != nil {
		return
	}
	buf.WriteByte(MessageServerStatus)

	if err = binary.Write(buf, binary.LittleEndian, m.UserCount); err != nil {
		return
	}
	if err = binary.Write(buf, binary.LittleEndian, m.FileCount); err != nil {
		return
	}

	data = buf.Bytes()
	size := len(data) - HeaderLength
	binary.LittleEndian.PutUint32(data[1:5], uint32(size)) // message size

	return
}

// Decode decodes the message from binary data.
func (m *ServerStatusMessage) Decode(data []byte) (err error) {
	header := Header{}
	err = header.Decode(data)
	if err != nil {
		return
	}
	pos := HeaderLength
	if len(data) < pos+int(header.Size) ||
		len(data) < pos+9 {
		return ErrShortBuffer
	}
	if data[5] != MessageServerStatus {
		return ErrWrongMessageType
	}
	m.Header = header
	pos++

	m.UserCount = binary.LittleEndian.Uint32(data[pos : pos+4])
	pos += 4
	m.FileCount = binary.LittleEndian.Uint32(data[pos : pos+4])

	return
}

// Type is the message type
func (m ServerStatusMessage) Type() uint8 {
	return MessageServerStatus
}

func (m ServerStatusMessage) String() string {
	b := bytes.Buffer{}
	b.WriteString("[server-status]\n")
	b.WriteString(m.Header.String())
	b.WriteString("\n")
	fmt.Fprintf(&b, "users: %d, files: %d", m.UserCount, m.FileCount)
	return b.String()
}

// ServerIdentMessage is a message sent from the server to the client.
// Contains a server hash,the server IP address and
// TCP port (which may be useful when connecting through a proxy) and also server description information.
type ServerIdentMessage struct {
	message
	// A GUID of the server (seems to be used for debug).
	Hash [16]byte
	// The IP address of the server.
	IP uint32
	// The TCP port on which the server listens.
	Port uint16

	Tags []Tag
}

// Encode encodes the message to binary data.
func (m *ServerIdentMessage) Encode() (data []byte, err error) {
	if m == nil {
		return
	}
	buf := new(bytes.Buffer)
	if _, err = m.Header.WriteTo(buf); err != nil {
		return
	}
	buf.WriteByte(MessageServerIdent)
	if _, err = buf.Write(m.Hash[:]); err != nil {
		return
	}
	if err = binary.Write(buf, binary.LittleEndian, m.IP); err != nil {
		return
	}
	if err = binary.Write(buf, binary.LittleEndian, m.Port); err != nil {
		return
	}
	if err = binary.Write(buf, binary.LittleEndian, uint32(len(m.Tags))); err != nil {
		return
	}
	for _, tag := range m.Tags {
		if _, err = tag.WriteTo(buf); err != nil {
			return
		}
	}

	data = buf.Bytes()
	size := len(data) - HeaderLength
	binary.LittleEndian.PutUint32(data[1:5], uint32(size)) // message size

	return
}

// Decode decodes the message from binary data.
func (m *ServerIdentMessage) Decode(data []byte) (err error) {
	header := Header{}
	err = header.Decode(data)
	if err != nil {
		return
	}
	pos := HeaderLength
	if len(data) < pos+int(m.Header.Size) ||
		len(data) < pos+29 {
		return ErrShortBuffer
	}
	if data[5] != MessageServerIdent {
		return ErrWrongMessageType
	}
	m.Header = header
	pos++

	pos += copy(m.Hash[:], data[pos:])
	m.IP = binary.LittleEndian.Uint32(data[pos : pos+4])
	pos += 4
	m.Port = binary.LittleEndian.Uint16(data[pos : pos+2])
	pos += 2
	tagCount := binary.LittleEndian.Uint32(data[pos : pos+4])
	pos += 4
	r := bytes.NewReader(data[pos:])
	for i := 0; i < int(tagCount); i++ {
		tag, err := ReadTag(r)
		if err != nil {
			return err
		}
		m.Tags = append(m.Tags, tag)
	}
	return
}

// Type is the message type
func (m ServerIdentMessage) Type() uint8 {
	return MessageServerIdent
}

func (m ServerIdentMessage) String() string {
	b := bytes.Buffer{}
	b.WriteString("[server-ident]\n")
	b.WriteString(m.Header.String())
	b.WriteString("\n")
	fmt.Fprintf(&b, "addr: %s:%d, hash: %X\n",
		net.IPv4(byte(m.IP&0xFF), byte((m.IP>>8)&0xFF), byte((m.IP>>16)&0xFF), byte((m.IP>>24)&0xFF)).String(), m.Port, m.Hash)
	for i, tag := range m.Tags {
		fmt.Fprintf(&b, "tag%d - %v: %v\n", i, tag.Name(), tag.Value())
	}
	return b.String()
}

type SearchRequestMessage struct {
	message
}
