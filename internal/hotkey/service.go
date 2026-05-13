package hotkey

import (
	"fmt"
	"strings"

	gohk "golang.design/x/hotkey"
)

// Service manages a configurable global hotkey.
// It is framework-agnostic: it calls onTrigger when the hotkey fires and
// leaves clipboard reading and window management to the caller.
type Service struct {
	hk        *gohk.Hotkey
	onTrigger func()
	stop      chan struct{}
}

// New creates a new Service. onTrigger is called in a new goroutine each time
// the hotkey is pressed.
func New(onTrigger func()) *Service {
	return &Service{
		onTrigger: onTrigger,
		stop:      make(chan struct{}),
	}
}

// Start registers the hotkey described by modifierKeys and keyCode and begins listening.
// modifierKeys is a comma-separated string (e.g. "cmd,shift"), keyCode is e.g. "c".
// It should be called after the application window is fully initialised.
func (s *Service) Start(modifierKeys, keyCode string) error {
	mods, err := ParseModifiers(strings.Split(modifierKeys, ","))
	if err != nil {
		return fmt.Errorf("parse modifiers: %w", err)
	}
	key, err := ParseKey(keyCode)
	if err != nil {
		return fmt.Errorf("parse key: %w", err)
	}
	s.hk = gohk.New(mods, key)
	if err := s.hk.Register(); err != nil {
		return fmt.Errorf("registering global hotkey: %w", err)
	}
	go s.listen()
	return nil
}

// Reconfigure unregisters the current hotkey and registers a new one.
// Calling Reconfigure while the service is not started is safe but a no-op
// for the unregister step.
func (s *Service) Reconfigure(modifierKeys, keyCode string) error {
	// Stop current listener goroutine.
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
	if s.hk != nil {
		_ = s.hk.Unregister()
	}

	mods, err := ParseModifiers(strings.Split(modifierKeys, ","))
	if err != nil {
		return fmt.Errorf("parse modifiers: %w", err)
	}
	key, err := ParseKey(keyCode)
	if err != nil {
		return fmt.Errorf("parse key: %w", err)
	}

	s.stop = make(chan struct{})
	s.hk = gohk.New(mods, key)
	if err := s.hk.Register(); err != nil {
		return fmt.Errorf("registering global hotkey: %w", err)
	}
	go s.listen()
	return nil
}

// Stop unregisters the hotkey and stops the listener goroutine.
func (s *Service) Stop() {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
	if s.hk != nil {
		_ = s.hk.Unregister()
	}
}

func (s *Service) listen() {
	for {
		select {
		case <-s.hk.Keydown():
			go s.onTrigger()
		case <-s.stop:
			return
		}
	}
}
