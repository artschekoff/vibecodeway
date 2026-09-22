package main

import (
	"errors"

	"github.com/artschekoff/vibecodeway/internal/symbols"
)

// SymbolsIndexResult is returned by SymbolsIndex.
type SymbolsIndexResult struct {
	Count  int `json:"count"`
	TookMs int `json:"tookMs"`
}

// symbolsErr keeps the ctags sentinel verbatim for the frontend string-match;
// other errors pass through unchanged.
func symbolsErr(err error) error {
	if errors.Is(err, symbols.ErrCtagsNotFound) {
		return symbols.ErrCtagsNotFound
	}
	return err
}

// SymbolsIndex force-rebuilds the symbol index for repoPath's worktree.
func (a *App) SymbolsIndex(repoPath string) (SymbolsIndexResult, error) {
	count, took, err := a.symbolsSvc.Index(repoPath)
	if err != nil {
		return SymbolsIndexResult{}, symbolsErr(err)
	}
	return SymbolsIndexResult{Count: count, TookMs: took}, nil
}

// SymbolLookup returns definitions for an exact symbol name.
func (a *App) SymbolLookup(repoPath, name string) ([]symbols.Symbol, error) {
	out, err := a.symbolsSvc.Lookup(repoPath, name)
	return out, symbolsErr(err)
}

// SymbolSearch returns ranked substring matches for the palette.
func (a *App) SymbolSearch(repoPath, query string, limit int) ([]symbols.Symbol, error) {
	out, err := a.symbolsSvc.Search(repoPath, query, limit)
	return out, symbolsErr(err)
}

// SymbolReferences lists usages of name via git grep (works without ctags).
func (a *App) SymbolReferences(repoPath, name string, limit int) ([]symbols.Reference, error) {
	return symbols.References(repoPath, name, limit)
}
