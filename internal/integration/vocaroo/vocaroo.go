package vocaroo

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"thumb-bot/internal/utils"
)

func Fetch(url string) (io.Reader, string, error) {
	shortCode, err := getShortcode(url)
	if err != nil {
		return nil, "", err
	}

	downloadUrl := fmt.Sprintf("https://media1.vocaroo.com/mp3/%s", shortCode)

	reader, err := getMP3Reader(downloadUrl)
	if err != nil {
		return nil, "", err
	}

	return reader, fmt.Sprintf("Vocaroo %s", shortCode), nil
}

func getShortcode(urlStr string) (string, error) {
	u, err := url.Parse(utils.RemoveQueryParams(urlStr))
	if err != nil {
		return "", err
	}
	path := strings.Split(u.Path, "/")
	if len(path) < 2 {
		return "", errors.New("invalid url")
	}
	return path[1], nil
}

func getMP3Reader(url string) (io.Reader, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "https://vocaroo.com/")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch MP3: %s", resp.Status)
	}

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return nil, err
	}

	return buf, nil
}
