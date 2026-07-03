// Package middleware 提供 mediahub HTTP 服务的通用 Gin 中间件。
//
// Auth 中间件负责把前端携带的 SSO token 交给用户中心校验，并把可信用户信息写入
// Gin Context。业务控制器只能读取这里写入的上下文值，不能信任前端表单里的 userId。
package middleware

import (
	"encoding/json"
	"enterprise-project1-mediahub/mediahub/pkg/config"
	"enterprise-project1-mediahub/mediahub/pkg/log"
	"enterprise-project1-mediahub/mediahub/pkg/zerror"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strings"
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

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimPrefix(c.Request.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			c.Next()
			return
		}
		user, err := checkAuth(token)
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

type userInfo struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	AvatarUrl string `json:"avatar_url"`
}

var httpClient = &http.Client{}

func checkAuth(token string) (*userInfo, error) {
	conf := config.GetConfig()
	path := "/api/v1/login/check/auth"
	url := fmt.Sprintf("%s%s?access_token=%s", conf.DependOn.User.Address, path, token)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == 401 {
		return nil, nil
	}
	if res.StatusCode == 500 {
		err = zerror.NewByMsg("服务器内部错误")
		return nil, err
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	log.InfoF("Response body: %s", string(body))

	contentType := res.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return nil, fmt.Errorf("unexpected content type: %s", contentType)
	}

	user := &userInfo{}
	err = json.Unmarshal(body, user)
	if err != nil {
		return nil, fmt.Errorf("unmarshal error: %w, response: %s", err, string(body))
	}

	//user := &userInfo{}
	//err = json.Unmarshal(body, user)
	//if err != nil {
	//	return nil, err
	//}
	return user, nil
}
