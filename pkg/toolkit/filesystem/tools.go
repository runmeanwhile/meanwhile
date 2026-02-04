package filesystem

import (
	"context"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"time"
	"unicode/utf8"

	"github.com/runmeanwhile/meanwhile/pkg/tool"
)

type readArgs struct {
	Path     string `json:"path" description:"Path to the file (absolute or relative to allowed root)"`
	MaxBytes int    `json:"max_bytes,omitempty" description:"Optional max bytes to read"`
}

type readOutput struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Encoding  string `json:"encoding"`
	Bytes     int    `json:"bytes"`
	Truncated bool   `json:"truncated"`
}

func newReadTool(fs *SafeFS, cfg Config) (tool.Tool, error) {
	readTool, err := tool.New[readArgs, readOutput]("fs_read", func(_ context.Context, args readArgs) (readOutput, error) {
		resolved, err := fs.Resolve(args.Path)
		if err != nil {
			return readOutput{}, err
		}
		limit := cfg.MaxReadBytes
		if args.MaxBytes > 0 && args.MaxBytes < limit {
			limit = args.MaxBytes
		}
		if limit <= 0 {
			limit = 64 * 1024
		}
		file, err := os.Open(resolved)
		if err != nil {
			return readOutput{}, err
		}
		reader := io.LimitReader(file, int64(limit)+1)
		data, readErr := io.ReadAll(reader)
		closeErr := file.Close()
		if readErr != nil {
			return readOutput{}, readErr
		}
		if closeErr != nil {
			return readOutput{}, closeErr
		}
		truncated := len(data) > limit
		if truncated {
			data = data[:limit]
		}
		encoding := "utf-8"
		content := string(data)
		if !utf8.Valid(data) {
			encoding = "base64"
			content = base64.StdEncoding.EncodeToString(data)
		}
		return readOutput{
			Path:      resolved,
			Content:   content,
			Encoding:  encoding,
			Bytes:     len(data),
			Truncated: truncated,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return readTool.WithDescription("Read a file within allowed roots"), nil
}

type listArgs struct {
	Path     string `json:"path" description:"Path to a directory (absolute or relative to allowed root)"`
	MaxDepth int    `json:"max_depth,omitempty" description:"Optional max depth (1 means current directory only)"`
	MaxItems int    `json:"max_items,omitempty" description:"Optional max number of entries to return"`
}

type listEntry struct {
	Path    string    `json:"path"`
	Name    string    `json:"name"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size,omitempty"`
	ModTime time.Time `json:"mod_time,omitempty"`
}

type listOutput struct {
	Path      string      `json:"path"`
	Entries   []listEntry `json:"entries"`
	Truncated bool        `json:"truncated"`
}

func newListTool(fs *SafeFS, cfg Config) (tool.Tool, error) {
	listTool, err := tool.New[listArgs, listOutput]("fs_list", func(_ context.Context, args listArgs) (listOutput, error) {
		resolved, err := fs.Resolve(args.Path)
		if err != nil {
			return listOutput{}, err
		}
		maxDepth := cfg.MaxDepth
		if args.MaxDepth > 0 {
			maxDepth = args.MaxDepth
		}
		if maxDepth <= 0 {
			maxDepth = 1
		}
		maxItems := cfg.MaxListEntries
		if args.MaxItems > 0 {
			maxItems = args.MaxItems
		}
		if maxItems <= 0 {
			maxItems = 200
		}

		type queueItem struct {
			path  string
			depth int
		}
		queue := []queueItem{{path: resolved, depth: 1}}
		entries := make([]listEntry, 0)
		truncated := false

		for len(queue) > 0 {
			item := queue[0]
			queue = queue[1:]
			if item.depth > maxDepth {
				continue
			}
			dirEntries, err := os.ReadDir(item.path)
			if err != nil {
				return listOutput{}, err
			}
			for _, entry := range dirEntries {
				info, err := entry.Info()
				if err != nil {
					return listOutput{}, err
				}
				path := filepath.Join(item.path, entry.Name())
				entries = append(entries, listEntry{
					Path:    path,
					Name:    entry.Name(),
					IsDir:   entry.IsDir(),
					Size:    info.Size(),
					ModTime: info.ModTime(),
				})
				if len(entries) >= maxItems {
					truncated = true
					queue = nil
					break
				}
				if entry.IsDir() && item.depth < maxDepth {
					queue = append(queue, queueItem{path: path, depth: item.depth + 1})
				}
			}
		}

		return listOutput{
			Path:      resolved,
			Entries:   entries,
			Truncated: truncated,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return listTool.WithDescription("List directory contents within allowed roots"), nil
}

type writeArgs struct {
	Path       string `json:"path" description:"Path to write (absolute or relative to allowed root)"`
	Content    string `json:"content" description:"File contents"`
	Append     bool   `json:"append,omitempty" description:"Append instead of overwrite"`
	CreateDirs bool   `json:"create_dirs,omitempty" description:"Create parent directories if missing"`
}

type writeOutput struct {
	Path    string `json:"path"`
	Bytes   int    `json:"bytes"`
	Append  bool   `json:"append"`
	Created bool   `json:"created"`
}

func newWriteTool(fs *SafeFS, cfg Config) (tool.Tool, error) {
	writeTool, err := tool.New[writeArgs, writeOutput]("fs_write", func(_ context.Context, args writeArgs) (writeOutput, error) {
		resolved, err := fs.Resolve(args.Path)
		if err != nil {
			return writeOutput{}, err
		}
		if args.CreateDirs {
			dir := filepath.Dir(resolved)
			if err := os.MkdirAll(dir, os.FileMode(cfg.DirMode)); err != nil {
				return writeOutput{}, err
			}
		}
		flags := os.O_WRONLY | os.O_CREATE
		if args.Append {
			flags |= os.O_APPEND
		} else {
			flags |= os.O_TRUNC
		}
		created := false
		if _, err := os.Stat(resolved); os.IsNotExist(err) {
			created = true
		}
		file, err := os.OpenFile(resolved, flags, os.FileMode(cfg.FileMode))
		if err != nil {
			return writeOutput{}, err
		}
		written, writeErr := io.WriteString(file, args.Content)
		closeErr := file.Close()
		if writeErr != nil {
			return writeOutput{}, writeErr
		}
		if closeErr != nil {
			return writeOutput{}, closeErr
		}
		return writeOutput{
			Path:    resolved,
			Bytes:   written,
			Append:  args.Append,
			Created: created,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return writeTool.WithDescription("Write a file within allowed roots"), nil
}

type statArgs struct {
	Path string `json:"path" description:"Path to stat (absolute or relative to allowed root)"`
}

type statOutput struct {
	Path    string    `json:"path"`
	Exists  bool      `json:"exists"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size,omitempty"`
	Mode    string    `json:"mode,omitempty"`
	ModTime time.Time `json:"mod_time,omitempty"`
}

func newStatTool(fs *SafeFS) (tool.Tool, error) {
	statTool, err := tool.New[statArgs, statOutput]("fs_stat", func(_ context.Context, args statArgs) (statOutput, error) {
		resolved, err := fs.Resolve(args.Path)
		if err != nil {
			return statOutput{}, err
		}
		info, err := os.Stat(resolved)
		if err != nil {
			if os.IsNotExist(err) {
				return statOutput{Path: resolved, Exists: false}, nil
			}
			return statOutput{}, err
		}
		return statOutput{
			Path:    resolved,
			Exists:  true,
			IsDir:   info.IsDir(),
			Size:    info.Size(),
			Mode:    info.Mode().String(),
			ModTime: info.ModTime(),
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return statTool.WithDescription("Stat a path within allowed roots"), nil
}
