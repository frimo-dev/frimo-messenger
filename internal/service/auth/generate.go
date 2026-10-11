package auth

//go:generate mockgen -destination=./mocks/mocks.go -package=mocks . Repository,SessionRepository,AccessTokenStorage,SessionStorage,RefreshResultStorage,Observer,PasswordManager,TokenGenerator,VerificationTokenCipher
