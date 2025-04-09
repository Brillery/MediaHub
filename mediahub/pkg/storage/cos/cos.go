package cos

import (
	"context"
	"encoding/base64"
	"enterprise-project1-mediahub/mediahub/pkg/storage"
	"github.com/tencentyun/cos-go-sdk-v5"
	"io"
	"net/http"
	url1 "net/url"
	"path"
	"strings"
)

type cosStorageFactory struct {
	bucketUrl string
	secretId  string
	secretKey string
	cdnDomain string
}

func NewCosStorageFactory(bucketUrl, secretId, secretKey, cdnDomain string) storage.StorageFactory {
	return &cosStorageFactory{
		bucketUrl: bucketUrl,
		secretId:  secretId,
		secretKey: secretKey,
		cdnDomain: cdnDomain,
	}
}

func (f *cosStorageFactory) CreateStorage() storage.Storage {
	return newCos(f.bucketUrl, f.secretId, f.secretKey, f.cdnDomain)
}

type cosStorage struct {
	bucketUrl string
	secretId  string
	secretKey string
	cdnDomain string
}

func newCos(bucketUrl, secretId, secretKey, cdnDomain string) storage.Storage {
	return &cosStorage{
		bucketUrl: bucketUrl,
		secretId:  secretId,
		secretKey: secretKey,
		cdnDomain: cdnDomain,
	}
}

// // 使用 mime 包自动推断 MIME 类型
//
//	func (s *cosStorage) getContentType(dstPath string) string {
//		ext := path.Ext(dstPath)
//		if ct := mime.TypeByExtension(ext); ct != "" {
//			return ct
//		}
//		return "application/octet-stream"
//	}

func (s *cosStorage) Upload(r io.Reader, md5Digest []byte, dstPath string) (url string, err error) {
	// 存储桶名称，由 bucketname-appid 组成，appid 必须填入，可以在 COS 控制台查看存储桶名称。 https://console.cloud.tencent.com/cos5/bucket
	// 替换为用户的 region，存储桶 region 可以在 COS 控制台“存储桶概览”查看 https://console.cloud.tencent.com/ ，关于地域的详情见 https://cloud.tencent.com/document/product/436/6224 。
	u, _ := url1.Parse(s.bucketUrl)
	b := &cos.BaseURL{BucketURL: u}
	client := cos.NewClient(b, &http.Client{
		Transport: &cos.AuthorizationTransport{
			// 通过环境变量获取密钥
			// 环境变量 SECRETID 表示用户的 SecretId，登录访问管理控制台查看密钥，https://console.cloud.tencent.com/cam/capi
			SecretID: s.secretId, // 用户的 SecretId，建议使用子账号密钥，授权遵循最小权限指引，降低使用风险。子账号密钥获取可参见 https://cloud.tencent.com/document/product/598/37140
			// 环境变量 SECRETKEY 表示用户的 SecretKey，登录访问管理控制台查看密钥，https://console.cloud.tencent.com/cam/capi
			SecretKey: s.secretKey, // 用户的 SecretKey，建议使用子账号密钥，授权遵循最小权限指引，降低使用风险。子账号密钥获取可参见 https://cloud.tencent.com/document/product/598/37140
		},
	})

	opt := &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			ContentType: s.getContentType(dstPath),
		},
		ACLHeaderOptions: &cos.ACLHeaderOptions{},
	}

	if len(md5Digest) != 0 {
		opt.ObjectPutHeaderOptions.ContentMD5 = base64.StdEncoding.EncodeToString(md5Digest)
	}

	_, err = client.Object.Put(context.Background(), dstPath, r, opt)
	if err != nil {
		return "", err
	}
	url = s.bucketUrl + dstPath
	if s.cdnDomain != "" {
		url = s.cdnDomain + dstPath
	}
	return url, err
}

func (s *cosStorage) getContentType(dstPath string) string {
	ext := strings.Trim(path.Ext(dstPath), ".")
	if ext == "jpg" {
		ext = "jpeg"
	}
	return "image/" + ext
}
