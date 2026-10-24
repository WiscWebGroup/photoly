package config

import "net/url"

func urlUser(user, pass string) string { return url.UserPassword(user, pass).String() }
