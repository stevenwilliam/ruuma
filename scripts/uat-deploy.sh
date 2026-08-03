#!/usr/bin/env bash
#
# Publish the built SPA for LAN user-acceptance testing.
#
#   sudo /home/dev/projects/ruuma/scripts/uat-deploy.sh
#
# Run `make web-build` first, as your normal user — this script deliberately
# does not build, because building under sudo leaves root-owned artefacts in
# /home/dev/projects/ruuma/web/node_modules and breaks the next plain build.
#
# Re-runnable: publishing a new build is `make web-build` then this again.
set -euo pipefail

readonly SRC=/home/dev/projects/ruuma/web/dist
readonly DEST=/opt/ruuma/web
readonly CONF_SRC=/home/dev/projects/ruuma/deploy/nginx/ruuma-uat.conf
readonly CONF_DEST=/etc/nginx/sites-available/ruuma
readonly LAN_IP=192.168.88.101

if [[ $EUID -ne 0 ]]; then
    echo "error: run with sudo — this writes /opt/ruuma and /etc/nginx" >&2
    exit 1
fi

if [[ ! -f "${SRC}/index.html" ]]; then
    echo "error: ${SRC}/index.html is missing — run 'make web-build' first" >&2
    exit 1
fi

echo "==> publishing ${SRC} to ${DEST}"
install -d -m 0755 /opt/ruuma
rm -rf "${DEST}.previous"
# Guarded with an else branch: a bare `[[ ... ]] && mv` returns non-zero on the
# very first run, which under `set -e` would abort before anything is copied.
if [[ -d "${DEST}" ]]; then
    mv "${DEST}" "${DEST}.previous"
fi
cp -r "${SRC}" "${DEST}"
chown -R www-data:www-data "${DEST}"
find "${DEST}" -type d -exec chmod 0755 {} +
find "${DEST}" -type f -exec chmod 0644 {} +

echo "==> installing ${CONF_DEST}"
# Keep whatever was there before UAT started; the API-only proxy block is worth
# being able to put back if UAT is abandoned. Only ever written once: a second
# run would otherwise overwrite that original with the UAT config itself and
# quietly destroy the thing the backup exists for.
if [[ -f "${CONF_DEST}" && ! -f "${CONF_DEST}.pre-uat" ]]; then
    cp -a "${CONF_DEST}" "${CONF_DEST}.pre-uat"
    echo "    pre-UAT config saved as ${CONF_DEST}.pre-uat"
fi
cp "${CONF_SRC}" "${CONF_DEST}"
ln -sfn "${CONF_DEST}" /etc/nginx/sites-enabled/ruuma

# ruuma claims default_server, so the stock catch-all has to stand down or
# nginx refuses to start with a duplicate default_server on :80.
if [[ -L /etc/nginx/sites-enabled/default ]]; then
    rm -f /etc/nginx/sites-enabled/default
    echo "    disabled the stock 'default' site (file kept in sites-available)"
fi

echo "==> testing and reloading nginx"
nginx -t
systemctl reload nginx

echo "==> checking"
# `systemctl reload` returns as soon as nginx is signalled, not when the new
# workers are live. Checking immediately races the old workers, which still
# answer from the previous config — that looks like a 404 from the stock
# default site and reads as a broken deploy when nothing is wrong. Retry.
wait_for() {
    local url=$1 tries=${2:-10}
    for ((i = 0; i < tries; i++)); do
        if curl -fsS -o /dev/null --max-time 3 "${url}"; then
            return 0
        fi
        sleep 0.5
    done
    return 1
}

if ! wait_for "http://127.0.0.1/"; then
    echo "error: nginx is not serving the SPA" >&2
    exit 1
fi
# Distinguish "the SPA is up but stale" from a genuinely working deploy: the
# index served must be the build we just published, not a cached welcome page.
if ! curl -fsS --max-time 3 "http://127.0.0.1/" | grep -q "Ruuma Eatery"; then
    echo "error: port 80 answered, but not with the ruuma SPA" >&2
    exit 1
fi
if ! wait_for "http://127.0.0.1/api/v1/stores"; then
    echo "warning: /api/v1/stores did not answer — is the API up? (make run)" >&2
fi

cat <<EOF

Done. Testers on the local network open:

    http://${LAN_IP}/          customer site
    http://${LAN_IP}/admin     admin (staff sign-in)

Port 80 is already open in ufw, so no firewall change is needed.
To roll back: rm -rf ${DEST} && mv ${DEST}.previous ${DEST}
              cp ${CONF_DEST}.pre-uat ${CONF_DEST}
              ln -sfn /etc/nginx/sites-available/default /etc/nginx/sites-enabled/default
              nginx -t && systemctl reload nginx
EOF
