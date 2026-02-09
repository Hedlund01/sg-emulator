package messages

// AccountCountPayload contains parameters for AccountCount (empty)
type AccountCountPayload struct{}

// AccountCountResponse contains the result of AccountCount
type AccountCountResponse struct {
	Count int
}
