package operator

// SessionPolicy classifies whether a machine mutation needs a physical operator session.
type SessionPolicy int

const (
	// SessionRequired: the mutation asserts a human is physically operating the machine.
	SessionRequired SessionPolicy = iota
	// SessionOptionalByOrigin: APP/on-machine sends a session (validated); remote admin may omit it.
	SessionOptionalByOrigin
	// SessionNotApplicable: operator session has no domain meaning.
	SessionNotApplicable
)
