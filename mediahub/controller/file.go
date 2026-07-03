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

// uploadImageMetadata 是上传链路识别出的图片格式元数据。
//
// Format 来自 image.DecodeConfig 对文件内容的识别结果，CanonicalExt 用于生成对象存储路径。
// 这里不能信任用户上传文件名后缀，否则 JPEG 内容可以伪装成 .txt/.svg 并污染 CDN 元数据。
type uploadImageMetadata struct {
	Format       string
	CanonicalExt string
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

	content, err := io.ReadAll(file)
	if err != nil {
		c.log.Error(zerror.NewByErr(err))
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}

	// multipart 文件流只能顺序读一次；先读入受限大小的内存，再用新的 reader 分别做格式识别和对象上传。
	imageMeta, ok := detectUploadImage(bytes.NewReader(content))
	if !ok {
		err = zerror.NewByMsg("仅支持jpg、png、gif、webp格式")
		c.log.Error(err)
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "仅支持jpg、png、gif、webp格式",
		})
		return
	}

	md5Digest := calMD5Digest(content)
	filePath := buildUploadFilePath(userId, md5Digest, imageMeta)

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

// detectUploadImage 只根据文件内容识别支持的图片格式。
//
// 用户上传的 filename 和 Content-Type 都可以伪造，所以上传路径和存储元数据必须以 DecodeConfig 的结果为准。
func detectUploadImage(r io.Reader) (uploadImageMetadata, bool) {
	_, format, err := image.DecodeConfig(r)
	if err != nil {
		return uploadImageMetadata{}, false
	}
	switch format {
	case "jpeg":
		return uploadImageMetadata{Format: format, CanonicalExt: ".jpg"}, true
	case "png", "gif", "webp":
		return uploadImageMetadata{Format: format, CanonicalExt: "." + format}, true
	default:
		return uploadImageMetadata{}, false
	}
}

// IsImage 保留给旧测试和潜在内部调用使用。
//
// 新上传链路应优先使用 detectUploadImage，避免只拿到 true/false 后又回退到不可信文件名后缀。
func IsImage(r io.Reader) bool {
	_, ok := detectUploadImage(r)
	return ok
}

// buildUploadFilePath 根据鉴权用户和图片真实格式生成对象存储路径。
//
// userID 为 0 代表公共资源，写入 /public；非 0 用户写入自己的用户目录。
// 路径中的扩展名必须来自服务端内容识别结果，不能来自用户提交的文件名。
func buildUploadFilePath(userID int64, md5Digest []byte, imageMeta uploadImageMetadata) string {
	filename := fmt.Sprintf("%x%s", md5Digest, imageMeta.CanonicalExt)
	if userID != 0 {
		return fmt.Sprintf("/%d/%s", userID, filename)
	}
	return "/public/" + filename
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
