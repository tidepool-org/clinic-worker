package test

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic-worker/redox"
	"io"
)

type RedoxClient struct {
	sourceId      string
	sourceName    string
	uploadEnabled bool
	Sent          []interface{}
	Uploaded      map[string]interface{}
}

var _ redox.Client = &RedoxClient{}

func NewTestRedoxClient(sourceId, sourceName string) *RedoxClient {
	return &RedoxClient{
		sourceId:   sourceId,
		sourceName: sourceName,
		Uploaded:   make(map[string]interface{}),
	}
}

func (t *RedoxClient) GetSource() (source struct {
	ID   *string `json:"ID"`
	Name *string `json:"Name"`
}) {
	source.ID = &t.sourceId
	source.Name = &t.sourceName
	return
}

func (t *RedoxClient) Send(ctx context.Context, payload interface{}) error {
	t.Sent = append(t.Sent, payload)
	return nil
}

func (t *RedoxClient) SetUploadFileEnabled(val bool) {
	t.uploadEnabled = val
}

func (t *RedoxClient) IsUploadFileEnabled() bool {
	return t.uploadEnabled
}

func (t *RedoxClient) UploadFile(ctx context.Context, fileName string, reader io.Reader) (*redox.UploadResult, error) {
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	t.Uploaded[fileName] = body
	return &redox.UploadResult{
		URI: fmt.Sprintf("https://blob.redoxengine.com/upload/%s", fileName),
	}, nil
}
