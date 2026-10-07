#!/bin/sh
# Checks Angel through a reverse proxy: health, PIN login, the PIN rate limit,
# and that a client can't dodge the limit with its own X-Forwarded-For.
#
#   ./check.sh [url]          default http://localhost:8080
#
# The PINs default to the demo's; set CAREGIVER_PIN for another setup. The run
# uses up the rate limit for your address for 15 minutes; in the demo,
# "docker compose restart angel" resets it.
set -eu

url=${1:-http://localhost:8080}
url=${url%/}
pin=${CAREGIVER_PIN:-1234}
wrong=0000
if [ "$pin" = "$wrong" ]; then wrong=0001; fi

failed=0
check() { # name, expected, actual
    if [ "$2" = "$3" ]; then
        echo "ok   $1"
    else
        echo "FAIL $1: expected $2, got $3"
        failed=1
    fi
}

status() { # curl arguments...
    curl -s -o /dev/null -w '%{http_code}' "$@"
}

check "GET /healthz" 200 "$(status "$url/healthz")"

headers=$(curl -s -o /dev/null -D - -d "pin=$pin" "$url/pin")
check "correct PIN redirects" 303 "$(echo "$headers" | awk 'NR==1 {print $2}')"
if echo "$headers" | grep -qi '^set-cookie: angel_session='; then
    check "correct PIN sets the session cookie" yes yes
else
    check "correct PIN sets the session cookie" yes no
fi

for i in 1 2 3 4 5; do
    code=$(status -d "pin=$wrong" "$url/pin")
    if [ "$code" != 401 ]; then
        check "wrong PIN $i is refused" 401 "$code"
        echo "Already rate limited? Wait 15 minutes, or restart Angel."
        exit 1
    fi
done
check "five wrong PINs are refused" 401 401
check "sixth wrong PIN is rate limited" 429 "$(status -d "pin=$wrong" "$url/pin")"

# If the proxy passed this header on, or Angel trusted the client, this would
# count as a new address and get 401.
check "faked X-Forwarded-For is still rate limited" 429 \
    "$(status -H 'X-Forwarded-For: 203.0.113.7' -d "pin=$wrong" "$url/pin")"

if [ "$failed" != 0 ]; then
    exit 1
fi
echo "All checks passed."
