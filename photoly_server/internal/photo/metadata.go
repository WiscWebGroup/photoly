package photo

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func num(m Meta, key string) (float64, bool) {
	switch x := m[key].(type) {
	case int64:
		return float64(x), true
	case float64:
		return x, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	}
	return 0, false
}

func has(m Meta, key string) bool { _, ok := m[key]; return ok }

func firstDate(m Meta) (string, bool) {
	for _, k := range []string{"DateTimeOriginal", "CreationDate", "CreateDate"} {
		if v, ok := m[k]; ok && pyStr(v) != "" && v != int64(0) {
			return pyStr(v), true
		}
	}
	return "", false
}

// PublicEXIF is MetadataReader.public (the /photo/getPhotoExif body).
func PublicEXIF(m Meta, path string) map[string]any {
	out := map[string]any{}
	for src, dst := range map[string]string{"Make": "Make", "Model": "Model", "LensMake": "Lens_Make", "LensModel": "Lens_Model",
		"Artist": "Artist", "ColorSpace": "Color_Space", "GPSAltitude": "GPS_Altitude", "GPSAltitudeRef": "GPS_Altitude_Ref",
		"GPSVersionID": "GPS_Version_ID", "GPSLongitudeRef": "GPS_Longitude_Ref", "GPSLatitudeRef": "GPS_Latitude_Ref",
		"XResolution": "X_Resolution", "YResolution": "Y_Resolution", "BitsPerSample": "Data_Precision"} {
		if v, ok := m[src]; ok {
			out[dst] = pyStr(v)
		}
	}
	for src, dst := range map[string]string{"GPSLatitude": "GPS_Latitude", "GPSLongitude": "GPS_Longitude"} {
		if f, ok := num(m, src); ok {
			out[dst] = fmt.Sprintf("%.4f", f)
		}
	}
	for src, c := range map[string][2]string{"ImageWidth": {"Image_Width", "pixels"}, "ImageHeight": {"Image_Height", "pixels"}, "FocalLength": {"Focal_Length", "mm"}} {
		if v, ok := m[src]; ok {
			out[c[0]] = pyStr(v) + " " + c[1]
		}
	}
	aperture, ok := m["ApertureValue"]
	if !ok {
		aperture, ok = m["FNumber"]
	}
	if ok {
		f, _ := num(Meta{"x": aperture}, "x")
		out["Aperture_Value"] = "f/" + pyG(f)
	}
	shutter, ok := m["ExposureTime"]
	if !ok {
		shutter, ok = m["ShutterSpeedValue"]
	}
	if ok {
		if f, _ := num(Meta{"x": shutter}, "x"); f != 0 {
			out["Shutter_Speed_Value"] = limitDenominator(f, 100000) + " sec"
		}
	}
	if v, ok := m["ISO"]; ok {
		out["ISO_Speed_Ratings"] = pyStr(v)
	}
	if t, ok := firstDate(m); ok {
		out["Time"] = t
	}
	if fi, err := os.Stat(path); err == nil {
		out["File_Size"] = fmt.Sprintf("%.2f MB", float64(fi.Size())/1024/1024)
	}
	return out
}

// Extra is the photo_extra row derived from metadata (MetadataReader.extra).
type Extra struct {
	Lat, Lon, Aperture *float64
	Camera, Lens       *string
	Focal, ISO, Shut   *int64
	Taken              *time.Time
}

func nf(m Meta, key string) *float64 {
	f, ok := num(m, key)
	if !ok {
		return nil
	}
	return &f
}

func ni(m Meta, key string) *int64 {
	f, ok := num(m, key)
	if !ok || f != f || f > 9.2e18 || f < -9.2e18 {
		return nil
	}
	n := int64(f) // int(float(x)): truncation toward zero
	return &n
}

func join(m Meta, keys ...string) *string {
	var parts []string
	for _, k := range keys {
		if v, ok := m[k]; ok && v != "" && v != int64(0) {
			parts = append(parts, pyStr(v))
		}
	}
	if len(parts) == 0 {
		return nil
	}
	s := strings.Join(parts, " ")
	return &s
}

// ExtraFrom is MetadataReader.extra.
func ExtraFrom(m Meta) Extra {
	var e Extra
	if stamp, ok := firstDate(m); ok {
		s := stamp
		if len(s) > 19 {
			s = s[:19]
		}
		if t, err := time.Parse("2006:01:02 15:04:05", s); err == nil && len(s) == 19 {
			e.Taken = &t
		}
	}
	e.Camera, e.Lens = join(m, "Make", "Model"), join(m, "LensMake", "LensModel")
	e.Lat, e.Lon = nf(m, "GPSLatitude"), nf(m, "GPSLongitude")
	// Python `a or b`: a zero first value falls through to the second.
	e.Aperture = nf(m, "ApertureValue")
	if e.Aperture == nil || *e.Aperture == 0 {
		e.Aperture = nf(m, "FNumber")
	}
	e.Focal, e.ISO = ni(m, "FocalLength"), ni(m, "ISO")
	st := nf(m, "ExposureTime")
	if st == nil || *st == 0 {
		if f := nf(m, "ShutterSpeedValue"); f != nil {
			st = f
		}
	}
	if st != nil && *st > 0 {
		r := roundHalfEven(1 / *st)
		e.Shut = &r
	}
	return e
}

func roundHalfEven(f float64) int64 {
	n := int64(f)
	d := f - float64(n)
	switch {
	case d > 0.5 || (d == 0.5 && n%2 != 0):
		return n + 1
	}
	return n
}
