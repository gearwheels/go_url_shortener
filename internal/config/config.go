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
	if strings.HasPrefix(serverAddress, "http://") {
		serverAddress = strings.TrimPrefix(serverAddress, "http://")
	}else if strings.HasPrefix(serverAddress, "https://"){
		serverAddress = strings.TrimPrefix(serverAddress, "https://")
	}

	AppConfig = &Config{
		ServerAddress: serverAddress,
		BaseURL:       baseURL,
	}
}
