package logrussentryhook

import (
	"errors"
	"reflect"
	"runtime"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
)

var (
	levelsMap = map[logrus.Level]sentry.Level{
		logrus.PanicLevel: sentry.LevelFatal,
		logrus.FatalLevel: sentry.LevelFatal,
		logrus.ErrorLevel: sentry.LevelError,
		logrus.WarnLevel:  sentry.LevelWarning,
		logrus.InfoLevel:  sentry.LevelInfo,
		logrus.DebugLevel: sentry.LevelDebug,
		logrus.TraceLevel: sentry.LevelDebug,
	}

	levels = []logrus.Level{logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel}
)

type SentryHook struct {
	IgnoredModules   []string
	dsn, environment string
}

func (hook *SentryHook) Levels() []logrus.Level {
	return levels
}

func (hook *SentryHook) Fire(entry *logrus.Entry) error {
	// вытаскиваем fields
	for key, value := range entry.Data {
		if key != logrus.ErrorKey {
			sentry.ConfigureScope(func(scope *sentry.Scope) {
				scope.SetExtra(key, value)
			})
		}
	}

	// если есть error - и ее вытаскиваем
	var err error
	if val, ok := entry.Data[logrus.ErrorKey]; ok {
		if errValue, ok := val.(error); ok {
			err = errValue
		}
	}

	// если ошибку не передали - сделаем пустую
	if err == nil {
		err = errors.New("nil")
	}

	//
	event := sentry.NewEvent()
	stacktrace := hook.newStacktrace()
	event.Level = levelsMap[entry.Level]
	event.Exception = []sentry.Exception{{
		Value:      err.Error(),
		Type:       reflect.TypeOf(err).String(),
		Stacktrace: stacktrace,
	}}

	sentry.CaptureEvent(event)
	return nil
}

func New(dsn, environment string, ignoredModules ...string) *SentryHook {
	// игнорю сам себя и логрус
	ignoredModules = append(ignoredModules, reflect.TypeOf(SentryHook{}).PkgPath(), "github.com/sirupsen/logrus")
	return &SentryHook{IgnoredModules: ignoredModules}
}

func (hook *SentryHook) SetLevels(newLevels []logrus.Level) {
	levels = newLevels
}

func (hook *SentryHook) Init() error {
	err := sentry.Init(sentry.ClientOptions{
		Dsn:         hook.dsn,
		Environment: hook.environment,
	})
	return err
}

func (hook *SentryHook) extractFrames(pcs []uintptr) []sentry.Frame {
	var frames []sentry.Frame
	callersFrames := runtime.CallersFrames(pcs)

	for {
		callerFrame, more := callersFrames.Next()

		frames = append([]sentry.Frame{
			sentry.NewFrame(callerFrame),
		}, frames...)

		if !more {
			break
		}
	}

	return frames
}

func (hook *SentryHook) filterFrames(frames []sentry.Frame) []sentry.Frame {
	if len(frames) == 0 {
		return nil
	}

	filteredFrames := make([]sentry.Frame, 0, len(frames))

OUTER:
	for _, frame := range frames {
		// Skip Go internal frames.
		if frame.Module == "runtime" || frame.Module == "testing" {
			continue
		}
		// Skip Sentry internal frames, except for frames in _test packages (for
		// testing).
		if strings.HasPrefix(frame.Module, "github.com/getsentry/sentry-go") &&
			!strings.HasSuffix(frame.Module, "_test") {
			continue
		}

		for _, module := range hook.IgnoredModules {
			if strings.HasPrefix(frame.Module, module) {
				continue OUTER
			}
		}

		filteredFrames = append(filteredFrames, frame)
	}

	return filteredFrames
}

func (hook *SentryHook) newStacktrace() *sentry.Stacktrace {
	pcs := make([]uintptr, 100)
	n := runtime.Callers(1, pcs)

	if n == 0 {
		return nil
	}

	frames := hook.extractFrames(pcs[:n])
	frames = hook.filterFrames(frames)

	stacktrace := sentry.Stacktrace{
		Frames: frames,
	}

	return &stacktrace
}
