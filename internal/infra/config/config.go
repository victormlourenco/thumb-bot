package config

import (
	"encoding/json"
	"os"
)

const configFile = "config/config.json"

type Config struct {
	TelegramToken string `json:"telegramToken"`
}

func (c *Config) Load() error {
	file, err := os.Open(configFile)
	if err != nil {
		return err
	}
	defer file.Close()

	jsonParser := json.NewDecoder(file)
	err = jsonParser.Decode(c)
	if err != nil {
		return err
	}

	return nil
}
