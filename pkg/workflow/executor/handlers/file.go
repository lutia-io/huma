package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/lutia-io/huma/pkg/file"
	"github.com/lutia-io/huma/pkg/node"
	"github.com/lutia-io/huma/pkg/workflow"
	"github.com/lutia-io/huma/pkg/workflow/executor"
)

const fileContentLimit = 1 << 20

type FileStore interface {
	Create(ctx context.Context, params file.CreateParams) (string, error)
	GetMeta(ctx context.Context, fileID string) (*file.File, bool, error)
	OpenContent(ctx context.Context, fileID string) (*file.Content, bool, error)
}

// File reads or writes workspace files.
//
// WRITE config: filename (required), content (templated string or value),
// contentType (default text/plain). Content may be a pure template path to a
// string or JSON value; non-strings are JSON-marshaled.
//
// WRITE output: { fileId, filename, contentType, sizeBytes }
// READ output:  { fileId, filename, contentType, sizeBytes, content }
type File struct {
	files FileStore
}

func NewFile(files FileStore) *File {
	return &File{files: files}
}

func (h *File) Type() node.Type {
	return node.TypeFile
}

func (h *File) Execute(ctx context.Context, execCtx executor.ExecutionContext, n workflow.SnapshotNode) (executor.Result, error) {
	if h.files == nil {
		return executor.Result{}, fmt.Errorf("FILE node requires a file service")
	}
	def, err := parseDefinition[node.FileContext](n, node.TypeFile)
	if err != nil {
		return executor.Result{}, err
	}
	switch def.Operation {
	case node.FileOpWrite:
		return h.write(ctx, execCtx, def)
	case node.FileOpRead:
		return h.read(ctx, execCtx, def)
	default:
		return executor.Result{}, fmt.Errorf("unsupported FILE operation %q", def.Operation)
	}
}

func (h *File) write(ctx context.Context, execCtx executor.ExecutionContext, def node.FileContext) (executor.Result, error) {
	filename, err := resolveString(def.Filename, execCtx)
	if err != nil {
		return executor.Result{}, fmt.Errorf("resolving FILE filename: %w", err)
	}
	if filename == "" {
		return executor.Result{}, fmt.Errorf("FILE WRITE requires a filename")
	}
	contentType, err := resolveString(def.ContentType, execCtx)
	if err != nil {
		return executor.Result{}, fmt.Errorf("resolving FILE contentType: %w", err)
	}
	if contentType == "" {
		contentType = "text/plain"
	}
	contentVal, err := resolveAny(def.Content, execCtx)
	if err != nil {
		return executor.Result{}, fmt.Errorf("resolving FILE content: %w", err)
	}
	content, err := contentString(contentVal)
	if err != nil {
		return executor.Result{}, fmt.Errorf("encoding FILE content: %w", err)
	}

	id, err := h.files.Create(ctx, file.CreateParams{
		Filename:           filename,
		ContentType:        contentType,
		OrganizationID:     execCtx.OrganizationID,
		OrganizationUserID: execCtx.OrganizationUserID,
		NetworkID:          execCtx.NetworkID,
		IdempotencyKey:     execCtx.IdempotencyKey,
		Content:            strings.NewReader(content),
	})
	if err != nil {
		return executor.Result{}, fmt.Errorf("creating file: %w", err)
	}

	meta, found, err := h.files.GetMeta(ctx, id)
	if err != nil {
		return executor.Result{}, fmt.Errorf("loading created file: %w", err)
	}
	size := int64(len(content))
	if found && meta != nil {
		filename = meta.Filename
		contentType = meta.ContentType
		size = meta.SizeBytes
	}

	ref := executor.FileRef{
		FileID:      id,
		Filename:    filename,
		ContentType: contentType,
		SizeBytes:   size,
	}
	result, err := executor.MarshalOutput(map[string]any{
		"fileId":      ref.FileID,
		"filename":    ref.Filename,
		"contentType": ref.ContentType,
		"sizeBytes":   ref.SizeBytes,
	})
	if err != nil {
		return executor.Result{}, err
	}
	result.Payload = executor.Payload{Files: []executor.FileRef{ref}}
	return result, nil
}

func (h *File) read(ctx context.Context, execCtx executor.ExecutionContext, def node.FileContext) (executor.Result, error) {
	fileID, err := resolveString(def.FileID, execCtx)
	if err != nil {
		return executor.Result{}, fmt.Errorf("resolving FILE fileId: %w", err)
	}
	if fileID == "" {
		return executor.Result{}, fmt.Errorf("FILE READ requires a fileId")
	}

	meta, found, err := h.files.GetMeta(ctx, fileID)
	if err != nil {
		return executor.Result{}, fmt.Errorf("loading file %q: %w", fileID, err)
	}
	if !found || meta == nil {
		return executor.Result{}, fmt.Errorf("file %q not found", fileID)
	}
	if meta.NetworkID != execCtx.NetworkID || meta.OrganizationID != execCtx.OrganizationID {
		return executor.Result{}, fmt.Errorf("file %q not found", fileID)
	}

	out := map[string]any{
		"fileId":      meta.ID,
		"filename":    meta.Filename,
		"contentType": meta.ContentType,
		"sizeBytes":   meta.SizeBytes,
	}

	content, found, err := h.files.OpenContent(ctx, fileID)
	if err != nil {
		return executor.Result{}, fmt.Errorf("opening file %q: %w", fileID, err)
	}
	if found && content != nil {
		defer content.Reader.Close()
		body, readErr := io.ReadAll(io.LimitReader(content.Reader, fileContentLimit+1))
		if readErr != nil {
			return executor.Result{}, fmt.Errorf("reading file %q: %w", fileID, readErr)
		}
		if int64(len(body)) > fileContentLimit {
			return executor.Result{}, fmt.Errorf("file %q exceeds %d byte read limit", fileID, fileContentLimit)
		}
		if utf8.Valid(body) {
			out["content"] = string(body)
		} else {
			out["content"] = string(bytes.ToValidUTF8(body, []byte("\uFFFD")))
		}
	}

	result, err := executor.MarshalOutput(out)
	if err != nil {
		return executor.Result{}, err
	}
	result.Payload = executor.Payload{Files: []executor.FileRef{{
		FileID:      meta.ID,
		Filename:    meta.Filename,
		ContentType: meta.ContentType,
		SizeBytes:   meta.SizeBytes,
	}}}
	return result, nil
}
