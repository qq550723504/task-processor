package listingadmin

import "github.com/gin-gonic/gin"

func installTestOwnerMiddleware(engine *gin.Engine) {
	engine.Use(func(c *gin.Context) {
		if c.GetHeader("X-User-ID") == "" {
			c.Request.Header.Set("X-User-ID", "test-owner")
		}
		c.Next()
	})
}
