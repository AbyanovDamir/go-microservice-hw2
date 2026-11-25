package models

import (
	"errors"
	"net/mail"
	"strings"
	"time"
)

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (u *User) Validate() error {
	u.Name = strings.TrimSpace(u.Name)
	u.Email = strings.TrimSpace(u.Email)
	if u.Name == "" || len(u.Name) > 200 {
		return errors.New("name must contain 1..200 bytes")
	}
	addr, err := mail.ParseAddress(u.Email)
	if err != nil || addr.Address != u.Email || len(u.Email) > 254 {
		return errors.New("invalid email address")
	}
	return nil
}

type Event struct {
	Sequence uint64    `json:"sequence"`
	Action   string    `json:"action"`
	User     User      `json:"user"`
	Time     time.Time `json:"time"`
}
