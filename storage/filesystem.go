package storage

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

const SettingsFileSuffix = "_settings.json"

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

func (fs *FilesystemDriver) GetGuildSettings(guildID string) (*GuildSettings, error) {
	for _, info := range fs.fileInfo {
		if !info.IsDir() {
			name := info.Name()
			fullPath := path.Join(fs.baseDir, name)
			if strings.Contains(fullPath, guildID+SettingsFileSuffix) {
				f, err := os.Open(fullPath)
				if err != nil {
					return nil, err
				}
				bytes, err := ioutil.ReadAll(f)
				f.Close()
				if err != nil {
					return nil, err
				}
				var gs GuildSettings
				err = json.Unmarshal(bytes, &gs)
				if err != nil {
					return nil, err
				}
				return &gs, nil
			}
		}
	}
	return nil, errors.New("no guild settings found for guildID " + guildID)
}

func (fs *FilesystemDriver) WriteGuildSettings(guildID string, gs *GuildSettings) error {
	jsonBytes, err := json.MarshalIndent(gs, "", "    ")
	if err != nil {
		return err
	}

	for _, fi := range fs.fileInfo {
		//TODO enforce naming scheme?
		if strings.Contains(fi.Name(), guildID+SettingsFileSuffix) {
			f, err := os.OpenFile(path.Join(fs.baseDir, fi.Name()), os.O_CREATE|os.O_WRONLY, 0660)
			if err != nil {
				return err
			}
			_, err = f.Write(jsonBytes)
			f.Close()
			return err //can be nil if it worked
		}
	}
	f, err := os.Create(path.Join(fs.baseDir, guildID+SettingsFileSuffix))
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
