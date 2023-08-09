package status

type simpleStatusHandler struct {
	cb    func(level Level, message string)
	trace bool
}

type simpleStatusLine struct {
}

func NewSimpleStatusHandler(cb func(level Level, message string), trace bool) StatusHandler {
	return &simpleStatusHandler{
		cb:    cb,
		trace: trace,
	}
}

func (s *simpleStatusHandler) IsTraceEnabled() bool {
	return s.trace
}

func (s *simpleStatusHandler) SetTrace(trace bool) {
	s.trace = trace
}

func (s *simpleStatusHandler) Stop() {
}

func (s *simpleStatusHandler) Flush() {
}

func (s *simpleStatusHandler) StartStatus(level Level, total int, message string) StatusLine {
	if message != "" {
		s.Message(level, message)
	}
	return &simpleStatusLine{}
}

func (s *simpleStatusHandler) Message(level Level, message string) {
	if level == LevelTrace && !s.trace {
		return
	}
	s.cb(level, message)
}

func (s *simpleStatusHandler) MessageFallback(level Level, message string) {
	s.Message(level, message)
}

func (sl *simpleStatusLine) SetTotal(total int) {
}

func (sl *simpleStatusLine) Increment() {
}

func (sl *simpleStatusLine) Update(message string) {
}

func (sl *simpleStatusLine) End(result EndResult) {
}
