// Package storage 定义 MediaHub 对象存储的最小适配接口。
//
// 控制器只依赖这里的 StorageFactory 和 Storage，不直接绑定 COS SDK。
// 输入是已经通过服务端校验的文件流、内容 MD5 和目标对象路径；输出是可被短链服务消费的公开 URL。
// 本包不负责鉴权、不负责图片格式校验、不负责生成短链，也不维护上传记录持久化。
package storage

import "io"

// Storage 是具体对象存储实现需要满足的上传能力。
//
// 调用方必须传入已经完成格式和大小校验的文件流；实现负责把对象写入远端存储并返回最终访问 URL。
type Storage interface {
	Upload(r io.Reader, md5Digest []byte, dstPath string) (url string, err error)
}

// StorageFactory 隔离控制器和具体存储实现。
//
// 当前生产实现是腾讯云 COS；测试可以通过该工厂注入内存 fake，避免上传校验测试触达真实云服务。
type StorageFactory interface {
	CreateStorage() Storage
}
