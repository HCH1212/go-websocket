package main

import (
	"github.com/gin-gonic/gin"
	"log"
)

func main() {
	r := gin.Default()
	gin.SetMode(gin.ReleaseMode)

	r.GET("/test", wsTest)

	err := r.Run()
	if err != nil {
		log.Fatalln("run error:", err)
	}
}
