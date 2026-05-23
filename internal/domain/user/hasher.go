package user

// PasswordHasher — порт хеширования и сверки паролей.
// Реализация (bcrypt/argon2) живёт в adapters.
type PasswordHasher interface {
	Hash(password string) ([]byte, error)
	Compare(hash []byte, password string) error
}
