//go:build tools

// Файл-якорь документации. Исключён из обычной сборки build-тегом `tools`,
// поэтому корневой каталог не образует package main без main().
//
// Кодогенерация выполняется через Makefile (надёжнее go:generate):
//
//	make generate   # sqlc + oapi-codegen
//	make migrate    # tern
//
// Версии инструментов закреплены в Makefile.
package tools
