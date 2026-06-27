package nutrition

// Coverage отражает, на сколько реальных данных опирается оценка (unknown ≠ 0).
// 0..1; низкий coverage → правило возвращает StatusUnknown, а не ложный pass.
type Coverage float64
