package notifier

/*
ESSENTIAL PROCESS:
Test logger fixture implementing log_interfaces.Logger for core unit tests.
Isolated to test builds (_test.go).

DATA FLOW:
Mock logger used in controller and notifier unit tests to avoid nil logger panics.
*/

import (
	log_interfaces "github.com/Bastien-Antigravity/universal-logger/src/interfaces"
)

type testNotifierLogger struct{}

func (t *testNotifierLogger) Debug(string, ...any)                                                         {}
func (t *testNotifierLogger) Info(string, ...any)                                                          {}
func (t *testNotifierLogger) Warning(string, ...any)                                                       {}
func (t *testNotifierLogger) Error(string, ...any)                                                         {}
func (t *testNotifierLogger) Critical(string, ...any)                                                      {}
func (t *testNotifierLogger) Stream(string, ...any)                                                        {}
func (t *testNotifierLogger) Logon(string, ...any)                                                         {}
func (t *testNotifierLogger) Logout(string, ...any)                                                        {}
func (t *testNotifierLogger) Trade(string, ...any)                                                         {}
func (t *testNotifierLogger) Schedule(string, ...any)                                                      {}
func (t *testNotifierLogger) Report(string, ...any)                                                        {}
func (t *testNotifierLogger) GetNotifQueue() <-chan *log_interfaces.NotifMessage                            { return nil }
func (t *testNotifierLogger) SetLocalNotifQueue(chan *log_interfaces.NotifMessage)                          {}
func (t *testNotifierLogger) Log(log_interfaces.Level, string, ...any)                                     {}
func (t *testNotifierLogger) LogWithCaller(log_interfaces.Level, string, string, string, string, string) {}
func (t *testNotifierLogger) SetLevel(log_interfaces.Level)                                                {}
func (t *testNotifierLogger) GetLevel() log_interfaces.Level                                                { return 0 }
func (t *testNotifierLogger) SetCallerSkip(int)                                                            {}
func (t *testNotifierLogger) SetMetadata(map[string]string)                                                {}
func (t *testNotifierLogger) AddMetadata(string, string)                                                   {}
func (t *testNotifierLogger) GetMetadata() map[string]string                                               { return nil }
func (t *testNotifierLogger) Close()                                                                       {}
