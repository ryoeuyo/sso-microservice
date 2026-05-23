package permissions_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/ryoeuyo/sso-microservice/internal/app/permissions"
	"github.com/ryoeuyo/sso-microservice/internal/mocks/usermock"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestIsAdmin(t *testing.T) {
	userID := uuid.New()
	const appID int32 = 1

	tests := []struct {
		name    string
		mock    func(m *usermock.MockRoleRepository)
		want    bool
		wantErr bool
	}{
		{
			name: "true",
			mock: func(m *usermock.MockRoleRepository) {
				m.EXPECT().IsAdmin(gomock.Any(), userID, appID).Return(true, nil)
			},
			want: true,
		},
		{
			name: "false",
			mock: func(m *usermock.MockRoleRepository) {
				m.EXPECT().IsAdmin(gomock.Any(), userID, appID).Return(false, nil)
			},
			want: false,
		},
		{
			name: "repo error",
			mock: func(m *usermock.MockRoleRepository) {
				m.EXPECT().IsAdmin(gomock.Any(), userID, appID).Return(false, errors.New("boom"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			roles := usermock.NewMockRoleRepository(ctrl)
			tc.mock(roles)

			svc := permissions.New(roles, discardLogger())
			got, err := svc.IsAdmin(context.Background(), userID, appID)

			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
