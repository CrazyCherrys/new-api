package model

const dbBatchChunkSize = 500

func forEachChunk[T any](items []T, fn func([]T) error) error {
	for start := 0; start < len(items); start += dbBatchChunkSize {
		end := start + dbBatchChunkSize
		if end > len(items) {
			end = len(items)
		}
		if err := fn(items[start:end]); err != nil {
			return err
		}
	}
	return nil
}
