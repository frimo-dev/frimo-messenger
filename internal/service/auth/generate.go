package auth

//go:generate mockgen -destination=./mocks/mocks.go -package=mocks . Repository,PasswordManager,TokenGenerator,VerificationTokenCipher,AccessTokenStorage,SessionRepository,Observer
