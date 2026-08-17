package server

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"private-chat/internal/auth"
	"private-chat/internal/file"
	"private-chat/internal/logger"
)

// handleUpload 处理图片/文件上传。
func (app *App) handleUpload(c *gin.Context) {
	user := auth.GetUser(c)
	kind := file.KindFile
	if c.PostForm("kind") == "image" {
		kind = file.KindImage
	}
	maxSize := app.cfg.Storage.MaxFileSize
	if kind == file.KindImage {
		maxSize = app.cfg.Storage.MaxImageSize
	}

	if err := c.Request.ParseMultipartForm(maxSize + (1 << 20)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 10008, "message": "file too large", "data": nil})
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 10007, "message": "no file field", "data": nil})
		return
	}
	src, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 10014, "message": "internal error", "data": nil})
		return
	}
	defer src.Close()

	f, err := app.fileSvc.Save(src, fh.Filename, fh.Size, kind, user.ID)
	if err != nil {
		logger.Warn("upload rejected", map[string]interface{}{"error": err.Error(), "user": user.Username})
		c.JSON(http.StatusBadRequest, gin.H{"code": 10009, "message": err.Error(), "data": nil})
		return
	}
	logger.Info("file uploaded", map[string]interface{}{"id": f.ID, "name": f.OriginalName, "user": user.Username})
	v := f.ToView()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data": gin.H{
			"id":            v.ID,
			"original_name": v.OriginalName,
			"mime_type":     v.MimeType,
			"size":          v.Size,
			"url":           v.URL,
			"download_url":  v.DownloadURL,
		},
	})
}

// handleServeFile 以内联方式返回文件（图片预览）。
func (app *App) handleServeFile(c *gin.Context) {
	id := c.Param("id")
	f, data, err := app.fileSvc.Read(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 10005, "message": "file not found", "data": nil})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=%q", sanitizeName(f.OriginalName)))
	c.Data(http.StatusOK, f.MimeType, data)
}

// handleDownloadFile 以附件方式返回文件（下载）。
func (app *App) handleDownloadFile(c *gin.Context) {
	id := c.Param("id")
	f, data, err := app.fileSvc.Read(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 10005, "message": "file not found", "data": nil})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", sanitizeName(f.OriginalName)))
	c.Data(http.StatusOK, f.MimeType, data)
}

// sanitizeName 去除文件名中的控制字符与路径分隔符，防止 header 注入。
func sanitizeName(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		switch r {
		case '/', '\\', '\n', '\r', '\t', '\x00':
			continue
		default:
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return "download"
	}
	return string(out)
}
