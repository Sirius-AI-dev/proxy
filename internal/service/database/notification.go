package database

type Notification struct {
	Channel string
	Payload string
}

// NotifyHandler describe function for handle pg_notify from Postgres
type NotifyHandler func(Notification)

// NotifyListener describe types of listener struct
type NotifyListener interface {
	Listen(NotifyHandler)
	Stop()
	Name() string
}
