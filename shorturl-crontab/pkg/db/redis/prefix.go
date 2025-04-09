package redis

// 引入strings包以使用其Join函数，用于拼接字符串切片。
import "strings"

// ServicePrefix 是存储服务内部使用的前缀。
// 用于区分不同服务或应用的键值对。
const ServicePrefix = "shorturl_"

// GetKey 生成带有服务前缀的键。
// 主要用于统一键的格式，以便在Redis中存储和查询。
// 参数:
//
//	key - 基础键名，用于构成最终键的核心部分。
//	parts - 可变参数，用于构成键的附加部分，增加键的维度。
//
// 返回值:
//
//	返回最终生成的键字符串。
func GetKey(key string, parts ...string) string {
	// 给基础键添加服务前缀。
	key = ServicePrefix + key
	// 如果没有附加部分，则直接返回带有前缀的键。
	if len(parts) == 0 {
		return key
	}
	// 如果有附加部分，将它们用下划线连接起来，并附加到键上。
	key += "_" + strings.Join(parts, "_")
	// 返回最终构成的键。
	return key
}
