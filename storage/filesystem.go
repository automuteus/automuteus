package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"
	"strings"
)

func guildFileName(guildID string) string {
	return fmt.Sprintf("guild_%s_settings.json", guildID)
}

func userFileName(userID string) string {
	return fmt.Sprintf("user_%s_settings.json", userID)
}

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
			if strings.Contains(fullPath, guildFileName(guildID)) {
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
		if strings.Contains(fi.Name(), guildFileName(guildID)) {
			f, err := os.OpenFile(path.Join(fs.baseDir, fi.Name()), os.O_CREATE|os.O_WRONLY, 0660)
			if err != nil {
				return err
			}
			_, err = f.Write(jsonBytes)
			f.Close()
			return err //can be nil if it worked
		}
	}
	f, err := os.Create(path.Join(fs.baseDir, guildFileName(guildID)))
	if err != nil {
		return err
	}
	_, err = f.Write(jsonBytes)
	f.Close()
	return err
}

var userSettingsRegex = regexp.MustCompile(`^user_(?P<userid>\d+)_settings.json$`)

func (fs *FilesystemDriver) GetAllUserSettings() *UserSettingsCollection {
	col := MakeUserSettingsCollection()
	for _, info := range fs.fileInfo {
		if !info.IsDir() {
			name := info.Name()
			fullPath := path.Join(fs.baseDir, name)
			if match := userSettingsRegex.FindStringSubmatch(name); match != nil {
				userID := match[userSettingsRegex.SubexpIndex("userid")]
				f, err := os.Open(fullPath)
				if err != nil {
					log.Println(err)
					return col
				}
				bytes, err := ioutil.ReadAll(f)
				f.Close()
				if err != nil {
					log.Println(err)
					return col
				}
				var us UserSettings
				err = json.Unmarshal(bytes, &us)
				if err != nil {
					log.Println(err)
					return col
				}
				col.users[userID] = &us
			}
		}
	}
	return col
}

func (fs *FilesystemDriver) WriteUserSettings(userID string, gs *UserSettings) error {
	jsonBytes, err := json.MarshalIndent(gs, "", "    ")
	if err != nil {
		return err
	}

	for _, fi := range fs.fileInfo {
		//TODO enforce naming scheme?
		if strings.Contains(fi.Name(), userFileName(userID)) {
			f, err := os.OpenFile(path.Join(fs.baseDir, fi.Name()), os.O_CREATE|os.O_WRONLY, 0660)
			if err != nil {
				return err
			}
			_, err = f.Write(jsonBytes)
			f.Close()
			return err //can be nil if it worked
		}
	}
	f, err := os.Create(path.Join(fs.baseDir, userFileName(userID)))
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
