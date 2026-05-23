package token

import (
	"time"

	"github.com/google/uuid"
)

// Issuer — порт выпуска и парсинга токенов.
// Реализация на JWT живёт в adapters/jwt.
//
// IssueRefresh возвращает RefreshToken (с уже посчитанным TokenHash)
// и plain-строку, которую отдаём клиенту один раз.
type Issuer interface {
	IssueAccess(userID uuid.UUID, appID int32, roles []string, secret string) (signed string, expiresAt time.Time, err error)

	// ParseAccessUnverified возвращает claims без проверки подписи.
	// Нужно, чтобы по app_id из claims загрузить нужный secret и затем верифицировать.
	ParseAccessUnverified(signed string) (Claims, error)

	// ParseAccess проверяет подпись секретом и срок жизни.
	ParseAccess(signed, secret string) (Claims, error)

	IssueRefresh(userID uuid.UUID, appID int32) (rt RefreshToken, plain string, err error)
	HashRefresh(plain string) []byte
}
