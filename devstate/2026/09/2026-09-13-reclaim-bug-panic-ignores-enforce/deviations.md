# Deviations

- [x] taken  occupy the key with a closer slot on enforced panic endings instead of calling endMappedClose on the same incarnation
  Asked: keep the panicking slot mapped slotBusy across Close, then end with endMappedClose / unmapAfterClose.
  Instead: record createErr on the ended incarnation, occupy the key with a new slotBusy closer, close the old ready, dispose, then unmapAfterClose on the closer.
  Owner: `reclaim/table.go` `endBusyAfterPanic`
  Why: the same slot's createErr is what Wake waiters replay; a later Open that parks during Close must create after Close, which TestRepro_WakePanicUnmapsBeforeCloseWithEnforce locks. Sharing one ready would give that later Open the Wake error.
  By: implement
  Requester: not asked
