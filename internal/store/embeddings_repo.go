package store

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

// UpsertMessageEmbedding stores or replaces the embedding for a message.
func (s *Store) UpsertMessageEmbedding(ctx context.Context, messageID, model string, vec []float32) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO message_embeddings (message_id, embedding, model, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(message_id) DO UPDATE SET embedding = excluded.embedding, model = excluded.model
	`, messageID, float32sToBytes(vec), model, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("upsert message embedding: %w", err)
	}
	return nil
}

// ListContextEmbeddings returns all stored (messageID, embedding) pairs for a context.
// Used for brute-force cosine ranking.
func (s *Store) ListContextEmbeddings(ctx context.Context, contextID string) (map[string][]float32, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT me.message_id, me.embedding
		FROM message_embeddings me
		JOIN messages m ON m.id = me.message_id
		WHERE m.context_id = ?
	`, contextID)
	if err != nil {
		return nil, fmt.Errorf("list context embeddings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string][]float32)
	for rows.Next() {
		var msgID string
		var blob []byte
		if err := rows.Scan(&msgID, &blob); err != nil {
			return nil, fmt.Errorf("scan embedding: %w", err)
		}
		result[msgID] = bytesToFloat32s(blob)
	}
	return result, rows.Err()
}

// HasEmbedding reports whether a message already has a stored embedding.
func (s *Store) HasEmbedding(ctx context.Context, messageID string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM message_embeddings WHERE message_id = ?`, messageID,
	).Scan(&n)
	return n > 0, err
}

// float32sToBytes serialises a float32 slice as little-endian bytes (4 bytes each).
func float32sToBytes(v []float32) []byte {
	b := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(f))
	}
	return b
}

// bytesToFloat32s deserialises little-endian bytes back to a float32 slice.
func bytesToFloat32s(b []byte) []float32 {
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}
