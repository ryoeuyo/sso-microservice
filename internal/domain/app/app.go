package app

// App — клиентское приложение SSO. Каждое имеет свой secret,
// которым подписываются access-токены для его пользователей.
type App struct {
	ID     int32
	Name   string
	Secret string
}
