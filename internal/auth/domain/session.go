package authdomain

import "time"

type Session struct {
	ID           int32     `json:"id"`
	User         User      `json:"user"`
	SessionToken string    `json:"session_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (s *Session) IsExpired() bool {
	return s.ExpiresAt.Before(time.Now())
}

func (s Session) WithExpiration(t time.Time) Session {
	s.ExpiresAt = t
	return s
}
