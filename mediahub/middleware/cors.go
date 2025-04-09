package middleware

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func Cors() gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowAllOrigins: true,
		AllowHeaders: []string{
			"Origin", "Content-Length", "Content-Type", "Authorization",
		},
		AllowMethods: []string{
			"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS",
		},
		ExposeHeaders: []string{
			"Content-Length", "Access-Control-Allow-Origin", "Access-Control-Allow-Headers", "Cache-Control", "Content-Language", "Content-Type",
		},
		AllowCredentials: true,
	})
}

// Cors 相较于手动实现，更推荐使用跨域包github.com/gin-contrib/cors
//func Cors() gin.HandlerFunc {
//	return func(c *gin.Context) {
//		method := c.Request.Method
//		origin := c.Request.Header.Get("Origin")
//		if origin != "" {
//			c.Header("Access-Control-Allow-Origin", "*") // 可将将 * 替换为指定的域名
//			c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
//			c.Header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")
//			c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Cache-Control, Content-Language, Content-Type")
//			c.Header("Access-Control-Allow-Credentials", "true")
//		}
// if method == "OPTIONS" 语句用于检查请求的方法是否为 OPTIONS。
//如果是，则调用 c.AbortWithStatus(http.StatusNoContent)
//来终止请求并返回一个204 No Content的状态码。
// 在使用 github.com/gin-contrib/cors 包的情况下，
//这个逻辑已经被处理了，因此你不需要手动实现这个部分。
//cors.New 函数会自动处理 OPTIONS 请求并返回适当的响应。
//		if method == "OPTIONS" {
//			c.AbortWithStatus(http.StatusNoContent)
//		}
//		c.Next()
//	}
//}
