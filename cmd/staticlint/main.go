// Package main реализует multichecker — статический анализатор кода проекта.
//
// # Запуск
//
// Сборка и запуск:
//
//	go build -o staticlint ./cmd/staticlint
//	./staticlint ./...
//
// Запуск без сборки:
//
//	go run ./cmd/staticlint/... ./...
//
// Анализ конкретного пакета:
//
//	./staticlint github.com/gearwheels/go_url_shortener/internal/handler
//
// # Состав анализаторов
//
// ## Стандартные анализаторы (golang.org/x/tools/go/analysis/passes)
//
//   - appends       — обнаруживает ошибочные вызовы append (результат не используется)
//   - asmdecl       — проверяет соответствие объявлений asm-файлов и Go-файлов
//   - assign        — находит бесполезные присваивания (x = x)
//   - atomic        — проверяет корректное использование пакета sync/atomic
//   - atomicalign   — предупреждает о 64-битных полях, требующих выравнивания
//   - bools         — находит избыточные булевы выражения
//   - buildtag      — проверяет синтаксис build-тегов
//   - cgocall       — обнаруживает вызовы CGo с нарушением правил передачи указателей
//   - composite     — требует явного указания имён полей в составных литералах
//   - copylock      — запрещает копирование типов, содержащих sync.Mutex и подобные
//   - deepequalerrors — предупреждает об использовании reflect.DeepEqual с ошибками
//   - defers        — обнаруживает частые ошибки с defer
//   - directive     — проверяет Go-директивы (//go:generate, //go:build и др.)
//   - errorsas      — проверяет, что второй аргумент errors.As — указатель
//   - fieldalignment — предлагает более компактную раскладку полей структур
//   - framepointer  — обнаруживает некорректное использование frame pointer в asm
//   - hostport      — обнаруживает конкатенацию host:port вместо net.JoinHostPort
//   - httpresponse  — проверяет, что тело HTTP-ответа закрывается
//   - httpmux       — проверяет паттерны маршрутизации net/http
//   - ifaceassert   — находит невозможные утверждения типов интерфейсов
//   - loopclosure   — обнаруживает захват переменной цикла в замыканиях
//   - lostcancel    — проверяет, что context.CancelFunc всегда вызывается
//   - nilfunc       — обнаруживает сравнение функций с nil
//   - nilness       — обнаруживает разыменование nil-указателей
//   - printf        — проверяет форматные строки printf-подобных функций
//   - reflectvaluecompare — предупреждает о сравнении reflect.Value через ==
//   - shadow        — обнаруживает затенение переменных
//   - shift         — обнаруживает сдвиги на величину, превышающую ширину типа
//   - sigchanyzer   — проверяет аргументы signal.Notify
//   - slog          — проверяет корректное использование log/slog
//   - sortslice     — обнаруживает неправильное использование sort.Slice
//   - stdmethods    — проверяет сигнатуры методов стандартных интерфейсов
//   - stdversion    — предупреждает об использовании символов новее целевой версии Go
//   - stringintconv — обнаруживает подозрительные преобразования string(int)
//   - structtag     — проверяет синтаксис тегов структур
//   - testinggoroutine — обнаруживает вызов t.Fatal из горутины в тесте
//   - tests         — проверяет именование тестов, бенчмарков и примеров
//   - timeformat    — обнаруживает некорректные форматные строки времени
//   - unmarshal     — проверяет аргументы json.Unmarshal и подобных функций
//   - unreachable   — обнаруживает недостижимый код
//   - unsafeptr     — проверяет корректное использование unsafe.Pointer
//   - unusedresult  — проверяет, что результаты чистых функций используются
//   - unusedwrite   — обнаруживает запись в переменную, которая нигде не читается
//   - usesgenerics  — сообщает, использует ли пакет обобщённые типы
//   - waitgroup     — обнаруживает некорректное использование sync.WaitGroup
//
// ## Анализаторы класса SA пакета staticcheck.io
//
// Все анализаторы вида SA#### из пакета honnef.co/go/tools/staticcheck:
// SA1xxx — корректное использование стандартной библиотеки,
// SA2xxx — проблемы конкурентности,
// SA3xxx — тесты,
// SA4xxx — бесполезный код,
// SA5xxx — корректность,
// SA6xxx — производительность,
// SA9xxx — сомнительные конструкции.
//
// ## Анализаторы класса S (simple) пакета staticcheck.io
//
// Анализаторы вида S1### из пакета honnef.co/go/tools/simple —
// предлагают упрощения кода без изменения смысла.
//
// ## Анализаторы класса ST (stylecheck) пакета staticcheck.io
//
// Анализаторы вида ST1### из пакета honnef.co/go/tools/stylecheck —
// проверяют соответствие кода соглашениям Go-сообщества.
//
// ## Публичные анализаторы
//
//   - bodyclose (github.com/timakin/bodyclose) — проверяет, что тело HTTP-ответа
//     закрывается (resp.Body.Close()) во всех путях выполнения.
//   - nilerr (github.com/gostaticanalysis/nilerr) — обнаруживает ситуации, когда
//     функция проверяет ошибку на nil, но возвращает nil вместо ошибки.
//
// ## Собственный анализатор
//
//   - osexitcheck — запрещает прямой вызов os.Exit в функции main пакета main.
//     Подробнее: [osexitcheck].
package main

import (
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
	"golang.org/x/tools/go/analysis/passes/appends"
	"golang.org/x/tools/go/analysis/passes/asmdecl"
	"golang.org/x/tools/go/analysis/passes/assign"
	"golang.org/x/tools/go/analysis/passes/atomic"
	"golang.org/x/tools/go/analysis/passes/atomicalign"
	"golang.org/x/tools/go/analysis/passes/bools"
	"golang.org/x/tools/go/analysis/passes/buildtag"
	"golang.org/x/tools/go/analysis/passes/cgocall"
	"golang.org/x/tools/go/analysis/passes/composite"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/deepequalerrors"
	"golang.org/x/tools/go/analysis/passes/defers"
	"golang.org/x/tools/go/analysis/passes/directive"
	"golang.org/x/tools/go/analysis/passes/errorsas"
	"golang.org/x/tools/go/analysis/passes/framepointer"
	"golang.org/x/tools/go/analysis/passes/hostport"
	"golang.org/x/tools/go/analysis/passes/httpmux"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
	"golang.org/x/tools/go/analysis/passes/ifaceassert"
	"golang.org/x/tools/go/analysis/passes/loopclosure"
	"golang.org/x/tools/go/analysis/passes/lostcancel"
	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"golang.org/x/tools/go/analysis/passes/nilness"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/reflectvaluecompare"
	"golang.org/x/tools/go/analysis/passes/shadow"
	"golang.org/x/tools/go/analysis/passes/shift"
	"golang.org/x/tools/go/analysis/passes/sigchanyzer"
	"golang.org/x/tools/go/analysis/passes/slog"
	"golang.org/x/tools/go/analysis/passes/sortslice"
	"golang.org/x/tools/go/analysis/passes/stdmethods"
	"golang.org/x/tools/go/analysis/passes/stdversion"
	"golang.org/x/tools/go/analysis/passes/stringintconv"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/testinggoroutine"
	"golang.org/x/tools/go/analysis/passes/tests"
	"golang.org/x/tools/go/analysis/passes/timeformat"
	"golang.org/x/tools/go/analysis/passes/unmarshal"
	"golang.org/x/tools/go/analysis/passes/unreachable"
	"golang.org/x/tools/go/analysis/passes/unsafeptr"
	"golang.org/x/tools/go/analysis/passes/unusedresult"
	"golang.org/x/tools/go/analysis/passes/unusedwrite"
	"golang.org/x/tools/go/analysis/passes/usesgenerics"
	"golang.org/x/tools/go/analysis/passes/waitgroup"

	"honnef.co/go/tools/simple"
	"honnef.co/go/tools/staticcheck"
	"honnef.co/go/tools/stylecheck"

	"github.com/gostaticanalysis/nilerr"
	"github.com/timakin/bodyclose/passes/bodyclose"

	"github.com/gearwheels/go_url_shortener/cmd/staticlint/osexitcheck"
)

func main() {
	checks := []*analysis.Analyzer{
		// Стандартные анализаторы passes
		appends.Analyzer,
		asmdecl.Analyzer,
		assign.Analyzer,
		atomic.Analyzer,
		atomicalign.Analyzer,
		bools.Analyzer,
		buildtag.Analyzer,
		cgocall.Analyzer,
		composite.Analyzer,
		copylock.Analyzer,
		deepequalerrors.Analyzer,
		defers.Analyzer,
		directive.Analyzer,
		errorsas.Analyzer,
		framepointer.Analyzer,
		hostport.Analyzer,
		httpresponse.Analyzer,
		httpmux.Analyzer,
		ifaceassert.Analyzer,
		loopclosure.Analyzer,
		lostcancel.Analyzer,
		nilfunc.Analyzer,
		nilness.Analyzer,
		printf.Analyzer,
		reflectvaluecompare.Analyzer,
		shadow.Analyzer,
		shift.Analyzer,
		sigchanyzer.Analyzer,
		slog.Analyzer,
		sortslice.Analyzer,
		stdmethods.Analyzer,
		stdversion.Analyzer,
		stringintconv.Analyzer,
		structtag.Analyzer,
		testinggoroutine.Analyzer,
		tests.Analyzer,
		timeformat.Analyzer,
		unmarshal.Analyzer,
		unreachable.Analyzer,
		unsafeptr.Analyzer,
		unusedresult.Analyzer,
		unusedwrite.Analyzer,
		usesgenerics.Analyzer,
		waitgroup.Analyzer,

		// Публичные анализаторы
		bodyclose.Analyzer,
		nilerr.Analyzer,

		// Собственный анализатор
		osexitcheck.Analyzer,
	}

	// Все SA-анализаторы staticcheck
	for _, a := range staticcheck.Analyzers {
		if strings.HasPrefix(a.Analyzer.Name, "SA") {
			checks = append(checks, a.Analyzer)
		}
	}

	// S-анализаторы (simple) — упрощения кода
	for _, a := range simple.Analyzers {
		checks = append(checks, a.Analyzer)
	}

	// ST-анализаторы (stylecheck) — стилевые проверки
	for _, a := range stylecheck.Analyzers {
		checks = append(checks, a.Analyzer)
	}

	multichecker.Main(checks...)
}
