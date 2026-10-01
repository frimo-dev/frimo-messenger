package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"net/netip"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/frimo-dev/frimo-messenger/internal/dto"
	"github.com/frimo-dev/frimo-messenger/internal/outbox"
)

type TokenGenerator interface {
	Generate() (rawToken string, tokenHash []byte, err error)
	Hash(rawToken string) []byte
}

type PasswordManager interface {
	Hash(password string) (string, error)
	Verify(encodedHash, password string) error
}

type VerificationTokenCipher interface {
	Encrypt(plaintext []byte, additionalData []byte) ([]byte, error)
}

type User struct {
	ID        uuid.UUID
	Email     string
	CreatedAt time.Time
}

type RegistrationInput struct {
	Email    string
	Password string
}

type LoginInput struct {
	Email      string
	Password   string
	IP         netip.Addr
	DeviceName string
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

type Service struct {
	repository        Repository
	sessionRepository SessionRepository

	accessTokenStorage AccessTokenStorage
	sessionStorage     SessionStorage

	passwordManager PasswordManager
	tokenGenerator  TokenGenerator
	tokenCipher     VerificationTokenCipher
	now             func() time.Time

	verificationTokenLifetime time.Duration
	accessTokenLifetime       time.Duration
	sessionInactivityTimeout  time.Duration
}

func NewService(
	repository Repository,
	sessionRepository SessionRepository,
	accessTokenStorage AccessTokenStorage,
	sessionStorage SessionStorage,
	passwordManager PasswordManager,
	tokenGenerator TokenGenerator,
	tokenCipher VerificationTokenCipher,
	now func() time.Time,

	verificationTokenLifetime time.Duration,
	accessTokenLifetime time.Duration,
	sessionInactivityTimeout time.Duration,
) *Service {
	return &Service{
		repository:         repository,
		sessionRepository:  sessionRepository,
		accessTokenStorage: accessTokenStorage,
		sessionStorage:     sessionStorage,
		passwordManager:    passwordManager,
		tokenGenerator:     tokenGenerator,
		tokenCipher:        tokenCipher,
		now:                now,

		verificationTokenLifetime: verificationTokenLifetime,
		accessTokenLifetime:       accessTokenLifetime,
		sessionInactivityTimeout:  sessionInactivityTimeout,
	}
}

// Authenticate TODO: добавить логирование недоступности Redis
func (s *Service) Authenticate(ctx context.Context, rawAccessToken string) (Identity, error) {
	identity, err := s.accessTokenStorage.Get(ctx, s.tokenGenerator.Hash(rawAccessToken))
	if err != nil {
		return Identity{}, fmt.Errorf("failed to authenticate: %w", err)
	}

	isActive, err := s.sessionStorage.GetSession(ctx, identity.SessionID)
	if err == nil {
		if !isActive {
			return Identity{}, ErrSessionInactive
		}

		return identity, nil
	}

	sessionState, err := s.sessionRepository.GetSessionState(ctx, identity.SessionID)
	if err != nil {
		return Identity{}, fmt.Errorf("get session state: %w", err)
	}

	now := s.now()

	isActive = sessionState.RevokedAt == nil

	ttl := 5 * time.Minute
	if isActive {
		ttl = sessionState.ExpiresAt.Sub(now)
	}

	_ = s.sessionStorage.SetSession(ctx, identity.SessionID, isActive, ttl)

	if !isActive {
		return Identity{}, ErrSessionInactive
	}

	return identity, nil
}

func (s *Service) Refresh(ctx context.Context, oldRawRefreshToken string) (TokenPair, error) {
	oldRefreshTokenHash := s.tokenGenerator.Hash(oldRawRefreshToken)

	rawRefreshToken, refreshTokenHash, err := s.tokenGenerator.Generate()
	if err != nil {
		return TokenPair{}, fmt.Errorf("failed generate refresh token: %w", err)
	}

	// TODO: hash must be saved in Redis preferably in a transaction
	rawAccessToken, accessTokenHash, err := s.tokenGenerator.Generate()
	if err != nil {
		return TokenPair{}, fmt.Errorf("failed generate access token: %w", err)
	}

	now := s.now()

	refreshToken := RefreshToken{
		ID:        uuid.New(),
		TokenHash: refreshTokenHash,
		CreatedAt: now,
	}

	extendSessionInput := ExtendSessionInput{
		OldRefreshTokenHash: oldRefreshTokenHash,
		NewRefreshToken:     refreshToken,
		SessionLifetime:     s.sessionInactivityTimeout,
	}

	identity, err := s.sessionRepository.ExtendSession(ctx, extendSessionInput)
	if err != nil {
		return TokenPair{}, fmt.Errorf("failed to extend session: %w", err)
	}

	err = s.sessionStorage.SetSession(ctx, identity.SessionID, true, s.sessionInactivityTimeout)
	if err != nil {
		// TODO: report recoverable cache error via observability mechanism.
	}

	if err = s.accessTokenStorage.Store(ctx, accessTokenHash, identity, s.accessTokenLifetime); err != nil {
		return TokenPair{AccessToken: rawAccessToken, RefreshToken: rawRefreshToken}, errors.Join(ErrAccessTokenNotStored, err)
	}

	return TokenPair{AccessToken: rawAccessToken, RefreshToken: rawRefreshToken}, nil
}

// Login - Возврат ошибки ErrAccessTokenNotStored не означает, что операция не выполнена.
// Это означает, что access token система не запомнила из-за недоступности кэша, но refresh сделать можно, он в БД
func (s *Service) Login(ctx context.Context, input LoginInput) (TokenPair, error) {
	email := normalizeEmail(input.Email)

	if err := validateEmail(email); err != nil {
		return TokenPair{}, err
	}

	if err := validatePassword(input.Password); err != nil {
		return TokenPair{}, err
	}

	loginUser, err := s.repository.GetUserForLogin(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return TokenPair{}, ErrInvalidCredentials
		}
		return TokenPair{}, fmt.Errorf("failed get user for login: %w", err)
	}

	err = s.passwordManager.Verify(loginUser.PasswordHash, input.Password)
	if err != nil {
		if !errors.Is(err, ErrInvalidCredentials) {
			return TokenPair{}, fmt.Errorf("failed verify password: %w", err)
		}

		return TokenPair{}, err
	}

	if loginUser.VerifiedAt == nil {
		return TokenPair{}, ErrEmailNotVerified
	}

	deviceName := strings.TrimSpace(input.DeviceName)
	if err := validateDeviceName(deviceName); err != nil {
		return TokenPair{}, err
	}

	now := s.now().UTC()

	session := Session{
		ID:     uuid.New(),
		UserID: loginUser.ID,

		DeviceName: deviceName,

		// TODO: правильно ли БД будет интерпретировать zero value netip.Addr{}
		CreatedIP: input.IP,
		LastIP:    input.IP,

		CreatedAt:  now,
		LastSeenAt: now,

		ExpiresAt: now.Add(s.sessionInactivityTimeout),
	}

	rawRefreshToken, refreshTokenHash, err := s.tokenGenerator.Generate()
	if err != nil {
		return TokenPair{}, fmt.Errorf("failed generate refresh token: %w", err)
	}

	refreshToken := RefreshToken{
		ID:        uuid.New(),
		TokenHash: refreshTokenHash,
		CreatedAt: now,
	}

	err = s.sessionRepository.CreateSession(ctx, session, refreshToken)
	if err != nil {
		return TokenPair{}, fmt.Errorf("failed create session: %w", err)
	}

	// TODO: hash must be saved in Redis preferably in a transaction
	rawAccessToken, accessTokenHash, err := s.tokenGenerator.Generate()
	if err != nil {
		return TokenPair{}, fmt.Errorf("failed generate access token: %w", err)
	}

	if err = s.accessTokenStorage.Store(ctx, accessTokenHash, Identity{UserID: loginUser.ID, SessionID: session.ID}, s.accessTokenLifetime); err != nil {
		return TokenPair{AccessToken: rawAccessToken, RefreshToken: rawRefreshToken}, errors.Join(ErrAccessTokenNotStored, err)
	}

	return TokenPair{AccessToken: rawAccessToken, RefreshToken: rawRefreshToken}, nil
}

func (s *Service) Register(ctx context.Context, input RegistrationInput) (User, error) {
	email := normalizeEmail(input.Email)

	if err := validateEmail(email); err != nil {
		return User{}, err
	}

	if err := validatePassword(input.Password); err != nil {
		return User{}, err
	}

	passwordHash, err := s.passwordManager.Hash(input.Password)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}

	now := s.now().UTC()
	userID := uuid.New()

	verificationInput, outboxEvent, err := s.prepareEmailVerification(email, now)
	if err != nil {
		return User{}, err
	}

	createdUser, err := s.repository.CreateUser(
		ctx,
		CreateUserInput{
			ID:           userID,
			Email:        email,
			PasswordHash: passwordHash,
			CreatedAt:    now,

			Verification: verificationInput,

			OutboxEvent: outboxEvent,
		},
	)
	if err != nil {
		return User{}, err
	}

	return createdUser, nil
}

func (s *Service) ConfirmEmail(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return ErrInvalidToken
	}

	tokenHash := s.tokenGenerator.Hash(rawToken)

	return s.repository.ConfirmEmail(ctx, tokenHash, s.now().UTC())
}

func (s *Service) ResendVerification(ctx context.Context, email string) error {
	email = normalizeEmail(email)

	if err := validateEmail(email); err != nil {
		return err
	}

	now := s.now().UTC()

	verificationInput, outboxEvent, err := s.prepareEmailVerification(email, now)
	if err != nil {
		return err
	}

	err = s.repository.ResendVerification(
		ctx,
		ResendVerificationInput{
			Email:       email,
			RequestedAt: now,

			Verification: verificationInput,

			OutboxEvent: outboxEvent,
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func (s *Service) generateVerificationInput(now time.Time) (VerificationInput, error) {
	rawToken, tokenHash, err := s.tokenGenerator.Generate()
	if err != nil {
		return VerificationInput{}, fmt.Errorf("generate verification token: %w", err)
	}

	verificationID := uuid.New()

	tokenCiphertext, err := s.tokenCipher.Encrypt([]byte(rawToken), verificationID[:])
	if err != nil {
		return VerificationInput{}, fmt.Errorf("encrypt verification token: %w", err)
	}

	return VerificationInput{
		ID:              verificationID,
		TokenHash:       tokenHash,
		TokenCiphertext: tokenCiphertext,
		ExpiresAt:       now.Add(s.verificationTokenLifetime),
	}, nil
}

func (s *Service) prepareEmailVerification(email string, now time.Time) (VerificationInput, outbox.Event, error) {
	verification, err := s.generateVerificationInput(now)
	if err != nil {
		return VerificationInput{}, outbox.Event{}, err
	}

	payload, err := json.Marshal(
		dto.EmailVerificationRequested{
			VerificationID: verification.ID,
			Recipient:      email,
		},
	)
	if err != nil {
		return VerificationInput{}, outbox.Event{},
			fmt.Errorf("marshal verification event: %w", err)
	}

	event := outbox.Event{
		ID:          uuid.New(),
		Type:        dto.EmailVerificationRequestedType,
		Payload:     payload,
		CreatedAt:   now,
		AvailableAt: now,
	}

	return verification, event, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validateEmail(email string) error {
	if email == "" {
		return &ValidationError{
			Code:    "email_required",
			Field:   "email",
			Message: "email is required",
		}
	}

	if len(email) > 254 {
		return &ValidationError{
			Code:    "email_too_long",
			Field:   "email",
			Message: "email is too long",
		}
	}

	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return &ValidationError{
			Code:    "invalid_email",
			Field:   "email",
			Message: "email has invalid format",
		}
	}

	return nil
}

func validatePassword(password string) error {
	if password == "" {
		return &ValidationError{
			Code:    "password_required",
			Field:   "password",
			Message: "password is required",
		}
	}

	length := utf8.RuneCountInString(password)

	if length < 12 {
		return &ValidationError{
			Code:    "password_too_short",
			Field:   "password",
			Message: "password must contain at least 12 characters",
		}
	}

	if length > 128 {
		return &ValidationError{
			Code:    "password_too_long",
			Field:   "password",
			Message: "password must contain at most 128 characters",
		}
	}

	return nil
}

func validateDeviceName(deviceName string) error {
	if deviceName == "" {
		return &ValidationError{
			Code:    "device_name_required",
			Field:   "device_name",
			Message: "device name is required",
		}
	}

	length := utf8.RuneCountInString(deviceName)

	if length < 3 {
		return &ValidationError{
			Code:    "device_name_too_short",
			Field:   "device_name",
			Message: "device name must contain at least 3 characters",
		}
	}

	if length > 64 {
		return &ValidationError{
			Code:    "device_name_too_long",
			Field:   "device_name",
			Message: "device name must contain at most 64 characters",
		}
	}

	return nil
}
