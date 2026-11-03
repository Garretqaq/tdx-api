// Package protocol 实现通达信行情服务器的二进制协议编解码，不依赖任何第三方库。
//
// 帧格式（请求）：
//
//	0x0c | MsgID(4,小端) | Control(1) | Length(2,小端) | Length(2,小端) | Type(2,小端) | Data
//
// 帧格式（响应）：
//
//	0xB1CB7400 | Control(1) | MsgID(4) | Unknown(1) | Type(2) | ZipLength(2) | Length(2) | Data
//
// 数据域在 ZipLength != Length 时用 zlib 压缩。
package protocol

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	// PrefixReq 请求帧头。
	PrefixReq = 0x0C
	// PrefixResp 响应帧头。
	PrefixResp = 0xB1CB7400

	// Control01 请求控制码，目前观察到的取值。
	Control01 = 0x01
)

// 请求类型。
const (
	TypeConnect            uint16 = 0x000D // 建立连接
	TypeHeart              uint16 = 0x0004 // 心跳
	TypeGbbq               uint16 = 0x000F // 除权除息
	TypeCount              uint16 = 0x044E // 证券数量
	TypeCode               uint16 = 0x0450 // 证券代码
	TypeQuote              uint16 = 0x053E // 五档行情
	TypeMinute             uint16 = 0x051D // 当日分时
	TypeHistoryMinute      uint16 = 0x0FB4 // 历史分时
	TypeMinuteTrade        uint16 = 0x0FC5 // 当日分笔成交
	TypeHistoryMinuteTrade uint16 = 0x0FB5 // 历史分笔成交
	TypeKline              uint16 = 0x052D // K线
)

// K线周期（协议内的 Type 字段）。
const (
	Kline5Minute  uint8 = 0 // 5分钟
	Kline15Minute uint8 = 1 // 15分钟
	Kline30Minute uint8 = 2 // 30分钟
	Kline60Minute uint8 = 3 // 60分钟
	KlineDay2     uint8 = 4 // 日线（价格需/100，未被使用）
	KlineWeek     uint8 = 5 // 周线
	KlineMonth    uint8 = 6 // 月线
	KlineMinute   uint8 = 7 // 1分钟
	KlineMinute2  uint8 = 8 // 1分钟（与7未知差异）
	KlineDay      uint8 = 9 // 日线
)

// 交易所市场码（协议内的 Exchange 字段）。
const (
	ExchangeSZ uint8 = 0 // 深圳
	ExchangeSH uint8 = 1 // 上海
	ExchangeBJ uint8 = 2 // 北京
)

// ExchangeName 返回市场缩写。
func ExchangeName(ex uint8) string {
	switch ex {
	case ExchangeSZ:
		return "sz"
	case ExchangeSH:
		return "sh"
	case ExchangeBJ:
		return "bj"
	default:
		return "unknown"
	}
}

// Frame 请求帧。
type Frame struct {
	MsgID   uint32
	Control uint8
	Type    uint16
	Data    []byte
}

// Bytes 序列化为字节流。
func (f *Frame) Bytes() []byte {
	length := uint16(len(f.Data) + 2)
	out := make([]byte, 12+len(f.Data))
	out[0] = PrefixReq
	binary.LittleEndian.PutUint32(out[1:5], f.MsgID)
	out[5] = f.Control
	binary.LittleEndian.PutUint16(out[6:8], length)
	binary.LittleEndian.PutUint16(out[8:10], length)
	binary.LittleEndian.PutUint16(out[10:12], f.Type)
	copy(out[12:], f.Data)
	return out
}

// Response 响应帧。
type Response struct {
	Control   uint8
	MsgID     uint32
	Type      uint16
	ZipLength uint16
	Length    uint16
	Data      []byte
}

// DecodeResponse 解析一个完整响应帧（含 4 字节帧头）。
func DecodeResponse(bs []byte) (*Response, error) {
	if len(bs) < 16 {
		return nil, errors.New("响应长度不足 16 字节")
	}
	if binary.BigEndian.Uint32(bs[:4]) != PrefixResp {
		return nil, fmt.Errorf("响应帧头错误: %x", bs[:4])
	}
	r := &Response{
		Control:   bs[4],
		MsgID:     binary.LittleEndian.Uint32(bs[5:9]),
		Type:      binary.LittleEndian.Uint16(bs[10:12]),
		ZipLength: binary.LittleEndian.Uint16(bs[12:14]),
		Length:    binary.LittleEndian.Uint16(bs[14:16]),
		Data:      bs[16:],
	}
	// Control 码含义尚未完全明确（0x0c 和 0x1c 均属正常响应），此处不做错误判断。
	if int(r.ZipLength) != len(r.Data) {
		return nil, fmt.Errorf("数据长度不匹配: 声明 %d, 实际 %d", r.ZipLength, len(r.Data))
	}
	if r.ZipLength != r.Length {
		zr, err := zlib.NewReader(bytes.NewReader(r.Data))
		if err != nil {
			return nil, fmt.Errorf("解压失败: %w", err)
		}
		defer zr.Close()
		if r.Data, err = io.ReadAll(zr); err != nil {
			return nil, fmt.Errorf("读取解压数据失败: %w", err)
		}
	}
	if int(r.Length) != len(r.Data) {
		return nil, fmt.Errorf("解压后长度不匹配: 声明 %d, 实际 %d", r.Length, len(r.Data))
	}
	return r, nil
}

// ReadResponse 从连接中读出一个完整响应帧（含 4 字节帧头）。
func ReadResponse(r io.Reader) ([]byte, error) {
	head := make([]byte, 4)
	for {
		if _, err := io.ReadFull(r, head); err != nil {
			return nil, err
		}
		if binary.BigEndian.Uint32(head) == PrefixResp {
			break
		}
		// 帧头未对齐，逐字节滑动继续找
		copy(head, head[1:])
		if _, err := io.ReadFull(r, head[3:]); err != nil {
			return nil, err
		}
	}
	buf := make([]byte, 12)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	n := int(binary.LittleEndian.Uint16(buf[8:10]))
	if n <= 0 {
		return nil, errors.New("响应数据长度为 0")
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return append(append(head, buf...), body...), nil
}
