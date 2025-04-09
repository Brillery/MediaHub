package zerror

// ZErrorCode 定义了一个错误码的类型
type ZErrorCode string

// getErrMsg 根据错误码获取错误消息
// 参数:
//
//	errCode - 错误码
//
// 返回值:
//
//	string - 错误消息，如果错误码不存在则返回空字符串
func getErrMsg(errCode ZErrorCode) string {
	// 尝试从errorMsgs中获取与errCode相关联的错误消息
	msg, ok := errorMsgs[errCode]
	// 如果找到了相关联的错误消息，则返回该消息
	if ok {
		return msg
	}
	// 如果没有找到相关联的错误消息，则返回空字符串
	return ""
}

// errorMsgs 是一个错误码与错误消息的映射
// 这个映射用于存储和管理所有的错误码和它们对应的错误消息
var errorMsgs = map[ZErrorCode]string{}
