package storage

import (
	"log"
	"os"
	"path"
)

type FilesystemDriver struct {
	baseDir  string
	fileInfo []os.FileInfo
}

func (fs *FilesystemDriver) Init(directory string) error {
	f, err := os.Open(directory)
	if err != nil {
		return err
	}
	fs.baseDir = directory
	defer f.Close()

	fInfos, err := f.Readdir(0)
	if err != nil {
		return err
	}

	for _, v := range fInfos {
		log.Println(v.Name())
	}

	fs.fileInfo = fInfos
	return nil
}

func (fs *FilesystemDriver) GetGuildData(string) (map[string]interface{}, error) {
	for _, info := range fs.fileInfo {
		if !info.IsDir() {
			name := info.Name()
			fullPath := path.Join(fs.baseDir, name)
			log.Println(fullPath)
		}
	}
	return map[string]interface{}{}, nil
}

func (fs *FilesystemDriver) Close() error {
	return nil
}
