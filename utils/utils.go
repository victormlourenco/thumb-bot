package utils

import (
	"net/url"
	"regexp"
)

func ExtractLinks(texto string) []string {
	// Define uma expressão regular para encontrar URLs
	regex := regexp.MustCompile(`https?://[^\s]+`)

	// Encontra todas as correspondências na string
	matches := regex.FindAllString(texto, -1)

	return matches
}

func RemoveQueryParams(inputURL string) string {
	// Parse a URL
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return ""
	}

	// Remove query params
	parsedURL.RawQuery = ""

	// Reconstrua a URL sem os query params
	resultURL := parsedURL.String()

	return resultURL
}
