package grpcadapter_test

import (
	"testing"

	"buf.build/go/protovalidate"
	"github.com/stretchr/testify/require"

	ssov1 "github.com/ryoeuyo/sso-microservice/gen/sso/v1"
)

// TestProtovalidate_ReadsLocalValidateAnnotations проверяет,
// что protovalidate-go видит наши buf.validate.* правила из локально сгенерённых
// .pb.go (gen/sso/v1) — критично, потому что мы вендорим validate.proto в third_party,
// а runtime-либа подтягивает свою версию через BSR-модуль. Если дескрипторы не совпадут,
// валидация молча пропустит мусорный input.
func TestProtovalidate_ReadsLocalValidateAnnotations(t *testing.T) {
	v, err := protovalidate.New()
	require.NoError(t, err)

	t.Run("rejects invalid email", func(t *testing.T) {
		err := v.Validate(&ssov1.RegisterRequest{
			Email:    "not-an-email",
			Password: "longenough",
		})
		require.Error(t, err)
	})

	t.Run("rejects short password", func(t *testing.T) {
		err := v.Validate(&ssov1.RegisterRequest{
			Email:    "alice@example.com",
			Password: "short",
		})
		require.Error(t, err)
	})

	t.Run("rejects app_id = 0", func(t *testing.T) {
		err := v.Validate(&ssov1.LoginRequest{
			Email:    "alice@example.com",
			Password: "longenough",
			AppId:    0,
		})
		require.Error(t, err)
	})

	t.Run("rejects bad uuid", func(t *testing.T) {
		err := v.Validate(&ssov1.IsAdminRequest{UserId: "not-uuid", AppId: 1})
		require.Error(t, err)
	})

	t.Run("accepts valid input", func(t *testing.T) {
		err := v.Validate(&ssov1.RegisterRequest{
			Email:    "alice@example.com",
			Password: "longenough",
		})
		require.NoError(t, err)
	})
}
