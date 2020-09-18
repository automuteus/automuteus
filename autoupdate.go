package main

import (
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

// ReleasesURL const
const ReleasesURL = "https://api.github.com/repos/denverquane/amongusdiscord/releases/latest"

func main() {
	resp, err := http.Get(ReleasesURL)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	lines := strings.Split(string(body), ",")
	for _, line := range lines {
		line = strings.ReplaceAll(line, "\"", "")
		line = strings.ReplaceAll(strings.ReplaceAll(line, "[", ""), "]", "")
		line = strings.ReplaceAll(strings.ReplaceAll(line, "{", ""), "}", "")
		if strings.HasPrefix(line, "browser_download_url") && strings.HasSuffix(line, "amongusdiscord.exe") {
			url := strings.Replace(line, "browser_download_url:", "", 1)
			log.Println(url)
			err := DownloadFile("amongusdiscord.exe", url)
			if err != nil {
				log.Println("Error in downloading exe:")
				log.Println(err)
			}
		}
	}
}

// DownloadFile will download a url to a local file. It's efficient because it will
// write as it downloads and not load the whole file into memory.
func DownloadFile(filepath string, url string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}
