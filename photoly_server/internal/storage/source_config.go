package storage

import "encoding/json"

func SourceValues(s Source) (map[string]string, error) {
	values := map[string]string{}
	if err := json.Unmarshal(s.Config, &values); err != nil {
		return nil, err
	}
	if len(s.Secret) > 0 {
		b, err := open(s.Secret)
		if err != nil {
			return nil, err
		}
		sec := map[string]string{}
		if err = json.Unmarshal(b, &sec); err != nil {
			return nil, err
		}
		for k, v := range sec {
			values[k] = v
		}
	}
	return values, nil
}
func SourceStore(s Source) (Store, error) {
	v, e := SourceValues(s)
	if e != nil {
		return nil, e
	}
	return Build(s.Kind, v)
}
func SealValues(v map[string]string) ([]byte, []byte, error) {
	plain, secret := map[string]string{}, map[string]string{}
	for k, val := range v {
		if k == "secret_key" || k == "access_key" || k == "username" || k == "password" || k == "domain" {
			secret[k] = val
		} else {
			plain[k] = val
		}
	}
	p, err := json.Marshal(plain)
	if err != nil {
		return nil, nil, err
	}
	if len(secret) == 0 {
		return p, nil, nil
	}
	b, err := json.Marshal(secret)
	if err != nil {
		return nil, nil, err
	}
	sealed, err := seal(b)
	return p, sealed, err
}

func (s Source) Tree() bool { return s.Kind == "webdav" || s.Kind == "smb" }
func NewTree(kind string, v map[string]string) (TreeStore, error) {
	if kind == "smb" {
		return NewSMB(v)
	}
	return NewDAV(v)
}
