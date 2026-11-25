# Домашнее задание 2 — микросервис пользователей

Go 1.22+, Gorilla Mux, MinIO S3, Prometheus и token bucket 1000 запросов/с.
Полный отчёт находится рядом с проектом в архиве.

## Запуск с Docker

```sh
docker compose up -d --build
docker compose ps
curl -f http://localhost:8080/healthz
curl -f http://localhost:8080/api/integration/status
```

MinIO: http://localhost:9001 (minioadmin / minioadmin123), Prometheus: http://localhost:9090.
Для учебного локального запуска заданы значения по умолчанию. Для облака задайте собственные
MINIO_ENDPOINT (хост:порт без схемы), MINIO_ACCESS_KEY, MINIO_SECRET_KEY, MINIO_BUCKET,
MINIO_SECURE=true. Облачное развёртывание и доступ к реальному облаку не проверялись.
Конфигурация образов зафиксирована для совместимости с Go 1.22 и не является набором актуальных производственных версий.
HTTP_ADDR=:8080; RATE_BURST=5000; S3_ENABLED=true. Скорость всегда 1000/с на процесс.

MinIO собирается из официального исходного кода фиксированного релиза
RELEASE.2024-10-13T13-34-11Z: старые ссылки на готовые community-образы недоступны.
Сборка описана в deploy/minio.Dockerfile, лицензия upstream включается в образ.
Базовые образы Go и Alpine берутся из официального зеркала Docker Library в ECR,
Prometheus — из quay.io/prometheus. Первая сборка MinIO может занять несколько минут.

Внутри Compose MinIO создаёт постоянный том; приложение ждёт S3 до 30 секунд и при
неуспешном старте перезапускается. Порт метрик не ограничивается limiter, как и healthz.
У /healthz роль liveness, состояние S3 проверяет /api/integration/status.

## CRUD

```sh
curl -i -X POST http://localhost:8080/api/users -H 'Content-Type: application/json' -d '{"name":"Alice","email":"alice@example.org"}'
curl -i http://localhost:8080/api/users
curl -i http://localhost:8080/api/users/1
curl -i -X PUT http://localhost:8080/api/users/1 -H 'Content-Type: application/json' -d '{"name":"Alice Updated","email":"alice@example.org"}'
curl -i -X DELETE http://localhost:8080/api/users/1
```

POST → 201 и Location; GET/PUT → 200; DELETE → 204; отсутствующий ID → 404;
ошибочный JSON/ID/email → 400; исчерпанный лимит → 429 с Retry-After; полная очередь → 503.
ID назначает сервер. В PUT передаются name/email без id. Лимит тела 4096 байт;
неизвестные поля, null и несколько JSON-объектов отклоняются.

## Хранение и конкурентность

Основные данные находятся в памяти под RWMutex и очищаются при перезапуске.
Для каждой мутации формируется неизменяемое событие с sequence и снимком пользователя.
Один worker читает ограниченный канал из 10000 событий, пишет audit, сохраняет JSON
в `users/events/<run-id>/<sequence>.json`, затем выводит уведомление в журнал.
Это локальный адаптер уведомлений, SMTP/вебхук не реализованы.
Событие DELETE сохраняет факт удаления, старые события остаются в архиве.
В MinIO Console откройте bucket users → events → run-id; JSON содержит action,
user.id, user.name, user.email, sequence, time. Проверяйте CREATE, UPDATE, DELETE.
run-id случаен на каждый запуск; sequence начинается с 1.

Очередь принимается под той же блокировкой, что и изменение map. Если она полна,
запрос получает 503, данные не меняются. На S3 делается до 3 попыток по 3 секунды
с тем же ключом. После исчерпания попыток ошибка журналируется и растёт
async_backup_failures_total. Повторного воспроизведения после рестарта нет.
Это асинхронный архив событий, не транзакционная БД и не надёжный outbox.
Ответ 201/200/204 означает фиксацию в памяти и принятие в очередь, а не запись в S3.
При SIGTERM сервер прекращает приём, завершает запросы и дренирует очередь;
Docker может прервать длительный дренаж после stop_grace_period=120s.

## Метрики

```sh
curl http://localhost:8080/metrics
```

- RPS: `sum(rate(http_requests_total[1m]))`
- Успешный RPS: `sum(rate(http_requests_total{code=~"2.."}[1m]))`
- Средняя latency: `sum(rate(http_request_duration_seconds_sum[1m])) / sum(rate(http_request_duration_seconds_count[1m]))`
- p95: `histogram_quantile(0.95, sum by (le) (rate(http_request_duration_seconds_bucket[1m])))`
- Ошибки: `sum(rate(http_errors_total[1m]))`
- Фоновые операции: `async_queue_depth`, `async_processed_total`, `async_backup_failures_total`.

endpoint — шаблон маршрута (/api/users/{id}), method ограничен известными HTTP-методами.
Метрики считают также 429, 404 и 405; /metrics и /healthz не входят в статистику API.
Завершившиеся HTTP-ошибки учитываются в middleware; ошибки S3 обрабатывает worker.

## Проверки

```sh
go mod download
go build ./...
go vet ./...
go test -count=1 -v ./...
go test -race ./...  # нужен C-компилятор, на Windows — MinGW-w64
```

Тесты покрывают CRUD и валидацию, конкурентные мутации, отказ очереди без изменения
состояния, повторы worker, метрики и limiter. S3 тест использует реальный minio-go
и локальный HTTP-стенд, проверяя подписанный PutObject. Это не запуск настоящего MinIO.

Отдельная проверка с настоящим MinIO после старта Compose:

```sh
RUN_E2E=1 go test -count=1 -v ./integration
```

Этот тест выполняет CRUD через работающий HTTP API и читает архивные JSON-объекты
через S3, проверяя последовательность CREATE → UPDATE → DELETE и снимки данных.
Без RUN_E2E=1 он пропускается. API_URL и MINIO_ENDPOINT при необходимости задают
адреса стенда; MINIO_ACCESS_KEY и MINIO_SECRET_KEY — его учебные S3-реквизиты.

Локальный режим без S3:

```powershell
$env:S3_ENABLED='false'
go run .
```

## Нагрузка и доказательства

На Linux/macOS с установленными Docker, curl, wrk:

```sh
sh scripts/load.sh
```

Скрипт выполняет исходный прогон без пауз и сохраняет результаты в evidence.
На ВМ kubuntu_24 этот сценарий воспроизводит перегрузку; исходные журналы сохранены
как vm-wrk-default.txt. Снимки окончательного теста включены в отчёт.

Для финального управляемого профиля на Linux:

```sh
sudo sh scripts/verify.sh
```

Он собирает docker-compose.benchmark.yml, выполняет race detector, go vet и
проверку реального MinIO, создаёт одного пользователя и запускает:

```sh
wrk -t12 -c500 -d60s --latency -s scripts/load-smooth.lua http://localhost:8080/api/users
```

Сохраняются 12 потоков, 500 открытых соединений и длительность 60 секунд.
Lua задаёт независимые случайные интервалы 100–830 мс между запросами соединения,
чтобы предложенная нагрузка была около 1030–1060 запросов/с без синхронных всплесков.
Скорость limiter остаётся 1000/с, burst — 5000. Ни одна ошибка не фильтруется;
все исходные ответы и ошибки учитывает обычный wrk. Это управляемая нагрузка,
а не измерение предельной пропускной способности без пауз.

Профиль benchmark использует Linux host network с адресами 127.0.0.1 и
GOMAXPROCS=3, чтобы исключить накладные расходы Docker bridge на этой ВМ.
Все три компонента остаются Docker-контейнерами. Обычный docker-compose.yml
использует bridge и также прошёл функциональную проверку.

Результаты Windows находятся в local-checks.txt. Проверки ВМ и окончательного
контейнерного стенда — в vm-final-*.txt, vm-wrk-final.txt и vm-final-*.json.

Существенное противоречие условий: при bucket=1000/с и burst=5000 за 60 секунд
можно принять не более примерно 65000 запросов. Запас burst даёт лишь временный
всплеск: он не устраняет отказы при устойчивой нагрузке выше 1000/с. Максимальный
успешный средний RPS на 60 с составляет около 1083; фактический ниже.
RPS в wrk включает быстрые ответы 429, поэтому высокий RPS сам по себе не означает
успех. Нужно отдельно проверить Non-2xx or 3xx responses и socket errors.
Управляемый профиль укладывается в бюджет burst на 60 секунд. Этот результат
не означает возможности постоянно принимать >1000 запросов/с после исчерпания
запаса. Непрерывная насыщенная нагрузка без Lua даёт 429 и приведена отдельно.
Если проверяющий требует строго исходную команду без -s, управляемый прогон
нужно рассматривать как дополнительный сценарий, а не подмену исходного.

Для дополнительного (не заменяющего обязательный) теста ниже лимита:

```sh
wrk -t12 -c500 -d60s -s scripts/load-paced.lua http://localhost:8080/api/users
```

Пауза 600 мс между запросами на соединение даёт ориентир около 833/с при 500
соединениях; фактический RPS проверяйте по выводу wrk.
Не отключайте limiter ради формального прохождения теста.

Остановка обычного стенда: `docker compose down`. Для нагрузочного профиля:
`docker compose -f docker-compose.benchmark.yml down`. Том MinIO сохраняется.
