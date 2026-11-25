#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")/.."
test_user=${SUDO_USER:-$(id -un)}
as_user() {
 if [ "$(id -u)" = 0 ] && [ "$test_user" != root ]; then
  runuser -u "$test_user" -- "$@"
 else
  "$@"
 fi
}
mkdir -p evidence
export LC_ALL=C
as_user go test -race -count=1 -v ./... > evidence/vm-final-race.txt 2>&1
cat evidence/vm-final-race.txt
as_user go vet ./... > evidence/vm-final-vet.txt 2>&1
printf 'go vet ./...: PASS\n' >> evidence/vm-final-vet.txt
docker compose -f docker-compose.benchmark.yml --progress plain build > evidence/vm-final-build.txt 2>&1
docker compose -f docker-compose.benchmark.yml up -d --no-build
attempt=0
until curl -fsS http://localhost:8080/healthz >/dev/null; do
 attempt=$((attempt+1));if [ "$attempt" -ge 60 ];then exit 1;fi;sleep 1
done
as_user env RUN_E2E=1 go test -count=1 -v ./integration > evidence/vm-final-e2e.txt 2>&1
cat evidence/vm-final-e2e.txt
curl -fsS -X POST http://localhost:8080/api/users -H 'Content-Type: application/json' -d '{"name":"Load test","email":"load@example.org"}' > evidence/vm-final-seed.json
curl -fsS http://localhost:8080/api/users > evidence/vm-final-users.json
curl -fsS http://localhost:8080/metrics > evidence/vm-final-metrics-before.txt
{
 set -x
 wrk -t12 -c500 -d60s --latency -s scripts/load-smooth.lua http://localhost:8080/api/users
 set +x
} > evidence/vm-wrk-final.txt 2>&1
cat evidence/vm-wrk-final.txt
curl -fsS http://localhost:8080/metrics > evidence/vm-final-metrics-after.txt
curl -fsS http://localhost:8080/api/integration/status > evidence/vm-final-integration.json
curl -fsS http://localhost:9090/api/v1/targets > evidence/vm-final-targets.json
docker compose -f docker-compose.benchmark.yml ps > evidence/vm-final-compose.txt
docker stats --no-stream > evidence/vm-final-stats.txt
docker compose -f docker-compose.benchmark.yml logs --no-color app minio > evidence/vm-final-services.log
docker compose -f docker-compose.benchmark.yml exec -T app sh -c 'printf "GOMAXPROCS=%s RATE_BURST=%s HTTP_ADDR=%s\n" "$GOMAXPROCS" "$RATE_BURST" "$HTTP_ADDR"' > evidence/vm-final-config.txt
echo 'Final verification complete'
