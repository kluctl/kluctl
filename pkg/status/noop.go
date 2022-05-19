package status

type NoopStatusHandler struct {
}

type NoopStatusLine struct {
}

func (n NoopStatusHandler) SetTrace(trace bool) {
}

func (n NoopStatusHandler) Stop() {
}

func (n NoopStatusHandler) StartStatus(total int, message string) StatusLine {
	return &NoopStatusLine{}
}

func (n NoopStatusHandler) Info(message string) {
}

func (n NoopStatusHandler) Warning(message string) {
}

func (n NoopStatusHandler) Error(message string) {
}

func (n NoopStatusHandler) Trace(message string) {
}

func (n NoopStatusHandler) PlainText(text string) {
}

func (n NoopStatusHandler) InfoFallback(message string) {
}

var _ StatusHandler = &NoopStatusHandler{}

func (n NoopStatusLine) SetTotal(total int) {
}

func (n NoopStatusLine) Increment() {
}

func (n NoopStatusLine) Update(message string) {
}

func (n NoopStatusLine) End(result EndResult) {
}
