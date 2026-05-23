package app

import "context"

// Repository — порт хранилища приложений.
// ByID возвращает ErrAppNotFound, если приложения нет.
type Repository interface {
	ByID(ctx context.Context, id int32) (App, error)
}
