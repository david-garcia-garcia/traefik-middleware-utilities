---
url: https://github.com/dragonflydb/dragonfly/blob/36eaa127c2e5dd4f84724828a4c7c1412c4be90c/src/server/main_service.cc
ref: dragonflydb/dragonfly@36eaa127:src/server/main_service.cc
title: main_service.cc — EVAL error reply
fetched: 2026-09-11
authority: source
---

if (result == Interpreter::RUN_ERR) {
  string resp = StrCat("Error running script (call to ", eval_args.sha, "): ", error);
  server_family_.script_mgr()->OnScriptError(eval_args.sha, error);
  return cmd_cntx->SendError(resp, facade::kScriptErrType);
}

Also strips "Error running script (call to …): " prefix in some error paths when is_eval.
