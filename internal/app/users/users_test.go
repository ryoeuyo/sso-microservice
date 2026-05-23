package users_test

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

	"github.com/ryoeuyo/sso-microservice/internal/app/users"
	"github.com/ryoeuyo/sso-microservice/internal/domain/user"
	"github.com/ryoeuyo/sso-microservice/internal/mocks/usermock"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestGetUser(t *testing.T) {
	id := uuid.New()
	email, _ := user.NewEmail("a@b.c")
	want := user.User{ID: id, Email: email, CreatedAt: time.Now().UTC()}

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := usermock.NewMockRepository(ctrl)
		repo.EXPECT().ByID(gomock.Any(), id).Return(want, nil)

		svc := users.New(repo, discardLogger())
		got, err := svc.GetUser(context.Background(), id)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})

	t.Run("not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := usermock.NewMockRepository(ctrl)
		repo.EXPECT().ByID(gomock.Any(), id).Return(user.User{}, user.ErrUserNotFound)

		svc := users.New(repo, discardLogger())
		_, err := svc.GetUser(context.Background(), id)
		require.ErrorIs(t, err, user.ErrUserNotFound)
	})
}

func TestDeleteUser(t *testing.T) {
	id := uuid.New()

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := usermock.NewMockRepository(ctrl)
		repo.EXPECT().SoftDelete(gomock.Any(), id).Return(nil)

		svc := users.New(repo, discardLogger())
		require.NoError(t, svc.DeleteUser(context.Background(), id))
	})

	t.Run("repo error is wrapped", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := usermock.NewMockRepository(ctrl)
		boom := errors.New("boom")
		repo.EXPECT().SoftDelete(gomock.Any(), id).Return(boom)

		svc := users.New(repo, discardLogger())
		err := svc.DeleteUser(context.Background(), id)
		require.ErrorIs(t, err, boom)
	})
}
