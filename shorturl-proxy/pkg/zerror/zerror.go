package zerror

import (
	"errors"
	"fmt"
)

// ZError 是一个自定义的错误结构体，包含错误码、错误信息和多个错误的列表。
// 该结构体用于封装更复杂的错误情况，可以携带多个错误，并支持 JSON 序列化。
type ZError struct {
	ErrCode ZErrorCode `json:"err_code,omitempty"` // ErrCode 表示错误码，JSON 序列化时可选字段。	omit empty 省略空的，0值忽略
	ErrMsg  string     `json:"err_msg,omitempty"`  // ErrMsg 表示错误信息，JSON 序列化时可选字段。
	errs    []error    // errs 存储多个错误的列表，内部使用。
	// errs什么时候被初始化，
	// new新的ZError实例时
	// append时
	// json序列化反序列化时如果不为空，则自动初始化并填充
}

// Error 实现了 error 接口，返回错误信息的字符串表示。
// 如果存在 ErrMsg，则优先返回 ErrMsg；否则返回 errs 中所有错误的组合字符串。
func (e *ZError) Error() string {
	if e == nil {
		return ""
	}
	if e.ErrMsg != "" {
		return fmt.Sprintf("ErrCode:%s' ErrMsg:%s;", e.ErrCode, e.ErrMsg)
	}
	rs := ""
	if e.errs == nil || len(e.errs) == 0 {
		return rs
	}
	// e.errs == nil：表示错误列表未初始化。
	// len(e.errs) == 0：表示错误列表已初始化但没有元素。
	// 仅判断 e.errs == nil 不够全面，因为一个空的但已初始化的切片
	//（即 []error{}）也代表没有错误，但不会等于 nil。
	var first = true
	for _, err := range e.errs {
		if first {
			rs = err.Error()
			first = false
		} else {
			rs += ";" + err.Error()
		}
	}
	return rs
}

// Errors 返回 ZError 内部存储的所有错误列表。
// 如果 ZError 为 nil，则返回 nil。
func (e *ZError) Errors() []error {
	if e == nil {
		return nil
	}
	return e.errs
}

// Append 向 ZError 的错误列表中添加一个新的错误。
// 如果传入的错误是 ZError 类型，则将其内部的错误列表展开并添加到当前 ZError 中。
// 否则直接将错误添加到列表中。
func (e *ZError) Append(err error) {
	if e == nil || err == nil {
		return
	}
	var ze *ZError
	ok := errors.As(err, &ze)
	if ok {
		e.errs = append(e.errs, ze.errs...)
	} else {
		e.errs = append(e.errs, err)
	}
}

// NewByErr 根据传入的一个或多个错误创建一个新的 ZError 实例。
// 如果传入的错误中有 ZError 类型的错误，则将其内部的错误列表展开并合并到新的 ZError 中。
// 最终返回包含所有错误的 ZError 实例，如果没有错误则返回 nil。
func NewByErr(err ...error) error {
	rs := &ZError{
		errs: make([]error, 0),
	}
	for _, e := range err {
		if e == nil {
			continue
		}
		var ze *ZError
		ok := errors.As(e, &ze)
		if ok {
			rs.errs = append(rs.errs, ze.errs...)
		} else {
			rs.errs = append(rs.errs, e)
		}
	}
	if len(rs.errs) > 0 {
		return rs
	}
	return nil
}

// NewByCode 根据错误码和可选的错误信息创建一个新的 ZError 实例。
// 如果提供了错误信息，则使用提供的信息；否则根据错误码获取默认的错误信息。
// 返回包含指定错误码和错误信息的 ZError 实例。
func NewByCode(errCode ZErrorCode, errMsg ...string) error {
	msg := ""
	if len(errMsg) > 0 {
		msg = errMsg[0]
	} else {
		msg = getErrMsg(errCode)
	}
	return &ZError{
		ErrCode: errCode,
		ErrMsg:  msg,
	}
}

// NewByMsg 根据给定的消息创建一个新的 ZError 实例。
// 首先使用标准库的 errors.New 创建一个普通错误，然后通过 NewByErr 将其包装成 ZError。
// 返回包含指定消息的 ZError 实例。
func NewByMsg(msg string) error {
	err := errors.New(msg)
	return NewByErr(err)
}

// Errors 将给定的错误转换为一个错误列表。
// 如果传入的错误是 ZError 类型，则返回其内部的错误列表；
// 否则返回包含该错误的单元素列表。如果传入的错误为 nil，则返回 nil。
func Errors(err error) []error {
	if err == nil {
		return nil
	}
	var ze *ZError
	ok := errors.As(err, &ze) // *ZError已经是指针了，为什么传&ze而不是ze？
	// *ZError 是一个指针类型，指向 ZError 结构体的实例。然而，在使用 errors.As 函数时，
	// 第二个参数需要的是一个指向目标类型的指针，即使目标类型本身已经是指针类型。
	// errors.As 需要的是一个可以修改的引用。因此，传递 &ze（指向指针的指针） 可以让 errors.As 修改 ze 指向的内存位置。
	// 如果只传递 ze，那么 errors.As 只能接收到 *ZError 类型的副本，无法修改原始变量 ze 的值。
	if !ok {
		return []error{err}
	}
	return append(([]error)(nil), ze.Errors()...)
}
