package sim

// Command expresses simulation intent in semantic units: a movement intent, an
// interaction, an impulse, a scheduled block tick. A command carries no packet
// identifier and no encoded wire value.
//
// This is an interface rather than a struct because the intents belong to the
// rules that consume them, and no rule exists yet. Nothing hashes a command:
// the digest covers the result, which records concrete outcomes instead.
//
// The adapter that creates a command remains responsible for authentication and
// network-level authorization. A profile decides only whether the actor and the
// current state permit it.
type Command interface {
	// CommandKind returns a namespaced kind, such as "movement.walk".
	CommandKind() string
}

// CommandOutcome records what a tick did with one command.
//
// Index is the command's position in the tick's input, so a rejection is
// traceable back to what caused it without the outcome copying the command.
type CommandOutcome struct {
	Index    int
	Kind     string
	Accepted bool
	// Reason explains a rejection. It is empty when Accepted is true.
	Reason string
}
