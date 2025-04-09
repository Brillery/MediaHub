package utils

import "regexp"

// IsUrl 检查提供的字符串是否为有效的URL
// 参数:
//
//	url: 需要验证的字符串
//
// 返回值:
//
//	bool: 如果字符串符合URL格式返回true，否则返回false
func IsUrl(url string) bool {
	// 定义匹配HTTP/HTTPS协议的有效域名及可选路径的正则表达式模式
	pattern := `^(http|https)://[a-zA-Z0-9\-\.]+\.[a-zA-Z]{2,}(?:/[^/]*)*$`
	regExp := regexp.MustCompile(pattern)
	return regExp.MatchString(url)
}
