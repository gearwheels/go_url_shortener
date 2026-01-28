package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/go-resty/resty/v2"
)

func main() {
	endpoint := "http://localhost:8080/"
	// приглашение в консоли
	client := resty.New()
	fmt.Println("Введите длинный URL")
	// открываем потоковое чтение из консоли
	reader := bufio.NewReader(os.Stdin)
	// читаем строку из консоли
	long, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}

	// Убираем перевод строки и лишние пробелы
	long = strings.TrimSpace(long)

	fmt.Printf("Отправляем: %q\n", long)

	response, err := client.R().
		SetHeader("Content-Type", "text/plain").
		SetBody(long).
		Post(endpoint)

	if err != nil {
		panic(err)
	}
	// выводим код ответа
	fmt.Println("Статус-код ", response.Status())

	fmt.Println(response.String())
}
