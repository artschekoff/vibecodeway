package main

import (
	"context"
	"encoding/json"
	"fmt"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/spf13/cobra"
)

// Ask AI chat transcripts — for inspecting past conversations (prompt debugging).

func newAskChatListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ask-chat-list",
		Short: "List saved Ask AI chats, newest first",
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck
			chats, err := env.store.ListAskChats(context.Background())
			if err != nil {
				return err
			}
			if chats == nil {
				chats = []appstore.AskChat{}
			}
			return writeJSON(chats)
		},
	}
	return setAgentDocs(cmd,
		"List saved Ask AI conversations (no message bodies). Use ask-chat-get for the transcript.",
		"(none)",
		`[{"id":"uuid","title":"string","createdAt":"RFC3339","updatedAt":"RFC3339"}]`,
		`vibecodeway-cli ask-chat-list`)
}

func newAskChatGetCmd() *cobra.Command {
	var id string
	cmd := &cobra.Command{
		Use:   "ask-chat-get",
		Short: "Dump one Ask AI chat transcript including tool calls",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if id == "" {
				return fmt.Errorf("--id is required")
			}
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck
			chat, err := env.store.GetAskChat(context.Background(), id)
			if err != nil {
				return err
			}
			var log json.RawMessage = []byte(chat.LogJSON)
			if !json.Valid(log) {
				log = []byte("[]")
			}
			return writeJSON(map[string]any{
				"id": chat.ID, "title": chat.Title,
				"createdAt": chat.CreatedAt, "updatedAt": chat.UpdatedAt,
				"log": log,
			})
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "Chat id from ask-chat-list")
	return setAgentDocs(cmd,
		"Return a saved Ask AI chat with its full log: user turns, assistant answers and every tool call (name, args, raw result).",
		"--id string (required)",
		`{"id":"uuid","title":"string","createdAt":"RFC3339","updatedAt":"RFC3339","log":[{"role":"user","content":"string"},{"role":"assistant","content":"string","steps":[{"tool":"string","args":"json","result":"json"}]},{"role":"summary","content":"string"}]}`,
		`vibecodeway-cli ask-chat-get --id <uuid>`)
}
