package middleware

import (
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
)

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log the stack trace
				gin.DefaultErrorWriter.Write([]byte(
					"[PANIC] " + time.Now().Format("2006/01/02 - 15:04:05") +
						" | " + c.Request.Method +
						" | " + c.Request.URL.Path +
						" | " + c.ClientIP() +
						" | " + c.GetString("RequestID") +
						" | " + err.(string) + "\n" +
						string(debug.Stack()) + "\n",
				))

				// Return 500 error
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Internal server error",
				})
			}
		}()

		c.Next()
	}
}