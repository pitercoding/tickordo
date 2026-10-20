# Manual database migration runner for Tickordo (golang-migrate).
#
# Build from the repository root (the build context must be "."):
#   docker build -f docker/migrate.Dockerfile -t tickordo-migrate .
#
# Run explicitly, passing the database URL from the host environment:
#   docker run --rm -e DATABASE_URL tickordo-migrate version
#
# No credentials are stored in this image; DATABASE_URL is read at runtime.
# Nothing runs against a database at build time or without an explicit command.
#
# Entrypoint behavior:
# - stderr is filtered line by line (unbuffered); from any "scheme://" to the
#   end of the line is redacted, since parse errors can echo password fragments.
#   stdout and stdin are passed through untouched.
# - SIGINT and SIGTERM (Ctrl+C, docker stop) are forwarded to migrate. Once it
#   handles SIGINT (after connecting), it gets SIGINT, its graceful stop: it
#   finishes the running migration, then exits. While still connecting (no
#   migration started yet), it gets SIGTERM and exits right away.
# - The exit code of migrate is preserved.

FROM migrate/migrate:v4.20.1

COPY migrations/*.sql /migrations/

ENTRYPOINT ["/bin/sh", "-c", "\
case \"$1\" in ''|-h|-help|--help|-version|--version) exec migrate \"$@\" ;; esac; \
if [ -z \"$DATABASE_URL\" ]; then echo 'error: DATABASE_URL is not set' >&2; exit 1; fi; \
fifo=$(mktemp -u) && mkfifo -m 600 \"$fifo\" || exit 1; \
awk '{ sub(\"[A-Za-z][A-Za-z0-9+.-]*://.*\", \"<redacted-url>\"); print; fflush() }' <\"$fifo\" >&2 & \
filter=$!; pid=; \
fwd() { [ -n \"$pid\" ] || return 0; \
  ign=$(sed -n 's/^SigIgn:[[:space:]]*//p' /proc/$pid/status 2>/dev/null); [ -n \"$ign\" ] || return 0; \
  if [ $((0x$ign & 2)) -ne 0 ]; then kill -TERM \"$pid\"; else kill -INT \"$pid\"; fi 2>/dev/null; }; \
trap fwd INT TERM; \
exec 3<&0; \
migrate -path /migrations -database \"$DATABASE_URL\" \"$@\" <&3 3<&- 2>\"$fifo\" & \
pid=$!; exec 3<&-; \
wait \"$pid\"; code=$?; \
while kill -0 \"$pid\" 2>/dev/null; do wait \"$pid\"; code=$?; done; \
wait \"$filter\"; rm -f \"$fifo\"; exit \"$code\"", "migrate"]
CMD []
