package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SavePacket persists an InferredRequirementsPacket. PacketID and GeneratedAt
// are assigned here if not already set.
func (s *Store) SavePacket(ctx context.Context, pkt *InferredRequirementsPacket) error {
	if pkt.PacketID == "" {
		pkt.PacketID = uuid.New().String()
	}
	if pkt.GeneratedAt == "" {
		pkt.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	}
	raw, err := json.Marshal(pkt)
	if err != nil {
		return fmt.Errorf("marshal packet: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO inference_packets (id, context_id, session_id, generated_at, packet_json)
		VALUES (?, ?, ?, ?, ?)
	`, pkt.PacketID, pkt.ContextID, pkt.SyncSessionID, pkt.GeneratedAt, string(raw))
	if err != nil {
		return fmt.Errorf("save inference packet: %w", err)
	}
	return nil
}

// GetLatestPacket returns the most recently generated packet for a context,
// or nil if none exists.
func (s *Store) GetLatestPacket(ctx context.Context, contextID string) (*InferredRequirementsPacket, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT packet_json FROM inference_packets
		WHERE context_id = ?
		ORDER BY generated_at DESC
		LIMIT 1
	`, contextID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest packet: %w", err)
	}
	var pkt InferredRequirementsPacket
	if err := json.Unmarshal([]byte(raw), &pkt); err != nil {
		return nil, fmt.Errorf("unmarshal packet: %w", err)
	}
	return &pkt, nil
}

// ListPackets returns the most recent packets for a context, newest first.
func (s *Store) ListPackets(ctx context.Context, contextID string, limit int) ([]InferredRequirementsPacket, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT packet_json FROM inference_packets
		WHERE context_id = ?
		ORDER BY generated_at DESC
		LIMIT ?
	`, contextID, limit)
	if err != nil {
		return nil, fmt.Errorf("list packets: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []InferredRequirementsPacket
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan packet: %w", err)
		}
		var pkt InferredRequirementsPacket
		if err := json.Unmarshal([]byte(raw), &pkt); err != nil {
			return nil, fmt.Errorf("unmarshal packet: %w", err)
		}
		result = append(result, pkt)
	}
	return result, rows.Err()
}
