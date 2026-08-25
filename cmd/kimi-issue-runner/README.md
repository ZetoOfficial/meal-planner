# Kimi Issue Runner

Минимальный локальный runner последовательно выполняет GitHub Issues в одном checkout. Без `--issues` он снимает очередь всех открытых Issues по возрастанию номера; явный список сохраняет указанный порядок.

Требования: Go из `go.mod`, авторизованные `gh` и `kimi`, чистый worktree и отсутствие параллельных изменений в этом checkout.

```bash
# Только показать текущий backlog
go run ./cmd/kimi-issue-runner --dry-run

# Выполнить весь открытый backlog
go run ./cmd/kimi-issue-runner

# Выполнить выбранные Issues в заданном порядке
go run ./cmd/kimi-issue-runner --issues 20,21,22

# Продолжить сохранённую очередь
go run ./cmd/kimi-issue-runner --resume

# Показать ещё не выполненную часть сохранённой очереди
go run ./cmd/kimi-issue-runner --resume --dry-run

# Открыть итоговый отчёт
cat "$(git rev-parse --git-dir)/kimi-issue-runner/report.md"
```

Состояние и отчёт хранятся в `.git/kimi-issue-runner`, поэтому не попадают в commits. Для каждой попытки сохраняются отдельные `stdout` и `stderr`. `report.md` содержит итоговый статус, полный commit SHA и заявленные Kimi проверки.

Каждый Issue запускается в новой неинтерактивной сессии `kimi --auto`. Commit создаёт только runner и только после явного `completed`, успешных проверок и `git diff --cached --check`. При `FAILED`/`BLOCKED` изменения откатываются, если HEAD не менялся; неожиданный сдвиг HEAD останавливает очередь для ручного восстановления.

Runner не делает AI review, merge, push, deploy и production/infra операции.
