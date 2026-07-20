# gosdkctl

`gosdkctl` — менеджер Go SDK без root-доступа. Хранит версии Go в `~/sdk`, отслеживает SDK по умолчанию через `~/sdk/go-current` и не пишет в `/usr/local` или другие каталоги, принадлежащие root.

Рабочий процесс намеренно похож на инструменты вроде `uv`: один небольшой бинарник отвечает за установку, обнаружение, переключение и диагностику локальной среды разработчика.

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
gosdkctl install <archive.tar.gz>
gosdkctl migrate-local
gosdkctl switch <goX.Y.Z>
gosdkctl switch
gosdkctl choose
gosdkctl doctor
gosdkctl env [goX.Y.Z|path|default]
```

`switch` без аргумента работает как `choose` и запрашивает версию из списка установленных.

## Установка Go SDK

Скачайте архив Go и установите его без root:

```bash
gosdkctl install ~/Downloads/go1.26.1.linux-amd64.tar.gz
```

Команда распаковывает архив в `~/sdk/go1.26.1`, проверяет `go/VERSION` и `go/bin/go`, обновляет `~/sdk/go-current`. Существующие каталоги SDK не удаляются. Повторная установка того же архива идемпотентна: существующий SDK переиспользуется и становится основным.

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

Бинарник не может напрямую изменить родительскую оболочку, поэтому `gosdkctl env` выводит команды экспорта:

```bash
eval "$(gosdkctl env go1.24.2)"
eval "$(gosdkctl env default)"
eval "$(gosdkctl env /opt/custom-go)"
```

Рекомендуемые хелперы для zsh:

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

Добавьте настройку окружения по умолчанию перед любым ранним `return` в `~/.zshrc`:

```bash
export GOPATH=$HOME/go

if [[ -z "$GOROOT" ]]; then
  if [[ -L "$HOME/sdk/go-current" && -x "$HOME/sdk/go-current/bin/go" ]]; then
    export GOROOT="$HOME/sdk/go-current"
  elif [[ -d "$HOME/.local/go" ]]; then
    export GOROOT="$HOME/.local/go"
  else
    _latest_go_sdk=$(command find "$HOME/sdk" -maxdepth 1 -mindepth 1 -type d -name 'go[0-9]*' -exec basename {} \; 2>/dev/null | sort -V 2>/dev/null | tail -n 1)
    if [[ -n "$_latest_go_sdk" ]]; then
      export GOROOT="$HOME/sdk/$_latest_go_sdk"
    fi
    unset _latest_go_sdk
  fi
fi

typeset -U path PATH
path=(
  ${GOROOT:+$GOROOT/bin}
  $GOPATH/bin
  $HOME/sdk
  $HOME/.local/bin
  $path
)
```

## Диагностика

```bash
gosdkctl doctor
```

Отчёт включает `GOROOT`, `GOPATH`, `PATH`, целевой каталог `go-current`, наличие устаревшего `~/.local/go`, видимость бинарника `go` в `PATH` и установленные версии SDK.

## Сборка

```bash
go build -o ~/.local/bin/gosdkctl ./cmd/gosdkctl
```

Чтобы добавить команду в PATH:

```bash
mkdir -p ~/.local/bin
go build -o ~/.local/bin/gosdkctl ./cmd/gosdkctl
ln -sf ~/.local/bin/gosdkctl ~/.local/bin/go-sdk
```
