package auth

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/aiulian25/keeps/internal/config"
	"github.com/aiulian25/keeps/internal/httpapi"
	"github.com/aiulian25/keeps/internal/money"
)

const passkeyBodyLimit = 64 << 10

// errorMessages maps domain errors to stable API codes and plain messages.
var errorMessages = map[error]*httpapi.Error{
	ErrInvalidCredentials:    httpapi.NewError(http.StatusUnauthorized, "invalid_credentials", "That username and password don't match."),
	ErrInvalidCode:           httpapi.FieldError("code", "invalid_code", "That code didn't work. Check your authenticator and try again."),
	ErrAlreadyClaimed:        httpapi.NewError(http.StatusGone, "setup_code_used", "This Keeps has already been set up. Sign in instead."),
	ErrSetupCodeInvalid:      httpapi.FieldError("setupCode", "setup_code_invalid", "That setup code isn't right. Copy it again from the server log."),
	ErrSetupCodeExpired:      httpapi.FieldError("setupCode", "setup_code_expired", "This setup code has expired. Run keeps bootstrap-code on the server for a new one."),
	ErrPasswordTooShort:      httpapi.FieldError("password", "password_too_short", fmt.Sprintf("Use at least %d characters. A few words together work well.", config.PasswordMinimumLength)),
	ErrPasswordTooLong:       httpapi.FieldError("password", "password_too_long", "That password is too long."),
	ErrInvalidDisplayName:    httpapi.FieldError("name", "invalid_name", "Enter a name up to 80 characters."),
	ErrInvalidUsername:       httpapi.FieldError("username", "invalid_username", "Use a username without spaces, up to 60 characters."),
	ErrInvalidTheme:          httpapi.FieldError("theme", "invalid_theme", "Choose Dark or Light."),
	ErrTOTPAlreadyEnabled:    httpapi.NewError(http.StatusConflict, "totp_already_enabled", "An authenticator is already set up."),
	ErrTOTPNotStarted:        httpapi.NewError(http.StatusConflict, "totp_not_started", "Start authenticator setup again."),
	ErrTOTPNotEnabled:        httpapi.NewError(http.StatusConflict, "totp_not_enabled", "Set up an authenticator first."),
	ErrPasskeysUnavailable:   httpapi.NewError(http.StatusConflict, "passkeys_unavailable", "Passkeys need Keeps to be opened over HTTPS at its configured address."),
	ErrPasskeyChallenge:      httpapi.NewError(http.StatusBadRequest, "passkey_challenge_expired", "That passkey request expired. Try again."),
	ErrPasskeyRejected:       httpapi.NewError(http.StatusUnauthorized, "passkey_rejected", "That passkey wasn't accepted."),
	money.ErrInvalidCurrency: httpapi.FieldError("currency", "invalid_currency", "Use a three-letter currency code such as GBP."),
}

func translate(err error) error {
	for domainError, apiError := range errorMessages {
		if errors.Is(err, domainError) {
			return apiError
		}
	}
	var limited *ErrRateLimited
	if errors.As(err, &limited) {
		seconds := int(math.Ceil(limited.RetryAfter.Seconds()))
		return &httpapi.Error{Status: http.StatusTooManyRequests, Code: "rate_limited",
			Message: fmt.Sprintf("Too many attempts. Try again in %s.", humaniseWait(seconds))}
	}
	return err
}

func humaniseWait(seconds int) string {
	if seconds < int(time.Minute.Seconds()) {
		return strconv.Itoa(max(seconds, 1)) + " seconds"
	}
	return strconv.Itoa(seconds/int(time.Minute.Seconds())+1) + " minutes"
}

// Routes registers the authentication and account endpoints.
func (s *Service) Routes(mux *http.ServeMux, clientAddress func(*http.Request) string) {
	handle := func(pattern string, public bool, handler httpapi.Handler) {
		wrapped := func(writer http.ResponseWriter, request *http.Request) error {
			return translate(handler(writer, request))
		}
		if public {
			mux.Handle(pattern, s.Public(httpapi.Handler(wrapped)))
			return
		}
		mux.Handle(pattern, s.Protect(httpapi.Handler(wrapped)))
	}
	reauthenticated := RequireRecentReauthentication

	handle("GET /api/v1/bootstrap", true, s.handleBootstrapState)
	handle("POST /api/v1/bootstrap", true, func(writer http.ResponseWriter, request *http.Request) error {
		return s.handleClaim(writer, request, clientAddress(request))
	})
	handle("POST /api/v1/sessions", true, func(writer http.ResponseWriter, request *http.Request) error {
		return s.handleSignIn(writer, request, clientAddress(request))
	})
	handle("POST /api/v1/sessions/second-factor", true, s.handleSecondFactor)
	handle("POST /api/v1/recover", true, func(writer http.ResponseWriter, request *http.Request) error {
		return s.handleRecover(writer, request, clientAddress(request))
	})
	handle("POST /api/v1/sessions/passkey/begin", true, func(writer http.ResponseWriter, request *http.Request) error {
		return s.handlePasskeyLoginBegin(writer, request, clientAddress(request))
	})
	handle("POST /api/v1/sessions/passkey/finish", true, func(writer http.ResponseWriter, request *http.Request) error {
		return s.handlePasskeyLoginFinish(writer, request, clientAddress(request))
	})

	handle("GET /api/v1/session", false, s.handleCurrentSession)
	handle("DELETE /api/v1/session", false, s.handleSignOut)
	handle("GET /api/v1/sessions", false, s.handleListSessions)
	handle("DELETE /api/v1/sessions/{id}", false, s.handleRevokeSession)
	handle("POST /api/v1/sessions/revoke-others", false, s.handleRevokeOthers)

	handle("GET /api/v1/account", false, s.handleAccount)
	handle("PATCH /api/v1/account", false, s.handleUpdateAccount)
	handle("POST /api/v1/account/reauthenticate", false, s.handleReauthenticate)
	handle("POST /api/v1/account/password", false, s.handleChangePassword)
	handle("POST /api/v1/account/totp", false, reauthenticated(s.handleTOTPBegin))
	handle("POST /api/v1/account/totp/confirm", false, reauthenticated(s.handleTOTPConfirm))
	handle("DELETE /api/v1/account/totp", false, reauthenticated(s.handleTOTPDisable))
	handle("POST /api/v1/account/recovery-codes", false, reauthenticated(s.handleRecoveryCodes))
	handle("POST /api/v1/account/passkeys/begin", false, reauthenticated(s.handlePasskeyRegisterBegin))
	handle("POST /api/v1/account/passkeys/finish", false, reauthenticated(s.handlePasskeyRegisterFinish))
	handle("PATCH /api/v1/account/passkeys/{id}", false, s.handlePasskeyRename)
	handle("DELETE /api/v1/account/passkeys/{id}", false, reauthenticated(s.handlePasskeyRemove))
}

func (s *Service) handleBootstrapState(writer http.ResponseWriter, request *http.Request) error {
	claimed, err := s.IsClaimed(request.Context())
	if err != nil {
		return err
	}
	httpapi.WriteJSON(writer, http.StatusOK, map[string]bool{"claimed": claimed, "passkeysAvailable": s.PasskeysAvailable()})
	return nil
}

func (s *Service) handleClaim(writer http.ResponseWriter, request *http.Request, clientAddress string) error {
	var body struct {
		SetupCode     string `json:"setupCode"`
		HouseholdName string `json:"householdName"`
		Username      string `json:"username"`
		Password      string `json:"password"`
	}
	if err := httpapi.DecodeJSON(writer, request, config.MaxJSONBodyBytes, &body); err != nil {
		return err
	}
	householdName, err := cleanName(body.HouseholdName, householdNameMaximumLength)
	if err != nil {
		return httpapi.FieldError("householdName", "invalid_name", "Enter a household name.")
	}
	username, err := ValidateUsername(body.Username)
	if err != nil {
		return err
	}
	if err := ValidateNewPassword(body.Password); err != nil {
		return err
	}
	token, expires, err := s.Claim(request.Context(), ClaimRequest{
		SetupCode: body.SetupCode, HouseholdName: householdName, Username: username, Password: body.Password,
		UserAgent: request.UserAgent(), ClientAddress: clientAddress,
	})
	if err != nil {
		return err
	}
	s.setSessionCookie(writer, token, expires)
	httpapi.WriteJSON(writer, http.StatusCreated, map[string]bool{"signedIn": true})
	return nil
}

func (s *Service) handleSignIn(writer http.ResponseWriter, request *http.Request, clientAddress string) error {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := httpapi.DecodeJSON(writer, request, config.MaxJSONBodyBytes, &body); err != nil {
		return err
	}
	result, err := s.SignInWithPassword(request.Context(), body.Username, body.Password, request.UserAgent(), clientAddress)
	if err != nil {
		return err
	}
	s.setSessionCookie(writer, result.Token, result.Expires)
	httpapi.WriteJSON(writer, http.StatusOK, map[string]bool{"awaitingSecondFactor": result.AwaitingSecondFactor})
	return nil
}

func (s *Service) handleSecondFactor(writer http.ResponseWriter, request *http.Request) error {
	var body struct {
		Code         string `json:"code"`
		RecoveryCode string `json:"recoveryCode"`
	}
	if err := httpapi.DecodeJSON(writer, request, config.MaxJSONBodyBytes, &body); err != nil {
		return err
	}
	pending, err := s.sessionFromRequest(request)
	if err != nil {
		return httpapi.NewError(http.StatusUnauthorized, "second_factor_expired", "Sign in again.")
	}
	result, err := s.CompleteSecondFactor(request.Context(), pending, body.Code, body.RecoveryCode, request.UserAgent())
	if err != nil {
		return err
	}
	s.setSessionCookie(writer, result.Token, result.Expires)
	httpapi.WriteJSON(writer, http.StatusOK, map[string]bool{"signedIn": true})
	return nil
}

func (s *Service) handlePasskeyLoginBegin(writer http.ResponseWriter, request *http.Request, clientAddress string) error {
	start, err := s.BeginPasskeyLogin(request.Context(), clientAddress)
	if err != nil {
		return err
	}
	httpapi.WriteJSON(writer, http.StatusOK, start)
	return nil
}

func (s *Service) handlePasskeyLoginFinish(writer http.ResponseWriter, request *http.Request, clientAddress string) error {
	var body passkeyCredentialBody
	if err := httpapi.DecodeJSON(writer, request, passkeyBodyLimit, &body); err != nil {
		return err
	}
	result, err := s.FinishPasskeyLogin(request.Context(), body.ChallengeID, body.credentialBytes(), request.UserAgent(), clientAddress)
	if err != nil {
		return err
	}
	s.setSessionCookie(writer, result.Token, result.Expires)
	httpapi.WriteJSON(writer, http.StatusOK, map[string]bool{"signedIn": true})
	return nil
}

func (s *Service) handleCurrentSession(writer http.ResponseWriter, request *http.Request) error {
	account, err := s.LoadAccount(request.Context(), PrincipalFromRequest(request))
	if err != nil {
		return err
	}
	httpapi.WriteJSON(writer, http.StatusOK, account)
	return nil
}

func (s *Service) handleAccount(writer http.ResponseWriter, request *http.Request) error {
	return s.handleCurrentSession(writer, request)
}

func (s *Service) handleSignOut(writer http.ResponseWriter, request *http.Request) error {
	if err := s.SignOut(request.Context(), PrincipalFromRequest(request)); err != nil {
		return err
	}
	s.clearSessionCookie(writer)
	writer.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Service) handleListSessions(writer http.ResponseWriter, request *http.Request) error {
	sessions, err := s.ListSessions(request.Context(), PrincipalFromRequest(request))
	if err != nil {
		return err
	}
	httpapi.WriteJSON(writer, http.StatusOK, sessions)
	return nil
}

func pathIdentifier(request *http.Request) (int64, error) {
	identifier, err := strconv.ParseInt(request.PathValue("id"), 10, 64)
	if err != nil {
		return 0, httpapi.ErrNotFound
	}
	return identifier, nil
}

func (s *Service) handleRevokeSession(writer http.ResponseWriter, request *http.Request) error {
	identifier, err := pathIdentifier(request)
	if err != nil {
		return err
	}
	if err := s.RevokeSession(request.Context(), PrincipalFromRequest(request), identifier); err != nil {
		return err
	}
	writer.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Service) handleRevokeOthers(writer http.ResponseWriter, request *http.Request) error {
	if err := s.RevokeOtherSessions(request.Context(), PrincipalFromRequest(request)); err != nil {
		return err
	}
	writer.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Service) handleUpdateAccount(writer http.ResponseWriter, request *http.Request) error {
	var changes AccountChanges
	if err := httpapi.DecodeJSON(writer, request, config.MaxJSONBodyBytes, &changes); err != nil {
		return err
	}
	if err := s.UpdateAccount(request.Context(), PrincipalFromRequest(request), changes); err != nil {
		return err
	}
	return s.handleCurrentSession(writer, request)
}

type identityProof struct {
	Password     string `json:"password"`
	Code         string `json:"code"`
	RecoveryCode string `json:"recoveryCode"`
}

func (s *Service) handleReauthenticate(writer http.ResponseWriter, request *http.Request) error {
	var body identityProof
	if err := httpapi.DecodeJSON(writer, request, config.MaxJSONBodyBytes, &body); err != nil {
		return err
	}
	if err := s.Reauthenticate(request.Context(), PrincipalFromRequest(request), body.Password, body.Code, body.RecoveryCode); err != nil {
		return err
	}
	writer.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Service) handleChangePassword(writer http.ResponseWriter, request *http.Request) error {
	var body struct {
		identityProof
		NewPassword string `json:"newPassword"`
	}
	if err := httpapi.DecodeJSON(writer, request, config.MaxJSONBodyBytes, &body); err != nil {
		return err
	}
	err := s.ChangePassword(request.Context(), writer, PrincipalFromRequest(request), body.Password, body.NewPassword, body.Code, body.RecoveryCode, request.UserAgent())
	if errors.Is(err, ErrPasswordTooShort) {
		return httpapi.FieldError("newPassword", "password_too_short", errorMessages[ErrPasswordTooShort].Message)
	}
	if errors.Is(err, ErrInvalidCredentials) {
		return httpapi.FieldError("password", "invalid_credentials", "Your current password isn't right.")
	}
	if err != nil {
		return err
	}
	writer.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Service) handleTOTPBegin(writer http.ResponseWriter, request *http.Request) error {
	setup, err := s.BeginTOTP(request.Context(), PrincipalFromRequest(request))
	if err != nil {
		return err
	}
	httpapi.WriteJSON(writer, http.StatusOK, setup)
	return nil
}

func (s *Service) handleTOTPConfirm(writer http.ResponseWriter, request *http.Request) error {
	var body struct {
		Code string `json:"code"`
	}
	if err := httpapi.DecodeJSON(writer, request, config.MaxJSONBodyBytes, &body); err != nil {
		return err
	}
	codes, err := s.ConfirmTOTP(request.Context(), PrincipalFromRequest(request), body.Code)
	if err != nil {
		return err
	}
	httpapi.WriteJSON(writer, http.StatusOK, map[string][]string{"recoveryCodes": codes})
	return nil
}

func (s *Service) handleTOTPDisable(writer http.ResponseWriter, request *http.Request) error {
	if err := s.DisableTOTP(request.Context(), PrincipalFromRequest(request)); err != nil {
		return err
	}
	writer.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Service) handleRecoveryCodes(writer http.ResponseWriter, request *http.Request) error {
	codes, err := s.RegenerateRecoveryCodes(request.Context(), PrincipalFromRequest(request))
	if err != nil {
		return err
	}
	httpapi.WriteJSON(writer, http.StatusOK, map[string][]string{"recoveryCodes": codes})
	return nil
}

func (s *Service) handlePasskeyRegisterBegin(writer http.ResponseWriter, request *http.Request) error {
	start, err := s.BeginPasskeyRegistration(request.Context(), PrincipalFromRequest(request))
	if err != nil {
		return err
	}
	httpapi.WriteJSON(writer, http.StatusOK, start)
	return nil
}

func (s *Service) handlePasskeyRegisterFinish(writer http.ResponseWriter, request *http.Request) error {
	var body passkeyCredentialBody
	if err := httpapi.DecodeJSON(writer, request, passkeyBodyLimit, &body); err != nil {
		return err
	}
	if err := s.FinishPasskeyRegistration(request.Context(), PrincipalFromRequest(request), body.ChallengeID, body.Name, body.credentialBytes()); err != nil {
		return err
	}
	return s.handleCurrentSession(writer, request)
}

func (s *Service) handlePasskeyRename(writer http.ResponseWriter, request *http.Request) error {
	identifier, err := pathIdentifier(request)
	if err != nil {
		return err
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := httpapi.DecodeJSON(writer, request, config.MaxJSONBodyBytes, &body); err != nil {
		return err
	}
	if err := s.RenamePasskey(request.Context(), PrincipalFromRequest(request), identifier, body.Name); err != nil {
		return err
	}
	return s.handleCurrentSession(writer, request)
}

func (s *Service) handlePasskeyRemove(writer http.ResponseWriter, request *http.Request) error {
	identifier, err := pathIdentifier(request)
	if err != nil {
		return err
	}
	if err := s.RemovePasskey(request.Context(), PrincipalFromRequest(request), identifier); err != nil {
		return httpapi.ErrNotFound
	}
	return s.handleCurrentSession(writer, request)
}
