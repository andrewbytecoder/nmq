package api

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type fileViewMetaResponse struct {
	Root string `json:"root"`
}

type fileViewListResponse struct {
	Storages []string `json:"storages"`
	Dirname  string   `json:"dirname"`
	Files    []gin.H  `json:"files"`
	ReadOnly bool     `json:"read_only"`
}

type fileViewOperationResponse = fileViewListResponse

type fileViewDeleteRequest struct {
	Path  string `json:"path"`
	Items []struct {
		Path string `json:"path"`
		Type string `json:"type"`
	} `json:"items"`
}

type fileViewRenameRequest struct {
	Path string `json:"path"`
	Item string `json:"item"`
	Name string `json:"name"`
}

type fileViewCreateRequest struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

type fileViewSaveRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

const fileViewStorage = "workdir"

func (s *Server) handleFileViewMeta(c *gin.Context) {
	c.JSON(http.StatusOK, fileViewMetaResponse{
		Root: s.fileViewRoot(),
	})
}

func (s *Server) handleFileViewList(c *gin.Context) {
	relativePath := strings.TrimSpace(c.Query("path"))
	response, err := s.fileViewListResponse(relativePath)
	if err != nil {
		s.respondFileViewError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func (s *Server) handleFileViewDelete(c *gin.Context) {
	var request fileViewDeleteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		s.respondFileViewError(c, http.StatusBadRequest, err)
		return
	}

	for _, item := range request.Items {
		targetPath, _, err := s.resolveFileViewPath(item.Path)
		if err != nil {
			s.respondFileViewError(c, http.StatusBadRequest, err)
			return
		}

		if err := os.RemoveAll(targetPath); err != nil {
			s.respondFileViewError(c, http.StatusInternalServerError, err)
			return
		}
	}

	response, err := s.fileViewListResponse(request.Path)
	if err != nil {
		s.respondFileViewError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, fileViewOperationResponse(response))
}

func (s *Server) handleFileViewRename(c *gin.Context) {
	var request fileViewRenameRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		s.respondFileViewError(c, http.StatusBadRequest, err)
		return
	}

	if strings.TrimSpace(request.Name) == "" {
		s.respondFileViewError(c, http.StatusBadRequest, errors.New("new name is required"))
		return
	}

	sourcePath, _, err := s.resolveFileViewPath(request.Item)
	if err != nil {
		s.respondFileViewError(c, http.StatusBadRequest, err)
		return
	}

	parentPath := filepath.Dir(sourcePath)
	destinationPath := filepath.Join(parentPath, request.Name)
	destinationPath = filepath.Clean(destinationPath)
	if err := s.ensureFileViewWithinRoot(destinationPath); err != nil {
		s.respondFileViewError(c, http.StatusBadRequest, err)
		return
	}

	if err := os.Rename(sourcePath, destinationPath); err != nil {
		s.respondFileViewError(c, http.StatusInternalServerError, err)
		return
	}

	requestedDir := request.Path
	if strings.TrimSpace(requestedDir) == "" {
		requestedDir = s.parentVirtualPath(request.Item)
	}
	response, err := s.fileViewListResponse(requestedDir)
	if err != nil {
		s.respondFileViewError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, fileViewOperationResponse(response))
}

func (s *Server) handleFileViewCreateFile(c *gin.Context) {
	var request fileViewCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		s.respondFileViewError(c, http.StatusBadRequest, err)
		return
	}

	parentPath, _, err := s.resolveFileViewPath(request.Path)
	if err != nil {
		s.respondFileViewError(c, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(request.Name) == "" {
		s.respondFileViewError(c, http.StatusBadRequest, errors.New("file name is required"))
		return
	}

	targetPath := filepath.Join(parentPath, request.Name)
	targetPath = filepath.Clean(targetPath)
	if err := s.ensureFileViewWithinRoot(targetPath); err != nil {
		s.respondFileViewError(c, http.StatusBadRequest, err)
		return
	}

	file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		s.respondFileViewError(c, http.StatusInternalServerError, err)
		return
	}
	_ = file.Close()

	response, err := s.fileViewListResponse(request.Path)
	if err != nil {
		s.respondFileViewError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, fileViewOperationResponse(response))
}

func (s *Server) handleFileViewCreateFolder(c *gin.Context) {
	var request fileViewCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		s.respondFileViewError(c, http.StatusBadRequest, err)
		return
	}

	parentPath, _, err := s.resolveFileViewPath(request.Path)
	if err != nil {
		s.respondFileViewError(c, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(request.Name) == "" {
		s.respondFileViewError(c, http.StatusBadRequest, errors.New("folder name is required"))
		return
	}

	targetPath := filepath.Join(parentPath, request.Name)
	targetPath = filepath.Clean(targetPath)
	if err := s.ensureFileViewWithinRoot(targetPath); err != nil {
		s.respondFileViewError(c, http.StatusBadRequest, err)
		return
	}

	if err := os.Mkdir(targetPath, 0o755); err != nil {
		s.respondFileViewError(c, http.StatusInternalServerError, err)
		return
	}

	response, err := s.fileViewListResponse(request.Path)
	if err != nil {
		s.respondFileViewError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, fileViewOperationResponse(response))
}

func (s *Server) handleFileViewPreview(c *gin.Context) {
	relativePath := strings.TrimSpace(c.Query("path"))
	absPath, _, err := s.resolveFileViewPath(relativePath)
	if err != nil {
		s.respondFileViewError(c, http.StatusBadRequest, err)
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		s.respondFileViewError(c, http.StatusNotFound, err)
		return
	}
	if info.IsDir() {
		s.respondFileViewError(c, http.StatusBadRequest, errors.New("cannot preview a directory"))
		return
	}

	file, err := os.Open(absPath)
	if err != nil {
		s.respondFileViewError(c, http.StatusInternalServerError, err)
		return
	}
	defer file.Close()

	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(absPath)))
	if contentType == "" {
		buffer := make([]byte, 512)
		readSize, readErr := file.Read(buffer)
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			s.respondFileViewError(c, http.StatusInternalServerError, readErr)
			return
		}
		contentType = http.DetectContentType(buffer[:readSize])
		if _, seekErr := file.Seek(0, io.SeekStart); seekErr != nil {
			s.respondFileViewError(c, http.StatusInternalServerError, seekErr)
			return
		}
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", "inline; filename=\""+filepath.Base(absPath)+"\"")
	http.ServeContent(c.Writer, c.Request, filepath.Base(absPath), info.ModTime(), file)
}

func (s *Server) handleFileViewUpload(c *gin.Context) {
	relativePath := strings.TrimSpace(c.PostForm("path"))
	targetDir, cleanPath, err := s.resolveFileViewPath(relativePath)
	if err != nil {
		s.respondFileViewError(c, http.StatusBadRequest, err)
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		s.respondFileViewError(c, http.StatusBadRequest, err)
		return
	}

	uploadedFiles := form.File["file"]
	if len(uploadedFiles) == 0 {
		s.respondFileViewError(c, http.StatusBadRequest, errors.New("no file uploaded"))
		return
	}

	results := make([]gin.H, 0, len(uploadedFiles))
	for _, header := range uploadedFiles {
		safeName := filepath.Base(header.Filename)
		targetPath := filepath.Join(targetDir, safeName)
		targetPath = filepath.Clean(targetPath)
		if err := s.ensureFileViewWithinRoot(targetPath); err != nil {
			s.respondFileViewError(c, http.StatusBadRequest, err)
			return
		}

		if err := c.SaveUploadedFile(header, targetPath); err != nil {
			s.respondFileViewError(c, http.StatusInternalServerError, err)
			return
		}

		item, buildErr := s.buildFileViewItem(cleanPath, targetPath)
		if buildErr == nil {
			results = append(results, item)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"uploaded": results,
	})
}

func (s *Server) handleFileViewDownload(c *gin.Context) {
	relativePath := strings.TrimSpace(c.Query("path"))
	absPath, _, err := s.resolveFileViewPath(relativePath)
	if err != nil {
		s.respondFileViewError(c, http.StatusBadRequest, err)
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		s.respondFileViewError(c, http.StatusNotFound, err)
		return
	}
	if info.IsDir() {
		s.respondFileViewError(c, http.StatusBadRequest, errors.New("cannot download a directory"))
		return
	}

	c.FileAttachment(absPath, filepath.Base(absPath))
}

func (s *Server) handleFileViewSearch(c *gin.Context) {
	filter := strings.TrimSpace(c.Query("filter"))
	if filter == "" {
		c.JSON(http.StatusOK, gin.H{"files": []gin.H{}})
		return
	}

	relativePath := strings.TrimSpace(c.Query("path"))
	absPath, cleanPath, err := s.resolveFileViewPath(relativePath)
	if err != nil {
		s.respondFileViewError(c, http.StatusBadRequest, err)
		return
	}

	matches := make([]gin.H, 0)
	err = filepath.WalkDir(absPath, func(currentPath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if currentPath == absPath {
			return nil
		}

		name := strings.ToLower(entry.Name())
		if !strings.Contains(name, strings.ToLower(filter)) {
			return nil
		}

		item, buildErr := s.buildFileViewItem(cleanPath, currentPath)
		if buildErr != nil {
			return nil
		}
		matches = append(matches, item)
		return nil
	})
	if err != nil {
		s.respondFileViewError(c, http.StatusInternalServerError, err)
		return
	}

	sort.SliceStable(matches, func(i, j int) bool {
		return strings.ToLower(mockString(matches[i], "basename")) < strings.ToLower(mockString(matches[j], "basename"))
	})

	c.JSON(http.StatusOK, gin.H{
		"path":  cleanPath,
		"files": matches,
	})
}

func (s *Server) handleFileViewSave(c *gin.Context) {
	var request fileViewSaveRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		s.respondFileViewError(c, http.StatusBadRequest, err)
		return
	}

	targetPath, _, err := s.resolveFileViewPath(request.Path)
	if err != nil {
		s.respondFileViewError(c, http.StatusBadRequest, err)
		return
	}

	if err := os.WriteFile(targetPath, []byte(request.Content), 0o644); err != nil {
		s.respondFileViewError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, targetPath)
}

func (s *Server) fileViewRoot() string {
	workdir, err := os.Getwd()
	if err != nil {
		return "."
	}

	return filepath.Clean(workdir)
}

func (s *Server) fileViewListResponse(relativePath string) (fileViewListResponse, error) {
	absPath, cleanPath, err := s.resolveFileViewPath(relativePath)
	if err != nil {
		return fileViewListResponse{}, err
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return fileViewListResponse{}, err
	}

	files := make([]gin.H, 0, len(entries))
	for _, entry := range entries {
		item, buildErr := s.buildFileViewEntry(cleanPath, absPath, entry)
		if buildErr != nil {
			s.logger.Warn(
				"skip unreadable fileview entry",
				zap.String("path", entry.Name()),
				zap.Error(buildErr),
			)
			continue
		}
		files = append(files, item)
	}

	sort.SliceStable(files, func(i, j int) bool {
		leftType := files[i]["type"]
		rightType := files[j]["type"]
		if leftType != rightType {
			return leftType == "dir"
		}
		return strings.ToLower(mockString(files[i], "basename")) < strings.ToLower(mockString(files[j], "basename"))
	})

	return fileViewListResponse{
		Storages: []string{fileViewStorage},
		Dirname:  cleanPath,
		Files:    files,
		ReadOnly: false,
	}, nil
}

func (s *Server) resolveFileViewPath(relativePath string) (string, string, error) {
	root := s.fileViewRoot()
	cleanRelative, err := s.normalizeFileViewVirtualPath(relativePath)
	if err != nil {
		return "", "", err
	}

	joinedPath := filepath.Join(root, filepath.FromSlash(cleanRelative))
	resolvedPath := filepath.Clean(joinedPath)
	if err := s.ensureFileViewWithinRoot(resolvedPath); err != nil {
		return "", "", err
	}

	relativeToRoot, err := filepath.Rel(root, resolvedPath)
	if err != nil {
		return "", "", err
	}

	cleanPath := s.toFileViewVirtualPath(relativeToRoot)

	return resolvedPath, cleanPath, nil
}

func (s *Server) normalizeFileViewVirtualPath(value string) (string, error) {
	cleanValue := strings.TrimSpace(value)
	if cleanValue == "" || cleanValue == "/" || cleanValue == "." || cleanValue == fileViewStorage+"://" {
		return "", nil
	}

	if strings.HasPrefix(cleanValue, fileViewStorage+"://") {
		cleanValue = strings.TrimPrefix(cleanValue, fileViewStorage+"://")
	} else if strings.Contains(cleanValue, "://") {
		return "", fmt.Errorf("unsupported storage path %q", cleanValue)
	}

	cleanValue = strings.TrimPrefix(cleanValue, "/")
	cleanValue = strings.TrimPrefix(cleanValue, `\`)
	return cleanValue, nil
}

func (s *Server) toFileViewVirtualPath(relativePath string) string {
	cleanPath := filepath.ToSlash(relativePath)
	if cleanPath == "." || cleanPath == "" {
		return fileViewStorage + "://"
	}

	return fileViewStorage + "://" + cleanPath
}

func (s *Server) parentVirtualPath(path string) string {
	cleanValue, err := s.normalizeFileViewVirtualPath(path)
	if err != nil || cleanValue == "" {
		return fileViewStorage + "://"
	}

	parent := filepath.Dir(filepath.FromSlash(cleanValue))
	return s.toFileViewVirtualPath(parent)
}

func (s *Server) ensureFileViewWithinRoot(targetPath string) error {
	root := s.fileViewRoot()
	relativeToRoot, err := filepath.Rel(root, targetPath)
	if err != nil {
		return err
	}
	if relativeToRoot == ".." || strings.HasPrefix(relativeToRoot, ".."+string(filepath.Separator)) {
		return errors.New("path escapes workdir")
	}

	return nil
}

func (s *Server) buildFileViewEntry(relativeDir string, absoluteDir string, entry os.DirEntry) (gin.H, error) {
	return s.buildFileViewItem(relativeDir, filepath.Join(absoluteDir, entry.Name()))
}

func (s *Server) buildFileViewItem(relativeDir string, absolutePath string) (gin.H, error) {
	info, err := os.Stat(absolutePath)
	if err != nil {
		return nil, err
	}

	dirVirtualPath := relativeDir
	if dirVirtualPath == "" || dirVirtualPath == "." {
		dirVirtualPath = fileViewStorage + "://"
	}

	baseName := filepath.Base(absolutePath)
	entryPath := baseName
	if dirVirtualPath != fileViewStorage+"://" {
		entryPath = strings.TrimPrefix(dirVirtualPath, fileViewStorage+"://") + "/" + baseName
	}
	virtualPath := s.toFileViewVirtualPath(entryPath)

	entryType := "file"
	var fileSize any = info.Size()
	var mimeType any = mime.TypeByExtension(strings.ToLower(filepath.Ext(baseName)))
	if info.IsDir() {
		entryType = "dir"
		fileSize = nil
		mimeType = nil
	}

	return gin.H{
		"dir":           dirVirtualPath,
		"basename":      baseName,
		"extension":     strings.TrimPrefix(strings.ToLower(filepath.Ext(baseName)), "."),
		"path":          virtualPath,
		"storage":       fileViewStorage,
		"type":          entryType,
		"file_size":     fileSize,
		"last_modified": info.ModTime().UnixMilli(),
		"mime_type":     mimeType,
		"read_only":     false,
		"visibility":    "private",
	}, nil
}

func (s *Server) respondFileViewError(c *gin.Context, status int, err error) {
	s.logger.Warn(
		"fileview request failed",
		zap.Int("status", status),
		zap.Error(err),
	)
	c.JSON(status, gin.H{
		"message": err.Error(),
	})
}
