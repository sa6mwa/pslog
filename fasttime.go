package pslog

import (
	"time"
)

func formatterForLayout(layout string) func(time.Time) string {
	switch layout {
	case time.RFC3339:
		return formatRFC3339
	case time.RFC3339Nano:
		return formatRFC3339Nano
	default:
		return nil
	}
}

func formatRFC3339(t time.Time) string {
	const layoutFallback = time.RFC3339
	year, month, day := t.Date()
	if year < 0 || year > 9999 {
		return t.Format(layoutFallback)
	}
	hour, min, sec := t.Clock()
	_, offset := t.Zone()
	if offset < -(18*3600) || offset > 18*3600 {
		return t.Format(layoutFallback)
	}
	buf := make([]byte, 0, 32)
	buf = appendFourDigits(buf, year)
	buf = append(buf, '-')
	buf = appendTwoDigits(buf, int(month))
	buf = append(buf, '-')
	buf = appendTwoDigits(buf, day)
	buf = append(buf, 'T')
	buf = appendTwoDigits(buf, hour)
	buf = append(buf, ':')
	buf = appendTwoDigits(buf, min)
	buf = append(buf, ':')
	buf = appendTwoDigits(buf, sec)
	if offset == 0 {
		buf = append(buf, 'Z')
		return string(buf)
	}
	if offset < 0 {
		buf = append(buf, '-')
		offset = -offset
	} else {
		buf = append(buf, '+')
	}
	oh := offset / 3600
	om := (offset % 3600) / 60
	buf = appendTwoDigits(buf, oh)
	buf = append(buf, ':')
	buf = appendTwoDigits(buf, om)
	return string(buf)
}

func formatRFC3339Nano(t time.Time) string {
	const layoutFallback = time.RFC3339Nano
	year, month, day := t.Date()
	if year < 0 || year > 9999 {
		return t.Format(layoutFallback)
	}
	hour, min, sec := t.Clock()
	nano := t.Nanosecond()
	_, offset := t.Zone()
	if offset < -(18*3600) || offset > 18*3600 {
		return t.Format(layoutFallback)
	}
	buf := make([]byte, 0, 40)
	buf = appendFourDigits(buf, year)
	buf = append(buf, '-')
	buf = appendTwoDigits(buf, int(month))
	buf = append(buf, '-')
	buf = appendTwoDigits(buf, day)
	buf = append(buf, 'T')
	buf = appendTwoDigits(buf, hour)
	buf = append(buf, ':')
	buf = appendTwoDigits(buf, min)
	buf = append(buf, ':')
	buf = appendTwoDigits(buf, sec)
	if nano != 0 {
		buf = appendFraction(buf, nano)
	}
	if offset == 0 {
		buf = append(buf, 'Z')
		return string(buf)
	}
	if offset < 0 {
		buf = append(buf, '-')
		offset = -offset
	} else {
		buf = append(buf, '+')
	}
	oh := offset / 3600
	om := (offset % 3600) / 60
	buf = appendTwoDigits(buf, oh)
	buf = append(buf, ':')
	buf = appendTwoDigits(buf, om)
	return string(buf)
}

func appendFraction(buf []byte, nano int) []byte {
	buf = append(buf, '.')
	var digits [9]byte
	for i := 8; i >= 0; i-- {
		digits[i] = byte('0' + nano%10)
		nano /= 10
	}
	n := 9
	for n > 0 && digits[n-1] == '0' {
		n--
	}
	buf = append(buf, digits[:n]...)
	return buf
}

func appendFourDigits(buf []byte, v int) []byte {
	buf = appendTwoDigits(buf, v/100)
	buf = appendTwoDigits(buf, v%100)
	return buf
}

func appendTwoDigits(buf []byte, value int) []byte {
	buf = append(buf, byte('0'+value/10))
	buf = append(buf, byte('0'+value%10))
	return buf
}
