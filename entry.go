package main

import (
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// ParseInput accepts an otpauth:// URI or a raw base32 secret.
func ParseInput(s string) (Entry, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "otpauth://") {
		key, err := otp.NewKeyFromURL(s)
		if err != nil {
			return Entry{}, err
		}
		e := Entry{
			Issuer:  key.Issuer(),
			Account: key.AccountName(),
			Secret:  key.Secret(),
			Digits:  6,
			Period:  int(key.Period()),
		}
		if e.Period == 0 {
			e.Period = 30
		}
		return e, nil
	}
	// raw secret: normalize (Google shows it with spaces, lowercase)
	sec := strings.ToUpper(strings.ReplaceAll(s, " ", ""))
	return Entry{Secret: sec, Digits: 6, Period: 30}, nil
}

func (e Entry) Code(t time.Time) string {
	code, err := totp.GenerateCodeCustom(e.Secret, t, totp.ValidateOpts{
		Period: uint(e.Period),
		Digits: otp.DigitsSix,
	})
	if err != nil {
		return "ERROR!"
	}
	return code
}

func (e Entry) SecondsLeft(t time.Time) int {
	return e.Period - int(t.Unix())%e.Period
}
