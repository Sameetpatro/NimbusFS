package heartbeat

// monitor.go intentionally lives in the same package as sender.go because both sides
// of the heartbeat protocol share domain types and logging conventions.
// the EvaluateNode implementation is in sender.go next to Monitor's struct definition
// to keep the monitor small; split files when Run loops land in phase 2.
