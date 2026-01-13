package config

import "strings"

type Config struct {
	ServerAddress string // Адрес сервера
	BaseURL       string // Базовый URL для коротких ссылок
}

var AppConfig *Config

func Init(serverAddress string, baseURL string) {

	// if strings.HasPrefix(str, prefix) {
    //     fmt.Println("Строка начинается с префикса.")
    // } else {
    //     fmt.Println("Префикса нет.")
    // }

	serverAddress1 := strings.TrimPrefix(serverAddress, "http://")
	serverAddress1 = strings.TrimPrefix(serverAddress, "https://")
	AppConfig = &Config{
		ServerAddress: serverAddress1,
		BaseURL:       baseURL,
	}
}
