#!/bin/sh
set -eu

umask 077

db_service=${FANTI_DB_SERVICE:-db}
app_service=${FANTI_APP_SERVICE:-app}
db_user=${FANTI_DB_USER:-fanti}
db_name=${FANTI_DB_NAME:-fanti}
backup_dir=${FANTI_BACKUP_DIR:-backups}
app_was_running=0

usage() {
  printf 'usage: %s backup [directory]\n' "$0" >&2
  printf '       %s restore <archive>\n' "$0" >&2
  exit 2
}

compose() {
  docker compose "$@"
}

validate_archive() {
  archive=$1
  list_file=$(mktemp "${TMPDIR:-/tmp}/fanti-archive-list.XXXXXX")

  if ! compose exec -T "$db_service" pg_restore --list < "$archive" > "$list_file"; then
    rm -f "$list_file"
    printf 'archive validation failed: %s\n' "$archive" >&2
    return 1
  fi

  if ! grep -Eq 'TABLE[[:space:]]+public[[:space:]]+goose_db_version([[:space:]]|$)' "$list_file"; then
    rm -f "$list_file"
    printf 'archive is not a Fanti database backup: %s\n' "$archive" >&2
    return 1
  fi

  rm -f "$list_file"
}

create_backup() {
  backup_dir=$1
  label=$2

  mkdir -p "$backup_dir"
  timestamp=$(date -u +%Y%m%dT%H%M%SZ)
  temporary=$(mktemp "$backup_dir/.fanti-$timestamp.XXXXXX")
  suffix=${temporary##*.}
  destination="$backup_dir/fanti-$timestamp-$suffix$label.dump"

  trap 'rm -f "$temporary"' 0 HUP INT TERM

  if ! compose exec -T "$db_service" pg_dump \
    -U "$db_user" \
    -d "$db_name" \
    --format=custom \
    --no-owner \
    --no-privileges > "$temporary"; then
    printf 'backup failed; incomplete archive removed: %s\n' "$temporary" >&2
    return 1
  fi

  if [ ! -s "$temporary" ]; then
    printf 'backup failed; database dump was empty: %s\n' "$temporary" >&2
    return 1
  fi

  if ! validate_archive "$temporary"; then
    printf 'backup failed; invalid archive removed: %s\n' "$temporary" >&2
    return 1
  fi

  mv "$temporary" "$destination"
  trap - 0 HUP INT TERM
  printf '%s\n' "$destination"
}

restart_app() {
  if [ "$app_was_running" -eq 0 ]; then
    return 0
  fi

  if ! compose start "$app_service" >/dev/null; then
    printf 'database operation completed, but restarting %s failed\n' "$app_service" >&2
    return 1
  fi

  app_was_running=0
}

restore_archive() {
  archive=$1
  compose exec -T "$db_service" pg_restore \
    --clean \
    --if-exists \
    --exit-on-error \
    --no-owner \
    --no-privileges \
    -U "$db_user" \
    -d "$db_name" < "$archive"
}

restore_database() {
  archive=$1

  if [ ! -f "$archive" ] || [ ! -r "$archive" ]; then
    printf 'restore archive is not a readable file: %s\n' "$archive" >&2
    return 1
  fi

  validate_archive "$archive"

  printf 'Restore replaces all Fanti data. Type RESTORE to continue: ' >&2
  confirmation=
  IFS= read -r confirmation || true
  if [ "$confirmation" != "RESTORE" ]; then
    printf 'restore cancelled; database unchanged\n' >&2
    return 1
  fi

  if compose ps --status running --services | grep -Fxq "$app_service"; then
    app_was_running=1
    trap 'restart_app || true' 0 HUP INT TERM
    compose stop "$app_service" >/dev/null
  fi

  if ! recovery_archive=$(create_backup "$backup_dir" "-pre-restore"); then
    restart_app || true
    trap - 0 HUP INT TERM
    printf 'restore cancelled; recovery backup could not be created\n' >&2
    return 1
  fi

  if ! restore_archive "$archive"; then
    printf 'requested restore failed; rolling back from: %s\n' "$recovery_archive" >&2
    if restore_archive "$recovery_archive"; then
      restart_app
      trap - 0 HUP INT TERM
      printf 'original database restored from: %s\n' "$recovery_archive" >&2
      return 1
    fi

    restart_app || true
    trap - 0 HUP INT TERM
    printf 'automatic rollback also failed; manual recovery archive: %s\n' "$recovery_archive" >&2
    return 1
  fi

  restart_app
  trap - 0 HUP INT TERM
  printf 'restore complete: %s\n' "$archive"
  printf 'recovery backup retained: %s\n' "$recovery_archive"
}

case ${1:-} in
  backup)
    [ "$#" -le 2 ] || usage
    create_backup "${2:-backups}" ""
    ;;
  restore)
    [ "$#" -eq 2 ] || usage
    restore_database "$2"
    ;;
  *)
    usage
    ;;
esac
