package main

import _ "embed"

// newsPhoto — картинка-шапка дайджеста, вшита в бинарник, чтобы не зависеть от
// наличия файла рядом с исполняемым (install.sh assets/ не копирует).
//
//go:embed assets/news.jpg
var newsPhoto []byte
