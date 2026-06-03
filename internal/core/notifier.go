package core

type Notifier interface {
	Send(event Event) error
}
