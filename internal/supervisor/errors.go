package supervisor

import "errors"

// ErrDuplicateService is returned by Register when a service with the
// same name is already registered.
var ErrDuplicateService = errors.New("supervisor: service already registered")

// ErrUnknownService is returned by Health when the queried name is not
// registered.
var ErrUnknownService = errors.New("supervisor: unknown service")

// ErrAlreadyRunning is returned by Run when invoked while the
// supervisor is already running. Callers must Wait or cancel the
// previous invocation first.
var ErrAlreadyRunning = errors.New("supervisor: already running")
