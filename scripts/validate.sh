#!/usr/bin/env sh
set -eu
project_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_root"
set -a
if [ -f .env ]; then . ./.env; else . ./.env.example; fi
set +a
(command -v jq >/dev/null 2>&1) || { echo "jq is required for API validation" >&2; exit 1; }
(cd backend && go test ./... && go test -race ./... && go vet ./... && go build ./...)
(cd frontend && npm install --no-audit --no-fund && npm run typecheck && npm run build)
docker compose config --quiet
docker compose down -v --remove-orphans >/dev/null 2>&1 || true
docker compose up -d --build
cleanup() { docker compose down -v --remove-orphans; }
if [ "${KEEP_RUNNING:-0}" = "1" ]; then
  trap cleanup INT TERM
else
  trap cleanup EXIT INT TERM
fi
i=0
until curl -fsS "http://127.0.0.1:${BACKEND_PORT:-19513}/healthz" >/dev/null; do
  i=$((i+1)); [ "$i" -lt 60 ] || { docker compose logs; exit 1; }; sleep 2
done
curl -fsS "http://127.0.0.1:${FRONTEND_PORT:-18513}/" >/dev/null
token=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT:-19513}/api/auth/login" -H 'Content-Type: application/json' -d '{"username":"admin","password":"Admin123!"}' | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
[ -n "$token" ]
operator_token=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT:-19513}/api/auth/login" -H 'Content-Type: application/json' -d '{"username":"operator","password":"Admin123!"}' | jq -er '.data.token')
reviewer_token=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT:-19513}/api/auth/login" -H 'Content-Type: application/json' -d '{"username":"reviewer","password":"Admin123!"}' | jq -er '.data.token')
viewer_token=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT:-19513}/api/auth/login" -H 'Content-Type: application/json' -d '{"username":"viewer","password":"Admin123!"}' | jq -er '.data.token')
curl -fsS "http://127.0.0.1:${BACKEND_PORT:-19513}/api/overview" -H "Authorization: Bearer $token" >/dev/null
curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/session" -H "Authorization: Bearer $token" | jq -e '.data.role == "admin" and (.data.requestId | length > 0)' >/dev/null
curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/runtime" -H "Authorization: Bearer $token" | jq -e '.data.appName and .data.databaseDriver and (.data.requestLimit > 0)' >/dev/null
paths=$(sed -n "s/.*path: '\\([^']*\\)'.*/\\1/p" frontend/src/types/status.ts)
for path in $paths; do
  curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/$path?page=1&pageSize=20" -H "Authorization: Bearer $token" | jq -e '.data | type == "array"' >/dev/null
done
entity_config=$(sed -n "s/.*path: '\\([^']*\\)'.*statuses: \\['\\([^']*\\)', '\\([^']*\\)'.*/\\1|\\2|\\3/p" frontend/src/types/status.ts | head -n 1)
resource=$(printf '%s' "$entity_config" | cut -d '|' -f 1)
initial_status=$(printf '%s' "$entity_config" | cut -d '|' -f 2)
next_status=$(printf '%s' "$entity_config" | cut -d '|' -f 3)
now=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
code="SMOKE-$(date +%s)"
payload=$(printf '{"code":"%s","name":"Runtime smoke record","description":"Automated Compose workflow validation","facility":"Validation Lab","owner":"admin","category":"smoke","riskLevel":"low","metricValue":1,"metricUnit":"unit","effectiveAt":"%s","evidence":"scripts/validate.sh","relatedCode":"SMOKE"}' "$code" "$now")created=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/$resource" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "$payload")
id=$(printf '%s' "$created" | jq -er '.data.id')
version=$(printf '%s' "$created" | jq -er '.data.version')
printf '%s' "$created" | jq -e --arg status "$initial_status" '.data.status == $status' >/dev/null
transition=$(printf '{"status":"%s","expectedVersion":%s,"reason":"automated runtime validation"}' "$next_status" "$version")
curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/$resource/$id/transition" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "$transition" | jq -e --arg status "$next_status" '.data.status == $status' >/dev/null
curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/audits?page=1&pageSize=100" -H "Authorization: Bearer $token" | jq -e '.meta.total >= 2' >/dev/null
curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/audit-summary?windowHours=24" -H "Authorization: Bearer $token" | jq -e '.data.total >= 2 and .data.transitions >= 1' >/dev/null

# Read-only accounts cannot cross the write boundary.
viewer_status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:${BACKEND_PORT}/api/zones" -H "Authorization: Bearer $viewer_token" -H 'Content-Type: application/json' -d "$payload")
[ "$viewer_status" = "403" ]

# A planned valve execution cannot bypass the two-person remote-control workflow.
# The clean case binds to seeded zone GZ-001 and plan IP-001; their latest
# validated reading is fresh and below the 35% stop line.
control_code="CONTROL-$(date +%s)"
control_payload=$(printf '{"code":"%s","name":"双人远程启动验证","description":"Operator request and independent reviewer confirmation","facility":"Validation Greenhouse A","owner":"operator","category":"remote-control","riskLevel":"high","metricValue":42,"metricUnit":"L/min","effectiveAt":"%s","evidence":"valve connectivity checked","relatedCode":"%s","zoneCode":"GZ-001","planCode":"IP-001"}' "$control_code" "$now" "$code")
control_created=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "$control_payload")
control_id=$(printf '%s' "$control_created" | jq -er '.data.id')
control_version=$(printf '%s' "$control_created" | jq -er '.data.version')
printf '%s' "$control_created" | jq -e '.data.zoneCode == "GZ-001" and .data.planCode == "IP-001" and .data.controlCheckStatus == "pending"' >/dev/null
direct_status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions/$control_id/transition" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "{\"status\":\"running\",\"expectedVersion\":$control_version,\"reason\":\"attempted direct remote start\"}")
[ "$direct_status" = "422" ]
unchecked_status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions/$control_id/control-request" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "{\"expectedVersion\":$control_version,\"confirmed\":false,\"reason\":\"missing explicit operator confirmation\"}")
[ "$unchecked_status" = "422" ]
control_requested=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions/$control_id/control-request" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "{\"expectedVersion\":$control_version,\"confirmed\":true,\"reason\":\"operator checked plan and valve connectivity\"}")
requested_version=$(printf '%s' "$control_requested" | jq -er '.data.version')
printf '%s' "$control_requested" | jq -e '.data.status == "planned" and .data.controlRequestedBy == "operator" and (.data.controlRequestedAt | length > 0)' >/dev/null

# Control detail exposes same-zone running tasks, latest validated reading and
# the plan stop-moisture line before confirmation.
curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/executions/$control_id/control-detail" -H "Authorization: Bearer $reviewer_token" | jq -e '.data.live == true and .data.snapshot.zoneCode == "GZ-001" and .data.snapshot.planCode == "IP-001" and .data.snapshot.stopMoistureLine == 35 and (.data.snapshot.latestReading.moisture > 0) and (.data.snapshot.conflicts | length == 0)' >/dev/null

control_confirmed=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions/$control_id/control-confirm" -H "Authorization: Bearer $reviewer_token" -H 'Content-Type: application/json' -d "{\"expectedVersion\":$requested_version,\"confirmed\":true,\"reason\":\"independent reviewer approved remote start\"}")
printf '%s' "$control_confirmed" | jq -e '.data.status == "running" and .data.controlRequestedBy == "operator" and .data.controlConfirmedBy == "reviewer" and (.data.controlConfirmedAt | length > 0) and .data.controlCheckStatus == "passed" and (.data.controlCheckSnapshot | length > 0)' >/dev/null
# The passing check snapshot is persisted and remains readable in control detail.
curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/executions/$control_id/control-detail" -H "Authorization: Bearer $reviewer_token" | jq -e '.data.live == false and .data.snapshot.status == "passed" and .data.snapshot.checkedBy == "reviewer" and (.data.snapshot.latestReading.measuredAt | length > 0)' >/dev/null

# Conflict case: zone GZ-002 already has a running task (VE-002) and its only
# validated reading is older than the 30-minute freshness window. The reviewer
# confirmation must stay planned, record a conflict number and freeze a snapshot.
conflict_code="CONFLICT-$(date +%s)"
conflict_payload=$(printf '{"code":"%s","name":"夜班冲突拦截验证","description":"Running task plus stale reading must block remote start","facility":"Validation Greenhouse B","owner":"operator","category":"remote-control","riskLevel":"high","metricValue":42,"metricUnit":"L/min","effectiveAt":"%s","evidence":"valve connectivity checked","relatedCode":"%s","zoneCode":"GZ-002","planCode":"IP-002"}' "$conflict_code" "$now" "$code")
conflict_created=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "$conflict_payload")
conflict_id=$(printf '%s' "$conflict_created" | jq -er '.data.id')
conflict_version=$(printf '%s' "$conflict_created" | jq -er '.data.version')
conflict_requested=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions/$conflict_id/control-request" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "{\"expectedVersion\":$conflict_version,\"confirmed\":true,\"reason\":\"operator requests night-shift start\"}")
conflict_requested_version=$(printf '%s' "$conflict_requested" | jq -er '.data.version')
conflict_blocked=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions/$conflict_id/control-confirm" -H "Authorization: Bearer $reviewer_token" -H 'Content-Type: application/json' -d "{\"expectedVersion\":$conflict_requested_version,\"confirmed\":true,\"reason\":\"reviewer confirms against stale evidence\"}")
printf '%s' "$conflict_blocked" | jq -e '.data.status == "planned" and .data.controlCheckStatus == "blocked" and (.data.controlConflictNo | startswith("CF-")) and (.data.controlDetail | contains("含水率")) and (.data.controlCheckSnapshot | length > 0) and (.data.controlConfirmedAt == null)' >/dev/null
curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/executions/$conflict_id/control-detail" -H "Authorization: Bearer $reviewer_token" | jq -e '.data.snapshot.status == "blocked" and ([.data.snapshot.conflicts[].kind] | index("running_task_same_zone")) and ([.data.snapshot.conflicts[].kind] | index("validated_reading_stale")) and (.data.snapshot.runningTasks | length >= 1) and ([.data.snapshot.conflicts[].code] | all(startswith("CF-")))' >/dev/null

# Zone GZ-003 reading is fresh but above the 40% stop line: must block too.
wet_code="WET-$(date +%s)"
wet_payload=$(printf '%s' "$conflict_payload" | jq -c --arg code "$wet_code" '.code = $code | .zoneCode = "GZ-003" | .planCode = "IP-002"')
# IP-002 targets GZ-002, so a GZ-003/IP-002 binding must be rejected at create.
mismatch_status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "$wet_payload")
[ "$mismatch_status" = "422" ]
wet_payload=$(printf '%s' "$conflict_payload" | jq -c --arg code "$wet_code" '.code = $code | .zoneCode = "GZ-003" | .planCode = "IP-003"')
wet_created=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "$wet_payload")
wet_id=$(printf '%s' "$wet_created" | jq -er '.data.id')
wet_version=$(printf '%s' "$wet_created" | jq -er '.data.version')
wet_requested=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions/$wet_id/control-request" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "{\"expectedVersion\":$wet_version,\"confirmed\":true,\"reason\":\"operator requests wet-zone start\"}")
wet_requested_version=$(printf '%s' "$wet_requested" | jq -er '.data.version')
wet_blocked=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions/$wet_id/control-confirm" -H "Authorization: Bearer $reviewer_token" -H 'Content-Type: application/json' -d "{\"expectedVersion\":$wet_requested_version,\"confirmed\":true,\"reason\":\"reviewer checks above stop line\"}")
printf '%s' "$wet_blocked" | jq -e '.data.status == "planned" and .data.controlCheckStatus == "blocked"' >/dev/null
curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/executions/$wet_id/control-detail" -H "Authorization: Bearer $reviewer_token" | jq -e '[.data.snapshot.conflicts[].kind] | index("above_stop_moisture_line")' >/dev/null

# Admin is allowed on both routes but still cannot satisfy both confirmations alone.
self_code="SELF-$(date +%s)"
self_payload=$(printf '%s' "$control_payload" | jq -c --arg code "$self_code" '.code = $code | .name = "同人复核阻断验证"')
self_created=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "$self_payload")
self_id=$(printf '%s' "$self_created" | jq -er '.data.id')
self_version=$(printf '%s' "$self_created" | jq -er '.data.version')
self_requested=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions/$self_id/control-request" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "{\"expectedVersion\":$self_version,\"confirmed\":true,\"reason\":\"admin submitted control request\"}")
self_requested_version=$(printf '%s' "$self_requested" | jq -er '.data.version')
self_status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions/$self_id/control-confirm" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "{\"expectedVersion\":$self_requested_version,\"confirmed\":true,\"reason\":\"same admin attempted confirmation\"}")
[ "$self_status" = "422" ]

curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/audits?page=1&pageSize=100" -H "Authorization: Bearer $token" | jq -e --arg id "$control_id" --arg conflict "$conflict_id" '([.data[] | select(.entityType == "ValveExecution" and (.entityId | tostring) == $id and .action == "control_request")] | length) >= 1 and ([.data[] | select(.entityType == "ValveExecution" and (.entityId | tostring) == $id and .action == "control_confirm")] | length) >= 1 and ([.data[] | select(.entityType == "ValveExecution" and (.entityId | tostring) == $conflict and .action == "control_conflict")] | length) >= 1' >/dev/null
docker compose ps
[ "${KEEP_RUNNING:-0}" = "1" ] && echo "KEEP_RUNNING=1: containers left running for browser validation"
