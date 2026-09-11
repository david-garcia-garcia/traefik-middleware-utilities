---
url: https://github.com/david-garcia-garcia/crowdsec-bouncer-traefik-plugin/blob/6548da47e933efe60309954be5f764f839b69e3f/pkg/simpleredis/simpleredis_test.go
ref: david-garcia-garcia/crowdsec-bouncer-traefik-plugin@6548da47:pkg/simpleredis/simpleredis_test.go
title: pkg/simpleredis unit tests
fetched: 2026-09-11
authority: source
---

No miniredis, httptest, or docker Redis. Fake servers: net.Listen("tcp", "127.0.0.1:0").
startFakeRedis: in-memory map; AUTH/SELECT +OK; GET/MGET bulk or $-1; SET stores; other cmds +OK.
startStaticRedis: canned RESP reply for every command.
Tests: TestGetHitAndMiss; TestConnectionIsReused (25 Gets, 1 conn); TestConcurrentCommandsStayWithinPool (≤8 conns); TestValueWithNewlinesSurvives; TestMGetHitsMissesAndEmpty; TestMGetKeepsValuesWithNewlinesAligned; TestMGetRejectsShortReply → redis:issue?; TestRejectedAuthIsReturned (NOAUTH); TestSetReturnsReplyError (duration -1, ERR text passthrough); TestDelSucceeds (+OK); TestUnreachableHost 127.0.0.1:1; TestStaleConnectionIsRetried; TestCloseDrainsIdleAndDoesNotRepool; TestAuthAndSelectOncePerDial (pass secret, db 2); TestTimeoutOnReusedConnIsNotRetried; TestIoTimeout; TestIdleTimeoutOpensANewConnection.
