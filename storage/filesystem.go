package storage

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

const FileSuffix = "_config.json"

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

	fs.fileInfo = fInfos
	return nil
}

func (fs *FilesystemDriver) GetGuildData(guildID string) (map[string]interface{}, error) {
	for _, info := range fs.fileInfo {
		if !info.IsDir() {
			name := info.Name()
			fullPath := path.Join(fs.baseDir, name)
			if strings.Contains(fullPath, guildID+FileSuffix) {
				f, err := os.Open(fullPath)
				if err != nil {
					return nil, err
				}
				bytes, err := ioutil.ReadAll(f)
				f.Close()
				if err != nil {
					return nil, err
				}
				var intf map[string]interface{}
				err = json.Unmarshal(bytes, &intf)
				if err != nil {
					return nil, err
				}
				return intf, nil
			}
		}
	}
	return map[string]interface{}{}, errors.New("no config json found")
}

func (fs *FilesystemDriver) WriteGuildData(guildID string, data map[string]interface{}) error {
	jsonBytes, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return err
	}

	for _, fi := range fs.fileInfo {
		//TODO enforce naming scheme?
		if strings.Contains(fi.Name(), guildID+FileSuffix) {
			f, err := os.OpenFile(path.Join(fs.baseDir, fi.Name()), os.O_CREATE|os.O_WRONLY, 0660)
			if err != nil {
				return err
			}
			_, err = f.Write(jsonBytes)
			f.Close()
			return err //can be nil if it worked
		}
	}
	f, err := os.Create(path.Join(fs.baseDir, guildID+FileSuffix))
	if err != nil {
		return err
	}
	_, err = f.Write(jsonBytes)
	f.Close()
	return err
}

func (fs *FilesystemDriver) Close() error {
	return nil
}
