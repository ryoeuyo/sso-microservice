package auth_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ryoeuyo/sso-microservice/internal/app/auth"
	"github.com/ryoeuyo/sso-microservice/internal/domain/app"
	"github.com/ryoeuyo/sso-microservice/internal/domain/token"
	"github.com/ryoeuyo/sso-microservice/internal/domain/user"
	"github.com/ryoeuyo/sso-microservice/internal/mocks/appmock"
	"github.com/ryoeuyo/sso-microservice/internal/mocks/tokenmock"
	"github.com/ryoeuyo/sso-microservice/internal/mocks/usermock"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

type deps struct {
	users  *usermock.MockRepository
	apps   *appmock.MockRepository
	tokens *tokenmock.MockRepository
	roles  *usermock.MockRoleRepository
	hasher *usermock.MockPasswordHasher
	issuer *tokenmock.MockIssuer
}

func newSvc(t *testing.T) (*auth.Service, deps) {
	t.Helper()
	ctrl := gomock.NewController(t)
	d := deps{
		users:  usermock.NewMockRepository(ctrl),
		apps:   appmock.NewMockRepository(ctrl),
		tokens: tokenmock.NewMockRepository(ctrl),
		roles:  usermock.NewMockRoleRepository(ctrl),
		hasher: usermock.NewMockPasswordHasher(ctrl),
		issuer: tokenmock.NewMockIssuer(ctrl),
	}
	svc := auth.New(d.users, d.apps, d.tokens, d.roles, d.hasher, d.issuer, discardLogger())
	return svc, d
}

func fixtureUser(t *testing.T) user.User {
	t.Helper()
	email, err := user.NewEmail("alice@example.com")
	require.NoError(t, err)
	return user.User{
		ID:        uuid.New(),
		Email:     email,
		PassHash:  []byte("hash"),
		CreatedAt: time.Now().UTC(),
	}
}

func fixtureApp() app.App {
	return app.App{ID: 1, Name: "test", Secret: "s3cr3t"}
}

// ---------- Register ----------

func TestRegister_Success(t *testing.T) {
	svc, d := newSvc(t)

	d.hasher.EXPECT().Hash("pa$$word1").Return([]byte("hashed"), nil)
	d.users.EXPECT().
		Save(gomock.Any(), gomock.AssignableToTypeOf(user.User{})).
		DoAndReturn(func(_ context.Context, u user.User) error {
			require.Equal(t, user.Email("alice@example.com"), u.Email)
			require.Equal(t, []byte("hashed"), u.PassHash)
			require.NotEqual(t, uuid.Nil, u.ID)
			return nil
		})

	id, err := svc.Register(context.Background(), "  Alice@Example.com  ", "pa$$word1")
	require.NoError(t, err)
	_, err = uuid.Parse(id)
	require.NoError(t, err)
}

func TestRegister_InvalidEmail(t *testing.T) {
	svc, _ := newSvc(t)

	_, err := svc.Register(context.Background(), "not-an-email", "pa$$word1")
	require.ErrorIs(t, err, user.ErrInvalidEmail)
}

func TestRegister_AlreadyExists(t *testing.T) {
	svc, d := newSvc(t)

	d.hasher.EXPECT().Hash(gomock.Any()).Return([]byte("h"), nil)
	d.users.EXPECT().Save(gomock.Any(), gomock.Any()).Return(user.ErrUserAlreadyExists)

	_, err := svc.Register(context.Background(), "alice@example.com", "pa$$word1")
	require.ErrorIs(t, err, user.ErrUserAlreadyExists)
}

func TestRegister_HashError(t *testing.T) {
	svc, d := newSvc(t)
	boom := errors.New("hash boom")
	d.hasher.EXPECT().Hash(gomock.Any()).Return(nil, boom)

	_, err := svc.Register(context.Background(), "alice@example.com", "pa$$word1")
	require.ErrorIs(t, err, boom)
}

// ---------- Login ----------

func TestLogin_Success(t *testing.T) {
	svc, d := newSvc(t)
	u := fixtureUser(t)
	a := fixtureApp()

	expAccess := time.Now().Add(15 * time.Minute).UTC()
	expRefresh := time.Now().Add(720 * time.Hour).UTC()
	rt := token.RefreshToken{
		ID:        uuid.New(),
		UserID:    u.ID,
		AppID:     a.ID,
		TokenHash: []byte("rhash"),
		ExpiresAt: expRefresh,
		CreatedAt: time.Now().UTC(),
	}

	d.users.EXPECT().ByEmail(gomock.Any(), u.Email).Return(u, nil)
	d.hasher.EXPECT().Compare(u.PassHash, "pa$$word1").Return(nil)
	d.apps.EXPECT().ByID(gomock.Any(), a.ID).Return(a, nil)
	d.roles.EXPECT().Roles(gomock.Any(), u.ID, a.ID).Return([]user.Role{user.RoleUser}, nil)
	d.issuer.EXPECT().
		IssueAccess(u.ID, a.ID, []string{"user"}, a.Secret).
		Return("access-jwt", expAccess, nil)
	d.issuer.EXPECT().IssueRefresh(u.ID, a.ID).Return(rt, "plain-refresh", nil)
	d.tokens.EXPECT().Save(gomock.Any(), rt).Return(nil)

	pair, err := svc.Login(context.Background(), u.Email.String(), "pa$$word1", a.ID)
	require.NoError(t, err)
	require.Equal(t, "access-jwt", pair.AccessToken)
	require.Equal(t, "plain-refresh", pair.RefreshToken)
	require.Equal(t, expAccess, pair.AccessExpiresAt)
	require.Equal(t, expRefresh, pair.RefreshExpiresAt)
}

func TestLogin_UserNotFound_HidesAsInvalidCredentials(t *testing.T) {
	svc, d := newSvc(t)
	d.users.EXPECT().ByEmail(gomock.Any(), gomock.Any()).Return(user.User{}, user.ErrUserNotFound)

	_, err := svc.Login(context.Background(), "ghost@example.com", "pa$$word1", 1)
	require.ErrorIs(t, err, user.ErrInvalidCredentials)
}

func TestLogin_WrongPassword_HidesAsInvalidCredentials(t *testing.T) {
	svc, d := newSvc(t)
	u := fixtureUser(t)

	d.users.EXPECT().ByEmail(gomock.Any(), u.Email).Return(u, nil)
	d.hasher.EXPECT().Compare(u.PassHash, "wrong").Return(errors.New("mismatch"))

	_, err := svc.Login(context.Background(), u.Email.String(), "wrong", 1)
	require.ErrorIs(t, err, user.ErrInvalidCredentials)
}

func TestLogin_InvalidEmail_HidesAsInvalidCredentials(t *testing.T) {
	svc, _ := newSvc(t)

	_, err := svc.Login(context.Background(), "not-an-email", "pa$$word1", 1)
	require.ErrorIs(t, err, user.ErrInvalidCredentials)
}

// ---------- Refresh ----------

func activeRefresh(userID uuid.UUID, appID int32) token.RefreshToken {
	return token.RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		AppID:     appID,
		TokenHash: []byte("old-hash"),
		ExpiresAt: time.Now().Add(time.Hour).UTC(),
		CreatedAt: time.Now().Add(-time.Minute).UTC(),
	}
}

func TestRefresh_Success_RotatesToken(t *testing.T) {
	svc, d := newSvc(t)
	u := fixtureUser(t)
	a := fixtureApp()
	old := activeRefresh(u.ID, a.ID)

	newRT := token.RefreshToken{
		ID:        uuid.New(),
		UserID:    u.ID,
		AppID:     a.ID,
		TokenHash: []byte("new-hash"),
		ExpiresAt: time.Now().Add(720 * time.Hour).UTC(),
		CreatedAt: time.Now().UTC(),
	}
	expAccess := time.Now().Add(15 * time.Minute).UTC()

	gomock.InOrder(
		d.issuer.EXPECT().HashRefresh("plain-old").Return([]byte("old-hash")),
		d.tokens.EXPECT().ByHash(gomock.Any(), []byte("old-hash")).Return(old, nil),
		d.users.EXPECT().ByID(gomock.Any(), u.ID).Return(u, nil),
		d.apps.EXPECT().ByID(gomock.Any(), a.ID).Return(a, nil),
		d.roles.EXPECT().Roles(gomock.Any(), u.ID, a.ID).Return([]user.Role{user.RoleAdmin}, nil),
		d.tokens.EXPECT().Revoke(gomock.Any(), old.ID).Return(nil),
		d.issuer.EXPECT().
			IssueAccess(u.ID, a.ID, []string{"admin"}, a.Secret).
			Return("new-access", expAccess, nil),
		d.issuer.EXPECT().IssueRefresh(u.ID, a.ID).Return(newRT, "new-plain", nil),
		d.tokens.EXPECT().Save(gomock.Any(), newRT).Return(nil),
	)

	pair, err := svc.Refresh(context.Background(), "plain-old", a.ID)
	require.NoError(t, err)
	require.Equal(t, "new-access", pair.AccessToken)
	require.Equal(t, "new-plain", pair.RefreshToken)
}

func TestRefresh_TokenNotFound(t *testing.T) {
	svc, d := newSvc(t)

	d.issuer.EXPECT().HashRefresh("p").Return([]byte("h"))
	d.tokens.EXPECT().ByHash(gomock.Any(), []byte("h")).Return(token.RefreshToken{}, token.ErrTokenNotFound)

	_, err := svc.Refresh(context.Background(), "p", 1)
	require.ErrorIs(t, err, token.ErrTokenInvalid)
}

func TestRefresh_Revoked(t *testing.T) {
	svc, d := newSvc(t)
	revokedAt := time.Now().Add(-time.Minute).UTC()
	rt := activeRefresh(uuid.New(), 1)
	rt.RevokedAt = &revokedAt

	d.issuer.EXPECT().HashRefresh(gomock.Any()).Return([]byte("h"))
	d.tokens.EXPECT().ByHash(gomock.Any(), gomock.Any()).Return(rt, nil)

	_, err := svc.Refresh(context.Background(), "p", 1)
	require.ErrorIs(t, err, token.ErrTokenRevoked)
}

func TestRefresh_Expired(t *testing.T) {
	svc, d := newSvc(t)
	rt := activeRefresh(uuid.New(), 1)
	rt.ExpiresAt = time.Now().Add(-time.Minute).UTC()

	d.issuer.EXPECT().HashRefresh(gomock.Any()).Return([]byte("h"))
	d.tokens.EXPECT().ByHash(gomock.Any(), gomock.Any()).Return(rt, nil)

	_, err := svc.Refresh(context.Background(), "p", 1)
	require.ErrorIs(t, err, token.ErrTokenExpired)
}

func TestRefresh_AppMismatch(t *testing.T) {
	svc, d := newSvc(t)
	rt := activeRefresh(uuid.New(), 1)

	d.issuer.EXPECT().HashRefresh(gomock.Any()).Return([]byte("h"))
	d.tokens.EXPECT().ByHash(gomock.Any(), gomock.Any()).Return(rt, nil)

	_, err := svc.Refresh(context.Background(), "p", 2)
	require.ErrorIs(t, err, token.ErrTokenInvalid)
}

// ---------- Logout ----------

func TestLogout_Success(t *testing.T) {
	svc, d := newSvc(t)
	rt := activeRefresh(uuid.New(), 1)

	d.issuer.EXPECT().HashRefresh("p").Return([]byte("h"))
	d.tokens.EXPECT().ByHash(gomock.Any(), []byte("h")).Return(rt, nil)
	d.tokens.EXPECT().Revoke(gomock.Any(), rt.ID).Return(nil)

	require.NoError(t, svc.Logout(context.Background(), "p"))
}

func TestLogout_Idempotent_WhenTokenNotFound(t *testing.T) {
	svc, d := newSvc(t)

	d.issuer.EXPECT().HashRefresh(gomock.Any()).Return([]byte("h"))
	d.tokens.EXPECT().ByHash(gomock.Any(), gomock.Any()).Return(token.RefreshToken{}, token.ErrTokenNotFound)

	require.NoError(t, svc.Logout(context.Background(), "p"))
}

// ---------- Validate ----------

func TestValidate_Success(t *testing.T) {
	svc, d := newSvc(t)
	a := fixtureApp()
	userID := uuid.New()
	unverified := token.Claims{UserID: userID, AppID: a.ID}
	verified := token.Claims{
		UserID:    userID,
		AppID:     a.ID,
		Roles:     []string{"user"},
		ExpiresAt: time.Now().Add(15 * time.Minute).UTC(),
	}

	gomock.InOrder(
		d.issuer.EXPECT().ParseAccessUnverified("access").Return(unverified, nil),
		d.apps.EXPECT().ByID(gomock.Any(), a.ID).Return(a, nil),
		d.issuer.EXPECT().ParseAccess("access", a.Secret).Return(verified, nil),
	)

	got, err := svc.Validate(context.Background(), "access")
	require.NoError(t, err)
	require.Equal(t, verified, got)
}

func TestValidate_UnverifiedParseFails(t *testing.T) {
	svc, d := newSvc(t)

	d.issuer.EXPECT().ParseAccessUnverified("bad").Return(token.Claims{}, errors.New("malformed"))

	_, err := svc.Validate(context.Background(), "bad")
	require.ErrorIs(t, err, token.ErrTokenInvalid)
}

func TestValidate_AppNotFound(t *testing.T) {
	svc, d := newSvc(t)

	d.issuer.EXPECT().ParseAccessUnverified(gomock.Any()).Return(token.Claims{AppID: 42}, nil)
	d.apps.EXPECT().ByID(gomock.Any(), int32(42)).Return(app.App{}, app.ErrAppNotFound)

	_, err := svc.Validate(context.Background(), "access")
	require.ErrorIs(t, err, token.ErrTokenInvalid)
}

func TestValidate_VerifyFails(t *testing.T) {
	svc, d := newSvc(t)
	a := fixtureApp()

	d.issuer.EXPECT().ParseAccessUnverified(gomock.Any()).Return(token.Claims{AppID: a.ID}, nil)
	d.apps.EXPECT().ByID(gomock.Any(), a.ID).Return(a, nil)
	d.issuer.EXPECT().ParseAccess(gomock.Any(), a.Secret).Return(token.Claims{}, token.ErrTokenExpired)

	_, err := svc.Validate(context.Background(), "access")
	require.ErrorIs(t, err, token.ErrTokenExpired)
}
