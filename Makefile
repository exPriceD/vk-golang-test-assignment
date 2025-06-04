BINARY=vk-golang-test-assignment
MAIN=cmd/main.go
CONFIG=config/config.yaml
GO=go

ifeq ($(OS),Windows_NT)
	EXT=.exe
	RM=del /F /Q
	DEVNULL=NUL
else
	EXT=
	RM=rm -f
	DEVNULL=/dev/null
endif

.PHONY: all deps build run test tidy clean help

all: deps build run

deps:
	@echo Установка зависимостей...
	@$(GO) mod tidy > $(DEVNULL) 2>&1
	@$(GO) mod download > $(DEVNULL) 2>&1

build: deps
	@echo Сборка бинарника...
	@$(GO) vet $(MAIN) > $(DEVNULL) 2>&1
	@$(GO) build -o $(BINARY)$(EXT) $(MAIN) > $(DEVNULL) 2>&1

run: build
	@echo Запуск приложения...
	@./$(BINARY)$(EXT) -config=$(CONFIG)
	@$(MAKE) clean

test:
	@echo Запуск тестов
	@$(GO) test -v ./...

clean:
	@echo Очистка бинарника и кэша...
	@-$(RM) $(BINARY)$(EXT) > $(DEVNULL) 2>&1
	@$(GO) clean -cache -modcache > $(DEVNULL) 2>&1

help:
	@echo Доступные команды:
	@echo   make         — Полная сборка и запуск
	@echo   make build   — Собрать бинарник
	@echo   make run     — Запустить приложение
	@echo   make test    — Запустить тесты
	@echo   make deps    — Установить зависимости
	@echo   make clean   — Очистить кэш