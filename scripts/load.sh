#!/usr/bin/env sh
set -eu
mkdir -p evidence
docker compose up -d --build
attempt=0
until curl -fsS http://localhost:8080/healthz >/dev/null; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 60 ]; then
    docker compose logs --no-color app
    echo 'Application did not become healthy within 60 attempts' >&2
    exit 1
  fi
  sleep 1
done
curl -fsS -X POST http://localhost:8080/api/users -H 'Content-Type: application/json' -d '{"name":"Load test","email":"load@example.org"}' > evidence/seed.json
docker compose ps > evidence/compose-ps.txt
curl -fsS http://localhost:8080/metrics > evidence/metrics-before.txt
# Keep wrk exit status, rather than hiding it behind a tee pipeline.
wrk -t12 -c500 -d60s --latency http://localhost:8080/api/users > evidence/wrk.txt 2>&1
cat evidence/wrk.txt
curl -fsS http://localhost:8080/metrics > evidence/metrics-after.txt
curl -fsS http://localhost:8080/api/integration/status > evidence/integration.json
docker compose logs --no-color app > evidence/app.log
