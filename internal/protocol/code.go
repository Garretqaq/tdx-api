package protocol

import (
	"fmt"
	"strings"
)

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// AddPrefix 给裸 6 位代码补市场前缀，无法识别时原样返回。
// 注意 000001 按深市股票处理（sz000001）；上证指数请显式使用 sh000001。
func AddPrefix(code string) string {
	if len(code) != 6 || !isDigits(code) {
		return code
	}
	switch {
	case code[:1] == "6":
		return "sh" + code
	case code[:1] == "0" || code[:2] == "30":
		return "sz" + code
	case code[:2] == "92":
		return "bj" + code
	case isSHETF(code):
		return "sh" + code
	case code[:2] == "15" || code[:2] == "16":
		return "sz" + code
	case code == "999999":
		return "sh" + code
	case code[:3] == "399":
		return "sz" + code
	case code[:3] == "899":
		return "bj" + code
	case code[:3] == "880" || code[:3] == "881":
		return "sh" + code
	}
	return code
}

func isSHETF(num string) bool {
	switch num[:2] {
	case "50", "51", "52", "53", "56", "58":
		return true
	}
	return false
}

// IsETFCode 判断带前缀代码是否为 ETF。
func IsETFCode(full string) bool { return Kind(full) == "etf" }

// ParseCode 解析 "sh600000" / "SZ000001" / "600000"，返回市场码和 6 位代码。
func ParseCode(s string) (uint8, string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) == 6 && isDigits(s) {
		full := AddPrefix(s)
		if full == s {
			return 0, "", fmt.Errorf("无法识别的代码: %q", s)
		}
		s = full
	}
	if len(s) != 8 || !isDigits(s[2:]) {
		return 0, "", fmt.Errorf("代码格式错误: %q，应为 600000 或 sh600000", s)
	}
	switch s[:2] {
	case "sh":
		return ExchangeSH, s[2:], nil
	case "sz":
		return ExchangeSZ, s[2:], nil
	case "bj":
		return ExchangeBJ, s[2:], nil
	}
	return 0, "", fmt.Errorf("不支持的市场前缀: %q", s[:2])
}

// FullCode 规整为带市场前缀的小写代码，如 sh600000。
func FullCode(s string) (string, error) {
	ex, num, err := ParseCode(s)
	if err != nil {
		return "", err
	}
	return ExchangeName(ex) + num, nil
}

// Kind 判断带前缀代码的类型：stock / etf / index，其他返回空串。
func Kind(full string) string {
	if len(full) != 8 || !isDigits(full[2:]) {
		return ""
	}
	ex, num := full[:2], full[2:]
	switch ex {
	case "sh":
		switch {
		case num[:1] == "6":
			return "stock"
		case isSHETF(num):
			return "etf"
		case num[:3] == "000" || num == "999999" || num[:3] == "880" || num[:3] == "881":
			return "index"
		}
	case "sz":
		switch {
		case num[:1] == "0" || num[:2] == "30":
			return "stock"
		case num[:2] == "15" || num[:2] == "16":
			return "etf"
		case num[:3] == "399":
			return "index"
		}
	case "bj":
		switch {
		case num[:2] == "92":
			return "stock"
		case num[:3] == "899":
			return "index"
		}
	}
	return ""
}
