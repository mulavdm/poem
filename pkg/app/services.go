package app

import (
	"context"
	"errors"
	"time"
)

var ErrServiceUnavailable = errors.New("poem: platform service unavailable")

type PermissionStatus uint8

const (
	PermissionUnknown PermissionStatus = iota
	PermissionDenied
	PermissionPrompt
	PermissionGranted
)

type LocationSample struct {
	Latitude, Longitude, Altitude        float64
	HorizontalAccuracy, VerticalAccuracy float64
	Speed, Bearing                       float64
	At                                   time.Time
}

type LocationService interface {
	Permission(context.Context) (PermissionStatus, error)
	Watch(context.Context, func(LocationSample)) error
}

type SpeechRequest struct {
	Text, Locale string
	Interrupt    bool
}
type SpeechService interface {
	Speak(context.Context, SpeechRequest) error
	Stop(context.Context) error
}
type HapticService interface {
	Pulse(context.Context, string) error
}
type Notification struct {
	ID, Title, Body string
	Ongoing         bool
}
type NotificationService interface {
	Show(context.Context, Notification) error
	Dismiss(context.Context, string) error
}
type WakeService interface {
	Acquire(context.Context, string) (func(), error)
}
type PreferenceService interface {
	Get(context.Context, string) ([]byte, error)
	Put(context.Context, string, []byte) error
	Delete(context.Context, string) error
}
type SecureStore interface {
	GetSecret(context.Context, string) ([]byte, error)
	PutSecret(context.Context, string, []byte) error
	DeleteSecret(context.Context, string) error
}

// PlatformServices is injected by a host and is visible only to asynchronous
// command and subscription work.
type PlatformServices struct {
	Location      LocationService
	Speech        SpeechService
	Haptics       HapticService
	Notifications NotificationService
	Wake          WakeService
	Preferences   PreferenceService
	Secrets       SecureStore
}

type platformServicesKey struct{}

func WithPlatformServices(ctx context.Context, services PlatformServices) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, platformServicesKey{}, services)
}

func ServicesFromContext(ctx context.Context) PlatformServices {
	if ctx == nil {
		return PlatformServices{}
	}
	services, _ := ctx.Value(platformServicesKey{}).(PlatformServices)
	return services
}
