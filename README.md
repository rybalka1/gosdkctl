# gosdkctl

`gosdkctl` — менеджер Go SDK без root-доступа. Хранит версии Go в `~/sdk`, отслеживает SDK по умолчанию через `~/sdk/go-current` и не пишет в `/usr/local` или другие каталоги, принадлежащие root.

Рабочий процесс намеренно похож на инструменты вроде `uv`: один небольшой бинарник отвечает за установку, обнаружение, переключение и диагностику локальной среды разработчика.

## Быстрый старт Linux x86_64

На чистой Linux x86_64-системе Go вручную ставить не нужно. Скачайте готовый бинарник из первого релиза:

```bash
curl -L -o gosdkctl https://github.com/rybalka1/gosdkctl/releases/download/v1.0.0/gosdkctl-linux-amd64
chmod +x gosdkctl
./gosdkctl self install
~/.local/bin/gosdkctl init zsh
~/.local/bin/gosdkctl install latest
exec zsh
```

Для bash замените `init zsh` и `exec zsh`:

```bash
~/.local/bin/gosdkctl init bash
exec bash
```

После этого доступны:

```bash
gosdkctl current
go version
```

## Структура каталогов

```text
~/sdk/
  go1.24.2/
  go1.25.1/
  go1.26.0/
  go-current -> /home/rybalka/sdk/go1.26.0
```

## Команды

```text
gosdkctl status
gosdkctl list
gosdkctl current
gosdkctl install <archive.tar.gz|goX.Y.Z|latest>
gosdkctl migrate-local
gosdkctl init [zsh|bash|auto]
gosdkctl self install
gosdkctl switch <goX.Y.Z>
gosdkctl switch
gosdkctl choose
gosdkctl doctor
gosdkctl env [goX.Y.Z|path|default]
```

`switch` без аргумента работает как `choose` и запрашивает версию из списка установленных.

## Установка Go SDK

Установить последнюю стабильную версию Go:

```bash
gosdkctl install latest
```

Установить конкретную версию:

```bash
gosdkctl install go1.26.1
```

Или установить заранее скачанный архив:

```bash
gosdkctl install ~/Downloads/go1.26.1.linux-amd64.tar.gz
```

Для загрузки с `go.dev` команда выбирает архив под текущие `GOOS/GOARCH` и проверяет `sha256` из официального download metadata. Затем распаковывает архив в `~/sdk/go1.26.1`, проверяет `go/VERSION` и `go/bin/go`, обновляет `~/sdk/go-current`. Существующие каталоги SDK не удаляются. Повторная установка той же версии идемпотентна: существующий SDK переиспользуется и становится основным.

## Миграция старого `~/.local/go`

Если старый Go был установлен в `~/.local/go`, его можно явно перенести в `~/sdk/goX.Y.Z`:

```bash
gosdkctl migrate-local
```

Команда читает версию из `~/.local/go/VERSION`, переносит каталог в `~/sdk/<version>` и обновляет `~/sdk/go-current`. Если такая версия уже есть в `~/sdk`, каталог не перезаписывается, а существующий SDK становится основным.

## Переключение SDK по умолчанию

```bash
gosdkctl switch go1.24.2
gosdkctl current
```

Обновляется только `~/sdk/go-current`. Уже открытые оболочки потребуют обновления окружения.

## Временное переключение в оболочке

Сначала установите managed block в конфиг оболочки:

```bash
gosdkctl init zsh
```

Для bash:

```bash
gosdkctl init bash
```

Команда полностью переписывает только блок между маркерами `# >>> gosdkctl init >>>` и `# <<< gosdkctl init <<<` в `~/.zshrc` или `~/.bashrc`. Остальной пользовательский конфиг не меняется.

После этого в новых shell-сессиях доступны `go`, `gosdkctl`, `go-sdk`, `usego`, `gosetdefault` и `gocurrent`.

Managed block выбирает `GOROOT` по каскаду: сначала `~/sdk/go-current`, затем legacy `~/.local/go`, затем самая новая версия `goX.Y.Z` из `~/sdk`.

Бинарник не может напрямую изменить уже запущенную родительскую оболочку, поэтому `gosdkctl env` также умеет выводить команды экспорта:

```bash
eval "$(gosdkctl env go1.24.2)"
eval "$(gosdkctl env default)"
eval "$(gosdkctl env /opt/custom-go)"
```

Managed block добавляет такие хелперы:

```bash
usego() {
  eval "$(gosdkctl env "${1:-default}")"
}

gosetdefault() {
  gosdkctl switch "$1"
  usego default
}

gocurrent() {
  gosdkctl current
  which go
  go version
}
```

## Диагностика

```bash
gosdkctl doctor
```

Отчёт включает `GOROOT`, `GOPATH`, `PATH`, целевой каталог `go-current`, наличие устаревшего `~/.local/go`, видимость бинарника `go` в `PATH` и установленные версии SDK.

## Сборка

Сборка из исходников нужна только для разработки самого `gosdkctl`. Для bootstrap чистой машины используйте готовый бинарник из GitHub Releases.

```bash
go build -o ~/.local/bin/gosdkctl ./cmd/gosdkctl
```

## Self install

Если бинарник уже запущен из временного места, он может сам установить себя в стандартный пользовательский путь:

```bash
gosdkctl self install
```

Команда создает `~/.local/bin/gosdkctl` и совместимый symlink `~/.local/bin/go-sdk`.

## Bootstrap на чистой системе

Минимальный сценарий после получения первого бинарника из GitHub Releases:

```bash
./gosdkctl self install
~/.local/bin/gosdkctl init zsh
~/.local/bin/gosdkctl install latest
exec zsh
```

Для bash:

```bash
./gosdkctl self install
~/.local/bin/gosdkctl init bash
~/.local/bin/gosdkctl install latest
exec bash
```
