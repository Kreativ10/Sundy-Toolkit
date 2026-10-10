<div align="center">

# Sundy Toolkit

**Единый центр управления Linux-сервером.**

Диагностика · Восстановление конфигурации · Управление службами · Minecraft

[![CI](https://github.com/Kreativ10/Sundy-Toolkit/actions/workflows/ci.yml/badge.svg)](https://github.com/Kreativ10/Sundy-Toolkit/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.23%2B-00ADD8?logo=go&logoColor=white)](go.mod)
[![Linux](https://img.shields.io/badge/Platform-Linux-F97316?logo=linux&logoColor=white)](#совместимость)
[![Лицензия](https://img.shields.io/badge/License-MIT-64748B)](LICENSE)

[English](README.md) · **Русский**

[Быстрый старт](#быстрый-старт) · [Minecraft](#minecraft) · [Команды](#команды) · [Разработка](#разработка)

</div>

---

Sundy Toolkit — CLI для администрирования Linux с оранжевой интерактивной панелью. Команда `sundy` открывает обзор системы, диагностику, установку поддерживаемых пакетов, управление Minecraft и сохранение выбранных конфигураций.

| Раздел | Возможности |
| :--- | :--- |
| **Диагностика** | Обзор хоста, JSON, аудит системы, проверка сети и упавших служб |
| **Исправление** | Выбор контролируемых действий; проверка известных конфигураций перед перезапуском |
| **Восстановление** | Выборочные снимки конфигурации системы и сети; применение через `--apply` |
| **Установка** | Инструменты администратора, веб-серверы, СУБД, Docker, VPN и другие пресеты |
| **Minecraft** | Новый Vanilla или существующий JAR-сервер; службы и отключаемая консоль |
| **Pterodactyl** | Новая стабильная Panel 1.x на Ubuntu 24.04 или подготовка узла Wings |
| **Поддержка** | Диагностический архив с маскированием секретов; обновление с проверкой SHA-256 |

## Быстрый старт

### Установка из релиза

Нужен GitHub Release с бинарным файлом для вашей архитектуры и `checksums.txt`.

```sh
curl -fsSL https://raw.githubusercontent.com/Kreativ10/Sundy-Toolkit/main/installer/install.sh | sh
sundy
```

Установщик проверяет SHA-256 и копирует бинарник в `/usr/local/bin`, при необходимости запрашивая `sudo`. Для установки в домашний каталог:

```sh
curl -fsSL https://raw.githubusercontent.com/Kreativ10/Sundy-Toolkit/main/installer/install.sh \
  | SUNDY_INSTALL_DIR="$HOME/.local/bin" sh
"$HOME/.local/bin/sundy" version
```

Чтобы выбрать релиз, передайте `SUNDY_VERSION=v0.1.0` **команде `sh` после символа `|`**. Переменная `SUNDY_REPO=owner/repository` задаёт другой репозиторий релизов. При нестандартном расположении добавьте каталог установки в `PATH`.

### Обновление с 0.1.1, если `update` возвращает 404

Updater в 0.1.1 использует старый адрес репозитория. Замените бинарник установщиком из актуального репозитория. Для стандартной установки выполните от root:

```sh
curl -fsSL https://raw.githubusercontent.com/Kreativ10/Sundy-Toolkit/main/installer/install.sh \
  | SUNDY_VERSION=v0.1.2 sh
hash -r
/usr/local/bin/sundy version
/usr/local/bin/sundy apps
```

Удалять старую установку не требуется. Установщик проверяет SHA-256 и заменяет только бинарник; реестр `/var/lib/sundy/apps.json`, каталоги серверов и миры остаются на месте. Перед заменой сохраните копию реестра; миры требуют отдельных резервных копий. Если бинарник установлен в другой каталог, передайте `SUNDY_INSTALL_DIR` с тем же путём: службы используют абсолютный путь к нему. Работающие supervisor продолжают использовать старую версию до перезапуска службы; перезапускайте серверы по одному после проверки их логов.

Если список пуст, проверьте запуск от root и значение `SUNDY_STATE_DIR`: другой каталог состояния показывает другой реестр. Обновление бинарника не восстанавливает уже потерянные записи. Сохраните текущий реестр и изучите существующие службы и каталоги перед повторной регистрацией; не создавайте новый Vanilla в каталоге существующего мира.

### Сборка из исходников

Требуются Go 1.23+ и Make. Для запуска собранного бинарника установка Go не нужна.

```sh
git clone https://github.com/Kreativ10/Sundy-Toolkit.git
cd Sundy-Toolkit
make check
make build
./bin/sundy
```

Перед созданием управляемых служб установите бинарник по постоянному пути:

```sh
sudo install -m 0755 bin/sundy /usr/local/bin/sundy
sudo sundy install minecraft
```

Служба сохраняет абсолютный путь к бинарнику. Не удаляйте его после установки. `go run` создаёт временный исполняемый файл и не подходит для установки постоянной Minecraft-службы.

## Minecraft

### Создание и регистрация

```sh
sudo sundy install minecraft
```

Выберите **Direct / console managed**, затем новый Vanilla или существующий каталог сервера. Мастер запрашивает уникальное имя, каталог, порт, максимальный heap, исполняемый файл Java, автозапуск, перезапуск после сбоя и явное принятие [Minecraft EULA](https://aka.ms/MinecraftEULA).

- Для Vanilla используются манифест Mojang, указанная в нём версия Java, размер загрузки и SHA-1. Каталог нового сервера должен быть пустым.
- При регистрации существующего сервера мир и остальные настройки сохраняются. Sundy меняет выбранный порт и, после согласия, EULA. При неоднозначном определении JAR укажите его имя самостоятельно.
- Имя содержит 1–48 латинских букв, цифр, `_` или `-` и начинается с буквы или цифры. Занятые имена, каталоги и порты повторно использовать нельзя.
- Память задаётся целым числом с `M` или `G`: например, `2048M` или `2G`. Начальный heap ограничен 512M. Оставляйте память ОС, нативным структурам Java и другим службам.
- Требование Java для Vanilla берётся из метаданных Mojang. Minecraft 1.20.5 требует Java 21, а 26.1 — Java 25. При слишком старой установленной Java мастер сообщает нужную версию. [Примечания 1.20.5](https://www.minecraft.net/pt-pt/article/minecraft-java-edition-1-20-5), [примечания 26.1](https://feedback.minecraft.net/hc/en-us/articles/42011663817357-Minecraft-Java-Edition-26-1-Snapshot-1).
- Для существующего JAR минимальная версия Java определяется по встроенным метаданным и версии класса запуска. Модпакам может требоваться конкретная Java, загрузчик или отдельный скрипт. Поддерживается непосредственный запуск `java -jar … nogui`.
- Если установлено несколько Java, выберите абсолютный путь к нужной. Автоматическая установка пакета Java выполняется только при отсутствии команды `java`; доступность пакета зависит от дистрибутива.

### Управление экземпляром

```sh
sudo sundy apps
sudo sundy minecraft status survival
sudo sundy minecraft start survival
sudo sundy minecraft console survival
sudo sundy minecraft stop survival
sudo sundy minecraft restart survival
```

Команда `:detach` отключает консоль, сохраняя сервер работающим. Команда `stop` внутри консоли останавливает сам Minecraft. При остановке службы supervisor запрашивает штатное завершение и ждёт до 90 секунд перед принудительным завершением.

```text
systemd / OpenRC → Sundy supervisor → Java-сервер
                         ↕
                  консоль через Unix-сокет
```

Блокировка supervisor предотвращает повторный запуск. Ограниченные очереди вывода не позволяют зависшему терминалу заблокировать сервер. При включённом перезапуске systemd использует `on-failure`, а OpenRC — `supervise-daemon` с ограничением частоты повторных запусков. См. [руководство OpenRC](https://github.com/OpenRC/openrc/blob/master/man/openrc-run.8).

### Диагностика неудачного запуска

```sh
sudo sundy minecraft status survival
sudo tail -n 100 /var/lib/sundy/minecraft/survival/console.log
# На systemd:
sudo journalctl -u sundy-minecraft-survival.service -n 100 --no-pager
```

Проверьте выбранную Java, EULA, путь к JAR, занятость порта и лимиты памяти. OOM-завершение возможно даже при heap меньше доступной памяти: Java использует память и за его пределами. Если настройка службы завершилась ошибкой после регистрации, Sundy сообщает сохранённое имя и причину; изучите состояние службы перед повторным запуском.

## Команды

| Задача | Команда |
| :--- | :--- |
| Интерактивная панель | `sundy` |
| Обзор системы | `sundy overview [--json]` |
| Быстрый / полный аудит | `sundy audit [--full] [--json]` |
| Выбрать исправления | `sudo sundy audit --full --fix` |
| Диагностика состояния | `sundy doctor` |
| Интерфейсы, маршруты, DNS, сокеты | `sundy network info` |
| Сохранить настройки сети | `sudo sundy network save` |
| Восстановить настройки сети | `sudo sundy network restore NAME --apply` |
| Список / просмотр снимков | `sundy snapshots` / `sundy snapshot show NAME` |
| Сохранить конфигурацию системы | `sudo sundy snapshot create` |
| Восстановить выбранные файлы | `sudo sundy snapshot restore NAME --components firewall,docker --apply` |
| Список / установка пресетов | `sundy install list` / `sudo sundy install PRESET` |
| Мастер Pterodactyl | `sudo sundy install pterodactyl` |
| Действия со службами | `sudo sundy service status|start|stop|restart|enable|disable NAME` |
| Диагностический архив | `sundy report --anonymous [--out FILE]` |
| Обновить бинарник | `sudo sundy update` |

Панель показывает ошибки вложенных операций. Мастер поддерживает вставку и передачу ответов через pipe; конец ввода не считается согласием на действие. Рамки учитывают ширину терминала, Unicode и цветовые ANSI-коды. `NO_COLOR=1` отключает оформление; `COLUMNS` задаёт ширину, когда определить её через терминал невозможно.

## Совместимость

| Компонент | Поддержка |
| :--- | :--- |
| Бинарники релизов | Linux amd64, arm64, ARMv7, riscv64; статическая сборка |
| Адаптеры пакетов | APT, DNF, YUM, Pacman, Zypper, APK, XBPS, emerge, ограниченный Nix |
| Адаптеры служб | systemd, OpenRC; базовые start/stop/restart/status для runit |
| Minecraft-службы | systemd или OpenRC; запускаемый JAR и совместимая Java |
| Нативная Panel | Только новая установка; Ubuntu 24.04 + systemd; стабильная версия 1.x |
| Пресет Wings | Ubuntu/Debian + systemd; amd64 или arm64; затем нужна настройка узла |

Наличие адаптера не гарантирует доступность пакетов каждого пресета во всех дистрибутивах. Для некоторых СУБД нужна отдельная инициализация по правилам дистрибутива. Ошибки служб сообщаются пользователю и не считаются успешной установкой.

Установщик Panel отказывается перезаписывать существующую установку или повторно генерировать её ключ приложения. Он проверяет checksum установщика Composer, настраивает локальный HTTP, PHP-FPM, MariaDB, Redis, NGINX, cron и обработчик очередей; телеметрия Panel отключена. До публикации создайте первый аккаунт, настройте TLS и соответствующий URL приложения. Перед установкой проверьте существующие настройки NGINX и его стандартного сайта. [Проверка Composer](https://getcomposer.org/doc/faqs/how-to-install-composer-programmatically.md), [команда настройки Panel](https://github.com/pterodactyl/panel/blob/1.0-develop/app/Console/Commands/Environment/AppSettingsCommand.php).

## Восстановление и диагностические данные

Снимки сохраняют выбранную **конфигурацию**, а не миры Minecraft, базы приложений или диски целиком. Для них нужны отдельные резервные копии. Компоненты SSH и Minecraft в системных снимках предназначены для ручного восстановления, а список пакетов — для справки. Системное восстановление возвращает файлы конфигурации, но не всё текущее состояние firewall и служб.

Для восстановления сети нужен `--apply`. Перед применением создаётся аварийный снимок, затем перезагружается поддерживаемый активный сетевой backend и проверяется связь. При ошибке применения или проверки выполняется попытка отката. Восстановление нельзя гарантировать при потере питания, внешних изменениях или обрыве SSH.

| Данные | Путь по умолчанию |
| :--- | :--- |
| Системное состояние и реестр | `/var/lib/sundy/` |
| Снимки конфигурации | `/var/lib/sundy/snapshots/` |
| Логи консоли Minecraft | `/var/lib/sundy/minecraft/NAME/console.log` |
| Unix-сокеты Minecraft | `/run/sundy/minecraft/` |
| Локальное состояние обычного пользователя | `~/.local/share/sundy/` |

`SUNDY_STATE_DIR` и `SUNDY_RUNTIME_DIR` переопределяют пути состояния и сокетов. Создаваемые Minecraft-службы сохраняют выбранные пути. Для приложений и снимков, созданных от root, последовательно используйте `sudo`.

Диагностические архивы маскируют распространённые шаблоны секретов в текстовых и JSON-данных. Анонимный режим также скрывает имя хоста и IP-адреса, кроме loopback. Просматривайте архив перед отправкой: шаблоны не распознают все специфичные для приложений секреты. Архивы и реестр имеют закрытые права доступа. Подробнее — [модель безопасности](docs/SECURITY-MODEL.md).

## Разработка

```sh
make check       # vet, race-тесты, синтаксис shell, автономная проверка установщика
make smoke       # сборка и проверка команд чтения
make dist VERSION=0.1.0
```

Необязательный тест настоящего Minecraft скачивает официальный сервер и принимает EULA только во временном тестовом окружении на localhost:

```sh
SUNDY_MC_E2E_VERSION=latest go test -tags=integration \
  -run TestRealVanillaLifecycle -v ./internal/minecraft -timeout 8m
```

Проверка установки и жизненного цикла OpenRC в одноразовом контейнере:

```sh
make build
docker build -f scripts/integration/Dockerfile.openrc -t sundy-openrc-test .
docker run --rm --memory=3g --cpus=2 sundy-openrc-test
```

См. [результаты и ограничения проверок](docs/VALIDATION.md), [архитектуру](docs/ARCHITECTURE.md), [правила участия](CONTRIBUTING.md) и [историю изменений](CHANGELOG.md).

```text
cmd/sundy/           CLI и панель
internal/minecraft/  установка, Java, загрузки, службы, supervisor и консоль
internal/ui/         терминальная разметка и ввод
internal/platform/   адаптеры пакетов и служб
internal/snapshot/   снимки конфигурации и восстановление
internal/audit/      диагностика и контролируемые исправления
internal/install/    пресеты и Pterodactyl
internal/apps/       реестр управляемых приложений
internal/report/     диагностические архивы с маскированием секретов
internal/selfupdate/ обновление бинарника
installer/           установщик и удаление
scripts/             smoke-тесты, тест установщика и контейнерная проверка
```

Тег `v*` запускает сборку релизов и генерацию checksum. Установщик и самообновление проверяют checksum из релиза; криптографическая подпись релиза не проверяется. Скрипт удаления убирает только бинарник: данные и службы остаются, а для их работы нужен исполняемый файл Sundy.

## Лицензия

[MIT](LICENSE) · Sundy Systems, 2026.
