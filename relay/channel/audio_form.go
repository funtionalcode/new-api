package channel

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
)

type AudioMultipartOptions struct {
	IncludeModel bool
	RequireFile  bool
	FileField    string
	SkipFields   map[string]struct{}
}

func BuildAudioMultipartRequest(c *gin.Context, model string, options AudioMultipartOptions) (io.Reader, error) {
	fileField := strings.TrimSpace(options.FileField)
	if fileField == "" {
		fileField = "file"
	}

	formData, err := common.ParseMultipartFormReusable(c)
	if err != nil {
		return nil, fmt.Errorf("error parsing multipart form: %w", err)
	}

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	if options.IncludeModel {
		if err := writer.WriteField("model", model); err != nil {
			return nil, fmt.Errorf("write model field failed: %w", err)
		}
	}

	skipFields := make(map[string]struct{}, len(options.SkipFields)+2)
	skipFields["model"] = struct{}{}
	skipFields[fileField] = struct{}{}
	for key := range options.SkipFields {
		skipFields[key] = struct{}{}
	}

	for key, values := range formData.Value {
		if _, ok := skipFields[key]; ok {
			continue
		}
		for _, value := range values {
			if err := writer.WriteField(key, value); err != nil {
				return nil, fmt.Errorf("write form field %s failed: %w", key, err)
			}
		}
	}

	fileHeaders := formData.File[fileField]
	if len(fileHeaders) == 0 {
		if options.RequireFile {
			return nil, errors.New(fileField + " is required")
		}
		if err := writer.Close(); err != nil {
			return nil, fmt.Errorf("close multipart writer failed: %w", err)
		}
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())
		return &requestBody, nil
	}

	fileHeader := fileHeaders[0]
	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("error opening audio file: %w", err)
	}
	defer file.Close()

	mimeHeader := make(textproto.MIMEHeader)
	mimeHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fileField, fileHeader.Filename))
	contentType := strings.TrimSpace(fileHeader.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	mimeHeader.Set("Content-Type", contentType)

	part, err := writer.CreatePart(mimeHeader)
	if err != nil {
		return nil, fmt.Errorf("create form file failed: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("copy file failed: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer failed: %w", err)
	}
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	return &requestBody, nil
}
