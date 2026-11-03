package protocol

import (
	"encoding/binary"
	"time"
)

// Price 价格，单位厘（1 元 = 1000 厘）。
type Price int64

// Yuan 返回以元为单位的价格。
func (p Price) Yuan() float64 { return float64(p) / 1000 }

// FromYuan 由元构造价格。
func FromYuan(f float64) Price { return Price(f * 1000) }

// GetVarInt 从 bs 头部解析一个通达信变长整数，返回剩余字节和数值。
//
// 编码规则：首字节低 6 位有效、次高位（0x40）为符号位、最高位（0x80）为续读标志；
// 后续字节低 7 位有效，最高位为续读标志。
func GetVarInt(bs []byte) ([]byte, int64) {
	var data int64
	for i := 0; i < len(bs); i++ {
		if i == 0 {
			data += int64(bs[0] & 0x3F)
		} else {
			data += int64(bs[i]&0x7F) << uint(6+(i-1)*7)
		}
		if bs[i]&0x80 == 0 {
			if bs[0]&0x40 != 0 {
				data = -data
			}
			return bs[i+1:], data
		}
	}
	return bs[len(bs):], data
}

// GetPrice 从 bs 头部解析一个价格（厘）。
func GetPrice(bs []byte) ([]byte, Price) {
	rest, v := GetVarInt(bs)
	return rest, Price(v)
}

// Uint16 小端读取 uint16。
func Uint16(bs []byte) uint16 { return binary.LittleEndian.Uint16(bs) }

// Uint32 小端读取 uint32。
func Uint32(bs []byte) uint32 { return binary.LittleEndian.Uint32(bs) }

// PutUint16 小端写入 uint16。
func PutUint16(bs []byte, v uint16) { binary.LittleEndian.PutUint16(bs, v) }

// VarInt 把整数编码为通达信变长整数。
func VarInt(v int64) []byte {
	neg := v < 0
	if neg {
		v = -v
	}
	var out []byte
	first := byte(v & 0x3F)
	v >>= 6
	if neg {
		first |= 0x40
	}
	if v > 0 {
		first |= 0x80
	}
	out = append(out, first)
	for v > 0 {
		b := byte(v & 0x7F)
		v >>= 7
		if v > 0 {
			b |= 0x80
		}
		out = append(out, b)
	}
	return out
}

// Float32 小端读取 float32。
func Float32(bs []byte) float32 {
	return float32frombits(binary.LittleEndian.Uint32(bs))
}

// GBK 把 GBK 编码的字节解码为 UTF-8 字符串，并去除空字节。
func GBK(bs []byte) string {
	out := make([]rune, 0, len(bs))
	for i := 0; i < len(bs); {
		b := bs[i]
		if b == 0x00 {
			i++
			continue
		}
		if b < 0x80 {
			out = append(out, rune(b))
			i++
			continue
		}
		if i+1 >= len(bs) {
			break
		}
		if r, ok := gbkDecode(b, bs[i+1]); ok {
			out = append(out, r)
		} else {
			out = append(out, '\uFFFD')
		}
		i += 2
	}
	return string(out)
}

// KlineTime 解析K线时间字段。分钟级返回带时分的时间，日线及以上返回当日 15:00。
func KlineTime(bs []byte, klineType uint8) time.Time {
	switch klineType {
	case KlineMinute, KlineMinute2, Kline5Minute, Kline15Minute, Kline30Minute, Kline60Minute:
		ymd := Uint16(bs[:2])
		hm := Uint16(bs[2:4])
		year := int(ymd>>11) + 2004
		month := int(ymd%2048) / 100
		day := int((ymd % 2048) % 100)
		return time.Date(year, time.Month(month), day, int(hm/60), int(hm%60), 0, 0, time.Local)
	default:
		n := Uint32(bs[:4])
		return time.Date(int(n/10000), time.Month(int(n%10000)/100), int(n%100), 15, 0, 0, 0, time.Local)
	}
}

// HourMinute 把"当日分钟数"两字节编码转为 "HH:MM"。
func HourMinute(bs []byte) string {
	n := Uint16(bs[:2])
	return time.Date(0, 1, 1, int(n/60), int(n%60), 0, 0, time.UTC).Format("15:04")
}
