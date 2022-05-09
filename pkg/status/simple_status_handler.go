package status

type simpleStatusHandler struct {
	cb    func(message string)
	trace bool
}

type simpleStatusLine struct {
}

func NewSimpleStatusHandler(cb func(message string), trace bool) StatusHandler {
	return &simpleStatusHandler{
		cb:    cb,
		trace: trace,
	}
}

func (s *simpleStatusHandler) Stop() {
}

func (s *simpleStatusHandler) StartStatus(total int, message string) StatusLine {
	if message != "" {
		s.Info(message)
	}
	return &simpleStatusLine{}
}

func (s *simpleStatusHandler) Info(message string) {
	s.cb(message)
}

func (s *simpleStatusHandler) Warning(message string) {
	s.InfoFallback(message)
}

func (s *simpleStatusHandler) Error(message string) {
	s.InfoFallback(message)
}

func (s *simpleStatusHandler) Trace(message string) {
	if s.trace {
		s.Info(message)
	}
}

func (s *simpleStatusHandler) PlainText(text string) {
	s.Info(text)
}

func (s *simpleStatusHandler) InfoFallback(message string) {
	s.Info(message)
}

func (sl *simpleStatusLine) SetTotal(total int) {
}

func (sl *simpleStatusLine) Increment() {
}

func (sl *simpleStatusLine) Update(message string) {
}

func (sl *simpleStatusLine) End(result EndResult) {
}
