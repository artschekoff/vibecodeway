package hotkey

import (
	"fmt"

	gohk "golang.design/x/hotkey"
)

// Service manages a global Cmd+Shift+C hotkey registration.
// It is framework-agnostic: it calls onTrigger when the hotkey fires and
// leaves clipboard reading and window management to the caller.
type Service struct {
	hk        *gohk.Hotkey
	onTrigger func()
	stop      chan struct{}
}

// New creates a new Service. onTrigger is called in a new goroutine each time
// the Cmd+Shift+C hotkey is pressed.
func New(onTrigger func()) *Service {
	return &Service{
		onTrigger: onTrigger,
		stop:      make(chan struct{}),
	}
}

// Start registers the Cmd+Shift+C global hotkey and begins listening.
// It should be called after the application window is fully initialised.
func (s *Service) Start() error {
	s.hk = gohk.New([]gohk.Modifier{gohk.ModCmd, gohk.ModShift}, gohk.KeyC)
	if err := s.hk.Register(); err != nil {
		return fmt.Errorf("registering global hotkey Cmd+Shift+C: %w", err)
	}
	go s.listen()
	return nil
}

// Stop unregisters the hotkey and stops the listener goroutine.
func (s *Service) Stop() {
	select {
	case <-s.stop:
		// already stopped
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
