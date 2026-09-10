package handlers

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/lutia-io/huma/pkg/file"
	"github.com/lutia-io/huma/pkg/node"
	"github.com/lutia-io/huma/pkg/pipeline"
	"github.com/lutia-io/huma/pkg/pipeline/executor"
)

type fakeFiles struct {
	id      string
	created file.CreateParams
	meta    *file.File
	content string
}

func (f *fakeFiles) Create(_ context.Context, params file.CreateParams) (string, error) {
	f.created = params
	body, err := io.ReadAll(params.Content)
	if err != nil {
		return "", err
	}
	f.content = string(body)
	f.meta = &file.File{
		ID:             f.id,
		Filename:       params.Filename,
		ContentType:    params.ContentType,
		SizeBytes:      int64(len(body)),
		NetworkID:      params.NetworkID,
		OrganizationID: params.OrganizationID,
	}
	return f.id, nil
}

func (f *fakeFiles) GetMeta(_ context.Context, fileID string) (*file.File, bool, error) {
	if f.meta == nil || f.meta.ID != fileID {
		return nil, false, nil
	}
	return f.meta, true, nil
}

func (f *fakeFiles) OpenContent(_ context.Context, fileID string) (*file.Content, bool, error) {
	if f.meta == nil || f.meta.ID != fileID {
		return nil, false, nil
	}
	return &file.Content{
		File:   f.meta,
		Reader: io.NopCloser(strings.NewReader(f.content)),
	}, true, nil
}

func TestFileWrite(t *testing.T) {
	files := &fakeFiles{id: "file-1"}
	h := NewFile(files)
	result, err := h.Execute(context.Background(), executor.ExecutionContext{
		NetworkID:          "net-1",
		OrganizationID:     "org-1",
		OrganizationUserID: "ou-1",
		IdempotencyKey:     "pipe:0:0",
		Input:              map[string]any{"propertyId": "prop-1"},
	}, pipeline.SnapshotNode{
		Type: node.TypeFile,
		Definition: node.FileContext{
			Operation:   node.FileOpWrite,
			Filename:    "notice-{{ .Input.propertyId }}.txt",
			ContentType: "text/plain",
			Content:     "sold {{ .Input.propertyId }}",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if files.created.Filename != "notice-prop-1.txt" || files.content != "sold prop-1" {
		t.Fatalf("created %#v content %q", files.created, files.content)
	}
	if len(result.Payload.Files) != 1 || result.Payload.Files[0].FileID != "file-1" {
		t.Fatalf("payload %#v", result.Payload)
	}
	if !bytes.Contains(result.Output, []byte(`"fileId":"file-1"`)) {
		t.Fatalf("output %s", result.Output)
	}
}

func TestFileRead(t *testing.T) {
	files := &fakeFiles{
		id:      "file-1",
		content: "hello",
		meta: &file.File{
			ID:             "file-1",
			Filename:       "a.txt",
			ContentType:    "text/plain",
			SizeBytes:      5,
			NetworkID:      "net-1",
			OrganizationID: "org-1",
		},
	}
	h := NewFile(files)
	result, err := h.Execute(context.Background(), executor.ExecutionContext{
		NetworkID:      "net-1",
		OrganizationID: "org-1",
		Input:          map[string]any{"fileId": "file-1"},
	}, pipeline.SnapshotNode{
		Type: node.TypeFile,
		Definition: node.FileContext{
			Operation: node.FileOpRead,
			FileID:    "{{ .Input.fileId }}",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(result.Output, []byte(`"content":"hello"`)) {
		t.Fatalf("output %s", result.Output)
	}
}
