**Sentry hook for Logrus**

Example usage:

```go
hook := huilogger.New("dsn", "environment")
hook.SetLevels([]logrus.Level{logrus.WarnLevel, logrus.ErrorLevel}) // Error, Fatal, Panic by default

err := hook.Init()
if err != nil {
    logger.WithError(err).Error("cannot init sentry")
}

logger.AddHook(hook)
```

Hook retrieves fields from WithField() functions and sends data to Sentry alongside error message and stack trace.