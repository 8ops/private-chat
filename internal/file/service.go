package file

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"private-chat/internal/model"
	"private-chat/internal/repo"
	"private-chat/internal/util"
)

// 允许的图片扩展名（PRD 第 10 节）。
var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
}

// 允许的文件扩展名白名单（PRD 第 11 节）。
var fileExts = map[string]bool{
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	".ppt": true, ".pptx": true, ".txt": true, ".zip": true, ".7z": true,
	".rar": true, ".csv": true, ".md": true, ".json": true, ".yaml": true,
	".yml": true, ".log": true,
}

// 明确禁止的扩展名（PRD 第 11 节）。
var blockedExts = map[string]bool{
	".exe": true, ".dll": true, ".sh": true, ".bat": true, ".cmd": true, ".ps1": true,
}

// 危险 MIME（即便扩展名通过也拦截）。
var blockedMimes = map[string]bool{
	"application/x-msdownload":       true,
	"application/x-msdos-program":    true,
	"application/x-executable":       true,
	"application/x-dosexec":          true,
	"application/x-sh":               true,
	"text/x-shellscript":             true,
	"application/vnd.ms-powerpoint":  false, // 允许 ppt
}

// Service 封装文件上传与校验。
type Service struct {
	files     *repo.FileRepo
	uploadDir string
	maxImage  int64
	maxFile   int64
}

func NewService(files *repo.FileRepo, uploadDir string, maxImage, maxFile int64) *Service {
	return &Service{files: files, uploadDir: uploadDir, maxImage: maxImage, maxFile: maxFile}
}

// Kind 区分图片与普通文件。
type Kind int

const (
	KindImage Kind = iota
	KindFile
)

// Save 校验并保存上传文件，返回文件记录。uploaderID 记录上传者。
func (s *Service) Save(r io.Reader, originalName string, size int64, kind Kind, uploaderID string) (*model.File, error) {
	ext := strings.ToLower(filepath.Ext(originalName))
	if ext == "" {
		return nil, errors.New("missing file extension")
	}
	if blockedExts[ext] {
		return nil, fmt.Errorf("file type %s not allowed", ext)
	}

	maxSize := s.maxFile
	allowed := fileExts
	if kind == KindImage {
		maxSize = s.maxImage
		allowed = imageExts
	}
	if !allowed[ext] {
		return nil, fmt.Errorf("file type %s not allowed", ext)
	}
	if size <= 0 {
		return nil, errors.New("empty file")
	}
	if size > maxSize {
		return nil, fmt.Errorf("file too large: %d > %d", size, maxSize)
	}

	// 读取内容做 MIME 校验（读取全部用于后续写盘）。
	buf, err := io.ReadAll(io.LimitReader(r, maxSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(buf)) > maxSize {
		return nil, fmt.Errorf("file too large: > %d", maxSize)
	}
	mime := http.DetectContentType(buf)
	if blockedMimes[mime] {
		return nil, fmt.Errorf("file mime %s not allowed", mime)
	}

	storedName := util.GenID() + ext
	rel := filepath.Join(time.Now().Format("2006"), time.Now().Format("01"), time.Now().Format("02"))
	dir := filepath.Join(s.uploadDir, rel)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, storedName)
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		return nil, err
	}

	f := &model.File{
		OriginalName: originalName,
		StoredName:   storedName,
		Path:         path,
		MimeType:     mime,
		Size:         int64(len(buf)),
		UploaderID:   uploaderID,
	}
	if err := s.files.Create(f); err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return f, nil
}

// Read 读取文件内容（用于下载/预览）。
func (s *Service) Read(fileID string) (*model.File, []byte, error) {
	f, err := s.files.GetByID(fileID)
	if err != nil {
		return nil, nil, err
	}
	if f.DeletedAt != nil {
		return nil, nil, errors.New("file not found")
	}
	data, err := os.ReadFile(f.Path)
	if err != nil {
		return nil, nil, err
	}
	return f, data, nil
}
