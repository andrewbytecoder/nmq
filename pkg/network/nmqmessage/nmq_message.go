package nmqmessage

import (
	"encoding/binary"
	"fmt"
)

type NmqMessage struct {
	Id   string
	Data []byte
}

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
type WebSocketFrameHeader struct {
	FIN     bool    // 是否为最后一帧
	RSV1    bool    // 扩展位1
	RSV2    bool    // 扩展位2
	RSV3    bool    // 扩展位3
	OpCode  byte    // 操作码
	MASK    bool    // 是否有掩码
	Length  int64   // 负载长度
	MaskKey [4]byte // 掩码密钥(如果MASK为true)
}

// 构造WebSocket二进制消息帧
func (msg *NmqMessage) ToWebSocketBinaryFrame() ([]byte, error) {
	data := msg.Data

	// 计算帧头大小
	header := make([]byte, 2)

	// 设置FIN位为1(最后一帧)，操作码为二进制帧(0x2)
	header[0] = 0x80 | OpBinary

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

// 从WebSocket帧解析出NmqMessage
func ParseWebSocketFrame(frame []byte) (*NmqMessage, error) {
	if len(frame) < 2 {
		return nil, fmt.Errorf("frame too short")
	}

	// 解析帧头
	header := frame[0]
	maskAndLength := frame[1]

	// 解析控制位
	fin := (header & 0x80) != 0
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
	if payloadLength == 126 {
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
func NewTextWebSocketFrame(text string) ([]byte, error) {
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
func NewCloseWebSocketFrame() []byte {
	header := make([]byte, 2)
	header[0] = 0x80 | OpClose // FIN=1, OpCode=Close
	header[1] = 0x00           // Length=0, No mask

	return header
}

// 构造Ping帧
func NewPingWebSocketFrame(data []byte) []byte {
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
func NewPongWebSocketFrame(data []byte) []byte {
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
