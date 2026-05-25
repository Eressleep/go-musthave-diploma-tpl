package luhn

func Valid(s string) bool {
	if len(s) < 2 {
		return false
	}

	var sum int
	alt := false

	for i := len(s) - 1; i >= 0; i-- {
		c := s[i]
		if c < '0' || c > '9' {
			return false
		}

		d := int(c - '0')

		if alt {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}

		sum += d
		alt = !alt
	}

	return sum%10 == 0
}
