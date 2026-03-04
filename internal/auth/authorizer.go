package auth

import "context"

// Action represents an operation that can be performed on a topic.
type Action string

const (
	ActionRead  Action = "read"
	ActionWrite Action = "write"
	ActionAdmin Action = "admin"
)

// PermissionLevel returns the numeric level for permission ordering.
// admin (3) >= write (2) >= read (1).
func PermissionLevel(a Action) int {
	switch a {
	case ActionAdmin:
		return 3
	case ActionWrite:
		return 2
	case ActionRead:
		return 1
	default:
		return 0
	}
}

// Authorizer checks whether a principal is allowed to perform actions on topics.
type Authorizer interface {
	// Check returns nil if the principal may perform the action on the topic.
	Check(ctx context.Context, principal *Principal, topicID string, action Action) error

	// AccessibleTopics returns the set of topic IDs the principal can access
	// with at least the given permission level.
	AccessibleTopics(ctx context.Context, principal *Principal, minAction Action) ([]string, error)
}
