package httpapi

//go:generate mockgen -destination=./mocks/mocks.go -package=mocks . AuthService,UserService,MessageService,AccessTokenAuthenticator
