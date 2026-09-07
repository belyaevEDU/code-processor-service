#!/bin/sh
# The image's entrypoint is fixed to run.sh.
# This wrapper takes in the TRANSLATOR env var set, decodes the user code from USER_CODE_B64

set -u

if [ -z "${USER_CODE_B64:-}" ]; then
    echo "USER_CODE_B64 is not set" >&2
    exit 2
fi

case "${TRANSLATOR:-}" in
python3)
    printf '%s' "$USER_CODE_B64" | base64 -d > /tmp/main.py
    exec python3 /tmp/main.py
    ;;
gcc)
    printf '%s' "$USER_CODE_B64" | base64 -d > /tmp/main.c
    gcc /tmp/main.c -o /tmp/prog && exec /tmp/prog
    ;;
clang)
    printf '%s' "$USER_CODE_B64" | base64 -d > /tmp/main.cpp
    clang++ /tmp/main.cpp -o /tmp/prog && exec /tmp/prog
    ;;
*)
    echo "unsupported translator: ${TRANSLATOR}" >&2
    exit 2
    ;;
esac
