---
url: https://redis.io/docs/latest/operate/oss_and_stack/management/security/encryption/
title: TLS
fetched: 2026-11-21
authority: official
---

"Running manually": `./src/redis-server --tls-port 6379 --port 0 --tls-cert-file ./tests/tls/redis.crt --tls-key-file ./tests/tls/redis.key --tls-ca-cert-file ./tests/tls/ca.crt`.

"You may specify `port 0` to disable the non-TLS port completely."

"By default, Redis uses mutual TLS and requires clients to authenticate with a valid certificate" … "You may use `tls-auth-clients no` to disable client authentication."

`tls-protocols` default: not stated on the page.
