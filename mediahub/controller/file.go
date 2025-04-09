package controller

import (
	"bytes"
	"context"
	"crypto/md5"
	"enterprise-project1-mediahub/mediahub/pkg/config"
	"enterprise-project1-mediahub/mediahub/pkg/log"
	"enterprise-project1-mediahub/mediahub/pkg/storage"
	"enterprise-project1-mediahub/mediahub/pkg/zerror"
	"enterprise-project1-mediahub/mediahub/services"
	"enterprise-project1-mediahub/mediahub/services/shorturl"
	"enterprise-project1-mediahub/mediahub/services/shorturl/proto"
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

func NewController(sf storage.StorageFactory, logger log.ILogger, cnf *config.Config) *Controller {
	return &Controller{
		sf:     sf,
		log:    logger,
		config: cnf,
	}
}

func (c *Controller) Upload(ctx *gin.Context) {
	userId := ctx.GetInt64("user_id")
	userName := ctx.PostForm("user_name") // 自动从form表单获取数据
	//ctx.Request.FormValue()	FormValue 会先从查询参数（URL 中的 ?key=value）中查找键，如果找不到再从表单数据中查找。
	// 它可以处理 GET 和 POST 请求中的表单数据和查询参数。
	// 如果没有找到对应的键，返回空字符串 ""。
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		c.log.Error(zerror.NewByErr(err))
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "formFile",
		})
		return
	}
	file, err := fileHeader.Open()
	defer file.Close()
	if err != nil {
		c.log.Error(zerror.NewByErr(err))
		ctx.JSON(http.StatusInternalServerError, gin.H{})
		return
	}

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
	}
	// bytes.NewReader(content) 生成的是 io.Reader，它没有 Close() 方法。
	// io.NopCloser(...) 将 io.Reader 包装成 io.ReadCloser，这样 isImage 如果接收 io.ReadCloser，也能正常使用。

	md5Digest := calMD5Digest(content)
	fmt.Printf("%x\n", md5Digest)
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
	_, _, err := image.Decode(r)
	if err != nil {
		return false
	}
	return true
}

func calMD5Digest(msg []byte) []byte {
	m := md5.New()
	m.Write(msg)
	bs := m.Sum(nil)
	return bs
}
