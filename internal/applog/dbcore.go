// Package applog provides a zap core that persists warn/error entries to SQLite.
package applog

import (
	"context"

	"go.uber.org/zap/zapcore"

	"github.com/artschekoff/context-consolidator/internal/store"
)

// DBCore is a zapcore.Core that writes warn-and-above entries to the app_logs table.
type DBCore struct {
	repo *store.AppLogRepo
	enc  zapcore.Encoder
	ctx  context.Context
}

// NewDBCore wraps repo with a zap core at WarnLevel and above.
func NewDBCore(ctx context.Context, repo *store.AppLogRepo) *DBCore {
	enc := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		TimeKey:    "ts",
		LevelKey:   "level",
		CallerKey:  "caller",
		MessageKey: "msg",
		EncodeLevel: zapcore.LowercaseLevelEncoder,
		EncodeTime:  zapcore.EpochTimeEncoder,
		EncodeCaller: zapcore.ShortCallerEncoder,
	})
	return &DBCore{repo: repo, enc: enc, ctx: ctx}
}

func (c *DBCore) Enabled(lvl zapcore.Level) bool {
	return lvl >= zapcore.WarnLevel
}

func (c *DBCore) With(fields []zapcore.Field) zapcore.Core {
	clone := *c
	enc := c.enc.Clone()
	for _, f := range fields {
		f.AddTo(enc)
	}
	clone.enc = enc
	return &clone
}

func (c *DBCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return ce.AddCore(entry, c)
	}
	return ce
}

func (c *DBCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	errMsg := ""
	for _, f := range fields {
		if f.Key == "error" && f.String != "" {
			errMsg = f.String
		}
	}
	_ = c.repo.Add(c.ctx, entry.Level.String(), float64(entry.Time.UnixMilli())/1000.0, entry.Caller.TrimmedPath(), entry.Message, errMsg)
	return nil
}

func (c *DBCore) Sync() error { return nil }
