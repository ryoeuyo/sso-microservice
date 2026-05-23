package bcrypt_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	bcryptadapter "github.com/ryoeuyo/sso-microservice/internal/adapters/crypto/bcrypt"
	"github.com/ryoeuyo/sso-microservice/internal/domain/user"
)

func TestHasher_HashAndCompare(t *testing.T) {
	// Берём минимальную стоимость (4) — тесты иначе еле ползают.
	h := bcryptadapter.New(4)

	hash, err := h.Hash("pa$$word1")
	require.NoError(t, err)
	require.NotEmpty(t, hash)

	require.NoError(t, h.Compare(hash, "pa$$word1"))
}

func TestHasher_Compare_Mismatch(t *testing.T) {
	h := bcryptadapter.New(4)

	hash, err := h.Hash("pa$$word1")
	require.NoError(t, err)

	err = h.Compare(hash, "wrong")
	require.ErrorIs(t, err, user.ErrInvalidCredentials)
}

func TestHasher_DefaultCostOnInvalid(t *testing.T) {
	h := bcryptadapter.New(999)
	hash, err := h.Hash("pa$$word1")
	require.NoError(t, err)
	require.NoError(t, h.Compare(hash, "pa$$word1"))
}
