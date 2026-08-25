package dto

import "uuid"

const EmailVerificationRequestedType = "email.verification.requested"

type EmailVerificationRequested struct {
	VerificationID uuid.UUID `json:"verification_id"`
	Recipient      string    `json:"recipient"`
}
