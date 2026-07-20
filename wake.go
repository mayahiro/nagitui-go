package tui

type runtimeWake func()

func (wake runtimeWake) notify() {
	if wake != nil {
		wake()
	}
}
