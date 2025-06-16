package handler

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/internal/service/metrics"
	"github.com/unibackend/uniproxy/utils"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	requestFieldKey     = "requestField"
	bodyKeyUploadedFile = "upload"
	filesStorageKey     = "files"
	sourceCaseFile      = "file"
	sourceCaseJSON      = "json"
	sourceCaseURL       = "url"

	destinationCaseFolder  = "folder"
	destinationCaseService = "service"
	destinationCaseFTP     = "ftp"

	paramDestination  = "destination"
	paramSource       = "source"
	paramFolder       = "folder"
	paramFileMode     = "fileMode"
	paramNameTemplate = "nameTemplate"
	paramMimExt       = "mimext"

	uploadMimeTypeKey     = "mimetype"
	uploadUserFileNameKey = "user_filename"
	uploadExtensionKey    = "extension"
	uploadSizeKey         = "size"
	uploadFilePathKey     = "filepath"
	uploadFileNameKey     = "filename"
)

/*
	Upload file to specified folder
	Run tasks with inject in first task information about uploaded file
*/

func (s *HandleService) UploadHandler(request *http.Request, w http.ResponseWriter) common.HandlerResponse {
	response := common.NewResponse()

	route := request.Context().Value(common.RouteKey).(*config.Route)

	log := request.Context().Value(common.LoggerKey).(*logger.Logger)
	if _, ok := route.Data[requestFieldKey]; !ok {
		log.Error(fmt.Sprintf("route data '%s' is absent", requestFieldKey)) // Log for internal use
		response.SetStatus(http.StatusInternalServerError)
		response.SetError(fmt.Errorf("upload file error"))
		return response
	}

	var Buf bytes.Buffer
	var extension, mimeType, userFileName string

	upload := make(map[string]interface{})

	requestBody := *(request.Context().Value(common.RequestBodyKey).(*map[string]interface{}))

	switch route.Data[paramSource] {
	case sourceCaseFile:
		file, headers, err := request.FormFile(route.Data[requestFieldKey].(string))
		if err != nil {
			response.SetStatus(http.StatusInternalServerError)
			response.SetError(fmt.Errorf("error requestField %s: %v", route.Data[requestFieldKey].(string), err))
			return response
		}
		userFileName = headers.Filename
		mimeType = headers.Header.Get("Content-Type")

		defer func() {
			err := file.Close()
			if err != nil {
				log.Errorf("Error close file handler %v", err)
			}
		}()

		_, errCopy := io.Copy(&Buf, file)
		if errCopy != nil {
			log.Error(fmt.Sprintf("error copy to buffer %v", errCopy)) // Log for internal use
			response.SetStatus(http.StatusInternalServerError)
			response.SetError(fmt.Errorf("upload file error"))
			return response
		}

	case sourceCaseJSON:
		base64DecodedData, errBase64 := base64.StdEncoding.DecodeString(requestBody[route.Data[requestFieldKey].(string)].(string))
		if errBase64 != nil {
			log.Error(fmt.Sprintf("incorrect base64 data: %v", errBase64)) // Log for internal use
			response.SetStatus(http.StatusBadRequest)
			response.SetError(fmt.Errorf("incorrect base64 data"))
			return response
		}
		_, errCopy := io.Copy(&Buf, strings.NewReader(string(base64DecodedData)))
		if errCopy != nil {
			log.Error(fmt.Sprintf("error copy to buffer %v", errCopy)) // Log for internal use
			response.SetStatus(http.StatusBadRequest)
			response.SetError(fmt.Errorf("upload file error"))
			return response
		}
	case sourceCaseURL:
		urlPath := requestBody[route.Data[requestFieldKey].(string)].(string)
		resp, errRead := http.Get(urlPath)

		if errRead != nil {
			log.Error(fmt.Sprintf("error reading from url %v", errRead)) // Log for internal use
			response.SetStatus(http.StatusInternalServerError)
			response.SetError(fmt.Errorf("upload file error"))
			return response
		}

		if u, errUrl := url.Parse(urlPath); errUrl == nil {
			userFileName = filepath.Base(u.EscapedPath())
		}
		mimeType = resp.Header.Get("Content-Type")

		_, errCopy := io.Copy(&Buf, resp.Body)
		if errCopy != nil {
			log.Error(fmt.Sprintf("error copy to buffer %v", errCopy)) // Log for internal use
			response.SetStatus(http.StatusInternalServerError)
			response.SetError(fmt.Errorf("upload file error"))
			return response
		}

	default:
		log.Error("unknown upload source") // Log for internal use
		response.SetStatus(http.StatusInternalServerError)
		response.SetError(fmt.Errorf("upload file error"))
		return response
	}

	upload[uploadUserFileNameKey] = userFileName
	upload[uploadMimeTypeKey] = mimeType

	// Detect extension from mimeType map
	if mimExt, ok := route.Data[paramMimExt]; ok {
		correctMime := false
		for mimeTypeRegex, ext := range mimExt.(map[string]interface{}) {
			var mime = regexp.MustCompile(mimeTypeRegex)
			if mime.MatchString(mimeType) {
				extension = ext.(string)
				correctMime = true
				break
			}
		}

		if !correctMime {
			log.Error(fmt.Sprintf("Upload file: Incorrect mime type: %s", mimeType)) // Log for internal use
			response.SetStatus(http.StatusBadRequest)
			response.SetError(fmt.Errorf("incorrect file type"))
			return response
		}
	}

	// Detect extension from user filename
	if extension == "" {
		var fileNameRegex = regexp.MustCompile(`([a-z0-9]+)$`)
		extension = fileNameRegex.FindString(userFileName)
	}
	upload[uploadExtensionKey] = extension

	switch route.Data[paramDestination] {
	// Save uploaded file to folder on server with proxy
	case destinationCaseFolder:
		nameTemplate := route.Data[paramNameTemplate].(string)
		if strings.Contains(nameTemplate, "$ext") {
			nameTemplate = strings.Replace(nameTemplate, "$ext", extension, 1)
		}

		tempFile, err := os.CreateTemp("/tmp", nameTemplate)
		if err != nil {
			log.Error(fmt.Sprintf("failed to create uploaded file: %v", err)) // Log for internal use
			response.SetStatus(http.StatusBadRequest)
			response.SetError(fmt.Errorf("upload file error"))
			return response
		}
		defer func() {
			err := tempFile.Close()
			if err != nil {
				log.Errorf("Error close tempFile handler %v", err)
			}
		}()

		size, err := tempFile.Write(Buf.Bytes())
		if err != nil {
			log.Error(fmt.Sprintf("failed to save uploaded tmp file: %v", err)) // Log for internal use
			response.SetStatus(http.StatusInternalServerError)
			response.SetError(fmt.Errorf("upload file error"))
			return response
		}

		upload[uploadFilePathKey] = filepath.Join(route.Data[paramFolder].(string), filepath.Base(tempFile.Name()))
		err = os.WriteFile(upload[uploadFilePathKey].(string), Buf.Bytes(), os.FileMode(0644))
		if err != nil {
			log.Error(fmt.Sprintf("failed to save uploaded dst file: %v", err)) // Log for internal use
			response.SetStatus(http.StatusInternalServerError)
			response.SetError(fmt.Errorf("upload file error"))
			return response
		}

		upload[uploadSizeKey] = size

		fileStat, _ := tempFile.Stat()
		upload[uploadFileNameKey] = fileStat.Name()

	// Send uploaded file to external service
	case destinationCaseService:
		// TODO: Implement me

	// Send uploaded file to ftp server
	case destinationCaseFTP:
		// TODO: Implement me

	default:
		response.SetStatus(http.StatusBadRequest)
		response.SetError(fmt.Errorf("unknown upload destination"))
		return response
	}

	requestBody[bodyKeyUploadedFile] = upload

	s.taskService.GetService(metrics.ServiceName).(metrics.Measurer).Counter("proxy_uploaded_files_count").Inc()

	tasks := make([]common.Task, 0, len(route.Tasks))

	if len(route.Tasks) > 0 {
		for _, taskItem := range route.Tasks {
			tasks = append(tasks, &taskItem)
		}

		// Inject requestBody data to first task
		if mapData, err := tasks[0].MapData(); err == nil {
			_ = utils.MapMerge(&requestBody, mapData)
		}
		tasks[0].Set(&requestBody)
	}

	// Run pipeline
	err := s.taskService.NewPipeline(tasks, request, response, nil).Run()

	if err != nil {
		response.SetError(err)
	}

	return response
}
