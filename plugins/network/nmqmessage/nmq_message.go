package nmqmessage

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type NmqMessage struct {
	Id   string
	Data []byte
}

/*
    0                   1                   2                   3
    0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
   +-+-+-+-+-------+-+-------------+-------------------------------+
   |F|R|R|R| opcode|M| Payload len |    Extended payload length    |
   |I|S|S|S|  (4)  |A|     (7)     |             (16/64)           |
   |N|V|V|V|       |S|             |   (if payload len==126/127)   |
   | |1|2|3|       |K|             |                               |
   +-+-+-+-+-------+-+-------------+ - - - - - - - - - - - - - - - +
   |     Extended payload length continued, if payload len == 127  |
   + - - - - - - - - - - - - - - - +-------------------------------+
   |                               |Masking-key, if MASK set to 1  |
   +-------------------------------+-------------------------------+
   | Masking-key (continued)       |          Payload Data         |
   +-------------------------------- - - - - - - - - - - - - - - - +
   :                     Payload Data continued ...                :
   + - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - +
   |                     Payload Data continued ...                |
   +---------------------------------------------------------------+

*/

// WebSocket操作码
const (
	OpContinuation = 0x0
	OpText         = 0x1
	OpBinary       = 0x2
	OpClose        = 0x8
	OpPing         = 0x9
	OpPong         = 0xA
)

// WebSocket帧头结构
type NmqFrameHeader struct {
	FIN     bool    // 是否为最后一帧
	RSV1    bool    // 扩展位1
	RSV2    bool    // 扩展位2
	RSV3    bool    // 扩展位3
	OpCode  byte    // 操作码
	MASK    bool    // 是否有掩码
	Length  int64   // 负载长度
	MaskKey [4]byte // 掩码密钥(如果MASK为true)
}

// ToNmqBinaryFrame 构造Nmq二进制消息帧
// 构造Nmq二进制消息帧
func (msg *NmqMessage) ToNmqBinaryFrame() ([]byte, error) {
	data := msg.Data

	// 计算帧头大小
	header := make([]byte, 2)

	// 设置FIN位为1(最后一帧)，操作码为二进制帧(0x2)
	header[0] = 0x80 | OpBinary

	// 设置MASK位为0(服务器发送给客户端不需要掩码)
	maskBit := byte(0x00)

	// 根据数据长度设置长度字段
	/* Frame format: http://tools.ietf.org/html/rfc6455#section-5.2 */
	var payloadLength []byte
	if len(data) < 126 {
		header[1] = maskBit | byte(len(data))
	} else if len(data) <= 0xFFFF {
		/* 16-bit length field */
		header[1] = maskBit | 126
		payloadLength = make([]byte, 2)
		binary.BigEndian.PutUint16(payloadLength, uint16(len(data)))
	} else {
		header[1] = maskBit | 127
		payloadLength = make([]byte, 8)
		binary.BigEndian.PutUint64(payloadLength, uint64(len(data)))
	}

	// 组装完整帧
	var frame []byte
	frame = append(frame, header...)
	frame = append(frame, payloadLength...)
	frame = append(frame, data...)

	return frame, nil
}

const (
	MaxMessageSize = 1024 * 1024 * 4 // 4MB
	MaxStackSize   = 4096
)

type Connection struct {
	// 模拟连接对象
	Reader io.Reader
	Writer io.Writer
	// 其他字段根据实际框架实现
}

// WebSocket数据处理回调
type DataHandler func(conn *Connection, opcode int, data []byte)

// 解析并处理WebSocket帧
func ReadWebsocket(conn *Connection, handler DataHandler) error {
	buf := make([]byte, MaxStackSize)
	var totalData []byte
	var expectedLength uint64
	var mask [4]byte
	var hasMask bool
	var opcode int

	for {
		// 读取第一个字节
		if _, err := io.ReadFull(conn.Reader, buf[:1]); err != nil {
			return err
		}
		firstByte := buf[0]
		opcode = int(firstByte & 0x0F)
		isFinal := (firstByte & 0x80) != 0

		// 读取第二个字节
		if _, err := io.ReadFull(conn.Reader, buf[:1]); err != nil {
			return err
		}
		secondByte := buf[0]
		hasMask = (secondByte & 0x80) != 0
		payloadLen := secondByte & 0x7F

		var err error
		expectedLength, err = readExtendedLength(conn.Reader, payloadLen)
		if err != nil {
			return err
		}

		// 读取掩码
		if hasMask {
			if _, err := io.ReadFull(conn.Reader, mask[:]); err != nil {
				return err
			}
		}

		// 分配内存
		var data []byte
		if expectedLength <= MaxStackSize {
			data = buf[:expectedLength]
		} else {
			data = make([]byte, expectedLength)
		}

		// 读取数据
		if _, err := io.ReadFull(conn.Reader, data); err != nil {
			return err
		}

		// 解码掩码
		if hasMask {
			for i := 0; i < len(data); i++ {
				data[i] ^= mask[i%4]
			}
		}

		// 处理 PING/PONG
		switch opcode {
		case OpPing:
			_ = WriteWebsocket(conn, OpPong, data)
			continue
		case OpPong:
			continue
		}

		// 聚合分片消息（简单处理）
		if opcode != OpContinuation {
			totalData = totalData[:0] // 新消息开始
		}
		totalData = append(totalData, data...)

		// 如果是最后一个帧，调用回调
		if isFinal {
			handler(conn, opcode, totalData)
			if opcode == OpClose {
				return errors.New("websocket closed")
			}
		}
	}
}

// 读取扩展长度字段
func readExtendedLength(r io.Reader, payloadLen byte) (uint64, error) {
	switch payloadLen {
	case 126:
		buf := make([]byte, 2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return 0, err
		}
		return uint64(binary.BigEndian.Uint16(buf)), nil
	case 127:
		buf := make([]byte, 8)
		if _, err := io.ReadFull(r, buf); err != nil {
			return 0, err
		}
		return binary.BigEndian.Uint64(buf), nil
	default:
		return uint64(payloadLen), nil
	}
}

// 写入WebSocket帧（用于PONG等）
func WriteWebsocket(conn *Connection, opcode int, data []byte) error {
	header := make([]byte, 10)
	header[0] = byte(opcode) | 0x80 // FIN=1
	n := 2
	payloadLen := len(data)

	if payloadLen < 126 {
		header[1] = byte(payloadLen)
	} else if payloadLen <= 0xFFFF {
		header[1] = 126
		binary.BigEndian.PutUint16(header[2:], uint16(payloadLen))
		n += 2
	} else {
		header[1] = 127
		binary.BigEndian.PutUint64(header[2:], uint64(payloadLen))
		n += 8
	}

	_, err := conn.Writer.Write(header[:n])
	if err != nil {
		return err
	}

	_, err = conn.Writer.Write(data)
	return err
}

// ParseNmqFrame 解析Nmq帧
// 从Nmq帧解析出NmqMessage
func ParseNmqFrame(frame []byte) (*NmqMessage, error) {
	if len(frame) < 2 {
		return nil, fmt.Errorf("frame too short")
	}

	// 解析帧头
	header := frame[0]
	maskAndLength := frame[1]

	// 解析控制位
	//fin := (header & 0x80) != 0
	opcode := header & 0x0F

	// 检查是否为支持的操作码
	if opcode != OpText && opcode != OpBinary {
		return nil, fmt.Errorf("unsupported opcode: %d", opcode)
	}

	// 解析掩码和长度
	mask := (maskAndLength & 0x80) != 0
	payloadLength := int64(maskAndLength & 0x7F)

	// 计算负载开始位置
	payloadStart := 2

	// 根据长度字段确定真实长度
	/* Frame format: http://tools.ietf.org/html/rfc6455#section-5.2 */
	if payloadLength == 126 {
		/* inline 7-bit length field */
		if len(frame) < 4 {
			return nil, fmt.Errorf("frame too short for extended payload length")
		}
		payloadLength = int64(binary.BigEndian.Uint16(frame[2:4]))
		payloadStart = 4
	} else if payloadLength == 127 {
		if len(frame) < 10 {
			return nil, fmt.Errorf("frame too short for extended payload length")
		}
		payloadLength = int64(binary.BigEndian.Uint64(frame[2:10]))
		payloadStart = 10
	}

	// 处理掩码密钥
	var maskKey [4]byte
	if mask {
		if len(frame) < payloadStart+4 {
			return nil, fmt.Errorf("frame too short for mask key")
		}
		copy(maskKey[:], frame[payloadStart:payloadStart+4])
		payloadStart += 4
	}

	// 验证帧长度
	if int64(len(frame)) < int64(payloadStart)+payloadLength {
		return nil, fmt.Errorf("frame too short for payload")
	}

	// 提取负载数据
	payload := frame[payloadStart : payloadStart+int(payloadLength)]

	// 如果有掩码，则进行解码
	if mask {
		for i := 0; i < len(payload); i++ {
			payload[i] ^= maskKey[i%4]
		}
	}

	// 构造NmqMessage
	// 注意：这里我们把整个payload作为Data，Id字段需要在应用层处理
	message := &NmqMessage{
		Id:   "", // Id需要在应用层协议中定义和解析
		Data: payload,
	}

	return message, nil
}

// 构造一个简单的文本消息帧(用于心跳等)
func NewTextNmqFrame(text string) ([]byte, error) {
	data := []byte(text)

	// 计算帧头大小
	header := make([]byte, 2)

	// 设置FIN位为1(最后一帧)，操作码为文本帧(0x1)
	header[0] = 0x80 | OpText

	// 设置MASK位为0(服务器发送给客户端不需要掩码)
	maskBit := byte(0x00)

	// 根据数据长度设置长度字段
	var payloadLength []byte
	if len(data) < 126 {
		header[1] = maskBit | byte(len(data))
	} else if len(data) <= 65535 {
		header[1] = maskBit | 126
		payloadLength = make([]byte, 2)
		binary.BigEndian.PutUint16(payloadLength, uint16(len(data)))
	} else {
		header[1] = maskBit | 127
		payloadLength = make([]byte, 8)
		binary.BigEndian.PutUint64(payloadLength, uint64(len(data)))
	}

	// 组装完整帧
	var frame []byte
	frame = append(frame, header...)
	frame = append(frame, payloadLength...)
	frame = append(frame, data...)

	return frame, nil
}

// 构造关闭帧
func NewCloseNmqFrame() []byte {
	header := make([]byte, 2)
	header[0] = 0x80 | OpClose // FIN=1, OpCode=Close
	header[1] = 0x00           // Length=0, No mask

	return header
}

// 构造Ping帧
func NewPingNmqFrame(data []byte) []byte {
	header := make([]byte, 2)
	header[0] = 0x80 | OpPing // FIN=1, OpCode=Ping

	var payloadLength []byte
	if len(data) < 126 {
		header[1] = byte(len(data))
	} else if len(data) <= 65535 {
		header[1] = 126
		payloadLength = make([]byte, 2)
		binary.BigEndian.PutUint16(payloadLength, uint16(len(data)))
	} else {
		header[1] = 127
		payloadLength = make([]byte, 8)
		binary.BigEndian.PutUint64(payloadLength, uint64(len(data)))
	}

	var frame []byte
	frame = append(frame, header...)
	frame = append(frame, payloadLength...)
	frame = append(frame, data...)

	return frame
}

// 构造Pong帧
func NewPongNmqFrame(data []byte) []byte {
	header := make([]byte, 2)
	header[0] = 0x80 | OpPong // FIN=1, OpCode=Pong

	var payloadLength []byte
	if len(data) < 126 {
		header[1] = byte(len(data))
	} else if len(data) <= 65535 {
		header[1] = 126
		payloadLength = make([]byte, 2)
		binary.BigEndian.PutUint16(payloadLength, uint16(len(data)))
	} else {
		header[1] = 127
		payloadLength = make([]byte, 8)
		binary.BigEndian.PutUint64(payloadLength, uint64(len(data)))
	}

	var frame []byte
	frame = append(frame, header...)
	frame = append(frame, payloadLength...)
	frame = append(frame, data...)

	return frame
}
