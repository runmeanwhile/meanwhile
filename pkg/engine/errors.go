package engine

import "errors"

var (
	// ErrRequestTimedOut indicates the request expired before a response arrived.
	ErrRequestTimedOut = errors.New("request timed out")
	// ErrRequestNotFound indicates a pending request could not be found.
	ErrRequestNotFound = errors.New("request not found")
	// ErrHumanRequestNotFound indicates a human request record could not be found.
	ErrHumanRequestNotFound = errors.New("human request not found")
	// ErrRequestRegistryRequired indicates a request registry is required.
	ErrRequestRegistryRequired = errors.New("request registry required")
	// ErrHumanRequestStoreRequired indicates a human request store is required.
	ErrHumanRequestStoreRequired = errors.New("human request store required")
	// ErrRequestNotTimedOut indicates the request has not reached its timeout yet.
	ErrRequestNotTimedOut = errors.New("request has not timed out")
	// ErrResponseRequired indicates a response message is required.
	ErrResponseRequired = errors.New("response required")
	// ErrTimeoutNoteRequired indicates a timeout note is required for continue policies.
	ErrTimeoutNoteRequired = errors.New("timeout note required")
	// ErrRetryParticipantRequired indicates a retry participant is required.
	ErrRetryParticipantRequired = errors.New("retry participant required")
	// ErrSessionIncomplete indicates the session was marked incomplete.
	ErrSessionIncomplete = errors.New("session marked incomplete")
	// ErrTimeoutPolicyRequired indicates a timeout policy is required.
	ErrTimeoutPolicyRequired = errors.New("timeout policy required")
	// ErrSessionNotResumable indicates a persisted session cannot resume pending requests.
	ErrSessionNotResumable = errors.New("session cannot resume pending requests")
)
