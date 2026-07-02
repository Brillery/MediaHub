// Package controller 承载 mediahub HTTP API 的业务控制器。
//
// 文件上传控制器负责把前端提交的图片资源保存到对象存储，并向 shorturl 服务申请短链。
// 输入是 Gin 请求上下文中的鉴权用户和 multipart file；输出是短链 URL。
// 本模块只维护单次请求内的校验和编排状态，不持久化文件元数据、不维护短链缓存，也不负责用户中心鉴权。
package controller

import (
	"bytes"
	"context"
	"crypto/md5"
	"enterprise-project1-mediahub/mediahub/middleware"
	"enterprise-project1-mediahub/mediahub/pkg/config"
	"enterprise-project1-mediahub/mediahub/pkg/log"
	"enterprise-project1-mediahub/mediahub/pkg/storage"
	"enterprise-project1-mediahub/mediahub/pkg/zerror"
	"enterprise-project1-mediahub/mediahub/services"
	"enterprise-project1-mediahub/mediahub/services/shorturl"
	"enterprise-project1-mediahub/mediahub/services/shorturl/proto"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"path"
)

/*
Go 的 image 标准库默认只支持解码，但不包含具体格式的实现，所以：
报错格式解析错误
如果要解析 JPEG，就要 import _ "image/jpeg"
如果要解析 PNG，就要 import _ "image/png"
如果要解析 GIF，就要 import _ "image/gif"
如果要解析 WebP，需要额外安装 golang.org/x/image/webp 并 import _ "golang.org/x/image/webp"
*/

type Controller struct {
	sf     storage.StorageFactory
	log    log.ILogger
	config *config.Config
}

const (
	// MaxUploadBytes 是单个图片文件允许的最大字节数。
	// 该限制必须在服务端执行，前端提示只能改善体验，不能作为安全边界。
	MaxUploadBytes = 20 << 20
	// maxMultipartOverheadBytes 给 multipart 边界和表单字段预留空间。
	// 请求体总限制略大于文件限制，避免合法文件因为 multipart 元数据被误杀。
	maxMultipartOverheadBytes = 1 << 20
)

func NewController(sf storage.StorageFactory, logger log.ILogger, cnf *config.Config) *Controller {
	return &Controller{
		sf:     sf,
		log:    logger,
		config: cnf,
	}
}

func (c *Controller) Upload(ctx *gin.Context) {
	// 用户 ID 只能来自鉴权中间件写入的上下文；前端表单中的同名字段不可信。
	userId := ctx.GetInt64(middleware.AuthUserIDKey)
	// 必须在任何 PostForm/FormFile 调用前设置请求体上限：
	// Gin 读取 multipart 时会触发整体解析，顺序错了会绕过服务端大小限制。
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, MaxUploadBytes+maxMultipartOverheadBytes)
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		c.log.Error(zerror.NewByErr(err))
		if isRequestBodyTooLarge(err) {
			ctx.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": "仅支持上传20M以内的图片",
			})
			return
		}
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "formFile",
		})
		return
	}
	userName := ctx.PostForm("user_name") // 自动从已解析的 form 表单获取展示用用户名。
	if fileHeader.Size > MaxUploadBytes {
		ctx.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": "仅支持上传20M以内的图片",
		})
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		c.log.Error(zerror.NewByErr(err))
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}
	defer file.Close()

	/*
			io.Reader 代表一次性可读的数据流，数据被读取后，指针会前进，已经读取过的部分不会再保留。
			Upload 方法中，你对 file 进行了两次读取
			io.ReadAll(file) 已经把 file 的内容读取完，导致 isImage(file) 里的 image.DecodeConfig(file) 读取不到数据。
			解决方案
			1。使用 bytes.NewReader(content) 复用数据
		content, _ := io.ReadAll(file) // ① 读取整个文件到内存
		reader := bytes.NewReader(content) // ② 创建新的 Reader

		if !isImage(reader) { // ③ 复用 Reader，不影响后续读取
		    return
		}

	*/

	content, err := io.ReadAll(file)
	if err != nil {
		c.log.Error(zerror.NewByErr(err))
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}

	// io.NopCloser 是 Go 标准库 io 包中的一个 适配器（adapter），它会 包装一个 io.Reader，
	// 并为它提供一个 Close 方法，但 Close 方法 实际上什么都不做
	// 接收一个 io.Reader，返回一个 io.ReadCloser
	// 生成的 ReadCloser 不会真正关闭资源，只是提供了 Close() 方法，
	//防止某些函数要求 io.ReadCloser 而 io.Reader 不能直接用的情况
	if !IsImage(io.NopCloser(bytes.NewReader(content))) {
		err = zerror.NewByMsg("仅支持jpg、png、gif格式")
		c.log.Error(err)
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "仅支持jpg、png、gif格式",
		})
		return
	}
	// bytes.NewReader(content) 生成的是 io.Reader，它没有 Close() 方法。
	// io.NopCloser(...) 将 io.Reader 包装成 io.ReadCloser，这样 isImage 如果接收 io.ReadCloser，也能正常使用。

	md5Digest := calMD5Digest(content)
	filename := fmt.Sprintf("%x%s", md5Digest, path.Ext(fileHeader.Filename))
	filePath := "/public/" + filename
	if userId != 0 {
		filePath = fmt.Sprintf("/%d/%s", userId, filename)
	}

	s := c.sf.CreateStorage()
	url, err := s.Upload(io.NopCloser(bytes.NewReader(content)), md5Digest, filePath)
	if err != nil {
		c.log.Error(zerror.NewByErr(err))
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}

	shortPool := shorturl.NewShortUrlClientPool()
	clientConn := shortPool.Get()
	defer shortPool.Put(clientConn)

	// 生成短链接
	// 现在有了pool就不用自己dial了
	//target := "localhost:50051"
	//clientConn, err := grpc.Dial(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	//if err != nil {
	//	c.log.Error(zerror.NewByErr(err))
	//	ctx.JSON(http.StatusInternalServerError, gin.H{})
	//	return
	//}
	//defer clientConn.Close()

	client := proto.NewShortUrlClient(clientConn)
	in := &proto.Url{
		Url:      url,
		UserID:   userId,
		IsPublic: userId == 0,
	}

	// 加一个拦截器认证参数
	outGoingCtx := context.Background()
	outGoingCtx = services.AppendBearerTokenToContext(outGoingCtx, c.config.DependOn.ShortUrl.AccessToken)

	outUrl, err := client.GetShortUrl(outGoingCtx, in)
	if err != nil {
		c.log.Error(zerror.NewByErr(err))
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"url":       outUrl.Url,
		"user_name": userName,
		"msg":       "上传成功",
	})
}

//func isImage(r io.Reader) bool {
//	// 第一次读取，已经把 file 的内容读取完，导致 isImage(file) 里的 image.DecodeConfig(file) 读取不到数据。
//	content, err := io.ReadAll(r)
//	if err != nil {
//		fmt.Println("ReadAll error:", err)
//		return false
//	}
//
//	_, format, err := image.DecodeConfig(bytes.NewReader(content))
//	if err != nil {
//		fmt.Println("DecodeConfig error:", err)
//		fmt.Println("File header (first 20 bytes):", content[:20]) // 打印文件头
//		return false
//	}
//
//	fmt.Println("Detected format:", format)
//
//	switch format {
//	case "jpeg", "png", "gif":
//		return true
//	default:
//		return false
//	}
//}

func IsImage(r io.Reader) bool {
	_, format, err := image.DecodeConfig(r)
	if err != nil {
		return false
	}
	switch format {
	case "jpeg", "png", "gif", "webp":
		return true
	default:
		return false
	}
}

func calMD5Digest(msg []byte) []byte {
	m := md5.New()
	m.Write(msg)
	bs := m.Sum(nil)
	return bs
}

func isRequestBodyTooLarge(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}
