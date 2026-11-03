package protocol

import (
	"math"
	"testing"
)

func TestVarIntRoundTrip(t *testing.T) {
	cases := []int64{0, 1, 63, 64, 100, 8191, 8192, 12345, 1000000, -1, -63, -12345}
	for _, v := range cases {
		enc := VarInt(v)
		rest, got := GetVarInt(append(enc, 0xFF))
		if got != v {
			t.Errorf("VarInt(%d) 往返得到 %d，编码 %x", v, got, enc)
		}
		if len(rest) != 1 || rest[0] != 0xFF {
			t.Errorf("VarInt(%d) 未正确消费字节，剩余 %x", v, rest)
		}
	}
}

func TestGetVarIntKnownBytes(t *testing.T) {
	// 0x05 单字节，值 5
	if _, v := GetVarInt([]byte{0x05}); v != 5 {
		t.Errorf("期望 5，得到 %d", v)
	}
	// 0x8C 0x01 → 低 6 位 0x0C=12，续读；次字节 0x01<<6 = 64 → 76
	if _, v := GetVarInt([]byte{0x8C, 0x01}); v != 76 {
		t.Errorf("期望 76，得到 %d", v)
	}
	// 0x45 符号位 0x40 置位 → -5
	if _, v := GetVarInt([]byte{0x45}); v != -5 {
		t.Errorf("期望 -5，得到 %d", v)
	}
}

func TestParseCode(t *testing.T) {
	cases := []struct {
		in   string
		ex   uint8
		num  string
		fail bool
	}{
		{in: "600000", ex: ExchangeSH, num: "600000"},
		{in: "sh600000", ex: ExchangeSH, num: "600000"},
		{in: "SH600000", ex: ExchangeSH, num: "600000"},
		{in: "000001", ex: ExchangeSZ, num: "000001"},
		{in: "sz000001", ex: ExchangeSZ, num: "000001"},
		{in: "510300", ex: ExchangeSH, num: "510300"},
		{in: "159919", ex: ExchangeSZ, num: "159919"},
		{in: "sh000001", ex: ExchangeSH, num: "000001"},
		{in: "399001", ex: ExchangeSZ, num: "399001"},
		{in: "920001", ex: ExchangeBJ, num: "920001"},
		{in: "abc", fail: true},
		{in: "xx600000", fail: true},
	}
	for _, c := range cases {
		ex, num, err := ParseCode(c.in)
		if c.fail {
			if err == nil {
				t.Errorf("ParseCode(%q) 应报错", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseCode(%q) 意外报错: %v", c.in, err)
			continue
		}
		if ex != c.ex || num != c.num {
			t.Errorf("ParseCode(%q) = (%d,%q)，期望 (%d,%q)", c.in, ex, num, c.ex, c.num)
		}
	}
}

func TestKind(t *testing.T) {
	cases := map[string]string{
		"sh600000": "stock",
		"sz000001": "stock",
		"sz300750": "stock",
		"sh510300": "etf",
		"sz159919": "etf",
		"sh000001": "index",
		"sz399001": "index",
		"sh880741": "index",
		"sh999999": "index",
		"sh110059": "",
	}
	for code, want := range cases {
		if got := Kind(code); got != want {
			t.Errorf("Kind(%q) = %q，期望 %q", code, got, want)
		}
	}
}

func TestDecodeXRXDs(t *testing.T) {
	// 构造一个含 1 条 Cat=1 记录的响应：前缀 2 字节 + 7 字节 + Count(2)
	// 数据域布局：[9:11] = Count，其余为记录头
	buf := make([]byte, 0, 64)
	buf = append(buf, 0, 0)                // 未知
	buf = append(buf, 1, 0, 0, 0, 0, 0, 0) // 未知 7 字节
	buf = append(buf, 1, 0)                // Count = 1

	// 记录头 13 字节：Exchange(1) + Code(6) + 未知(1) + Date(4) + Category(1)
	buf = append(buf, ExchangeSH)
	buf = append(buf, []byte("600000")...)
	buf = append(buf, 0)
	buf = append(buf, 0x6C, 0x27, 0x35, 0x01) // 20260716，小端
	buf = append(buf, 1)                      // Category = 1 除权除息

	// 4 个 float32：fenhong=4.20, peigujia=0, song=0, peigu=0
	putF := func(v float32) {
		bits := float32bits(v)
		buf = append(buf, byte(bits), byte(bits>>8), byte(bits>>16), byte(bits>>24))
	}
	putF(4.20)
	putF(0)
	putF(0)
	putF(0)

	xs, err := DecodeXRXDs(buf)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	if len(xs) != 1 {
		t.Fatalf("期望 1 条，得到 %d", len(xs))
	}
	x := xs[0]
	if x.Code != "sh600000" {
		t.Errorf("代码 = %q", x.Code)
	}
	if x.Time.Format("2006-01-02") != "2026-07-16" {
		t.Errorf("日期 = %s", x.Time.Format("2006-01-02"))
	}
	if diff := x.Fenhong - 4.20; diff > 1e-5 || diff < -1e-5 {
		t.Errorf("每股派现 = %v", x.Fenhong)
	}
}

func TestDecodeCodes(t *testing.T) {
	buf := make([]byte, 0, 32)
	buf = append(buf, 1, 0) // Count = 1
	rec := make([]byte, 29)
	copy(rec[0:6], "600000")
	rec[6] = 100
	rec[7] = 0
	copy(rec[8:16], []byte("浦发银行")) // 非 GBK，仅验证不 panic
	rec[20] = 2
	buf = append(buf, rec...)

	cs, err := DecodeCodes(buf)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	if len(cs) != 1 || cs[0].Code != "600000" || cs[0].Decimal != 2 {
		t.Fatalf("解码结果不符: %+v", cs)
	}
}

func TestDecodeKlinesIndexExtraField(t *testing.T) {
	// 两根日K：指数多 4 字节，股票没有；用同一份数据分别解码应都能成功
	build := func() []byte {
		buf := make([]byte, 0, 64)
		buf = append(buf, 1, 0)                   // Count = 1
		buf = append(buf, 0xEF, 0x28, 0x35, 0x01) // 2026-11-03，YYYYMMDD 小端
		buf = append(buf, VarInt(10)...)          // open delta
		buf = append(buf, VarInt(1)...)           // close delta
		buf = append(buf, VarInt(2)...)           // high
		buf = append(buf, VarInt(-1)...)          // low
		vol := float32bits(1000)
		buf = append(buf, byte(vol), byte(vol>>8), byte(vol>>16), byte(vol>>24))
		amt := float32bits(20000)
		buf = append(buf, byte(amt), byte(amt>>8), byte(amt>>16), byte(amt>>24))
		buf = append(buf, 0, 0, 0, 0) // 指数额外的涨跌家数
		return buf
	}

	ks, err := DecodeKlines(build(), KlineDay, true, false)
	if err != nil {
		t.Fatalf("指数日K解码失败: %v", err)
	}
	if len(ks) != 1 {
		t.Fatalf("期望 1 根，得到 %d", len(ks))
	}
	if ks[0].Time.IsZero() {
		t.Fatal("时间不应为零值")
	}
}

func float32bits(f float32) uint32 { return math.Float32bits(f) }
