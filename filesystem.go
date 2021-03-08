package gemini

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/a-h/gemini/log"
)

type Dir string

// Open implements FileSystem using os.Open, opening files for reading rooted
// and relative to the directory d.
func (d Dir) Open(name string) (File, error) {
	dir := string(d)
	if dir == "" {
		dir = "."
	}
	fullName := filepath.Join(dir, filepath.FromSlash(path.Clean("/"+name)))
	return os.Open(fullName)
}

// A FileSystem implements access to a collection of named files.
// The elements in a file path are separated by slash ('/', U+002F)
// characters, regardless of host operating system convention.
type FileSystem interface {
	Open(name string) (File, error)
}

// A File is returned by a FileSystem's Open method and can be
// served by the FileServer implementation.
//
// The methods should behave the same as those on an *os.File.
type File interface {
	io.Closer
	io.Reader
	Readdir(count int) ([]os.FileInfo, error)
	Stat() (os.FileInfo, error)
}

func DirectoryListingHandler(path string, f File) Handler {
	return HandlerFunc(func(w ResponseWriter, r *Request) {
		files, err := f.Readdir(-1)
		if err != nil {
			log.Warn("DirectoryListingHandler: readdir failed", log.String("reason", err.Error()), log.String("path", r.URL.Path), log.String("url", r.URL.String()))
			w.SetHeader(CodeTemporaryFailure, "readdir failed")
			return
		}
		sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
		w.SetHeader(CodeSuccess, DefaultMIMEType)
		fmt.Fprintf(w, "# Index of %s\n\n", path)
		fmt.Fprintln(w, "=> ../")
		for _, ff := range files {
			name := ff.Name()
			if ff.IsDir() {
				name += "/"
			}
			url := url.URL{Path: name}
			fmt.Fprintf(w, "=> %v\n", url.String())
		}
	})
}

func FileContentHandler(name string, f File) Handler {
	return HandlerFunc(func(w ResponseWriter, r *Request) {
		mType := mime.TypeByExtension(path.Ext(name))
		if mType == "" {
			mType = DefaultMIMEType
		}
		w.SetHeader(CodeSuccess, mType)
		io.Copy(w, f)
	})
}

func FileSystemHandler(fs FileSystem) Handler {
	return HandlerFunc(func(w ResponseWriter, r *Request) {
		if strings.Contains(r.URL.Path, "..") {
			// Possible directory traversal attack.
			BadRequest(w, r)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/") {
			r.URL.Path = "/" + r.URL.Path
		}
		f, err := fs.Open(r.URL.Path)
		if err != nil {
			log.Warn("FileSystemHandler: file open failed", log.String("reason", err.Error()), log.String("path", r.URL.Path), log.String("url", r.URL.String()))
			w.SetHeader(CodeTemporaryFailure, "file open failed")
			return
		}
		stat, err := f.Stat()
		if err != nil {
			log.Warn("FileSystemHandler: file stat failed", log.String("reason", err.Error()), log.String("path", r.URL.Path), log.String("url", r.URL.String()))
			w.SetHeader(CodeTemporaryFailure, "file stat failed")
			return
		}
		if stat.IsDir() {
			// Look for index.gmi first before listing contents.
			if !strings.HasSuffix(r.URL.Path, "/") {
				RedirectPermanentHandler(r.URL.Path+"/").ServeGemini(w, r)
				return
			}
			index, err := fs.Open(r.URL.Path + "index.gmi")
			if errors.Is(err, os.ErrNotExist) {
				DirectoryListingHandler(r.URL.Path, f).ServeGemini(w, r)
				return
			}
			FileContentHandler("index.gmi", index).ServeGemini(w, r)
			return
		}
		FileContentHandler(stat.Name(), f).ServeGemini(w, r)
	})
}
