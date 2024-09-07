package utils

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func Fetch(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		slog.Error("request error", slog.Any("Error", err))
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func GetFilesOfFolder(dir, fileType string) ([]string, error) {
	var files []string
	sep := string(os.PathSeparator)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			subFiles, err := GetFilesOfFolder(dir+sep+info.Name(), fileType)
			if err != nil {
				return err
			}
			files = append(files, subFiles...)
		} else {
			// 过滤指定格式的文件
			ok := strings.HasSuffix(info.Name(), fileType)
			if ok {
				files = append(files, dir+sep+info.Name())
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// for _, fi := range dirPath {
	//
	// }
	return files, nil
}

func IsURL(str string) bool {
	u, err := url.ParseRequestURI(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}
