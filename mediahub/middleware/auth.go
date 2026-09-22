// Package middleware 提供 mediahub HTTP 服务的通用 Gin 中间件。
//
// Auth 中间件负责把前端携带的 SSO token 交给用户中心校验，并把可信用户信息写入
// Gin Context。业务控制器只能读取这里写入的上下文值，不能信任前端表单里的 userId。
package middleware

import (
	"context"
	"encoding/json"
	"enterprise-project1-mediahub/mediahub/pkg/config"
	"enterprise-project1-mediahub/mediahub/pkg/log"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// AuthUserIDKey 是鉴权中间件写入 Gin Context 的登录用户 ID。
	// 控制器只能读取该 key，避免信任前端表单里的 user_id。
	AuthUserIDKey = "user_id"
	// AuthUserNameKey 是用户中心返回的用户昵称，主要用于响应展示或日志。
	AuthUserNameKey = "user_name"
	// AuthUserAvatarURLKey 是用户头像地址，只作为展示字段，不参与权限判断。
	AuthUserAvatarURLKey = "avatar_url"
)

// Auth 允许无凭证的匿名上传；携带凭证时必须得到有效用户，失败不得退回公共目录。
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimPrefix(c.Request.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			c.Next()
			return
		}
		user, err := checkAuth(c.Request.Context(), token)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			log.Error(err)
			return
		}
		if user == nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Set(AuthUserIDKey, user.ID)
		c.Set(AuthUserNameKey, user.Name)
		c.Set(AuthUserAvatarURLKey, user.AvatarUrl)
		c.Next()
	}
}

// userInfo 是用户中心成功响应；ID 必须大于 0，名称和头像仅用于展示。
type userInfo struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	AvatarUrl string `json:"avatar_url"`
}

const (
	// authTimeout 限制用户中心故障时单次鉴权等待时间。
	authTimeout = 5 * time.Second
	// maxAuthResponseBytes 限制身份响应大小，防止上游异常占满服务内存。
	maxAuthResponseBytes = 64 << 10
)

var httpClient = &http.Client{Timeout: authTimeout}

// checkAuth 校验外部用户中心响应，只接受 HTTP 200 和正整数用户 ID。
// 请求继承客户端取消且最多等待 5 秒；不记录令牌、响应正文或可能包含令牌的网络错误。
// 401/403 返回无身份，其余协议或网络错误返回错误，由 Auth 拒绝请求。
func checkAuth(ctx context.Context, token string) (*userInfo, error) {
	conf := config.GetConfig()
	endpoint, err := url.Parse(strings.TrimRight(conf.DependOn.User.Address, "/") + "/api/v1/login/check/auth")
	if err != nil {
		return nil, fmt.Errorf("invalid user service address")
	}
	query := endpoint.Query()
	query.Set("access_token", token)
	endpoint.RawQuery = query.Encode()
	ctx, cancel := context.WithTimeout(ctx, authTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create authentication request failed")
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("user service request failed")
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
		return nil, nil
	}
	// 网关错误也可能是 JSON，不能仅凭可解析就把它当作登录成功。
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected user service status: %d", res.StatusCode)
	}
	if !strings.Contains(res.Header.Get("Content-Type"), "application/json") {
		return nil, fmt.Errorf("unexpected user service content type")
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxAuthResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read user service response failed")
	}
	if len(body) > maxAuthResponseBytes {
		return nil, fmt.Errorf("user service response too large")
	}
	user := &userInfo{}
	if err := json.Unmarshal(body, user); err != nil {
		return nil, fmt.Errorf("invalid user service response")
	}
	// ID 0 在上传控制器代表匿名公共资源，不能由异常鉴权响应隐式产生。
	if user.ID <= 0 {
		return nil, fmt.Errorf("invalid authenticated user identity")
	}
	return user, nil
}
