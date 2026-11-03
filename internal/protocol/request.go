package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
)

// Request 一个待发送的请求：类型 + 数据域。
type Request struct {
	Type uint16
	Data []byte
}

func le16(b []byte, v uint16) []byte { return binary.LittleEndian.AppendUint16(b, v) }
func le32(b []byte, v uint32) []byte { return binary.LittleEndian.AppendUint32(b, v) }

func parseDate(date string) (uint32, error) {
	if len(date) != 8 {
		return 0, fmt.Errorf("日期格式错误: %q，应为 20060102", date)
	}
	n, err := strconv.ParseUint(date, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("日期格式错误: %q", date)
	}
	return uint32(n), nil
}

// ReqQuote 五档行情，可一次查多只。
func ReqQuote(codes []string) (*Request, error) {
	if len(codes) == 0 {
		return nil, errors.New("代码不能为空")
	}
	data := []byte{0x05, 0, 0, 0, 0, 0, 0, 0}
	data = le16(data, uint16(len(codes)))
	for _, c := range codes {
		ex, num, err := ParseCode(c)
		if err != nil {
			return nil, err
		}
		data = append(data, ex)
		data = append(data, num...)
	}
	return &Request{Type: TypeQuote, Data: data}, nil
}

// ReqCodes 证券代码列表，每次固定返回最多 1000 只。
func ReqCodes(ex uint8, start uint16) *Request {
	return &Request{Type: TypeCode, Data: []byte{ex, 0, byte(start), byte(start >> 8)}}
}

// ReqKline K线，单次 count 不得超过 800。
func ReqKline(code string, klineType uint8, start, count uint16) (*Request, error) {
	if count > 800 {
		return nil, errors.New("K线单次数量不能超过 800")
	}
	ex, num, err := ParseCode(code)
	if err != nil {
		return nil, err
	}
	data := []byte{ex, 0}
	data = append(data, num...)
	data = append(data, klineType, 0, 0x01, 0)
	data = le16(data, start)
	data = le16(data, count)
	data = append(data, make([]byte, 10)...)
	return &Request{Type: TypeKline, Data: data}, nil
}

// ReqMinute 当日分时。
func ReqMinute(code string) (*Request, error) {
	ex, num, err := ParseCode(code)
	if err != nil {
		return nil, err
	}
	data := []byte{ex, 0}
	data = append(data, num...)
	data = append(data, 0, 0, 0, 0)
	return &Request{Type: TypeMinute, Data: data}, nil
}

// ReqHistoryMinute 历史分时。date 为 20060102。
func ReqHistoryMinute(date, code string) (*Request, error) {
	d, err := parseDate(date)
	if err != nil {
		return nil, err
	}
	ex, num, err := ParseCode(code)
	if err != nil {
		return nil, err
	}
	data := le32(nil, d)
	data = append(data, ex)
	data = append(data, num...)
	return &Request{Type: TypeHistoryMinute, Data: data}, nil
}

// ReqTrade 当日分笔成交（分页）。
func ReqTrade(code string, start, count uint16) (*Request, error) {
	ex, num, err := ParseCode(code)
	if err != nil {
		return nil, err
	}
	data := []byte{ex, 0}
	data = append(data, num...)
	data = le16(data, start)
	data = le16(data, count)
	return &Request{Type: TypeMinuteTrade, Data: data}, nil
}

// ReqHistoryTrade 历史分笔成交（分页），服务器单次最多约 2000 条。
func ReqHistoryTrade(date, code string, start, count uint16) (*Request, error) {
	d, err := parseDate(date)
	if err != nil {
		return nil, err
	}
	ex, num, err := ParseCode(code)
	if err != nil {
		return nil, err
	}
	data := le32(nil, d)
	data = append(data, ex, 0)
	data = append(data, num...)
	data = le16(data, start)
	data = le16(data, count)
	return &Request{Type: TypeHistoryMinuteTrade, Data: data}, nil
}

// ReqGbbq 除权除息（股本变迁）。
func ReqGbbq(code string) (*Request, error) {
	ex, num, err := ParseCode(code)
	if err != nil {
		return nil, err
	}
	data := []byte{0x01, 0x00, ex}
	data = append(data, num...)
	return &Request{Type: TypeGbbq, Data: data}, nil
}
