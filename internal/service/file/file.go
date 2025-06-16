package file

import (
	"encoding/base64"
	"fmt"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/internal/service"
	"net/http"
	"os"
)

const (
	KeyReadPath   = "readPath"
	KeySavePath   = "savePath"
	KeyFileName   = "filename"
	KeyHeaders    = "headers"
	KeyBody       = "body"
	KeyBodyBase64 = "body_base64"
)
const ServiceName = "file"

type fileService struct {
	log *logger.Logger
}

type Service interface {
	service.Interface
}

type fileResult struct {
	code    int
	headers http.Header
	body    []byte
	err     error
}

func (r *fileResult) Code() int {
	return r.code
}

func (r *fileResult) Headers() *http.Header {
	return &r.headers
}

func (r *fileResult) Body() []byte {
	return r.body
}

func (r *fileResult) Error() error {
	return r.err
}

func (r *fileResult) MapData() (mapBody map[string]interface{}, err error) {
	return nil, nil
}

func (r *fileResult) Apply(data *map[string]interface{}) error {
	return nil
}

func (r *fileResult) Set(data *map[string]interface{}) {
}

func (r *fileResult) Tasks() []common.Task {
	return []common.Task{}
}

func New(log *logger.Logger) Service {
	return &fileService{
		log: log.WithField(logger.ServiceKey, ServiceName),
	}
}

func (s *fileService) Do(task common.Task) service.Result {
	serviceResult := &fileResult{
		code:    200,
		headers: http.Header{},
	}

	taskData, err := task.MapData()
	data := taskData
	if err != nil {
		serviceResult.err = err
		return serviceResult
	}

	// Save file before read
	if savePath, ok := data[KeySavePath]; ok {
		var fileBody []byte
		if fileData, ok := data[KeyBody]; ok {
			switch fileData.(type) {
			case string:
				fileBody = []byte(fileData.(string))
			case []uint8:
				fileBody = fileData.([]uint8)
			}

		} else if fileData, ok := data[KeyBodyBase64]; ok {
			fileDataDecoded, err := base64.StdEncoding.DecodeString(fileData.(string))
			if err != nil {
				serviceResult.code = http.StatusInternalServerError
				serviceResult.err = err
				return serviceResult
			}
			fileBody = fileDataDecoded
		}

		// Try to write file
		if errFile := os.WriteFile(savePath.(string), fileBody, 0644); err != nil {
			serviceResult.code = http.StatusInternalServerError
			serviceResult.err = errFile
			return serviceResult
		}
	}

	// Read file
	if readPath, ok := data[KeyReadPath]; ok {
		fileData, errFile := os.Open(readPath.(string))
		if errFile != nil {
			serviceResult.code = http.StatusInternalServerError
			serviceResult.err = errFile
			return serviceResult
		}
		defer fileData.Close()

		fileInfo, _ := fileData.Stat()
		fileSize := fileInfo.Size()

		body := make([]byte, fileSize)
		_, errFile = fileData.Read(body)
		if errFile != nil {
			serviceResult.code = http.StatusInternalServerError
			serviceResult.err = errFile
			return serviceResult
		}
		fileType := http.DetectContentType(body)

		serviceResult.headers.Add("Content-Length", fmt.Sprintf("%d", fileSize))
		serviceResult.headers.Add("Content-Type", fileType)
		serviceResult.headers.Add("Content-Control", "private, no-transform, no-store, must-revalidate")
		serviceResult.headers.Add("Expires", "0")

		if filename, ok := data[KeyFileName]; ok {
			serviceResult.headers.Add("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		}

		if headers, ok := data[KeyHeaders]; ok {
			if headersMap, mapOk := headers.(map[string]interface{}); mapOk {
				for key, value := range headersMap {
					serviceResult.headers.Add(key, fmt.Sprintf("%s", value))
				}
			}
		}

		serviceResult.body = body
	}
	return serviceResult
}
