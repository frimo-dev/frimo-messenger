package auth

//go:generate mockgen -destination=./mocks/mocks.go -package=mocks . Repository,PasswordHasher,VerificationTokenGenerator,VerificationTokenCipher
