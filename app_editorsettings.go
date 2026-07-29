package main

// GetEditorSettings returns the Monaco editor settings as an opaque JSON blob.
// Empty string means "never set" — the frontend then applies its own defaults.
func (a *App) GetEditorSettings() (string, error) {
	s, err := a.db.GetAppSettings(a.ctx)
	if err != nil {
		return "", err
	}
	return s.EditorSettings, nil
}

// SetEditorSettings persists the Monaco editor settings JSON blob. Go treats the
// value as opaque (the frontend owns its shape) and preserves all other app
// settings, mirroring SetAutoLinkRepos.
func (a *App) SetEditorSettings(jsonBlob string) error {
	current, err := a.db.GetAppSettings(a.ctx)
	if err != nil {
		return err
	}
	current.EditorSettings = jsonBlob
	return a.db.SetAppSettings(a.ctx, *current)
}
