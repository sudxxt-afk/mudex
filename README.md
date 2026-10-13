# tui-player — неоновый консольный плеер на Go + Bubble Tea

Работает в любом Linux-терминале: обложка рисуется клетками (`▀` + ANSI),
без Kitty Graphics / Sixel. Неоновый FFT-визуализатор, чистый Go.

## Установка

```sh
git clone https://github.com/sudxxt-afk/mudex.git
cd mudex
go build -o mudex ./cmd/player
```

Требуется Go 1.24+ (см. `go.mod`). Опционально `ffmpeg`/`ffprobe`
для m4a/opus/aac/wma и точных длительностей при скане.

## Форматы

- Нативно (без зависимостей): `mp3 / flac / wav / ogg`
- Через `ffmpeg` (если стоит): `m4a / opus / aac / wma` + всё остальное
- Звук: `miniaudio` (malgo) — собираются без `alsa-lib-devel`, `sudo` не нужен
- Стриминг: файлы не декодируются целиком — старт за ~15мс, seek за ~4мс, RAM почти не растёт
- Длительности при скане берёт `ffprobe` (если стоит)

## Запуск

```sh
cd ~/Музыка
mudex                # браузер с текущей папки
mudex ~/Музыка       # браузер с указанной папки
```

Имя бинарника релиза — `mudex` (локальная сборка из `go build`
по умолчанию даёт `player`, оба имени эквивалентны).

Сборка:

```sh
cd mudex
go build -o mudex ./cmd/player
```

## Управление

- `j/k` или `↑/↓` — навигация, `Enter` — войти в папку / играть, `Backspace`/`h`/`←` — наверх
- `Space` — пауза/продолжить, `n/p` — следующий/предыдущий трек в папке
- `,`/`.` — seek ∓5с, `+ / -` — громкость
- `s` — shuffle, `r` — repeat, `v` — режим визуализатора, `t/T` — тема
- `Ctrl+P` — настройки, `/` — фильтр, `R` — пересканировать папку, `q` — выход

## Как это устроено

- `internal/library` — скан папки, фильтр расширений
- `internal/player` — декод в буфер 44100 stereo + tap для визуализатора,
  вывод через malgo f32
- `internal/cover` — embedded APIC (mp3) → `cover.jpg/folder.jpg` →
  half-block рендер с Bayer-дизерингом (truecolor/256) → neon ASCII-заглушка
- `internal/visualizer` — Hann + FFT (1024) → 24 бара log-scale,
  attack/release сглаживание, 15 fps
- `internal/ui` — Bubble Tea model: список слева, обложка+мета+визуализатор
  справа, прогресс снизу; перерисовка обложки только при смене трека
