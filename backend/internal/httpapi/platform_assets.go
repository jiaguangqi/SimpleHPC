package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"simplehpc/backend/internal/service"
)

func (api *API) uploadPlatformAsset(c *gin.Context) {
	if _, ok := api.requireAdmin(c); !ok {
		return
	}
	kind := c.Param("kind")
	maxSize := int64(8 * 1024 * 1024)
	if kind == "logo" {
		maxSize = 2 * 1024 * 1024
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize+1024*1024)
	header, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择图片文件"})
		return
	}
	if header.Size > maxSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "图片文件过大"})
		return
	}
	file, err := header.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	config, format, err := image.DecodeConfig(file)
	file.Close()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无法识别图片格式"})
		return
	}
	if err := service.ValidatePlatformImage(kind, header.Size, config.Width, config.Height, format); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	extension := ".png"
	if format == "jpeg" {
		extension = ".jpg"
	}
	random := make([]byte, 12)
	if _, err := rand.Read(random); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	directory := filepath.Join(api.cfg.FrontendDir, "uploads", "platform")
	if err := os.MkdirAll(directory, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	name := kind + "-" + hex.EncodeToString(random) + extension
	input, err := header.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer input.Close()
	output, err := os.OpenFile(filepath.Join(directory, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_, copyErr := io.Copy(output, io.LimitReader(input, maxSize+1))
	closeErr := output.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(filepath.Join(directory, name))
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprint(copyErr)})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"url": "/uploads/platform/" + name, "width": config.Width, "height": config.Height, "size": header.Size})
}
