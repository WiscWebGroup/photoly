package photo

import (
	"bytes"
	"encoding/xml"
	"strings"
)

// Only classification fields are retained; XML entities are never fetched externally.
func readCategoryXMP(data []byte, m Meta) {
	d := xml.NewDecoder(bytes.NewReader(data))
	var active string
	allowed := func(name xml.Name) bool {
		return (name.Space == "http://ns.google.com/photos/1.0/panorama/" && (name.Local == "ProjectionType" || name.Local == "UsePanoramaViewer")) || (name.Space == "http://ns.google.com/photos/1.0/camera/" && name.Local == "SpecialTypeID")
	}
	for {
		token, e := d.Token()
		if e != nil {
			return
		}
		switch t := token.(type) {
		case xml.StartElement:
			for _, a := range t.Attr {
				if allowed(a.Name) {
					m[a.Name.Local] = a.Value
				}
			}
			if allowed(t.Name) {
				active = t.Name.Local
			} else {
				active = ""
			}
		case xml.CharData:
			if active != "" {
				m[active] = strings.TrimSpace(string(t))
			}
		case xml.EndElement:
			active = ""
		}
	}
}
