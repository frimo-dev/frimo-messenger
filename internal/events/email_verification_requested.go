package events

const EmailVerificationRequestedType = "email.verification.requested"

type EmailVerificationRequested struct {
	VerificationID string `json:"verification_id"`
	Recipient string `json:"recipient"`
}