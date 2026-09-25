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
payload=$(printf '{"code":"%s","name":"Runtime smoke record","description":"Automated Compose workflow validation","facility":"Validation Lab","owner":"admin","category":"smoke","riskLevel":"low","metricValue":1,"metricUnit":"unit","effectiveAt":"%s","evidence":"scripts/validate.sh","relatedCode":"SMOKE"}' "$code" "$now")
created=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/$resource" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "$payload")
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

# Remote valve start requires real zone/plan links and an independent pre-start
# check. Build coherent zones where each scenario's check is deterministic.
build_zone() {
  # $1 zone code, $2 plan code, $3 moisture, $4 stop line
  zcode=$1; pcode=$2; moisture=$3; stopline=$4
  zone_payload=$(printf '{"code":"%s","name":"复核分区 %s","description":"pre-start check validation zone","facility":"Validation Greenhouse %s","owner":"operator","category":"smoke","riskLevel":"medium","metricValue":0,"metricUnit":"%%","effectiveAt":"%s","evidence":"scripts/validate.sh","relatedCode":"SMOKE"}' "$zcode" "$zcode" "$zcode" "$now")
  curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/zones" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "$zone_payload" >/dev/null
  plan_payload=$(printf '{"code":"%s","name":"复核计划 %s","description":"plan with stop moisture line","facility":"Validation Greenhouse %s","owner":"operator","category":"smoke","riskLevel":"medium","metricValue":30,"metricUnit":"L/min","effectiveAt":"%s","evidence":"stop line reviewed","relatedCode":"SMOKE","zoneCode":"%s","stopMoisture":%s}' "$pcode" "$pcode" "$zcode" "$now" "$zcode" "$stopline")
  curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/plans" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "$plan_payload" >/dev/null
  rcode="RD-$zcode"
  reading_payload=$(printf '{"code":"%s","name":"复核读数 %s","description":"validated fresh reading","facility":"Validation Greenhouse %s","owner":"operator","category":"smoke","riskLevel":"low","metricValue":%s,"metricUnit":"%%","effectiveAt":"%s","evidence":"probe calibrated","relatedCode":"SMOKE","zoneCode":"%s"}' "$rcode" "$zcode" "$zcode" "$moisture" "$now" "$zcode")
  reading_resp=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/readings" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "$reading_payload")
  rid=$(printf '%s' "$reading_resp" | jq -er '.data.id')
  rversion=$(printf '%s' "$reading_resp" | jq -er '.data.version')
  curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/readings/$rid/transition" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "{\"status\":\"validated\",\"expectedVersion\":$rversion,\"reason\":\"reading validated for pre-start check\"}" >/dev/null
}

# Executions without zone/plan links are rejected at write time.
orphan_status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "{\"code\":\"ORPHAN-$(date +%s)\",\"name\":\"无分区关联执行\",\"facility\":\"Validation Greenhouse A\",\"owner\":\"operator\",\"category\":\"smoke\",\"riskLevel\":\"high\",\"metricValue\":42,\"metricUnit\":\"L/min\",\"effectiveAt\":\"$now\",\"evidence\":\"should be rejected\"}")
[ "$orphan_status" = "400" ]

# Scenario A: fresh reading below the stop line, no running task -> check passes.
stamp=$(date +%s)
build_zone "GZ-A$stamp" "IP-A$stamp" 18 32
control_code="CONTROL-$stamp"
control_payload=$(printf '{"code":"%s","name":"双人远程启动验证","description":"Operator request and independent reviewer confirmation","facility":"Validation Greenhouse A","owner":"operator","category":"remote-control","riskLevel":"high","metricValue":42,"metricUnit":"L/min","effectiveAt":"%s","evidence":"valve connectivity checked","relatedCode":"SMOKE","zoneCode":"GZ-A%s","planCode":"IP-A%s"}' "$control_code" "$now" "$stamp" "$stamp")
control_created=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "$control_payload")
control_id=$(printf '%s' "$control_created" | jq -er '.data.id')
control_version=$(printf '%s' "$control_created" | jq -er '.data.version')
printf '%s' "$control_created" | jq -e --arg z "GZ-A$stamp" --arg p "IP-A$stamp" '.data.zoneCode == $z and .data.planCode == $p' >/dev/null
direct_status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions/$control_id/transition" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "{\"status\":\"running\",\"expectedVersion\":$control_version,\"reason\":\"attempted direct remote start\"}")
[ "$direct_status" = "422" ]
unchecked_status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions/$control_id/control-request" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "{\"expectedVersion\":$control_version,\"confirmed\":false,\"reason\":\"missing explicit operator confirmation\"}")
[ "$unchecked_status" = "422" ]
control_requested=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions/$control_id/control-request" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "{\"expectedVersion\":$control_version,\"confirmed\":true,\"reason\":\"operator checked plan and valve connectivity\"}")
requested_version=$(printf '%s' "$control_requested" | jq -er '.data.version')
printf '%s' "$control_requested" | jq -e '.data.status == "planned" and .data.controlRequestedBy == "operator" and (.data.controlRequestedAt | length > 0)' >/dev/null

# Reviewer previews the pre-start check before confirming.
precheck=$(curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/executions/$control_id/control-check" -H "Authorization: Bearer $reviewer_token")
printf '%s' "$precheck" | jq -e '.data.conflicts | length == 0' >/dev/null
printf '%s' "$precheck" | jq -e '.data.readingFresh == true and (.data.runningTasks | length == 0) and .data.plan.stopMoisture == 32 and .data.reading.moisture == 18' >/dev/null

control_confirmed=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions/$control_id/control-confirm" -H "Authorization: Bearer $reviewer_token" -H 'Content-Type: application/json' -d "{\"expectedVersion\":$requested_version,\"confirmed\":true,\"reason\":\"independent reviewer approved remote start\"}")
printf '%s' "$control_confirmed" | jq -e '.data.status == "running" and .data.controlRequestedBy == "operator" and .data.controlConfirmedBy == "reviewer" and (.data.controlConfirmedAt | length > 0) and .data.controlCheckResult == "passed" and (.data.controlCheckSnapshot | contains("\"result\":\"passed\""))' >/dev/null

# Scenario B: same zone now has a running task; a second execution stays planned
# with a conflict number, reading time and moisture recorded in the detail.
blocked_code="BLOCKED-$stamp"
blocked_payload=$(printf '%s' "$control_payload" | jq -c --arg code "$blocked_code" '.code = $code | .name = "运行中任务冲突阻断验证"')
blocked_created=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "$blocked_payload")
blocked_id=$(printf '%s' "$blocked_created" | jq -er '.data.id')
blocked_version=$(printf '%s' "$blocked_created" | jq -er '.data.version')
blocked_requested=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions/$blocked_id/control-request" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "{\"expectedVersion\":$blocked_version,\"confirmed\":true,\"reason\":\"night shift requested second watering\"}")
blocked_req_version=$(printf '%s' "$blocked_requested" | jq -er '.data.version')
blocked_confirm=$(curl -sS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions/$blocked_id/control-confirm" -H "Authorization: Bearer $reviewer_token" -H 'Content-Type: application/json' -d "{\"expectedVersion\":$blocked_req_version,\"confirmed\":true,\"reason\":\"reviewer attempted confirmation while zone busy\"}")
printf '%s' "$blocked_confirm" | jq -e '.data.status == "planned" and .data.controlCheckResult == "blocked" and (.data.controlConflictNo | startswith("CFL-")) and (.data.controlDetail | contains("CFL-"))' >/dev/null
blocked_check=$(curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/executions/$blocked_id/control-check" -H "Authorization: Bearer $reviewer_token")
printf '%s' "$blocked_check" | jq -e '.data.conflicts | length >= 1 and .data.result == "blocked"' >/dev/null

# Scenario C: moisture at/above the plan stop line blocks start even when fresh.
stamp_c=$stamp
build_zone "GZ-C$stamp_c" "IP-C$stamp_c" 41 35
wet_code="WET-$stamp_c"
wet_payload=$(printf '{"code":"%s","name":"停灌线冲突阻断验证","description":"moisture above stop line must block","facility":"Validation Greenhouse C","owner":"operator","category":"remote-control","riskLevel":"high","metricValue":42,"metricUnit":"L/min","effectiveAt":"%s","evidence":"valve connectivity checked","relatedCode":"SMOKE","zoneCode":"GZ-C%s","planCode":"IP-C%s"}' "$wet_code" "$now" "$stamp_c" "$stamp_c")
wet_created=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "$wet_payload")
wet_id=$(printf '%s' "$wet_created" | jq -er '.data.id')
wet_version=$(printf '%s' "$wet_created" | jq -er '.data.version')
wet_requested=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions/$wet_id/control-request" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "{\"expectedVersion\":$wet_version,\"confirmed\":true,\"reason\":\"operator requested start on wet zone\"}")
wet_req_version=$(printf '%s' "$wet_requested" | jq -er '.data.version')
wet_confirm=$(curl -sS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions/$wet_id/control-confirm" -H "Authorization: Bearer $reviewer_token" -H 'Content-Type: application/json' -d "{\"expectedVersion\":$wet_req_version,\"confirmed\":true,\"reason\":\"reviewer checked latest reading\"}")
printf '%s' "$wet_confirm" | jq -e '.data.status == "planned" and .data.controlCheckResult == "blocked" and (.data.controlDetail | contains("41.0%")) and (.data.controlDetail | test("读数时间"))' >/dev/null

# After a newer validated reading drops below the stop line, re-check passes.
fixed_code="RD-FIX-$stamp_c"
fixed_payload=$(printf '{"code":"%s","name":"停灌后复测读数","description":"newest validated reading below stop line","facility":"Validation Greenhouse C","owner":"operator","category":"smoke","riskLevel":"low","metricValue":19,"metricUnit":"%%","effectiveAt":"%s","evidence":"rechecked moisture","relatedCode":"SMOKE","zoneCode":"GZ-C%s"}' "$fixed_code" "$now" "$stamp_c")
fixed_created=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/readings" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "$fixed_payload")
fixed_id=$(printf '%s' "$fixed_created" | jq -er '.data.id')
fixed_version=$(printf '%s' "$fixed_created" | jq -er '.data.version')
curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/readings/$fixed_id/transition" -H "Authorization: Bearer $operator_token" -H 'Content-Type: application/json' -d "{\"status\":\"validated\",\"expectedVersion\":$fixed_version,\"reason\":\"recheck validated\"}" >/dev/null
wet_current=$(curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/executions/$wet_id" -H "Authorization: Bearer $reviewer_token")
wet_current_version=$(printf '%s' "$wet_current" | jq -er '.data.version')
wet_retry=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions/$wet_id/control-confirm" -H "Authorization: Bearer $reviewer_token" -H 'Content-Type: application/json' -d "{\"expectedVersion\":$wet_current_version,\"confirmed\":true,\"reason\":\"recheck passed after fresh reading\"}")
printf '%s' "$wet_retry" | jq -e '.data.status == "running" and .data.controlCheckResult == "passed"' >/dev/null

# Admin is allowed on both routes but still cannot satisfy both confirmations alone.
stamp_s=$(date +%s)
build_zone "GZ-S$stamp_s" "IP-S$stamp_s" 18 32
self_code="SELF-$stamp_s"
self_payload=$(printf '{"code":"%s","name":"同人复核阻断验证","description":"same admin cannot confirm own request","facility":"Validation Greenhouse S","owner":"admin","category":"remote-control","riskLevel":"high","metricValue":42,"metricUnit":"L/min","effectiveAt":"%s","evidence":"valve connectivity checked","relatedCode":"SMOKE","zoneCode":"GZ-S%s","planCode":"IP-S%s"}' "$self_code" "$now" "$stamp_s" "$stamp_s")
self_created=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "$self_payload")
self_id=$(printf '%s' "$self_created" | jq -er '.data.id')
self_version=$(printf '%s' "$self_created" | jq -er '.data.version')
self_requested=$(curl -fsS -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions/$self_id/control-request" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "{\"expectedVersion\":$self_version,\"confirmed\":true,\"reason\":\"admin submitted control request\"}")
self_requested_version=$(printf '%s' "$self_requested" | jq -er '.data.version')
self_status=$(curl -sS -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:${BACKEND_PORT}/api/executions/$self_id/control-confirm" -H "Authorization: Bearer $token" -H 'Content-Type: application/json' -d "{\"expectedVersion\":$self_requested_version,\"confirmed\":true,\"reason\":\"same admin attempted confirmation\"}")
[ "$self_status" = "422" ]

curl -fsS "http://127.0.0.1:${BACKEND_PORT}/api/audits?page=1&pageSize=100" -H "Authorization: Bearer $token" | jq -e --arg id "$control_id" --arg blocked "$blocked_id" '([.data[] | select(.entityType == "ValveExecution" and (.entityId | tostring) == $id and .action == "control_request")] | length) >= 1 and ([.data[] | select(.entityType == "ValveExecution" and (.entityId | tostring) == $id and .action == "control_confirm")] | length) >= 1 and ([.data[] | select(.entityType == "ValveExecution" and (.entityId | tostring) == $blocked and .action == "control_blocked")] | length) >= 1' >/dev/null
docker compose ps
[ "${KEEP_RUNNING:-0}" = "1" ] && echo "KEEP_RUNNING=1: containers left running for browser validation"
