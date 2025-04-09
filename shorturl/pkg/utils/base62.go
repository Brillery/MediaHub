package utils

import "strings"

// chars 定义了用于Base62编码的字符集
const chars = "cLM01lmno26789abNOPQRSdefghij45stuUVWXvwxyzABCDEFGHIJKTYZkpqr3"

// ToBase62 将一个整数转换为Base62编码的字符串
// 参数:
//
//	num: 需要转换的整数
//
// 返回值:
//
//	Base62编码后的字符串
func ToBase62(num int64) string {
	rs := ""
	for num > 0 {
		rs = string(chars[num%62]) + rs
		num /= 62
	}
	return rs
}

// ToBase10 将一个Base62编码的字符串转换回整数
// 参数:
//
//	str: 需要转换的Base62编码字符串
//
// 返回值:
//
//	Base62编码解码后的整数
func ToBase10(str string) int64 {
	var rs int64 = 0
	for _, s := range str {
		index := strings.IndexRune(chars, s)
		rs = rs*62 + int64(index)
	}
	return rs
}
