// Package controller 承载 mediahub HTTP API 的业务控制器。
//
// 文件上传控制器负责把前端提交的图片资源保存到对象存储，并向 shorturl 服务申请短链。
// 输入是 Gin 请求上下文中的鉴权用户和 multipart file；输出是短链 URL。
// 本模块只维护单次请求内的校验和编排状态，不持久化文件元数据、不维护短链缓存，也不负责用户中心鉴权。
package controller

import (
	"context"
	"crypto/md5"
	"enterprise-project1-mediahub/mediahub/middleware"
	"enterprise-project1-mediahub/mediahub/pkg/config"
	"enterprise-project1-mediahub/mediahub/pkg/log"
	"enterprise-project1-mediahub/mediahub/pkg/storage"
	"enterprise-project1-mediahub/mediahub/pkg/zerror"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
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
	// shortener 是上传成功后的短链生成边界。
	// 生产环境使用 gRPC 实现，测试注入 fake，避免成功上传路径必须依赖外部 shorturl 服务。
	shortener ShortURLGenerator
	// uploadTempDir 是上传临时文件目录。
	// 默认走系统临时目录；测试会注入独立目录，验证临时文件不会在成功或失败路径残留。
	uploadTempDir string
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
	// maxUploadTextFieldBytes 是上传表单中文本字段的最大读取长度。
	// 当前只需要展示用 user_name，限制它可以避免恶意请求把非文件字段塞满内存。
	maxUploadTextFieldBytes = 4 << 10
)

var (
	// errUploadFileMissing 表示 multipart 表单里没有名为 file 的文件字段。
	// 这是客户端请求格式错误，不应该进入对象存储或短链生成阶段。
	errUploadFileMissing = errors.New("upload file field missing")
	// errUploadFileTooLarge 表示文件字段超过服务端允许的单文件大小。
	// 这里不依赖 fileHeader.Size，因为流式解析时请求头和 multipart 元数据都不能作为安全边界。
	errUploadFileTooLarge = errors.New("upload file exceeds max bytes")
	// errUploadTextFieldTooLarge 表示上传表单里的文本字段超过展示字段限制。
	// 当前 user_name 只用于回显，超限直接拒绝比继续读入内存更稳妥。
	errUploadTextFieldTooLarge = errors.New("upload text field exceeds max bytes")
	// errUploadMalformedMultipart 表示 multipart 边界、part 读取或字段内容异常。
	// 这类错误属于客户端请求不可解析，和磁盘写入失败这类服务端错误分开处理。
	errUploadMalformedMultipart = errors.New("upload multipart malformed")
)

func NewController(sf storage.StorageFactory, logger log.ILogger, cnf *config.Config) *Controller {
	return NewControllerWithShortener(sf, logger, cnf, newGRPCShortURLGenerator(cnf))
}

// NewControllerWithShortener 创建可注入短链生成器的上传控制器。
//
// 该构造函数主要服务单元测试和未来替换 shorturl 适配器；生产入口继续使用 NewController，保持路由初始化不变。
func NewControllerWithShortener(sf storage.StorageFactory, logger log.ILogger, cnf *config.Config, shortener ShortURLGenerator) *Controller {
	return &Controller{
		sf:        sf,
		log:       logger,
		config:    cnf,
		shortener: shortener,
	}
}

func (c *Controller) Upload(ctx *gin.Context) {
	// 用户 ID 只能来自鉴权中间件写入的上下文；前端表单中的同名字段不可信。
	userId := ctx.GetInt64(middleware.AuthUserIDKey)
	// 必须在读取 multipart 前设置请求体上限：
	// 后续走 MultipartReader 流式解析，避免 Gin 的 FormFile/PostForm 先把文件整体解析进内存或临时文件。
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, MaxUploadBytes+maxMultipartOverheadBytes)
	multipartReader, err := ctx.Request.MultipartReader()
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

	userName, tempFile, err := c.receiveUploadMultipart(multipartReader)
	if err != nil {
		if tempFile != nil {
			c.cleanupUploadTempFile(tempFile)
		}
		c.log.Error(zerror.NewByErr(err))
		if errors.Is(err, errUploadFileTooLarge) || isRequestBodyTooLarge(err) {
			ctx.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": "仅支持上传20M以内的图片",
			})
			return
		}
		if errors.Is(err, errUploadFileMissing) ||
			errors.Is(err, errUploadTextFieldTooLarge) ||
			errors.Is(err, errUploadMalformedMultipart) {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"error": "formFile",
			})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}
	defer c.cleanupUploadTempFile(tempFile)

	if tempFile == nil {
		c.log.Error(zerror.NewByErr(errUploadFileMissing))
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "formFile",
		})
		return
	}

	// 临时文件会被格式识别、MD5 计算和对象存储上传连续读取；
	// 每一阶段前都必须回到文件开头，否则下游会从上一阶段的 EOF 继续读。
	if err = rewindUploadTempFile(tempFile); err != nil {
		c.log.Error(zerror.NewByErr(err))
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}
	imageMeta, ok := detectUploadImage(tempFile)
	if !ok {
		err = zerror.NewByMsg("仅支持jpg、png、gif、webp格式")
		c.log.Error(err)
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "仅支持jpg、png、gif、webp格式",
		})
		return
	}

	if err = rewindUploadTempFile(tempFile); err != nil {
		c.log.Error(zerror.NewByErr(err))
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}
	md5Digest, err := calMD5DigestFromReader(tempFile)
	if err != nil {
		c.log.Error(zerror.NewByErr(err))
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}
	filePath := buildUploadFilePath(userId, md5Digest, imageMeta)

	s := c.sf.CreateStorage()
	if err = rewindUploadTempFile(tempFile); err != nil {
		c.log.Error(zerror.NewByErr(err))
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}
	url, err := s.Upload(tempFile, md5Digest, filePath)
	if err != nil {
		c.log.Error(zerror.NewByErr(err))
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}

	// 对象存储成功后再生成短链；短链失败时不能返回对象存储原始 URL，避免前端拿到未纳入短链系统的地址。
	outUrl, err := c.shortener.Generate(context.Background(), url, userId, userId == 0)
	if err != nil {
		c.log.Error(zerror.NewByErr(err))
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"url":       outUrl,
		"user_name": userName,
		"msg":       "上传成功",
	})
}

// receiveUploadMultipart 以流式方式读取上传表单，并把第一个 file 字段写入临时文件。
//
// 本函数只负责接收请求体中的原始输入：user_name 是展示字段，file 是唯一会进入后续图片校验的内容。
// 它不负责图片格式识别、不负责生成对象路径、不负责短链；错误会保留客户端输入错误和服务端 I/O 错误的边界。
func (c *Controller) receiveUploadMultipart(reader *multipart.Reader) (string, *os.File, error) {
	var userName string
	var tempFile *os.File

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return userName, tempFile, fmt.Errorf("%w: %w", errUploadMalformedMultipart, err)
		}

		switch part.FormName() {
		case "user_name":
			value, readErr := readUploadTextField(part)
			if readErr != nil {
				return userName, tempFile, readErr
			}
			if closeErr := closeUploadPart(part); closeErr != nil {
				return userName, tempFile, closeErr
			}
			userName = value
		case "file":
			if tempFile != nil {
				// 一个请求只允许一个文件进入上传链路；重复 file 字段直接丢弃，避免一次请求写多个对象。
				if closeErr := closeUploadPart(part); closeErr != nil {
					return userName, tempFile, closeErr
				}
				continue
			}
			created, createErr := c.createUploadTempFile()
			if createErr != nil {
				_ = part.Close()
				return userName, tempFile, createErr
			}
			tempFile = created
			copied, copyErr := copyUploadFileToTemp(tempFile, part)
			if copyErr != nil {
				_ = part.Close()
				return userName, tempFile, copyErr
			}
			if copied > MaxUploadBytes {
				return userName, tempFile, errUploadFileTooLarge
			}
			if closeErr := closeUploadPart(part); closeErr != nil {
				return userName, tempFile, closeErr
			}
		default:
			// 未识别字段不参与业务；关闭 part 会丢弃剩余内容，让解析器可以继续读取后续字段。
			if closeErr := closeUploadPart(part); closeErr != nil {
				return userName, tempFile, closeErr
			}
		}
	}

	if tempFile == nil {
		return userName, nil, errUploadFileMissing
	}
	return userName, tempFile, nil
}

// readUploadTextField 读取上传表单里的小文本字段。
//
// 当前字段只用于回显，不能让它和文件内容一样占用大块内存；
// LimitReader 多读 1 字节用于明确区分“刚好到边界”和“已经越界”。
func readUploadTextField(part *multipart.Part) (string, error) {
	content, err := io.ReadAll(io.LimitReader(part, maxUploadTextFieldBytes+1))
	if err != nil {
		return "", fmt.Errorf("%w: %w", errUploadMalformedMultipart, err)
	}
	if int64(len(content)) > maxUploadTextFieldBytes {
		return "", errUploadTextFieldTooLarge
	}
	return string(content), nil
}

// closeUploadPart 关闭并丢弃当前 multipart part 的剩余内容。
//
// Close 会继续读取到当前 part 结束；如果这里触发 MaxBytesReader 的上限错误，
// 必须向上返回，否则超大非 file 字段可能被误判成普通缺少文件。
func closeUploadPart(part *multipart.Part) error {
	if err := part.Close(); err != nil {
		return fmt.Errorf("%w: %w", errUploadMalformedMultipart, err)
	}
	return nil
}

// createUploadTempFile 创建单次上传请求使用的临时文件。
//
// 上传链路最多允许 20MB 图片，但并发上传时把文件整体读入内存仍会放大 RSS；
// 因此这里先落盘，再让格式识别、MD5 和存储上传复用同一份受控输入。
func (c *Controller) createUploadTempFile() (*os.File, error) {
	tempDir := c.uploadTempDir
	if tempDir == "" {
		tempDir = os.TempDir()
	}
	return os.CreateTemp(tempDir, "mediahub-upload-*")
}

// cleanupUploadTempFile 关闭并删除单次上传临时文件。
//
// 删除动作放在请求 defer 中统一收敛：无论图片校验失败、短链失败还是上传成功，
// 本服务都不持久化上传源文件，避免本地磁盘被异常请求逐步占满。
func (c *Controller) cleanupUploadTempFile(file *os.File) {
	if file == nil {
		return
	}
	if err := file.Close(); err != nil {
		c.log.Error(zerror.NewByErr(err))
	}
	if err := os.Remove(file.Name()); err != nil && !errors.Is(err, os.ErrNotExist) {
		c.log.Error(zerror.NewByErr(err))
	}
}

// copyUploadFileToTemp 把 multipart 文件复制到临时文件，并额外多读 1 字节识别越界输入。
//
// 流式 part 没有可信的服务端文件大小元数据；这里以 MaxUploadBytes+1 作为硬边界，
// 保证异常 multipart 或未来调用路径也不能把超限文件完整写入磁盘。
func copyUploadFileToTemp(dst *os.File, src io.Reader) (int64, error) {
	return io.Copy(dst, io.LimitReader(src, MaxUploadBytes+1))
}

// rewindUploadTempFile 把上传临时文件重置到开头。
//
// 同一文件句柄会被多个阶段顺序消费，seek 失败时必须中断请求，
// 否则可能用空内容计算 MD5 或向对象存储上传空文件。
func rewindUploadTempFile(file *os.File) error {
	_, err := file.Seek(0, io.SeekStart)
	return err
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

// calMD5DigestFromReader 基于流式读取计算上传内容 MD5。
//
// 控制器上传路径使用该函数避免再构造整文件内存副本；调用方需要在调用前自行把文件 seek 到开头。
func calMD5DigestFromReader(r io.Reader) ([]byte, error) {
	m := md5.New()
	if _, err := io.Copy(m, r); err != nil {
		return nil, err
	}
	return m.Sum(nil), nil
}

func isRequestBodyTooLarge(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}
