package api

import "github.com/gin-gonic/gin"

// Handler defines the interface for registering API routes.
type Handler interface {
	RegisterRoutes(group *gin.RouterGroup)
}
