package model

// FlushQuotaBatchForTest runs the existing batch writer once without starting
// the process-wide background loop. Only linked into model's test binary.
func FlushQuotaBatchForTest() {
	batchUpdate()
	flushBatchLogs()
}
