#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

fail() { printf '%s\n' "$*" >&2; exit 1; }
sha=${1:-}
[[ $sha =~ ^[0-9a-f]{40}$ ]] || fail 'Expected full tested commit SHA'
[[ $(git rev-parse HEAD) == "$sha" ]] || fail 'Checkout differs from tested SHA'
root=/home/robot/IMCProLogbook-be
state=$root/.cd
[[ -r $root/.env && -r $state/project ]] || fail 'Missing production .env or .cd/project'
read -r project < "$state/project"
[[ $project =~ ^[a-z0-9][a-z0-9_-]*$ ]] || fail 'Invalid Compose project'
exec 9>"$state/deploy.lock"
flock -n 9 || fail 'Another deployment holds host lock'
release=$state/releases/$sha-$(date +%s)
mkdir -p "$release"
compose=(docker compose --project-name "$project" --project-directory "$PWD" --env-file "$root/.env" -f "$PWD/docker-compose.yml")

# Snapshot resolved configuration and immutable running images before replacement.
for service in app web db; do
  id=$(docker ps -q --filter "label=com.docker.compose.project=$project" --filter "label=com.docker.compose.service=$service")
  [[ -n $id && $id != *$'\n'* ]] || fail "Expected one running $service container"
  if [[ $service == app ]]; then
    workdir=$(docker inspect -f '{{index .Config.Labels "com.docker.compose.project.working_dir"}}' "$id")
    files=$(docker inspect -f '{{index .Config.Labels "com.docker.compose.project.config_files"}}' "$id")
    previous=(docker compose --project-name "$project" --project-directory "$workdir" --env-file "$root/.env")
    IFS=, read -ra paths <<< "$files"
    for file in "${paths[@]}"; do previous+=(-f "$file"); done
    "${previous[@]}" config > "$release/previous.yml"
  fi
  if [[ $service != db ]]; then
    image=$(docker inspect -f '{{.Image}}' "$id")
    docker tag "$image" "imcpro-$service:rollback-$sha"
    printf '  %s:\n    image: imcpro-%s:rollback-%s\n' "$service" "$service" "$sha" >> "$release/old-images.body"
  fi
done
{ printf 'services:\n'; cat "$release/old-images.body"; } > "$release/old-images.yml"
cat > "$release/images.yml" <<EOF
services:
  app:
    image: imcpro-app:$sha
  migrate:
    image: imcpro-app:$sha
  web:
    image: imcpro-web:$sha
EOF
compose+=(-f "$release/images.yml")
"${compose[@]}" build app web
# Retain exact configuration, including secrets, only in private host state.
"${compose[@]}" config > "$release/current.yml"
current=(docker compose --project-name "$project" --env-file "$root/.env" -f "$release/current.yml")
rollback=(docker compose --project-name "$project" --env-file "$root/.env" -f "$release/previous.yml" -f "$release/old-images.yml")
changed=false
rollback_on_error() {
  status=$?
  trap - EXIT INT TERM
  if [[ $status != 0 && $changed == true ]]; then
    printf '%s\n' 'Deployment failed; restoring previous app/web images. Database migrations are not reversed.' >&2
    if "${rollback[@]}" up -d --no-deps --no-build --wait --wait-timeout 180 app web; then
      printf '%s\n' 'Previous app/web containers restored.' >&2
    else
      printf '%s\n' 'ROLLBACK FAILED: operator intervention required.' >&2
    fi
  fi
  exit "$status"
}
trap rollback_on_error EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
# Migrations must remain backward-compatible with the previous release.
"${current[@]}" run --rm --no-deps migrate
changed=true
"${current[@]}" up -d --no-deps --no-build --wait --wait-timeout 180 app web
curl --fail --silent --show-error --retry 5 --retry-delay 2 http://127.0.0.1:18472/ > /dev/null
printf '%s\n' "$sha" > "$state/deployed-sha"
printf 'Deployed %s; retained rollback config: %s\n' "$sha" "$release"
