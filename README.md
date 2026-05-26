# go-musthave-shortener-tpl

Шаблон репозитория для трека «Сервис сокращения URL».

## Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. В корне репозитория выполните команду `go mod init <name>` (где `<name>` — адрес вашего репозитория на GitHub без префикса `https://`) для создания модуля.

## Обновление шаблона

Чтобы иметь возможность получать обновления автотестов и других частей шаблона, выполните команду:

```
git remote add -m v2 template https://github.com/Yandex-Practicum/go-musthave-shortener-tpl.git
```

Для обновления кода автотестов выполните команду:

```
git fetch template && git checkout template/v2 .github
```

Затем добавьте полученные изменения в свой репозиторий.

## Запуск автотестов

Для успешного запуска автотестов называйте ветки `iter<number>`, где `<number>` — порядковый номер инкремента. Например, в ветке с названием `iter4` запустятся автотесты для инкрементов с первого по четвёртый.

При мёрже ветки с инкрементом в основную ветку `main` будут запускаться все автотесты.

Подробнее про локальный и автоматический запуск читайте в [README автотестов](https://github.com/Yandex-Practicum/go-autotests).

## Структура проекта

Приведённая в этом репозитории структура проекта является рекомендуемой, но не обязательной.

Это лишь пример организации кода, который поможет вам в реализации сервиса.

При необходимости можно вносить изменения в структуру проекта, использовать любые библиотеки и предпочитаемые структурные паттерны организации кода приложения, например:
- **DDD** (Domain-Driven Design)
- **Clean Architecture**
- **Hexagonal Architecture**
- **Layered Architecture**

## Профилирование памяти (pprof)

Базовый профиль снят командой:

```sh
go test -run=^$ -bench=. -benchmem -memprofile=profiles/base.pprof ./internal/service/
```

После оптимизаций снят итоговый профиль:

```sh
go test -run=^$ -bench=. -benchmem -memprofile=profiles/result.pprof ./internal/service/
```

### Diff между базовым и итоговым профилями

```sh
go tool pprof -top -diff_base=profiles/base.pprof profiles/result.pprof
```

```text
File: service.test.exe
Type: alloc_space
Showing nodes accounting for 2941.49MB, 101.89% of 2886.97MB total
Dropped 81 nodes (cum <= 14.43MB)
      flat  flat%   sum%        cum   cum%
 2983.99MB 103.36% 103.36%  2982.49MB 103.31%  repositories.(*URLShortener).GetListURLByUserID
 -305.50MB -10.58%  92.78%   -42.50MB  -1.47%  service.GenerateUniqueID
     211MB   7.31% 100.09%      263MB   9.11%  service.padBase62
      52MB   1.80% 101.89%       52MB   1.80%  service.encodeBase62 (inline)
```

### Что изменилось

| Бенчмарк | ns/op до | ns/op после | B/op до | B/op после | allocs/op до | allocs/op после |
| --- | --- | --- | --- | --- | --- | --- |
| `BenchmarkGetAllShortenerURL` | 21 802 | 7 339 | 41 336 | 23 808 | 12 | 3 |
| `BenchmarkGenerateUniqueID` | 58.56 | 45.96 | 16 | 11 | 1 | 1 |

**`GetListURLByUserID`** — удалена промежуточная сортированная slice: теперь итерация идёт прямо по `byID`, без дополнительных аллокаций.

**`GenerateUniqueID`** — функция `padBase62` использует стековый массив `[5]byte` вместо конкатенации строк на heap.

**`GenerateID`** — буфер `[]byte` для `crypto/rand` переиспользуется через `sync.Pool`.
