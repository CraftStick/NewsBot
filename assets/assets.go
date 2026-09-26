// Package assets — файлы, вшитые в бинарник.
package assets

import _ "embed"

// NewsPhoto — картинка-шапка дайджеста, вшита в бинарник, чтобы не зависеть от
// наличия файла рядом с исполняемым (install.sh assets/ не копирует).
//
//go:embed news.jpg
var NewsPhoto []byte
