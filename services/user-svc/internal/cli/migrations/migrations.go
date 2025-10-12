package migrations

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"smartorders/user-svc/internal/store/postgresql"
)

// Run — точка входа CLI-команды "migrations".
//
// Поддерживаемые подкоманды:
//
//   up      — применяет все доступные миграции (из директории Config.MigrationsDir),
//              которые ещё не были применены к текущей базе данных.
//
//   down    — откатывает последнюю выполненную миграцию.
//
//   status  — выводит текущую версию схемы базы данных и состояние (clean/dirty).
//
//   new     — создаёт новую пару файлов миграции (.up.sql и .down.sql)
//              в директории Config.MigrationsDir с автонумерацией.

func Run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: migrations [up|down|status|new <name>]")
	}

	cfg := postgresql.DefaultConfig()

	switch args[0] {
	case "up":
		return postgresql.ApplyMigrations(cfg)
	case "down":
		return postgresql.RollbackMigrations(cfg)
	case "status":
		return postgresql.StatusMigrations(cfg)
	case "new":
		if len(args) < 2 {
			return errors.New("usage: migrations new <name>")
		}
		name := strings.Join(args[1:], "_")
		return createMigrationFiles(cfg.MigrationsDir, name)
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

// createMigrationFiles создаёт новую пару файлов миграции: .up.sql и .down.sql
func createMigrationFiles(dir, name string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	// определяем следующий номер миграции
	next, err := nextMigrationNumber(dir)
	if err != nil {
		return fmt.Errorf("get next migration number: %w", err)
	}

	prefix := fmt.Sprintf("%04d_%s", next, sanitizeName(name))
	up := filepath.Join(dir, prefix+".up.sql")
	down := filepath.Join(dir, prefix+".down.sql")

	// создаём пустые файлы с шаблоном
	now := time.Now().Format("2006-01-02 15:04:05")
	upContent := fmt.Sprintf("-- +migrate Up\n-- created at %s\n\n", now)
	downContent := fmt.Sprintf("-- +migrate Down\n-- created at %s\n\n", now)

	if err := os.WriteFile(up, []byte(upContent), 0644); err != nil {
		return fmt.Errorf("write up file: %w", err)
	}
	if err := os.WriteFile(down, []byte(downContent), 0644); err != nil {
		return fmt.Errorf("write down file: %w", err)
	}

	fmt.Printf("created migration files:\n  %s\n  %s\n", up, down)
	return nil
}

// nextMigrationNumber ищет последний номер миграции и возвращает следующий.
func nextMigrationNumber(dir string) (int, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, err
	}

	re := regexp.MustCompile(`^(\d+)_.*\.(up|down)\.sql$`)
	max := 0
	for _, f := range files {
		if m := re.FindStringSubmatch(f.Name()); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil && n > max {
				max = n
			}
		}
	}
	return max + 1, nil
}

// sanitizeName очищает имя миграции.
func sanitizeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}
