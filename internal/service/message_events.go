package service

import "machine-logbook/internal/domain"

type MessageEvent struct {
	Type      string          `json:"type"`
	MachineID int64           `json:"machine_id"`
	MessageID string          `json:"message_id"`
	Message   *domain.Message `json:"message,omitempty"`
}

// SubscribeMessages closes slow subscribers so they reconnect and reload persisted history.
func (s *Logbooks) SubscribeMessages(machineID int64) (<-chan MessageEvent, func()) {
	s.messageMu.Lock()
	defer s.messageMu.Unlock()
	if s.messageSubscribers == nil {
		s.messageSubscribers = make(map[chan MessageEvent]int64)
	}
	ch := make(chan MessageEvent, 64)
	s.messageSubscribers[ch] = machineID
	return ch, func() {
		s.messageMu.Lock()
		defer s.messageMu.Unlock()
		if _, ok := s.messageSubscribers[ch]; ok {
			delete(s.messageSubscribers, ch)
			close(ch)
		}
	}
}

func (s *Logbooks) publishMessage(event MessageEvent) {
	s.messageMu.Lock()
	defer s.messageMu.Unlock()
	for ch, machineID := range s.messageSubscribers {
		if machineID != event.MachineID {
			continue
		}
		select {
		case ch <- event:
		default:
			delete(s.messageSubscribers, ch)
			close(ch)
		}
	}
}
